package relayconvert

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	claudemessages "github.com/QuantumNous/new-api/service/relayconvert/internal/claude_messages"
	geminichat "github.com/QuantumNous/new-api/service/relayconvert/internal/gemini_chat"
	oaichat "github.com/QuantumNous/new-api/service/relayconvert/internal/oai_chat"
	oairesponses "github.com/QuantumNous/new-api/service/relayconvert/internal/oai_responses"
	sharedgemini "github.com/QuantumNous/new-api/service/relayconvert/internal/shared/gemini"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/gin-gonic/gin"
)

func ClaudeMessagesRequestToOpenAIChat(claudeRequest dto.ClaudeRequest, info *relaycommon.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
	return claudemessages.ClaudeMessagesRequestToOpenAIChat(claudeRequest, info)
}

func OpenAIChatRequestToClaudeMessages(c *gin.Context, textRequest dto.GeneralOpenAIRequest) (*dto.ClaudeRequest, error) {
	return oaichat.OpenAIChatRequestToClaudeMessages(c, textRequest)
}

func GeminiGenerateContentRequestToOpenAIChat(geminiRequest *dto.GeminiChatRequest, info *relaycommon.RelayInfo) (*dto.GeneralOpenAIRequest, error) {
	return geminichat.GeminiGenerateContentRequestToOpenAIChat(geminiRequest, info)
}

func OpenAIChatRequestToGeminiGenerateContent(c *gin.Context, textRequest dto.GeneralOpenAIRequest, info *relaycommon.RelayInfo) (*dto.GeminiChatRequest, error) {
	return oaichat.OpenAIChatRequestToGeminiGenerateContent(c, textRequest, info)
}

func ApplyGeminiThinkingConfig(geminiRequest *dto.GeminiChatRequest, info *relaycommon.RelayInfo, oaiRequest ...dto.GeneralOpenAIRequest) {
	sharedgemini.ApplyThinkingConfig(geminiRequest, info, oaiRequest...)
}

func ChatCompletionsRequestToResponsesRequest(req *dto.GeneralOpenAIRequest) (*dto.OpenAIResponsesRequest, error) {
	return oaichat.ChatCompletionsRequestToResponsesRequest(req)
}

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	return oairesponses.ResponsesRequestToChatCompletionsRequest(req)
}

// ResponsesRequestToChatOptions configures Responses-to-Chat compatibility.
type ResponsesRequestToChatOptions = oairesponses.ResponsesRequestToChatOptions

// ResponsesToolPolicies selects how each Responses tool type is represented to
// a Chat Completions-only upstream.
type ResponsesToolPolicies = oairesponses.ResponsesToolPolicies

type ResponsesToolPolicyResolver = oairesponses.ResponsesToolPolicyResolver

type ResponsesToolPolicyDecision = oairesponses.ResponsesToolPolicyDecision

const (
	ResponsesToolPolicyPreserve = oairesponses.ResponsesToolPolicyPreserve
	ResponsesToolPolicyFlatten  = oairesponses.ResponsesToolPolicyFlatten
	ResponsesToolPolicyDrop     = oairesponses.ResponsesToolPolicyDrop
	ResponsesToolPolicyReject   = oairesponses.ResponsesToolPolicyReject
)

func ResponsesRequestToChatCompletionsRequestWithOptions(req *dto.OpenAIResponsesRequest, options ResponsesRequestToChatOptions) (*dto.GeneralOpenAIRequest, error) {
	return oairesponses.ResponsesRequestToChatCompletionsRequestWithOptions(req, options)
}

func ApplyResponsesToolPoliciesForPassthrough(rawTools json.RawMessage, policies ResponsesToolPolicies) json.RawMessage {
	return oairesponses.ApplyResponsesToolPoliciesForPassthrough(rawTools, policies)
}

func ApplyResponsesToolPolicies(rawTools json.RawMessage, resolver ResponsesToolPolicyResolver) (json.RawMessage, []ResponsesToolPolicyDecision, error) {
	return oairesponses.ApplyResponsesToolPolicies(rawTools, resolver)
}

func ApplyResponsesToolConflictPolicy(rawTools json.RawMessage, policy string) (json.RawMessage, []ResponsesToolPolicyDecision, error) {
	return oairesponses.ApplyResponsesToolConflictPolicy(rawTools, policy)
}

func ApplyResponsesToolConflictPolicyWithImplicitHostedTools(rawTools json.RawMessage, policy string, implicitHostedTools []string) (json.RawMessage, []ResponsesToolPolicyDecision, error) {
	return oairesponses.ApplyResponsesToolConflictPolicyWithImplicitHostedTools(rawTools, policy, implicitHostedTools)
}

func ValidateResponsesToolChoiceAfterPolicy(rawChoice json.RawMessage, decisions []ResponsesToolPolicyDecision) error {
	return oairesponses.ValidateResponsesToolChoiceAfterPolicy(rawChoice, decisions)
}

func DeduplicateConflictingTools(rawTools json.RawMessage) json.RawMessage {
	return oairesponses.DeduplicateConflictingTools(rawTools)
}

func OpenAIResponsesRequestToClaudeMessages(c *gin.Context, req *dto.OpenAIResponsesRequest) (*dto.ClaudeRequest, error) {
	return oairesponses.OpenAIResponsesRequestToClaudeMessages(c, req)
}

func OpenAIResponsesRequestToGeminiChat(c *gin.Context, req *dto.OpenAIResponsesRequest, info *relaycommon.RelayInfo) (*dto.GeminiChatRequest, error) {
	return oairesponses.OpenAIResponsesRequestToGeminiChat(c, req, info)
}

func ShouldChatCompletionsUseResponsesPolicy(policy model_setting.ChatCompletionsToResponsesPolicy, channelID int, channelType int, model string) bool {
	return oaichat.ShouldChatCompletionsUseResponsesPolicy(policy, channelID, channelType, model)
}

func ShouldChatCompletionsUseResponsesGlobal(channelID int, channelType int, model string) bool {
	return oaichat.ShouldChatCompletionsUseResponsesGlobal(channelID, channelType, model)
}
