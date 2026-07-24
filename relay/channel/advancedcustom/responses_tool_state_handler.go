package advancedcustom

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func advancedCustomResponsesToolStateScopeFor(info *relaycommon.RelayInfo, route dto.AdvancedCustomRoute, requestedModel string, upstreamModel string) advancedCustomResponsesToolStateScope {
	scope := advancedCustomResponsesToolStateScope{
		Route:          strings.Join([]string{route.IncomingPath, route.UpstreamPath, route.Converter}, "\x1f"),
		RequestedModel: requestedModel,
		UpstreamModel:  upstreamModel,
	}
	if info != nil {
		scope.TokenID = info.TokenId
	}
	return scope
}

func (a *Adaptor) cacheResponsesToolState(responseID string, mappings map[string]dto.ResponsesToolNameMapping, response *dto.OpenAIResponsesResponse) error {
	if a == nil || a.toolStateReplay == nil || !a.toolStateReplay.Enabled || response == nil {
		return nil
	}
	output := make([]dto.ResponsesOutput, 0, len(response.Output))
	for _, item := range response.Output {
		switch item.Type {
		case "function_call", "custom_tool_call":
			output = append(output, item)
		}
	}
	if len(output) == 0 {
		return nil
	}
	return defaultAdvancedCustomResponsesToolStateCache.Save(
		a.toolStateScope,
		responseID,
		advancedCustomResponsesToolState{Output: output, ToolNameMappings: mappings},
		time.Duration(a.toolStateReplay.TTLSeconds)*time.Second,
	)
}

func (a *Adaptor) chatToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(body, &chatResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := chatResp.GetOpenAIError(); oaiError != nil &&
		(oaiError.Type != "" ||
			(resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices) && oaiError.Message != "") {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	responseID := helper.GetResponseID(c)
	responsesResp, usage, err := service.ChatCompletionsResponseToResponsesResponse(&chatResp, responseID)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if err := relayconvert.ApplyResponsesToolNameMappings(responsesResp, info.ResponsesToolNameMappings); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if usage == nil || usage.TotalTokens == 0 {
		text := service.ExtractOutputTextFromResponses(responsesResp)
		usage = service.ResponseText2Usage(c, text, info.UpstreamModelName, info.GetEstimatePromptTokens())
		responsesResp.Usage = relayconvert.UsageFromChatUsage(usage)
	}
	if err := a.cacheResponsesToolState(responseID, info.ResponsesToolNameMappings, responsesResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	responseBody, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

func (a *Adaptor) chatToResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	responseID := helper.GetResponseID(c)
	state := relayconvert.NewChatToResponsesStreamState(responseID, info.UpstreamModelName)
	state.SetToolNameMappings(info.ResponsesToolNameMappings)
	streamErr := (*types.NewAPIError)(nil)

	sendEvent := func(event relayconvert.ChatToResponsesStreamEvent) bool {
		data, err := common.Marshal(event.Payload)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
			return false
		}
		helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: event.Type}, string(data))
		return true
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}
		var errorResp dto.OpenAITextResponse
		if err := common.UnmarshalJsonStr(data, &errorResp); err == nil {
			if oaiError := errorResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
				streamErr = types.WithOpenAIError(*oaiError, resp.StatusCode)
				sr.Stop(streamErr)
				return
			}
		}
		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			logger.LogError(c, "failed to unmarshal chat stream response: "+err.Error())
			sr.Error(err)
			return
		}
		events, err := relayconvert.ChatCompletionsStreamChunkToResponsesEvents(&chunk, state)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			sr.Stop(streamErr)
			return
		}
		for _, event := range events {
			if !sendEvent(event) {
				sr.Stop(streamErr)
				return
			}
		}
	})
	if streamErr != nil {
		return nil, streamErr
	}
	usage := state.Usage
	if usage == nil || usage.TotalTokens == 0 {
		usage = service.ResponseText2Usage(c, state.UsageText(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		state.Usage = relayconvert.UsageFromChatUsage(usage)
	}
	finalEvents, err := relayconvert.FinalizeChatCompletionsStreamToResponses(state)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	for _, event := range finalEvents {
		if event.Payload.Response != nil {
			if err := a.cacheResponsesToolState(responseID, info.ResponsesToolNameMappings, event.Payload.Response); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
		if !sendEvent(event) {
			return nil, streamErr
		}
	}
	return usage, nil
}
