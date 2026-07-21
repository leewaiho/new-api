package relayconvert

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	responsesInputTypeFunctionCall         = "function_call"
	responsesInputTypeFunctionCallOutput   = "function_call_output"
	responsesInputTypeCustomToolCall       = "custom_tool_call"
	responsesInputTypeCustomToolCallOutput = "custom_tool_call_output"
	responsesInputTypeToolSearchCall       = "tool_search_call"
	responsesInputTypeToolSearchOutput     = "tool_search_output"
)

const (
	ResponsesToolPolicyPreserve = "preserve"
	ResponsesToolPolicyFlatten  = "flatten"
	ResponsesToolPolicyDrop     = "drop"
	ResponsesToolPolicyReject   = "reject"
)

const (
	responsesNativeToolTypeCustom       = "custom"
	responsesNativeToolTypeShellCommand = "shell_command"
	responsesNativeToolTypeToolSearch   = "tool_search"
	responsesArgumentsCodecCustomInput  = "custom_input"
)

// ResponsesRequestToChatOptions controls lossy compatibility behavior needed
// when a Responses request must be sent to a Chat Completions-only upstream.
type ResponsesRequestToChatOptions struct {
	// Deprecated compatibility flags. Use ToolPolicies for per-tool behavior.
	FlattenNamespaceTools bool
	DropUnsupportedTools  bool
	ToolPolicies          ResponsesToolPolicies
	ToolPolicyResolver    ResponsesToolPolicyResolver
	ToolNameMappings      map[string]dto.ResponsesToolNameMapping
	// DropResponseFields lists Responses request field names that must be
	// removed when sending the converted Chat Completions request. This is
	// used for chat-only upstreams that reject Responses-shaped parameters
	// (for example "metadata").
	DropResponseFields map[string]struct{}
}

// ShouldDropResponseField reports whether the given Responses request field
// should be stripped from the converted Chat Completions request.
func (o ResponsesRequestToChatOptions) ShouldDropResponseField(field string) bool {
	if len(o.DropResponseFields) == 0 {
		return false
	}
	_, ok := o.DropResponseFields[strings.ToLower(strings.TrimSpace(field))]
	return ok
}

type ResponsesToolPolicies struct {
	Namespace       string
	Custom          string
	WebSearch       string
	ToolSearch      string
	ImageGeneration string
	Unknown         string
}

type ResponsesToolPolicyResolver func(toolType string, toolName string) string

type ResponsesToolPolicyDecision struct {
	ToolIndex int
	ToolType  string
	ToolName  string
	Policy    string
}

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	return ResponsesRequestToChatCompletionsRequestWithOptions(req, ResponsesRequestToChatOptions{})
}

func ResponsesRequestToChatCompletionsRequestWithOptions(req *dto.OpenAIResponsesRequest, options ResponsesRequestToChatOptions) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if req.Model == "" {
		return nil, errors.New("model is required")
	}
	if err := validateResponsesRequestChatUnsupportedFields(req); err != nil {
		return nil, err
	}

	tools, err := responsesRequestToolsToChat(req.Tools, options)
	if err != nil {
		return nil, err
	}
	loadedTools, err := responsesToolSearchOutputTools(req.Input)
	if err != nil {
		return nil, err
	}
	if len(loadedTools) > 0 {
		seenChatFunctionNames := chatFunctionToolNames(tools)
		for _, loadedRawTool := range loadedTools {
			raw, err := common.Marshal([]map[string]any{loadedRawTool})
			if err != nil {
				return nil, err
			}
			mappingsBeforeTool := cloneResponsesToolNameMappings(options.ToolNameMappings)
			loadedChatTools, err := responsesRequestToolsToChat(raw, options)
			if err != nil {
				return nil, err
			}
			loadedMappings := cloneResponsesToolNameMappings(options.ToolNameMappings)
			restoreResponsesToolNameMappings(options.ToolNameMappings, mappingsBeforeTool)
			for _, loadedTool := range loadedChatTools {
				if loadedTool.Type == "function" {
					name := strings.TrimSpace(loadedTool.Function.Name)
					if name != "" {
						if _, exists := seenChatFunctionNames[name]; exists {
							continue
						}
						seenChatFunctionNames[name] = struct{}{}
						if mapping, ok := loadedMappings[name]; ok && options.ToolNameMappings != nil {
							options.ToolNameMappings[name] = mapping
						}
					}
				}
				tools = append(tools, loadedTool)
			}
		}
	}

	messages, err := responsesRequestMessagesToChat(req, options.ToolNameMappings)
	if err != nil {
		return nil, err
	}

	toolChoice, err := responsesRequestToolChoiceToChat(req.ToolChoice, options, tools)
	if err != nil {
		return nil, err
	}

	responseFormat, err := responsesRequestTextToChatResponseFormat(req.Text)
	if err != nil {
		return nil, err
	}

	out := &dto.GeneralOpenAIRequest{
		Model:               req.Model,
		Messages:            messages,
		Stream:              req.Stream,
		MaxCompletionTokens: req.MaxOutputTokens,
		Temperature:         req.Temperature,
		TopP:                req.TopP,
		ResponseFormat:      responseFormat,
		Tools:               tools,
		ToolChoice:          toolChoice,
		User:                req.User,
		EnableThinking:      req.EnableThinking,
	}
	if !options.ShouldDropResponseField("stream_options") && req.StreamOptions != nil {
		out.StreamOptions = req.StreamOptions
	}
	if !options.ShouldDropResponseField("top_logprobs") {
		out.TopLogProbs = req.TopLogProbs
	}
	if !options.ShouldDropResponseField("store") {
		out.Store = req.Store
	}
	if !options.ShouldDropResponseField("metadata") {
		out.Metadata = req.Metadata
	}
	if !options.ShouldDropResponseField("safety_identifier") {
		out.SafetyIdentifier = req.SafetyIdentifier
	}
	if !options.ShouldDropResponseField("prompt_cache_retention") {
		out.PromptCacheRetention = req.PromptCacheRetention
	}
	if !options.ShouldDropResponseField("service_tier") {
		if req.ServiceTier != "" {
			out.ServiceTier, _ = common.Marshal(req.ServiceTier)
		}
	}
	if !options.ShouldDropResponseField("prompt_cache_key") && len(req.PromptCacheKey) > 0 && common.GetJsonType(req.PromptCacheKey) == "string" {
		var promptCacheKey string
		if err := common.Unmarshal(req.PromptCacheKey, &promptCacheKey); err == nil {
			out.PromptCacheKey = promptCacheKey
		}
	}
	if !options.ShouldDropResponseField("parallel_tool_calls") && len(req.ParallelToolCalls) > 0 && common.GetJsonType(req.ParallelToolCalls) == "boolean" {
		var parallelToolCalls bool
		if err := common.Unmarshal(req.ParallelToolCalls, &parallelToolCalls); err == nil {
			out.ParallelTooCalls = &parallelToolCalls
		}
	}
	if !options.ShouldDropResponseField("reasoning") && req.Reasoning != nil {
		out.ReasoningEffort = req.Reasoning.Effort
	}

	return out, nil
}

func cloneResponsesToolNameMappings(mappings map[string]dto.ResponsesToolNameMapping) map[string]dto.ResponsesToolNameMapping {
	if mappings == nil {
		return nil
	}
	clone := make(map[string]dto.ResponsesToolNameMapping, len(mappings))
	for name, mapping := range mappings {
		clone[name] = mapping
	}
	return clone
}

func restoreResponsesToolNameMappings(target map[string]dto.ResponsesToolNameMapping, source map[string]dto.ResponsesToolNameMapping) {
	if target == nil {
		return
	}
	for name := range target {
		delete(target, name)
	}
	for name, mapping := range source {
		target[name] = mapping
	}
}

func chatFunctionToolNames(tools []dto.ToolCallRequest) map[string]struct{} {
	names := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		if tool.Type == "function" {
			if name := strings.TrimSpace(tool.Function.Name); name != "" {
				names[name] = struct{}{}
			}
		}
	}
	return names
}

func validateResponsesRequestChatUnsupportedFields(req *dto.OpenAIResponsesRequest) error {
	unsupported := make([]string, 0, 4)
	if rawJSONPresent(req.Conversation) {
		unsupported = append(unsupported, "conversation")
	}
	if strings.TrimSpace(req.PreviousResponseID) != "" {
		unsupported = append(unsupported, "previous_response_id")
	}
	if rawJSONPresent(req.Prompt) {
		unsupported = append(unsupported, "prompt")
	}
	if rawJSONPresent(req.ContextManagement) {
		unsupported = append(unsupported, "context_management")
	}
	if len(unsupported) > 0 {
		return fmt.Errorf("responses to chat conversion does not support stateful fields: %s", strings.Join(unsupported, ", "))
	}
	return nil
}

func responsesRequestMessagesToChat(req *dto.OpenAIResponsesRequest, mappings map[string]dto.ResponsesToolNameMapping) ([]dto.Message, error) {
	messages := make([]dto.Message, 0)
	if rawJSONPresent(req.Instructions) {
		instructions, err := responsesJSONString(req.Instructions)
		if err != nil {
			return nil, fmt.Errorf("invalid instructions: %w", err)
		}
		if strings.TrimSpace(instructions) != "" {
			messages = append(messages, dto.Message{Role: "system", Content: instructions})
		}
	}

	if !rawJSONPresent(req.Input) {
		return messages, nil
	}

	switch common.GetJsonType(req.Input) {
	case "string":
		input, err := responsesJSONString(req.Input)
		if err != nil {
			return nil, fmt.Errorf("invalid input string: %w", err)
		}
		messages = append(messages, dto.Message{Role: "user", Content: input})
		return messages, nil
	case "array":
		var items []map[string]any
		if err := common.Unmarshal(req.Input, &items); err != nil {
			return nil, fmt.Errorf("invalid input array: %w", err)
		}
		for _, item := range items {
			nextMessages, err := responsesInputItemToChatMessages(item, messages, mappings)
			if err != nil {
				return nil, err
			}
			messages = nextMessages
		}
		return messages, nil
	default:
		return nil, fmt.Errorf("unsupported responses input type %q", common.GetJsonType(req.Input))
	}
}

func responsesInputItemToChatMessages(item map[string]any, messages []dto.Message, mappings map[string]dto.ResponsesToolNameMapping) ([]dto.Message, error) {
	itemType := strings.TrimSpace(common.Interface2String(item["type"]))
	switch itemType {
	case responsesInputTypeFunctionCall:
		toolCall, err := responsesFunctionCallItemToChatToolCall(item)
		if err != nil {
			return nil, err
		}
		return appendToolCallToLastAssistant(messages, toolCall), nil
	case responsesInputTypeCustomToolCall:
		toolCall, err := responsesCustomToolCallItemToChatToolCall(item, mappings)
		if err != nil {
			return nil, err
		}
		return appendToolCallToLastAssistant(messages, toolCall), nil
	case responsesInputTypeToolSearchCall:
		return appendToolCallToLastAssistant(messages, dto.ToolCallRequest{
			ID: responsesCallID(item), Type: "function",
			Function: dto.FunctionRequest{Name: responsesNativeToolTypeToolSearch, Arguments: responsesArgumentsString(item["arguments"])},
		}), nil
	case responsesInputTypeFunctionCallOutput, responsesInputTypeCustomToolCallOutput:
		callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
		content := responseToolOutputToChatContent(item["output"])
		return append(messages, dto.Message{Role: "tool", ToolCallId: callID, Content: content}), nil
	case responsesInputTypeToolSearchOutput:
		callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
		content := responseToolOutputToChatContent(item)
		return append(messages, dto.Message{Role: "tool", ToolCallId: callID, Content: content}), nil
	}

	role := strings.TrimSpace(common.Interface2String(item["role"]))
	if role == "" {
		role = "user"
	}
	rawContent := item["content"]
	if itemType == "input_text" {
		rawContent = item["text"]
	}
	content, err := responsesInputContentToChatContent(rawContent)
	if err != nil {
		return nil, err
	}
	return append(messages, dto.Message{Role: role, Content: content}), nil
}

func responsesToolSearchOutputTools(input json.RawMessage) ([]map[string]any, error) {
	if !rawJSONPresent(input) {
		return nil, nil
	}
	var items []map[string]any
	if err := common.Unmarshal(input, &items); err != nil {
		return nil, nil
	}
	var tools []map[string]any
	for _, item := range items {
		if strings.TrimSpace(common.Interface2String(item["type"])) != responsesInputTypeToolSearchOutput {
			continue
		}
		rawTools, ok := item["tools"].([]any)
		if !ok {
			continue
		}
		for _, rawTool := range rawTools {
			tool, ok := rawTool.(map[string]any)
			if !ok {
				return nil, errors.New("tool_search_output contains invalid tool")
			}
			tools = append(tools, tool)
		}
	}
	return tools, nil
}

func responsesInputContentToChatContent(content any) (any, error) {
	if content == nil {
		return "", nil
	}

	switch value := content.(type) {
	case string:
		return value, nil
	case []any:
		return responsesContentPartsToChatContent(value)
	case []map[string]any:
		parts := make([]any, 0, len(value))
		for _, part := range value {
			parts = append(parts, part)
		}
		return responsesContentPartsToChatContent(parts)
	default:
		return content, nil
	}
}

func responsesContentPartsToChatContent(parts []any) (any, error) {
	chatParts := make([]any, 0, len(parts))
	var textOnly strings.Builder
	onlyText := true

	for _, rawPart := range parts {
		part, ok := rawPart.(map[string]any)
		if !ok {
			onlyText = false
			chatParts = append(chatParts, rawPart)
			continue
		}

		partType := strings.TrimSpace(common.Interface2String(part["type"]))
		switch partType {
		case "input_text", "output_text", "text":
			text := common.Interface2String(part["text"])
			textOnly.WriteString(text)
			chatParts = append(chatParts, map[string]any{
				"type": dto.ContentTypeText,
				"text": text,
			})
		case "input_image":
			onlyText = false
			chatParts = append(chatParts, map[string]any{
				"type":      dto.ContentTypeImageURL,
				"image_url": responsesImagePartToChatImageURL(part),
			})
		case "input_file":
			onlyText = false
			chatParts = append(chatParts, map[string]any{
				"type": dto.ContentTypeFile,
				"file": responsesFilePartToChatFile(part),
			})
		case "input_audio":
			onlyText = false
			chatParts = append(chatParts, map[string]any{
				"type":        dto.ContentTypeInputAudio,
				"input_audio": responsesPartPayload(part, "input_audio"),
			})
		case "input_video":
			onlyText = false
			chatParts = append(chatParts, map[string]any{
				"type":      dto.ContentTypeVideoUrl,
				"video_url": responsesVideoPartToChatVideoURL(part),
			})
		default:
			onlyText = false
			chatParts = append(chatParts, part)
		}
	}

	if onlyText {
		return textOnly.String(), nil
	}
	return chatParts, nil
}

func responsesFunctionCallItemToChatToolCall(item map[string]any) (dto.ToolCallRequest, error) {
	name := strings.TrimSpace(common.Interface2String(item["name"]))
	if name == "" {
		return dto.ToolCallRequest{}, errors.New("function_call item is missing name")
	}
	return dto.ToolCallRequest{
		ID:   responsesCallID(item),
		Type: "function",
		Function: dto.FunctionRequest{
			Name:      name,
			Arguments: responsesArgumentsString(item["arguments"]),
		},
	}, nil
}

func responsesCustomToolCallItemToChatToolCall(item map[string]any, mappings map[string]dto.ResponsesToolNameMapping) (dto.ToolCallRequest, error) {
	name := strings.TrimSpace(common.Interface2String(item["name"]))
	if mapping, ok := mappings[name]; ok &&
		mapping.NativeToolType == responsesNativeToolTypeCustom &&
		mapping.ArgumentsCodec == responsesArgumentsCodecCustomInput {
		input := common.Interface2String(item["input"])
		arguments, err := common.Marshal(map[string]string{"input": input})
		if err != nil {
			return dto.ToolCallRequest{}, err
		}
		return dto.ToolCallRequest{
			ID:   responsesCallID(item),
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      name,
				Arguments: string(arguments),
			},
		}, nil
	}

	raw, err := common.Marshal(item)
	if err != nil {
		return dto.ToolCallRequest{}, err
	}
	return dto.ToolCallRequest{
		ID:     responsesCallID(item),
		Type:   dto.CustomType,
		Custom: raw,
		Function: dto.FunctionRequest{
			Name:      name,
			Arguments: responsesArgumentsString(item["input"]),
		},
	}, nil
}

func appendToolCallToLastAssistant(messages []dto.Message, toolCall dto.ToolCallRequest) []dto.Message {
	if len(messages) == 0 || messages[len(messages)-1].Role != "assistant" {
		messages = append(messages, dto.Message{Role: "assistant"})
	}

	idx := len(messages) - 1
	toolCalls := messages[idx].ParseToolCalls()
	toolCalls = append(toolCalls, toolCall)
	toolCallsRaw, _ := common.Marshal(toolCalls)
	messages[idx].ToolCalls = toolCallsRaw
	return messages
}

func responsesRequestToolsToChat(raw json.RawMessage, options ResponsesRequestToChatOptions) ([]dto.ToolCallRequest, error) {
	if !rawJSONPresent(raw) {
		return nil, nil
	}

	var tools []map[string]any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil, fmt.Errorf("invalid tools: %w", err)
	}

	out := make([]dto.ToolCallRequest, 0, len(tools))
	for _, tool := range tools {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		toolName := strings.TrimSpace(common.Interface2String(tool["name"]))
		policy := responsesToolPolicyForTool(options, toolType, toolName)
		switch toolType {
		case "computer", "computer_use", "computer_use_preview":
			switch policy {
			case ResponsesToolPolicyDrop:
				continue
			case ResponsesToolPolicyReject:
				return nil, fmt.Errorf("responses tool %q is not supported by this converter route", toolType)
			default:
				return nil, fmt.Errorf("responses tool %q has no registered Chat function adapter", toolType)
			}
		case "function":
			out = append(out, responsesFunctionToolToChat(tool, ""))
		case responsesNativeToolTypeCustom:
			if policy == ResponsesToolPolicyFlatten && toolName == "apply_patch" {
				out = append(out, responsesApplyPatchToolToChat(tool, options.ToolNameMappings))
				continue
			}
			if policy == ResponsesToolPolicyFlatten {
				return nil, fmt.Errorf("responses tool %q/%q has no registered Chat function adapter", toolType, toolName)
			}
			switch policy {
			case ResponsesToolPolicyDrop:
				continue
			case ResponsesToolPolicyReject:
				return nil, fmt.Errorf("responses tool %q is not supported by this converter route", toolType)
			default:
				chatTool, err := responsesRawToolToChat(toolType, tool)
				if err != nil {
					return nil, err
				}
				out = append(out, chatTool)
			}
		case responsesNativeToolTypeShellCommand:
			if policy == ResponsesToolPolicyFlatten {
				out = append(out, responsesShellCommandToolToChat(tool, options.ToolNameMappings))
				continue
			}
			switch policy {
			case ResponsesToolPolicyDrop:
				continue
			case ResponsesToolPolicyReject:
				return nil, fmt.Errorf("responses tool %q is not supported by this converter route", toolType)
			default:
				chatTool, err := responsesRawToolToChat(toolType, tool)
				if err != nil {
					return nil, err
				}
				out = append(out, chatTool)
			}
		case responsesNativeToolTypeToolSearch:
			if policy == ResponsesToolPolicyFlatten {
				out = append(out, responsesToolSearchToolToChat(options.ToolNameMappings))
				continue
			}
			switch policy {
			case ResponsesToolPolicyDrop:
				continue
			case ResponsesToolPolicyReject:
				return nil, fmt.Errorf("responses tool %q is not supported by this converter route", toolType)
			default:
				chatTool, err := responsesRawToolToChat(toolType, tool)
				if err != nil {
					return nil, err
				}
				out = append(out, chatTool)
			}
		case "namespace":
			switch policy {
			case ResponsesToolPolicyDrop:
				continue
			case ResponsesToolPolicyReject:
				return nil, fmt.Errorf("responses tool %q is not supported by this converter route", toolType)
			case ResponsesToolPolicyFlatten:
				flattened, err := responsesNamespaceToolToChat(tool, options.ToolNameMappings)
				if err != nil {
					return nil, err
				}
				out = append(out, flattened...)
			default:
				chatTool, err := responsesRawToolToChat(toolType, tool)
				if err != nil {
					return nil, err
				}
				out = append(out, chatTool)
			}
		default:
			switch policy {
			case ResponsesToolPolicyDrop:
				continue
			case ResponsesToolPolicyReject:
				return nil, fmt.Errorf("responses tool %q is not supported by this converter route", toolType)
			case ResponsesToolPolicyFlatten:
				return nil, fmt.Errorf("responses tool %q has no registered Chat function adapter", toolType)
			default:
				chatTool, err := responsesRawToolToChat(toolType, tool)
				if err != nil {
					return nil, err
				}
				out = append(out, chatTool)
			}
		}
	}
	return out, nil
}

func normalizeResponsesToolPolicies(options ResponsesRequestToChatOptions) ResponsesToolPolicies {
	if !responsesToolPoliciesEmpty(options.ToolPolicies) {
		return ResponsesToolPolicies{
			Namespace:       defaultResponseToolPolicy(options.ToolPolicies.Namespace, ResponsesToolPolicyFlatten),
			Custom:          defaultResponseToolPolicy(options.ToolPolicies.Custom, ResponsesToolPolicyDrop),
			WebSearch:       defaultResponseToolPolicy(options.ToolPolicies.WebSearch, ResponsesToolPolicyDrop),
			ToolSearch:      defaultResponseToolPolicy(options.ToolPolicies.ToolSearch, ResponsesToolPolicyDrop),
			ImageGeneration: defaultResponseToolPolicy(options.ToolPolicies.ImageGeneration, ResponsesToolPolicyDrop),
			Unknown:         defaultResponseToolPolicy(options.ToolPolicies.Unknown, ResponsesToolPolicyDrop),
		}
	}
	if options.FlattenNamespaceTools || options.DropUnsupportedTools {
		policies := ResponsesToolPolicies{Namespace: ResponsesToolPolicyPreserve}
		if options.FlattenNamespaceTools {
			policies.Namespace = ResponsesToolPolicyFlatten
		}
		if options.DropUnsupportedTools {
			policies.Custom = ResponsesToolPolicyDrop
			policies.WebSearch = ResponsesToolPolicyDrop
			policies.ToolSearch = ResponsesToolPolicyDrop
			policies.ImageGeneration = ResponsesToolPolicyDrop
			policies.Unknown = ResponsesToolPolicyDrop
		}
		return policies
	}
	return ResponsesToolPolicies{
		Namespace:       ResponsesToolPolicyPreserve,
		Custom:          ResponsesToolPolicyPreserve,
		WebSearch:       ResponsesToolPolicyPreserve,
		ToolSearch:      ResponsesToolPolicyPreserve,
		ImageGeneration: ResponsesToolPolicyPreserve,
		Unknown:         ResponsesToolPolicyPreserve,
	}
}

func responsesToolPoliciesEmpty(policies ResponsesToolPolicies) bool {
	return policies.Namespace == "" && policies.Custom == "" && policies.WebSearch == "" && policies.ToolSearch == "" && policies.ImageGeneration == "" && policies.Unknown == ""
}

func defaultResponseToolPolicy(policy string, fallback string) string {
	policy = strings.TrimSpace(policy)
	if policy == "" {
		return fallback
	}
	return policy
}

func responsesToolPolicyForType(policies ResponsesToolPolicies, toolType string) string {
	switch toolType {
	case "computer", "computer_use", "computer_use_preview":
		return ResponsesToolPolicyReject
	case "namespace":
		return policies.Namespace
	case "custom":
		return policies.Custom
	case "web_search", "web_search_preview":
		return policies.WebSearch
	case "tool_search":
		return policies.ToolSearch
	case "image_gen", "image_generation":
		return policies.ImageGeneration
	default:
		return defaultResponseToolPolicy(policies.Unknown, ResponsesToolPolicyPreserve)
	}
}

func responsesToolPolicyForTool(options ResponsesRequestToChatOptions, toolType string, toolName string) string {
	if options.ToolPolicyResolver != nil {
		if policy := strings.TrimSpace(options.ToolPolicyResolver(toolType, toolName)); policy != "" {
			return policy
		}
	}
	return responsesToolPolicyForType(normalizeResponsesToolPolicies(options), toolType)
}

func responsesFunctionToolToChat(tool map[string]any, namePrefix string) dto.ToolCallRequest {
	name := strings.TrimSpace(common.Interface2String(tool["name"]))
	return dto.ToolCallRequest{
		Type: "function",
		Function: dto.FunctionRequest{
			Name:        namePrefix + name,
			Description: common.Interface2String(tool["description"]),
			Parameters:  tool["parameters"],
		},
	}
}

func responsesApplyPatchToolToChat(tool map[string]any, mappings map[string]dto.ResponsesToolNameMapping) dto.ToolCallRequest {
	const name = "apply_patch"
	if mappings != nil {
		mappings[name] = dto.ResponsesToolNameMapping{
			Name:           name,
			NativeToolType: responsesNativeToolTypeCustom,
			ArgumentsCodec: responsesArgumentsCodecCustomInput,
		}
	}
	return dto.ToolCallRequest{
		Type: "function",
		Function: dto.FunctionRequest{
			Name:        name,
			Description: common.Interface2String(tool["description"]),
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"input"},
				"properties": map[string]any{
					"input": map[string]any{"type": "string"},
				},
			},
		},
	}
}

func responsesShellCommandToolToChat(tool map[string]any, mappings map[string]dto.ResponsesToolNameMapping) dto.ToolCallRequest {
	const name = responsesNativeToolTypeShellCommand
	if mappings != nil {
		mappings[name] = dto.ResponsesToolNameMapping{
			Name:           name,
			NativeToolType: responsesNativeToolTypeShellCommand,
		}
	}
	return dto.ToolCallRequest{
		Type: "function",
		Function: dto.FunctionRequest{
			Name:        name,
			Description: common.Interface2String(tool["description"]),
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": true,
				"required":             []string{"command"},
				"properties": map[string]any{
					"command":                map[string]any{"type": "string"},
					"workdir":                map[string]any{"type": "string"},
					"login":                  map[string]any{"type": "boolean"},
					"timeout_ms":             map[string]any{"type": "integer", "minimum": 0},
					"sandbox_permissions":    map[string]any{},
					"prefix_rule":            map[string]any{},
					"additional_permissions": map[string]any{},
					"justification":          map[string]any{},
				},
			},
		},
	}
}

func responsesToolSearchToolToChat(mappings map[string]dto.ResponsesToolNameMapping) dto.ToolCallRequest {
	if mappings != nil {
		mappings[responsesNativeToolTypeToolSearch] = dto.ResponsesToolNameMapping{Name: responsesNativeToolTypeToolSearch, NativeToolType: responsesNativeToolTypeToolSearch}
	}
	return dto.ToolCallRequest{Type: "function", Function: dto.FunctionRequest{
		Name:        responsesNativeToolTypeToolSearch,
		Description: "Search and load tools, plugins, connectors, and MCP namespaces for the current task.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"query": map[string]any{"type": "string"},
			"limit": map[string]any{"type": "integer"},
		}, "required": []string{"query"}},
	}}
}

func responsesNamespaceToolToChat(tool map[string]any, mappings map[string]dto.ResponsesToolNameMapping) ([]dto.ToolCallRequest, error) {
	namespace := strings.TrimSpace(common.Interface2String(tool["name"]))
	if namespace == "" {
		return nil, fmt.Errorf("namespace tool name is required")
	}
	rawTools, ok := tool["tools"].([]any)
	if !ok {
		return nil, fmt.Errorf("namespace tool %q tools must be an array", namespace)
	}
	out := make([]dto.ToolCallRequest, 0, len(rawTools))
	for _, rawTool := range rawTools {
		nested, ok := rawTool.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("namespace tool %q contains invalid tool", namespace)
		}
		nestedType := strings.TrimSpace(common.Interface2String(nested["type"]))
		if nestedType != "function" {
			continue
		}
		nestedName := strings.TrimSpace(common.Interface2String(nested["name"]))
		if nestedName == "" {
			return nil, fmt.Errorf("namespace tool %q contains function without name", namespace)
		}
		flatName := namespace + nestedName
		chatTool := responsesFunctionToolToChat(nested, namespace)
		out = append(out, chatTool)
		if mappings != nil {
			mappings[flatName] = dto.ResponsesToolNameMapping{Namespace: namespace, Name: nestedName}
		}
	}
	return out, nil
}

func responsesRawToolToChat(toolType string, tool map[string]any) (dto.ToolCallRequest, error) {
	// Preserve explicit Chat-compatible web_search options. Vendor-specific
	// defaults are applied later by the Advanced Custom route configuration.
	if toolType == "web_search" || toolType == "web_search_preview" {
		webSearch, _ := tool["web_search"].(map[string]any)
		return dto.ToolCallRequest{Type: "web_search", WebSearch: webSearch}, nil
	}
	rawTool, err := common.Marshal(tool)
	if err != nil {
		return dto.ToolCallRequest{}, fmt.Errorf("invalid responses tool %q: %w", toolType, err)
	}
	return dto.ToolCallRequest{
		Type:   toolType,
		Custom: rawTool,
	}, nil
}

func responsesRequestToolChoiceToChat(raw json.RawMessage, options ResponsesRequestToChatOptions, tools []dto.ToolCallRequest) (any, error) {
	if !rawJSONPresent(raw) {
		return nil, nil
	}
	if common.GetJsonType(raw) == "string" {
		var choice string
		if err := common.Unmarshal(raw, &choice); err != nil {
			return nil, fmt.Errorf("invalid tool_choice: %w", err)
		}
		return choice, nil
	}

	var choice map[string]any
	if err := common.Unmarshal(raw, &choice); err != nil {
		return nil, fmt.Errorf("invalid tool_choice: %w", err)
	}
	toolType := strings.TrimSpace(common.Interface2String(choice["type"]))
	policies := normalizeResponsesToolPolicies(options)

	if toolType == "function" {
		name := strings.TrimSpace(common.Interface2String(choice["name"]))
		if name == "" {
			return choice, nil
		}
		resolvedName, err := resolveResponsesFunctionToolChoiceName(name, tools, options.ToolNameMappings)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": resolvedName,
			},
		}, nil
	}

	if toolType == responsesNativeToolTypeCustom && responsesToolPolicyForTool(options, toolType, strings.TrimSpace(common.Interface2String(choice["name"]))) == ResponsesToolPolicyFlatten {
		name := strings.TrimSpace(common.Interface2String(choice["name"]))
		if name == "apply_patch" && hasChatFunctionTool(tools, name) {
			return responsesFunctionToolChoice(name), nil
		}
	}
	if toolType == responsesNativeToolTypeShellCommand && responsesToolPolicyForTool(options, toolType, "") == ResponsesToolPolicyFlatten {
		if hasChatFunctionTool(tools, responsesNativeToolTypeShellCommand) {
			return responsesFunctionToolChoice(responsesNativeToolTypeShellCommand), nil
		}
	}
	if toolType == responsesNativeToolTypeToolSearch && responsesToolPolicyForTool(options, toolType, "") == ResponsesToolPolicyFlatten {
		if hasChatFunctionTool(tools, responsesNativeToolTypeToolSearch) {
			return responsesFunctionToolChoice(responsesNativeToolTypeToolSearch), nil
		}
	}

	if toolType == "allowed_tools" && responsesToolPoliciesMutateTools(policies) {
		return nil, errors.New("responses tool_choice type \"allowed_tools\" cannot be safely converted when tool policies modify tools")
	}

	policy := responsesToolPolicyForTool(options, toolType, strings.TrimSpace(common.Interface2String(choice["name"])))
	if toolType == "namespace" {
		policy = policies.Namespace
	}
	switch policy {
	case ResponsesToolPolicyDrop, ResponsesToolPolicyReject:
		return nil, fmt.Errorf("responses tool_choice selects %q, but its converter policy is %q", toolType, policy)
	case ResponsesToolPolicyFlatten:
		return nil, fmt.Errorf("responses tool_choice selects %q, but an explicit flattened tool cannot be resolved safely", toolType)
	}

	if toolType == "custom" {
		name := strings.TrimSpace(common.Interface2String(choice["name"]))
		if name != "" {
			return map[string]any{
				"type": "custom",
				"custom": map[string]any{
					"name": name,
				},
			}, nil
		}
	}
	return choice, nil
}

func hasChatFunctionTool(tools []dto.ToolCallRequest, name string) bool {
	for _, tool := range tools {
		if tool.Type == "function" && tool.Function.Name == name {
			return true
		}
	}
	return false
}

func responsesFunctionToolChoice(name string) map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name": name,
		},
	}
}

func resolveResponsesFunctionToolChoiceName(name string, tools []dto.ToolCallRequest, mappings map[string]dto.ResponsesToolNameMapping) (string, error) {
	for _, tool := range tools {
		if tool.Type == "function" && tool.Function.Name == name {
			return name, nil
		}
	}

	matches := make([]string, 0, 1)
	for flatName, mapping := range mappings {
		if mapping.Name != name {
			continue
		}
		for _, tool := range tools {
			if tool.Type == "function" && tool.Function.Name == flatName {
				matches = append(matches, flatName)
				break
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("responses tool_choice function %q matches multiple flattened namespace tools", name)
	}
	return name, nil
}

func responsesToolPoliciesMutateTools(policies ResponsesToolPolicies) bool {
	for _, policy := range []string{
		policies.Namespace,
		policies.Custom,
		policies.WebSearch,
		policies.ToolSearch,
		policies.ImageGeneration,
		policies.Unknown,
	} {
		if policy != "" && policy != ResponsesToolPolicyPreserve {
			return true
		}
	}
	return false
}

func responsesRequestTextToChatResponseFormat(raw json.RawMessage) (*dto.ResponseFormat, error) {
	if !rawJSONPresent(raw) {
		return nil, nil
	}

	var textConfig map[string]any
	if err := common.Unmarshal(raw, &textConfig); err != nil {
		return nil, fmt.Errorf("invalid text config: %w", err)
	}
	format, ok := textConfig["format"].(map[string]any)
	if !ok {
		return nil, nil
	}

	formatType := strings.TrimSpace(common.Interface2String(format["type"]))
	if formatType == "" {
		return nil, nil
	}

	out := &dto.ResponseFormat{Type: formatType}
	if formatType == "json_schema" {
		schemaRaw, err := common.Marshal(format)
		if err != nil {
			return nil, err
		}
		out.JsonSchema = schemaRaw
	}
	return out, nil
}

func responsesImagePartToChatImageURL(part map[string]any) any {
	if imageURL, ok := part["image_url"]; ok {
		return imageURL
	}
	imageURL := map[string]any{}
	for _, key := range []string{"url", "file_id", "detail"} {
		if value, ok := part[key]; ok {
			imageURL[key] = value
		}
	}
	if len(imageURL) == 0 {
		return part
	}
	return imageURL
}

func responsesFilePartToChatFile(part map[string]any) any {
	if file, ok := part["file"]; ok {
		return file
	}
	file := map[string]any{}
	for _, key := range []string{"file_id", "file_data", "filename", "file_url"} {
		if value, ok := part[key]; ok {
			file[key] = value
		}
	}
	if len(file) == 0 {
		return part
	}
	return file
}

func responsesVideoPartToChatVideoURL(part map[string]any) any {
	if videoURL, ok := part["video_url"]; ok {
		if videoURLMap, ok := videoURL.(map[string]any); ok {
			if url := common.Interface2String(videoURLMap["url"]); url != "" {
				return url
			}
		}
		return videoURL
	}
	if url := common.Interface2String(part["url"]); url != "" {
		return url
	}
	return responsesPartPayload(part, "video_url")
}

func responsesPartPayload(part map[string]any, key string) any {
	if value, ok := part[key]; ok {
		return value
	}
	payload := make(map[string]any, len(part))
	for k, value := range part {
		if k == "type" {
			continue
		}
		payload[k] = value
	}
	return payload
}

func responsesCallID(item map[string]any) string {
	callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
	if callID != "" {
		return callID
	}
	return strings.TrimSpace(common.Interface2String(item["id"]))
}

func responsesArgumentsString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		raw, err := common.Marshal(v)
		if err != nil {
			return common.Interface2String(v)
		}
		return string(raw)
	}
}

func responseToolOutputToChatContent(value any) any {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		raw, err := common.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(raw)
	}
}

func responsesJSONString(raw json.RawMessage) (string, error) {
	if common.GetJsonType(raw) != "string" {
		return string(raw), nil
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return value, nil
}

func rawJSONPresent(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	return common.GetJsonType(raw) != "null"
}

// ApplyResponsesToolPolicies filters Responses tools before passthrough or
// format conversion. Function tools are protected and always preserved. Drop
// removes a tool, while reject stops the request with an explicit error.
func ApplyResponsesToolPolicies(rawTools json.RawMessage, resolver ResponsesToolPolicyResolver) (json.RawMessage, []ResponsesToolPolicyDecision, error) {
	if !rawJSONPresent(rawTools) || resolver == nil {
		return rawTools, nil, nil
	}

	var tools []map[string]any
	if err := common.Unmarshal(rawTools, &tools); err != nil {
		return nil, nil, fmt.Errorf("invalid tools: %w", err)
	}

	filtered := make([]map[string]any, 0, len(tools))
	decisions := make([]ResponsesToolPolicyDecision, 0)
	for toolIndex, tool := range tools {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		toolName := strings.TrimSpace(common.Interface2String(tool["name"]))
		if toolType == "function" {
			filtered = append(filtered, tool)
			continue
		}

		policy := strings.TrimSpace(resolver(toolType, toolName))
		if policy == "" {
			policy = ResponsesToolPolicyPreserve
		}
		switch policy {
		case ResponsesToolPolicyDrop:
			decisions = append(decisions, ResponsesToolPolicyDecision{ToolIndex: toolIndex, ToolType: toolType, ToolName: toolName, Policy: policy})
		case ResponsesToolPolicyReject:
			decisions = append(decisions, ResponsesToolPolicyDecision{ToolIndex: toolIndex, ToolType: toolType, ToolName: toolName, Policy: policy})
			return nil, decisions, fmt.Errorf("responses tool %s/%s was rejected by the Advanced Custom route policy", toolType, toolName)
		default:
			filtered = append(filtered, tool)
		}
	}

	if len(tools) > 0 && len(filtered) == 0 {
		return nil, decisions, fmt.Errorf("All Responses tools were removed by the Advanced Custom route policy: %s", formatResponsesToolPolicyDecisions(decisions))
	}
	if len(decisions) == 0 {
		return rawTools, nil, nil
	}
	result, err := common.Marshal(filtered)
	if err != nil {
		return nil, decisions, err
	}
	return result, decisions, nil
}

// ApplyResponsesToolPoliciesForPassthrough is retained for compatibility with
// existing callers that use only route-level type policies. New Advanced Custom
// code should use ApplyResponsesToolPolicies so reject and empty-tool errors are
// not hidden.
func ApplyResponsesToolPoliciesForPassthrough(rawTools json.RawMessage, policies ResponsesToolPolicies) json.RawMessage {
	if len(rawTools) == 0 || !responsesToolPoliciesMutateTools(policies) {
		return rawTools
	}
	var tools []map[string]any
	if err := common.Unmarshal(rawTools, &tools); err != nil {
		return rawTools
	}
	filtered := make([]map[string]any, 0, len(tools))
	changed := false
	for _, tool := range tools {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		if toolType == "function" {
			filtered = append(filtered, tool)
			continue
		}
		switch responsesToolPolicyForType(policies, toolType) {
		case ResponsesToolPolicyDrop, ResponsesToolPolicyReject:
			changed = true
		default:
			filtered = append(filtered, tool)
		}
	}
	if !changed {
		return rawTools
	}
	result, err := common.Marshal(filtered)
	if err != nil {
		return rawTools
	}
	return result
}

var responsesHostedToolTypes = map[string]struct{}{
	"apply_patch":          {},
	"code_interpreter":     {},
	"computer_use":         {},
	"computer_use_preview": {},
	"file_search":          {},
	"image_gen":            {},
	"image_generation":     {},
	"shell":                {},
	"tool_search":          {},
	"web_search":           {},
	"web_search_preview":   {},
}

// ApplyResponsesToolConflictPolicy handles conflicts only after unsupported
// tools have been filtered. This ordering prevents deleting a client tool for
// a hosted capability that will not be sent upstream.
func ApplyResponsesToolConflictPolicy(rawTools json.RawMessage, policy string) (json.RawMessage, []ResponsesToolPolicyDecision, error) {
	return ApplyResponsesToolConflictPolicyWithImplicitHostedTools(rawTools, policy, nil)
}

func ApplyResponsesToolConflictPolicyWithImplicitHostedTools(
	rawTools json.RawMessage,
	policy string,
	implicitHostedTools []string,
) (json.RawMessage, []ResponsesToolPolicyDecision, error) {
	policy = strings.TrimSpace(policy)
	if policy == "" {
		policy = dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate
	}
	if policy == dto.AdvancedCustomResponsesToolConflictPolicyPreserve || !rawJSONPresent(rawTools) {
		return rawTools, nil, nil
	}

	var tools []map[string]any
	if err := common.Unmarshal(rawTools, &tools); err != nil {
		return nil, nil, fmt.Errorf("invalid tools: %w", err)
	}
	hostedCapabilities := make(map[string]struct{}, len(implicitHostedTools))
	for _, rawCapability := range implicitHostedTools {
		capability := responsesToolPolicyType(rawCapability)
		if capability != "" {
			hostedCapabilities[capability] = struct{}{}
		}
	}
	for _, tool := range tools {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		if _, isHosted := responsesHostedToolTypes[toolType]; isHosted {
			hostedCapabilities[responsesToolPolicyType(toolType)] = struct{}{}
		}
	}
	if len(hostedCapabilities) == 0 {
		return rawTools, nil, nil
	}

	filtered := make([]map[string]any, 0, len(tools))
	decisions := make([]ResponsesToolPolicyDecision, 0)
	for _, tool := range tools {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		toolName := strings.TrimSpace(common.Interface2String(tool["name"]))
		capability := ""
		switch toolType {
		case "function":
			capability = toolName
			if idx := strings.IndexByte(toolName, '.'); idx > 0 {
				capability = toolName[:idx]
			}
		case "namespace":
			capability = toolName
		}
		capability = responsesToolPolicyType(capability)
		if capability != "" {
			if _, conflicts := hostedCapabilities[capability]; conflicts {
				decision := ResponsesToolPolicyDecision{ToolType: toolType, ToolName: toolName, Policy: policy}
				decisions = append(decisions, decision)
				if policy == dto.AdvancedCustomResponsesToolConflictPolicyReject {
					return nil, decisions, fmt.Errorf("responses %s tool %q conflicts with hosted tool %q", toolType, toolName, capability)
				}
				if policy == dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate {
					continue
				}
			}
		}
		filtered = append(filtered, tool)
	}
	if len(decisions) == 0 {
		return rawTools, nil, nil
	}
	result, err := common.Marshal(filtered)
	if err != nil {
		return nil, decisions, err
	}
	return result, decisions, nil
}

// DeduplicateConflictingTools preserves the old helper contract.
func DeduplicateConflictingTools(rawTools json.RawMessage) json.RawMessage {
	result, _, err := ApplyResponsesToolConflictPolicy(rawTools, dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate)
	if err != nil {
		return rawTools
	}
	return result
}

func ValidateResponsesToolChoiceAfterPolicy(rawChoice json.RawMessage, decisions []ResponsesToolPolicyDecision) error {
	if !rawJSONPresent(rawChoice) || len(decisions) == 0 || common.GetJsonType(rawChoice) == "string" {
		return nil
	}
	var choice map[string]any
	if err := common.Unmarshal(rawChoice, &choice); err != nil {
		return fmt.Errorf("invalid tool_choice: %w", err)
	}
	toolType := strings.TrimSpace(common.Interface2String(choice["type"]))
	toolName := responsesToolDisplayName(toolType, choice)
	for _, decision := range decisions {
		if responsesToolIdentityMatches(toolType, toolName, decision.ToolType, decision.ToolName) {
			return fmt.Errorf("tool_choice selects a tool removed by the channel policy: %s/%s", toolType, toolName)
		}
	}
	return nil
}

func responsesToolDisplayName(toolType string, tool map[string]any) string {
	name := strings.TrimSpace(common.Interface2String(tool["name"]))
	if name != "" {
		return name
	}
	return toolType
}

func responsesToolIdentityMatches(leftType string, leftName string, rightType string, rightName string) bool {
	leftType = strings.TrimSpace(leftType)
	rightType = strings.TrimSpace(rightType)
	if leftType == "function" && rightType == "namespace" {
		return responsesFunctionBelongsToNamespace(leftName, rightName)
	}
	if leftType == "namespace" && rightType == "function" {
		return responsesFunctionBelongsToNamespace(rightName, leftName)
	}
	leftPolicyType := responsesToolPolicyType(leftType)
	rightPolicyType := responsesToolPolicyType(rightType)
	if strings.TrimSpace(rightName) == "" && leftPolicyType == rightPolicyType {
		return true
	}
	return leftPolicyType == rightPolicyType && strings.TrimSpace(leftName) == strings.TrimSpace(rightName)
}

func responsesFunctionBelongsToNamespace(functionName string, namespaceName string) bool {
	prefix := strings.TrimSpace(functionName)
	if idx := strings.IndexByte(prefix, '.'); idx > 0 {
		prefix = prefix[:idx]
	}
	return responsesToolPolicyType(prefix) == responsesToolPolicyType(namespaceName)
}

func responsesToolPolicyType(toolType string) string {
	switch strings.TrimSpace(toolType) {
	case "image_gen", "image_generation":
		return "image_generation"
	case "web_search", "web_search_preview":
		return "web_search"
	default:
		return strings.TrimSpace(toolType)
	}
}

func formatResponsesToolPolicyDecisions(decisions []ResponsesToolPolicyDecision) string {
	parts := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		parts = append(parts, fmt.Sprintf("%s/%s=%s", decision.ToolType, decision.ToolName, decision.Policy))
	}
	return strings.Join(parts, ", ")
}
