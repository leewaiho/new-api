package relayconvert

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	chatFinishReasonLength                  = "length"
	chatFinishReasonContentFilter           = "content_filter"
	responsesEventReasoningSummaryPartAdded = "response.reasoning_summary_part.added"
)

func ChatCompletionsResponseToResponsesResponse(resp *dto.OpenAITextResponse, id string) (*dto.OpenAIResponsesResponse, *dto.Usage, error) {
	if resp == nil {
		return nil, nil, errors.New("response is nil")
	}

	usage := UsageFromChatUsage(&resp.Usage)
	out := &dto.OpenAIResponsesResponse{
		ID:        id,
		Object:    "response",
		CreatedAt: chatCreatedAt(resp.Created),
		Status:    []byte(`"completed"`),
		Model:     resp.Model,
		Output:    make([]dto.ResponsesOutput, 0),
		Usage:     usage,
	}

	if len(resp.Choices) == 0 {
		return out, usage, nil
	}

	choice := resp.Choices[0]
	if status, details := ResponsesStatusFromChatFinishReason(choice.FinishReason); status != "" {
		out.Status = []byte(fmt.Sprintf("%q", status))
		out.IncompleteDetails = details
	}

	if text := choice.Message.StringContent(); text != "" {
		out.Output = append(out.Output, dto.ResponsesOutput{
			Type:   responsesOutputTypeMessage,
			ID:     fmt.Sprintf("msg_%s_0", id),
			Status: responseOutputStatus(out),
			Role:   "assistant",
			Content: []dto.ResponsesOutputContent{
				{
					Type:        "output_text",
					Text:        text,
					Annotations: []interface{}{},
				},
			},
		})
	}
	if reasoning := choice.Message.GetReasoningContent(); reasoning != "" {
		out.Output = append(out.Output, dto.ResponsesOutput{
			Type:   responsesOutputTypeReasoning,
			ID:     fmt.Sprintf("%s_reasoning_0", id),
			Status: responseOutputStatus(out),
			Content: []dto.ResponsesOutputContent{
				{
					Type: "summary_text",
					Text: reasoning,
				},
			},
		})
	}

	for i, toolCall := range choice.Message.ParseToolCalls() {
		toolOutput, err := chatToolCallToResponsesOutput(toolCall, id, i, responseOutputStatus(out))
		if err != nil {
			return nil, nil, err
		}
		out.Output = append(out.Output, toolOutput)
	}

	return out, usage, nil
}

func ResponsesStatusFromChatFinishReason(finishReason string) (string, *dto.IncompleteDetails) {
	switch strings.TrimSpace(finishReason) {
	case chatFinishReasonLength:
		return "incomplete", &dto.IncompleteDetails{Reason: responsesIncompleteReasonMaxTokens}
	case chatFinishReasonContentFilter:
		return "incomplete", &dto.IncompleteDetails{Reason: responsesIncompleteReasonContentFilter}
	default:
		return "completed", nil
	}
}

func UsageFromChatUsage(src *dto.Usage) *dto.Usage {
	usage := &dto.Usage{}
	if src == nil {
		return usage
	}
	if src.PromptTokens != 0 {
		usage.PromptTokens = src.PromptTokens
		usage.InputTokens = src.PromptTokens
	}
	if src.CompletionTokens != 0 {
		usage.CompletionTokens = src.CompletionTokens
		usage.OutputTokens = src.CompletionTokens
	}
	if src.TotalTokens != 0 {
		usage.TotalTokens = src.TotalTokens
	} else {
		usage.TotalTokens = usage.InputTokens + usage.OutputTokens
	}
	if src.PromptTokensDetails.CachedTokens != 0 ||
		src.PromptTokensDetails.ImageTokens != 0 ||
		src.PromptTokensDetails.AudioTokens != 0 ||
		src.PromptTokensDetails.CachedCreationTokens != 0 ||
		src.PromptTokensDetails.TextTokens != 0 {
		details := src.PromptTokensDetails
		usage.InputTokensDetails = &details
	}
	if src.CompletionTokenDetails.ReasoningTokens != 0 ||
		src.CompletionTokenDetails.TextTokens != 0 ||
		src.CompletionTokenDetails.AudioTokens != 0 ||
		src.CompletionTokenDetails.ImageTokens != 0 {
		usage.CompletionTokenDetails = src.CompletionTokenDetails
	}
	return usage
}

type ChatToResponsesStreamEvent struct {
	Type    string
	Payload dto.ResponsesStreamResponse
}

type ChatToResponsesStreamState struct {
	ID      string
	Model   string
	Created int64
	Usage   *dto.Usage

	status            string
	incompleteDetails *dto.IncompleteDetails
	sentCreated       bool
	textOutputIndex   int
	textStarted       bool
	textDone          bool
	reasoningIndex    int
	reasoningStarted  bool
	reasoningDone     bool
	finalized         bool
	nextOutputIndex   int
	toolsByIndex      map[int]*chatToResponsesStreamTool
	outputOrder       []chatToResponsesOutputRef
	text              strings.Builder
	reasoning         strings.Builder
	toolNameMappings  map[string]dto.ResponsesToolNameMapping
}

type chatToResponsesStreamTool struct {
	ChatIndex      int
	OutputIndex    int
	ID             string
	CallID         string
	Name           string
	Arguments      strings.Builder
	CustomInput    string
	CustomInputSet bool
	Added          bool
	Done           bool
}

type chatToResponsesOutputRef struct {
	Kind      string
	ToolIndex int
}

func NewChatToResponsesStreamState(id string, model string) *ChatToResponsesStreamState {
	return &ChatToResponsesStreamState{
		ID:              id,
		Model:           model,
		Created:         time.Now().Unix(),
		Usage:           &dto.Usage{},
		status:          "completed",
		textOutputIndex: -1,
		reasoningIndex:  -1,
		toolsByIndex:    make(map[int]*chatToResponsesStreamTool),
	}
}

func (s *ChatToResponsesStreamState) SetToolNameMappings(mappings map[string]dto.ResponsesToolNameMapping) {
	s.toolNameMappings = mappings
}

func ChatCompletionsStreamChunkToResponsesEvents(chunk *dto.ChatCompletionsStreamResponse, state *ChatToResponsesStreamState) ([]ChatToResponsesStreamEvent, error) {
	if chunk == nil || state == nil {
		return nil, nil
	}
	if state.ID == "" {
		state.ID = chunk.Id
	}
	if state.Model == "" {
		state.Model = chunk.Model
	}
	if state.Created == 0 {
		state.Created = chunk.Created
	}
	if chunk.Usage != nil {
		state.Usage = UsageFromChatUsage(chunk.Usage)
	}

	events := make([]ChatToResponsesStreamEvent, 0)
	if !state.sentCreated {
		state.sentCreated = true
		events = append(events, responsesStreamEvent(responsesEventCreated, dto.ResponsesStreamResponse{
			Type:     responsesEventCreated,
			Response: state.createdResponse(),
		}))
	}
	for _, choice := range chunk.Choices {
		if choice.Delta.GetReasoningContent() != "" {
			events = append(events, state.appendReasoningDelta(choice.Delta.GetReasoningContent())...)
		}
		if choice.Delta.GetContentString() != "" {
			events = append(events, state.appendTextDelta(choice.Delta.GetContentString())...)
		}
		for _, toolCall := range choice.Delta.ToolCalls {
			toolEvents, err := state.appendToolCallDelta(toolCall)
			if err != nil {
				return nil, err
			}
			events = append(events, toolEvents...)
		}
		if choice.FinishReason != nil && strings.TrimSpace(*choice.FinishReason) != "" {
			state.applyFinishReason(*choice.FinishReason)
			doneEvents, err := state.doneDeltaEvents()
			if err != nil {
				return nil, err
			}
			events = append(events, doneEvents...)
		}
	}
	return events, nil
}

func FinalizeChatCompletionsStreamToResponses(state *ChatToResponsesStreamState) ([]ChatToResponsesStreamEvent, error) {
	if state == nil || state.finalized {
		return nil, nil
	}
	events, err := state.doneDeltaEvents()
	if err != nil {
		return nil, err
	}
	state.finalized = true
	resp := state.finalResponse()
	eventType := responsesEventCompleted
	if state.status == "incomplete" {
		eventType = responsesEventIncomplete
	}
	events = append(events, responsesStreamEvent(eventType, dto.ResponsesStreamResponse{
		Type:     eventType,
		Response: resp,
	}))
	return events, nil
}

func (s *ChatToResponsesStreamState) UsageText() string {
	if s == nil {
		return ""
	}
	return s.text.String()
}

func (s *ChatToResponsesStreamState) appendTextDelta(delta string) []ChatToResponsesStreamEvent {
	events := make([]ChatToResponsesStreamEvent, 0, 2)
	if !s.textStarted {
		s.textStarted = true
		s.textOutputIndex = s.nextIndex("message", -1)
		events = append(events, responsesStreamEvent(responsesEventOutputItemAdded, dto.ResponsesStreamResponse{
			Type:        responsesEventOutputItemAdded,
			OutputIndex: intPtr(s.textOutputIndex),
			Item: &dto.ResponsesOutput{
				Type:    responsesOutputTypeMessage,
				ID:      s.messageID(),
				Status:  "in_progress",
				Role:    "assistant",
				Content: []dto.ResponsesOutputContent{},
			},
		}))
	}
	s.text.WriteString(delta)
	events = append(events, responsesStreamEvent(responsesEventOutputTextDelta, dto.ResponsesStreamResponse{
		Type:         responsesEventOutputTextDelta,
		OutputIndex:  intPtr(s.textOutputIndex),
		ContentIndex: intPtr(0),
		Delta:        delta,
		ItemID:       s.messageID(),
	}))
	return events
}

func (s *ChatToResponsesStreamState) appendReasoningDelta(delta string) []ChatToResponsesStreamEvent {
	events := make([]ChatToResponsesStreamEvent, 0, 3)
	if !s.reasoningStarted {
		s.reasoningStarted = true
		s.reasoningIndex = s.nextIndex("reasoning", -1)
		events = append(events, responsesStreamEvent(responsesEventOutputItemAdded, dto.ResponsesStreamResponse{
			Type:        responsesEventOutputItemAdded,
			OutputIndex: intPtr(s.reasoningIndex),
			Item: &dto.ResponsesOutput{
				Type:    responsesOutputTypeReasoning,
				ID:      s.reasoningID(),
				Status:  "in_progress",
				Content: []dto.ResponsesOutputContent{},
			},
		}))
		events = append(events, responsesStreamEvent(responsesEventReasoningSummaryPartAdded, dto.ResponsesStreamResponse{
			Type:         responsesEventReasoningSummaryPartAdded,
			OutputIndex:  intPtr(s.reasoningIndex),
			SummaryIndex: intPtr(0),
			ItemID:       s.reasoningID(),
			Part: &dto.ResponsesReasoningSummaryPart{
				Type: "summary_text",
				Text: "",
			},
		}))
	}
	s.reasoning.WriteString(delta)
	events = append(events, responsesStreamEvent(responsesEventReasoningSummaryDelta, dto.ResponsesStreamResponse{
		Type:         responsesEventReasoningSummaryDelta,
		OutputIndex:  intPtr(s.reasoningIndex),
		SummaryIndex: intPtr(0),
		Delta:        delta,
		ItemID:       s.reasoningID(),
	}))
	return events
}

func (s *ChatToResponsesStreamState) appendToolCallDelta(toolCall dto.ToolCallResponse) ([]ChatToResponsesStreamEvent, error) {
	chatIndex := 0
	if toolCall.Index != nil {
		chatIndex = *toolCall.Index
	}
	tool := s.toolsByIndex[chatIndex]
	events := make([]ChatToResponsesStreamEvent, 0, 2)
	if tool == nil {
		tool = &chatToResponsesStreamTool{
			ChatIndex:   chatIndex,
			OutputIndex: s.nextIndex("tool", chatIndex),
			CallID:      strings.TrimSpace(toolCall.ID),
			Name:        strings.TrimSpace(toolCall.Function.Name),
		}
		s.toolsByIndex[chatIndex] = tool
	}
	if strings.TrimSpace(toolCall.ID) != "" {
		tool.CallID = strings.TrimSpace(toolCall.ID)
	}
	if strings.TrimSpace(toolCall.Function.Name) != "" {
		tool.Name = strings.TrimSpace(toolCall.Function.Name)
	}
	if toolCall.Function.Arguments != "" {
		tool.Arguments.WriteString(toolCall.Function.Arguments)
	}
	if !tool.Added && tool.Name != "" && tool.CallID != "" {
		tool.ID = s.toolItemID(tool.Name, tool.CallID, fmt.Sprintf("%s_call_%d", s.ID, chatIndex))
		tool.Added = true
		events = append(events, responsesStreamEvent(responsesEventOutputItemAdded, dto.ResponsesStreamResponse{
			Type:        responsesEventOutputItemAdded,
			OutputIndex: intPtr(tool.OutputIndex),
			ItemID:      tool.ID,
			Item:        s.toolOutput(tool, "in_progress"),
		}))
		if tool.Arguments.Len() > 0 && !s.isCustomTool(tool.Name) {
			events = append(events, responsesStreamEvent(responsesEventFunctionArgsDelta, dto.ResponsesStreamResponse{
				Type:        responsesEventFunctionArgsDelta,
				OutputIndex: intPtr(tool.OutputIndex),
				ItemID:      tool.ID,
				Delta:       tool.Arguments.String(),
			}))
		}
	} else if tool.Added && toolCall.Function.Arguments != "" && !s.isCustomTool(tool.Name) {
		events = append(events, responsesStreamEvent(responsesEventFunctionArgsDelta, dto.ResponsesStreamResponse{
			Type:        responsesEventFunctionArgsDelta,
			OutputIndex: intPtr(tool.OutputIndex),
			ItemID:      tool.ID,
			Delta:       toolCall.Function.Arguments,
		}))
	}
	return events, nil
}

func (s *ChatToResponsesStreamState) doneDeltaEvents() ([]ChatToResponsesStreamEvent, error) {
	events := make([]ChatToResponsesStreamEvent, 0)
	status := s.outputStatus()
	if s.textStarted && !s.textDone {
		s.textDone = true
		events = append(events, responsesStreamEvent("response.output_text.done", dto.ResponsesStreamResponse{
			Type:         "response.output_text.done",
			OutputIndex:  intPtr(s.textOutputIndex),
			ContentIndex: intPtr(0),
			ItemID:       s.messageID(),
		}))
		events = append(events, responsesStreamEvent(responsesEventOutputItemDone, dto.ResponsesStreamResponse{
			Type:        responsesEventOutputItemDone,
			OutputIndex: intPtr(s.textOutputIndex),
			Item:        s.messageOutput(status),
		}))
	}
	if s.reasoningStarted && !s.reasoningDone {
		s.reasoningDone = true
		events = append(events, responsesStreamEvent(responsesEventReasoningSummaryDone, dto.ResponsesStreamResponse{
			Type:         responsesEventReasoningSummaryDone,
			OutputIndex:  intPtr(s.reasoningIndex),
			SummaryIndex: intPtr(0),
			ItemID:       s.reasoningID(),
			Part: &dto.ResponsesReasoningSummaryPart{
				Type: "summary_text",
				Text: s.reasoning.String(),
			},
		}))
		events = append(events, responsesStreamEvent(responsesEventOutputItemDone, dto.ResponsesStreamResponse{
			Type:        responsesEventOutputItemDone,
			OutputIndex: intPtr(s.reasoningIndex),
			Item:        s.reasoningOutput(status),
		}))
	}
	for _, tool := range s.sortedTools() {
		if tool.Done || !tool.Added {
			continue
		}
		tool.Done = true
		if s.isCustomTool(tool.Name) {
			if err := s.prepareCustomToolInput(tool); err != nil {
				return nil, err
			}
			events = append(events, responsesStreamEvent(responsesEventCustomToolInputDelta, dto.ResponsesStreamResponse{
				Type:        responsesEventCustomToolInputDelta,
				OutputIndex: intPtr(tool.OutputIndex),
				ItemID:      tool.ID,
				Delta:       tool.CustomInput,
			}))
			events = append(events, responsesStreamEvent(responsesEventCustomToolInputDone, dto.ResponsesStreamResponse{
				Type:        responsesEventCustomToolInputDone,
				OutputIndex: intPtr(tool.OutputIndex),
				ItemID:      tool.ID,
			}))
		} else {
			events = append(events, responsesStreamEvent(responsesEventFunctionArgsDone, dto.ResponsesStreamResponse{
				Type:        responsesEventFunctionArgsDone,
				OutputIndex: intPtr(tool.OutputIndex),
				ItemID:      tool.ID,
			}))
		}
		events = append(events, responsesStreamEvent(responsesEventOutputItemDone, dto.ResponsesStreamResponse{
			Type:        responsesEventOutputItemDone,
			OutputIndex: intPtr(tool.OutputIndex),
			Item:        s.toolOutput(tool, status),
		}))
	}
	return events, nil
}

func (s *ChatToResponsesStreamState) applyFinishReason(finishReason string) {
	if status, details := ResponsesStatusFromChatFinishReason(finishReason); status != "" {
		s.status = status
		s.incompleteDetails = details
	}
}

func (s *ChatToResponsesStreamState) finalResponse() *dto.OpenAIResponsesResponse {
	output := make([]dto.ResponsesOutput, 0, len(s.outputOrder))
	status := s.outputStatus()
	for _, ref := range s.outputOrder {
		switch ref.Kind {
		case "message":
			output = append(output, *s.messageOutput(status))
		case "reasoning":
			output = append(output, *s.reasoningOutput(status))
		case "tool":
			if tool := s.toolsByIndex[ref.ToolIndex]; tool != nil && tool.Added {
				output = append(output, *s.toolOutput(tool, status))
			}
		}
	}
	return &dto.OpenAIResponsesResponse{
		ID:                s.ID,
		Object:            "response",
		CreatedAt:         int(s.Created),
		Status:            []byte(fmt.Sprintf("%q", s.status)),
		IncompleteDetails: s.incompleteDetails,
		Model:             s.Model,
		Output:            output,
		Usage:             s.Usage,
	}
}

func (s *ChatToResponsesStreamState) createdResponse() *dto.OpenAIResponsesResponse {
	return &dto.OpenAIResponsesResponse{
		ID:        s.ID,
		Object:    "response",
		CreatedAt: int(s.Created),
		Status:    []byte(`"in_progress"`),
		Model:     s.Model,
		Output:    []dto.ResponsesOutput{},
	}
}

func (s *ChatToResponsesStreamState) nextIndex(kind string, toolIndex int) int {
	index := s.nextOutputIndex
	s.nextOutputIndex++
	s.outputOrder = append(s.outputOrder, chatToResponsesOutputRef{Kind: kind, ToolIndex: toolIndex})
	return index
}

func (s *ChatToResponsesStreamState) sortedTools() []*chatToResponsesStreamTool {
	indexes := make([]int, 0, len(s.toolsByIndex))
	for index := range s.toolsByIndex {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	tools := make([]*chatToResponsesStreamTool, 0, len(indexes))
	for _, index := range indexes {
		tools = append(tools, s.toolsByIndex[index])
	}
	return tools
}

func (s *ChatToResponsesStreamState) outputStatus() string {
	if s.status == "incomplete" {
		return "incomplete"
	}
	return "completed"
}

func (s *ChatToResponsesStreamState) messageID() string {
	return fmt.Sprintf("msg_%s_0", s.ID)
}

func (s *ChatToResponsesStreamState) reasoningID() string {
	return fmt.Sprintf("%s_reasoning_0", s.ID)
}

func (s *ChatToResponsesStreamState) messageOutput(status string) *dto.ResponsesOutput {
	return &dto.ResponsesOutput{
		Type:   responsesOutputTypeMessage,
		ID:     s.messageID(),
		Status: status,
		Role:   "assistant",
		Content: []dto.ResponsesOutputContent{
			{
				Type:        "output_text",
				Text:        s.text.String(),
				Annotations: []interface{}{},
			},
		},
	}
}

func (s *ChatToResponsesStreamState) reasoningOutput(status string) *dto.ResponsesOutput {
	return &dto.ResponsesOutput{
		Type:   responsesOutputTypeReasoning,
		ID:     s.reasoningID(),
		Status: status,
		Content: []dto.ResponsesOutputContent{
			{
				Type: "summary_text",
				Text: s.reasoning.String(),
			},
		},
	}
}

func (s *ChatToResponsesStreamState) toolOutput(tool *chatToResponsesStreamTool, status string) *dto.ResponsesOutput {
	output := &dto.ResponsesOutput{
		Type:      responsesOutputTypeFunctionCall,
		ID:        tool.ID,
		Status:    status,
		CallId:    tool.CallID,
		Name:      tool.Name,
		Arguments: chatArgumentsRawMessage(tool.Arguments.String()),
	}
	if mapping, ok := s.toolNameMappings[tool.Name]; ok {
		switch mapping.NativeToolType {
		case responsesNativeToolTypeCustom:
			output.Type = responsesOutputTypeCustomToolCall
			output.Name = mapping.Name
			output.Arguments = nil
			if tool.CustomInputSet {
				output.Input, _ = common.Marshal(tool.CustomInput)
			}
		case responsesNativeToolTypeShellCommand:
			output.Name = mapping.Name
		}
		output.Namespace = mapping.Namespace
		if mapping.NativeToolType == "" {
			output.Name = mapping.Name
		}
	}
	return output
}

func (s *ChatToResponsesStreamState) toolMapping(name string) (dto.ResponsesToolNameMapping, bool) {
	if s == nil {
		return dto.ResponsesToolNameMapping{}, false
	}
	mapping, ok := s.toolNameMappings[name]
	return mapping, ok
}

func (s *ChatToResponsesStreamState) isCustomTool(name string) bool {
	mapping, ok := s.toolMapping(name)
	return ok &&
		mapping.NativeToolType == responsesNativeToolTypeCustom &&
		mapping.ArgumentsCodec == responsesArgumentsCodecCustomInput
}

func (s *ChatToResponsesStreamState) toolItemID(name string, callID string, fallback string) string {
	mapping, ok := s.toolMapping(name)
	if ok {
		switch mapping.NativeToolType {
		case responsesNativeToolTypeCustom:
			return responsesToolItemID("ctc", callID, fallback)
		case responsesNativeToolTypeShellCommand:
			return responsesToolItemID("fc", callID, fallback)
		}
	}
	if strings.TrimSpace(callID) != "" {
		return strings.TrimSpace(callID)
	}
	return strings.TrimSpace(fallback)
}

func (s *ChatToResponsesStreamState) prepareCustomToolInput(tool *chatToResponsesStreamTool) error {
	if tool == nil || !s.isCustomTool(tool.Name) || tool.CustomInputSet {
		return nil
	}
	input, err := customToolInputFromChatArguments(chatArgumentsRawMessage(tool.Arguments.String()))
	if err != nil {
		return fmt.Errorf("invalid %s stream arguments for %q: %w", responsesNativeToolTypeCustom, tool.Name, err)
	}
	tool.CustomInput = input
	tool.CustomInputSet = true
	return nil
}

func responseOutputStatus(resp *dto.OpenAIResponsesResponse) string {
	if resp == nil || responseStatusString(resp) != "incomplete" {
		return "completed"
	}
	return "incomplete"
}

func chatToolCallToResponsesOutput(toolCall dto.ToolCallRequest, responseID string, index int, status string) (dto.ResponsesOutput, error) {
	callID := strings.TrimSpace(toolCall.ID)
	if callID == "" {
		callID = fmt.Sprintf("%s_call_%d", responseID, index)
	}
	if toolCall.Type == "" || toolCall.Type == "function" {
		return dto.ResponsesOutput{
			Type:      responsesOutputTypeFunctionCall,
			ID:        callID,
			Status:    status,
			CallId:    callID,
			Name:      toolCall.Function.Name,
			Arguments: chatArgumentsRawMessage(toolCall.Function.Arguments),
		}, nil
	}
	return dto.ResponsesOutput{
		Type:      toolCall.Type,
		ID:        callID,
		Status:    status,
		CallId:    callID,
		Arguments: toolCall.Custom,
	}, nil
}

// ApplyResponsesToolNameMappings restores Responses namespace tool identity for
// function calls that were flattened before being sent to a Chat Completions upstream.
func ApplyResponsesToolNameMappings(resp *dto.OpenAIResponsesResponse, mappings map[string]dto.ResponsesToolNameMapping) error {
	if resp == nil || len(mappings) == 0 {
		return nil
	}
	for i := range resp.Output {
		output := &resp.Output[i]
		if output.Type != responsesOutputTypeFunctionCall {
			continue
		}
		mapping, ok := mappings[output.Name]
		if !ok {
			continue
		}
		switch mapping.NativeToolType {
		case responsesNativeToolTypeCustom:
			if mapping.ArgumentsCodec != responsesArgumentsCodecCustomInput {
				return fmt.Errorf("unsupported custom arguments codec %q for %q", mapping.ArgumentsCodec, output.Name)
			}
			input, err := customToolInputFromChatArguments(output.Arguments)
			if err != nil {
				return fmt.Errorf("invalid %s arguments for %q: %w", responsesNativeToolTypeCustom, output.Name, err)
			}
			output.Type = responsesOutputTypeCustomToolCall
			output.ID = responsesToolItemID("ctc", output.CallId, output.ID)
			output.Name = mapping.Name
			output.Namespace = ""
			output.Arguments = nil
			output.Input, err = common.Marshal(input)
			if err != nil {
				return err
			}
		case responsesNativeToolTypeShellCommand:
			output.ID = responsesToolItemID("fc", output.CallId, output.ID)
			output.Name = mapping.Name
			output.Namespace = ""
		default:
			output.Namespace = mapping.Namespace
			output.Name = mapping.Name
		}
	}
	return nil
}

func customToolInputFromChatArguments(arguments json.RawMessage) (string, error) {
	var rawArguments string
	if err := common.Unmarshal(arguments, &rawArguments); err == nil {
		arguments = []byte(rawArguments)
	}
	var payload map[string]any
	if err := common.Unmarshal(arguments, &payload); err != nil {
		return "", err
	}
	input, ok := payload["input"].(string)
	if !ok {
		return "", errors.New("input must be a string")
	}
	return input, nil
}

func responsesToolItemID(prefix string, callID string, fallback string) string {
	value := strings.TrimSpace(callID)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	if value == "" {
		value = "0"
	}
	return prefix + "_" + value
}

func chatArgumentsRawMessage(arguments string) []byte {
	raw, err := common.Marshal(arguments)
	if err != nil {
		return []byte(`""`)
	}
	return raw
}

func chatCreatedAt(created any) int {
	switch v := created.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		if parsed := common.String2Int(v); parsed != 0 {
			return parsed
		}
	}
	return int(time.Now().Unix())
}

func responsesStreamEvent(eventType string, payload dto.ResponsesStreamResponse) ChatToResponsesStreamEvent {
	payload.Type = eventType
	return ChatToResponsesStreamEvent{
		Type:    eventType,
		Payload: payload,
	}
}

func intPtr(v int) *int {
	return &v
}
