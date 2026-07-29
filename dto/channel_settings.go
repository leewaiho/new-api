package dto

import (
	"fmt"
	"net/url"
	"strings"
)

type ChannelSettings struct {
	ForceFormat            bool   `json:"force_format,omitempty"`
	ThinkingToContent      bool   `json:"thinking_to_content,omitempty"`
	Proxy                  string `json:"proxy"`
	PassThroughBodyEnabled bool   `json:"pass_through_body_enabled,omitempty"`
	SystemPrompt           string `json:"system_prompt,omitempty"`
	SystemPromptOverride   bool   `json:"system_prompt_override,omitempty"`
}

type VertexKeyType string

const (
	VertexKeyTypeJSON   VertexKeyType = "json"
	VertexKeyTypeAPIKey VertexKeyType = "api_key"
)

type AwsKeyType string

const (
	AwsKeyTypeAKSK   AwsKeyType = "ak_sk" // 默认
	AwsKeyTypeApiKey AwsKeyType = "api_key"
)

type ChannelOtherSettings struct {
	AzureResponsesVersion                 string                `json:"azure_responses_version,omitempty"`
	VertexKeyType                         VertexKeyType         `json:"vertex_key_type,omitempty"` // "json" or "api_key"
	OpenRouterEnterprise                  *bool                 `json:"openrouter_enterprise,omitempty"`
	ClaudeBetaQuery                       bool                  `json:"claude_beta_query,omitempty"`          // Claude 渠道是否强制追加 ?beta=true
	AllowServiceTier                      bool                  `json:"allow_service_tier,omitempty"`         // 是否允许 service_tier 透传（默认过滤以避免额外计费）
	AllowInferenceGeo                     bool                  `json:"allow_inference_geo,omitempty"`        // 是否允许 inference_geo 透传（仅 Claude，默认过滤以满足数据驻留合规
	AllowSpeed                            bool                  `json:"allow_speed,omitempty"`                // 是否允许 speed 透传（仅 Claude，默认过滤以避免意外切换推理速度模式）
	AllowSafetyIdentifier                 bool                  `json:"allow_safety_identifier,omitempty"`    // 是否允许 safety_identifier 透传（默认过滤以保护用户隐私）
	DisableStore                          bool                  `json:"disable_store,omitempty"`              // 是否禁用 store 透传（默认允许透传，禁用后可能导致 Codex 无法使用）
	AllowIncludeObfuscation               bool                  `json:"allow_include_obfuscation,omitempty"`  // 是否允许 stream_options.include_obfuscation 透传（默认过滤以避免关闭流混淆保护）
	DisableTaskPollingSleep               bool                  `json:"disable_task_polling_sleep,omitempty"` // 是否跳过异步任务轮询间隔
	AwsKeyType                            AwsKeyType            `json:"aws_key_type,omitempty"`
	UpstreamModelUpdateCheckEnabled       bool                  `json:"upstream_model_update_check_enabled,omitempty"`        // 是否检测上游模型更新
	UpstreamModelUpdateAutoSyncEnabled    bool                  `json:"upstream_model_update_auto_sync_enabled,omitempty"`    // 是否自动同步上游模型更新
	UpstreamModelUpdateLastCheckTime      int64                 `json:"upstream_model_update_last_check_time,omitempty"`      // 上次检测时间
	UpstreamModelUpdateLastDetectedModels []string              `json:"upstream_model_update_last_detected_models,omitempty"` // 上次检测到的可加入模型
	UpstreamModelUpdateLastRemovedModels  []string              `json:"upstream_model_update_last_removed_models,omitempty"`  // 上次检测到的可删除模型
	UpstreamModelUpdateIgnoredModels      []string              `json:"upstream_model_update_ignored_models,omitempty"`       // 手动忽略的模型
	AdvancedCustom                        *AdvancedCustomConfig `json:"advanced_custom,omitempty"`
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}

const (
	AdvancedCustomConverterNone                                         = "none"
	AdvancedCustomConverterAnthropicMessagesToOpenAIChatCompletions     = "anthropic_messages_to_openai_chat_completions"
	AdvancedCustomConverterOpenAIChatCompletionsToAnthropicMessages     = "openai_chat_completions_to_anthropic_messages"
	AdvancedCustomConverterOpenAIChatCompletionsToOpenAIResponses       = "openai_chat_completions_to_openai_responses"
	AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions       = "openai_responses_to_openai_chat_completions"
	AdvancedCustomConverterGeminiGenerateContentToOpenAIChatCompletions = "gemini_generate_content_to_openai_chat_completions"
	AdvancedCustomConverterOpenAIChatCompletionsToGeminiGenerateContent = "openai_chat_completions_to_gemini_generate_content"
)

const (
	AdvancedCustomAuthTypeNone   = "none"
	AdvancedCustomAuthTypeHeader = "header"
	AdvancedCustomAuthTypeQuery  = "query"
)

const (
	AdvancedCustomResponsesToolsModeCompatFlatten = "compat_flatten"
	AdvancedCustomResponsesToolsModePreserve      = "preserve"
)

const (
	AdvancedCustomResponsesToolPolicyPreserve = "preserve"
	AdvancedCustomResponsesToolPolicyFlatten  = "flatten"
	AdvancedCustomResponsesToolPolicyDrop     = "drop"
	AdvancedCustomResponsesToolPolicyReject   = "reject"
)

const (
	AdvancedCustomResponsesToolConflictPolicyPreserve    = "preserve"
	AdvancedCustomResponsesToolConflictPolicyDeduplicate = "deduplicate"
	AdvancedCustomResponsesToolConflictPolicyReject      = "reject"
)

type AdvancedCustomConfig struct {
	Routes         []AdvancedCustomRoute `json:"advanced_routes,omitempty"`
	ModelFetchURLs []string              `json:"model_fetch_urls,omitempty"`
}

type AdvancedCustomRoute struct {
	IncomingPath     string                          `json:"incoming_path,omitempty"`
	UpstreamPath     string                          `json:"upstream_path,omitempty"`
	Converter        string                          `json:"converter,omitempty"`
	Auth             *AdvancedCustomRouteAuth        `json:"auth,omitempty"`
	ConverterOptions *AdvancedCustomConverterOptions `json:"converter_options,omitempty"`
}

type AdvancedCustomConverterOptions struct {
	// ResponsesToolsMode is the legacy coarse-grained switch. It is kept for
	// backward compatibility and is expanded to ResponsesTools when ResponsesTools
	// is empty.
	//   - compat_flatten: flatten namespace function tools and drop unsupported tools
	//   - preserve: preserve original tool objects for upstreams that support them
	ResponsesToolsMode string `json:"responses_tools_mode,omitempty"`
	// ResponsesTools controls individual Responses tool types when converting
	// /v1/responses requests to Chat Completions upstreams. Supported policy values
	// are preserve, flatten, drop, and reject. Flatten is only valid for namespace.
	ResponsesTools *AdvancedCustomResponsesToolsOptions `json:"responses_tools,omitempty"`
	// ResponsesDropFields lists Responses request field names that must be
	// stripped from the converted Chat Completions request. Use it to
	// remove Responses-only fields (for example "metadata") that the chat
	// upstream rejects.
	ResponsesDropFields []string `json:"responses_drop_fields,omitempty"`
	// ResponsesToolConflictPolicy controls conflicts between hosted tools and
	// same-capability client tools. The safe default is deduplicate.
	ResponsesToolConflictPolicy string `json:"responses_tool_conflict_policy,omitempty"`
	// ResponsesImplicitHostedTools declares capabilities automatically supplied by
	// the upstream even when they are absent from the incoming tools array.
	ResponsesImplicitHostedTools []string `json:"responses_implicit_hosted_tools,omitempty"`
	// ResponsesToolParameters configures safe, tool-specific compatibility
	// rewrites after a Responses request is converted to Chat Completions.
	ResponsesToolParameters *AdvancedCustomResponsesToolParameters `json:"responses_tool_parameters,omitempty"`
	// ResponsesToolChoice configures compatibility for explicit Responses
	// tool_choice objects after converting to a Chat Completions upstream.
	ResponsesToolChoice *AdvancedCustomResponsesToolChoiceCompatibility `json:"responses_tool_choice,omitempty"`
	// ResponsesToolContinuation configures an opt-in user continuation for
	// pure Responses tool-output follow-ups sent to Chat Completions upstreams.
	ResponsesToolContinuation *AdvancedCustomResponsesToolContinuation `json:"responses_tool_continuation,omitempty"`
	// ResponsesToolStateReplay enables a bounded, encrypted Redis bridge for
	// previous_response_id tool-output follow-ups sent to Chat Completions upstreams.
	// It is disabled unless Enabled is true.
	ResponsesToolStateReplay *AdvancedCustomResponsesToolStateReplay `json:"responses_tool_state_replay,omitempty"`
	// ResponsesToolModelOverrides applies exact model-specific exceptions. A
	// model matches either the requested model name or the mapped upstream model
	// name. The same model may only appear in one override.
	ResponsesToolModelOverrides []AdvancedCustomResponsesToolModelOverride `json:"responses_tool_model_overrides,omitempty"`
}

type AdvancedCustomResponsesToolsOptions struct {
	Namespace       string `json:"namespace,omitempty"`
	Custom          string `json:"custom,omitempty"`
	WebSearch       string `json:"web_search,omitempty"`
	ToolSearch      string `json:"tool_search,omitempty"`
	ImageGeneration string `json:"image_generation,omitempty"`
	Unknown         string `json:"unknown,omitempty"`
}

type AdvancedCustomResponsesToolModelOverride struct {
	Models                       []string                                        `json:"models,omitempty"`
	ResponsesTools               *AdvancedCustomResponsesToolsOptions            `json:"responses_tools,omitempty"`
	ToolNames                    []AdvancedCustomResponsesToolNamePolicy         `json:"responses_tool_names,omitempty"`
	ResponsesImplicitHostedTools []string                                        `json:"responses_implicit_hosted_tools,omitempty"`
	ResponsesToolParameters      *AdvancedCustomResponsesToolParameters          `json:"responses_tool_parameters,omitempty"`
	ResponsesToolChoice          *AdvancedCustomResponsesToolChoiceCompatibility `json:"responses_tool_choice,omitempty"`
	ResponsesToolContinuation    *AdvancedCustomResponsesToolContinuation        `json:"responses_tool_continuation,omitempty"`
	ResponsesToolStateReplay     *AdvancedCustomResponsesToolStateReplay         `json:"responses_tool_state_replay,omitempty"`
}

type AdvancedCustomResponsesToolParameters struct {
	WebSearch *AdvancedCustomWebSearchParameterCompatibility `json:"web_search,omitempty"`
}

type AdvancedCustomResponsesToolChoiceCompatibility struct {
	WebSearch string `json:"web_search,omitempty"`
}

const AdvancedCustomResponsesToolContinuationAppendUser = "append_user_continuation"

type AdvancedCustomResponsesToolContinuation struct {
	WhenOnlyToolOutput string `json:"when_only_tool_output,omitempty"`
	Text               string `json:"text,omitempty"`
}

// AdvancedCustomResponsesToolStateReplay controls the opt-in state bridge used
// only by Responses to Chat conversion routes. TTLSeconds is required when enabled.
type AdvancedCustomResponsesToolStateReplay struct {
	Enabled    bool `json:"enabled,omitempty"`
	TTLSeconds int  `json:"ttl_seconds,omitempty"`
}

type AdvancedCustomWebSearchParameterCompatibility struct {
	WhenNestedOptionsMissing string                                   `json:"when_nested_options_missing,omitempty"`
	Defaults                 AdvancedCustomWebSearchParameterDefaults `json:"defaults,omitempty"`
}

type AdvancedCustomWebSearchParameterDefaults struct {
	Enable       *bool  `json:"enable,omitempty"`
	SearchResult *bool  `json:"search_result,omitempty"`
	SearchEngine string `json:"search_engine,omitempty"`
}

type AdvancedCustomResponsesToolNamePolicy struct {
	ToolType string `json:"tool_type,omitempty"`
	ToolName string `json:"tool_name,omitempty"`
	Policy   string `json:"policy,omitempty"`
}

type AdvancedCustomRouteAuth struct {
	Type  string `json:"type,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

const advancedCustomModelPlaceholder = "{model}"

// MatchPath returns the first route whose IncomingPath matches requestPath.
// Matching mirrors the relay adaptor: exact match, {model} placeholder, and
// :generateContent <-> :streamGenerateContent equivalence.
func (c *AdvancedCustomConfig) MatchPath(requestPath string) (AdvancedCustomRoute, bool) {
	if c == nil {
		return AdvancedCustomRoute{}, false
	}
	for _, route := range c.Routes {
		if matchAdvancedCustomIncomingPath(strings.TrimSpace(route.IncomingPath), requestPath) {
			return route, true
		}
	}
	return AdvancedCustomRoute{}, false
}

// SupportsPath reports whether any route matches requestPath.
func (c *AdvancedCustomConfig) SupportsPath(requestPath string) bool {
	_, ok := c.MatchPath(requestPath)
	return ok
}

func matchAdvancedCustomIncomingPath(configuredPath string, requestPath string) bool {
	if matchAdvancedCustomIncomingPathTemplate(configuredPath, requestPath) {
		return true
	}
	if strings.Contains(configuredPath, ":generateContent") {
		streamPath := strings.Replace(configuredPath, ":generateContent", ":streamGenerateContent", 1)
		return matchAdvancedCustomIncomingPathTemplate(streamPath, requestPath)
	}
	return false
}

func matchAdvancedCustomIncomingPathTemplate(configuredPath string, requestPath string) bool {
	if !strings.Contains(configuredPath, advancedCustomModelPlaceholder) {
		return configuredPath == requestPath
	}

	parts := strings.Split(configuredPath, advancedCustomModelPlaceholder)
	if len(parts) != 2 {
		return false
	}
	if !strings.HasPrefix(requestPath, parts[0]) || !strings.HasSuffix(requestPath, parts[1]) {
		return false
	}

	model := strings.TrimSuffix(strings.TrimPrefix(requestPath, parts[0]), parts[1])
	return model != "" && !strings.Contains(model, "/")
}

func IsAdvancedCustomConverterAllowed(converter string) bool {
	switch converter {
	case AdvancedCustomConverterNone,
		AdvancedCustomConverterAnthropicMessagesToOpenAIChatCompletions,
		AdvancedCustomConverterOpenAIChatCompletionsToAnthropicMessages,
		AdvancedCustomConverterOpenAIChatCompletionsToOpenAIResponses,
		AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
		AdvancedCustomConverterGeminiGenerateContentToOpenAIChatCompletions,
		AdvancedCustomConverterOpenAIChatCompletionsToGeminiGenerateContent:
		return true
	default:
		return false
	}
}

func (c *AdvancedCustomConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("advanced_custom is required")
	}
	if len(c.Routes) == 0 {
		return fmt.Errorf("advanced_custom requires at least one route")
	}

	for i, fetchURL := range c.ModelFetchURLs {
		fetchURL = strings.TrimSpace(fetchURL)
		if fetchURL == "" {
			continue
		}
		if err := validateAdvancedCustomModelFetchURL(i, fetchURL); err != nil {
			return err
		}
	}

	seenPaths := make(map[string]struct{}, len(c.Routes))
	for i := range c.Routes {
		route := c.Routes[i]
		route.IncomingPath = strings.TrimSpace(route.IncomingPath)
		upstreamPath := strings.TrimSpace(route.UpstreamPath)
		route.Converter = strings.TrimSpace(route.Converter)
		if route.Converter == "" {
			route.Converter = AdvancedCustomConverterNone
		}

		if route.IncomingPath == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].incoming_path is required", i)
		}
		if !strings.HasPrefix(route.IncomingPath, "/") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].incoming_path must start with /", i)
		}
		if strings.Contains(route.IncomingPath, "?") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].incoming_path must not include query", i)
		}
		if _, exists := seenPaths[route.IncomingPath]; exists {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].incoming_path must be unique: %s", i, route.IncomingPath)
		}
		seenPaths[route.IncomingPath] = struct{}{}

		if upstreamPath == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path is required", i)
		}
		if err := validateAdvancedCustomUpstreamTarget(i, upstreamPath); err != nil {
			return err
		}

		if !IsAdvancedCustomConverterAllowed(route.Converter) {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter is not registered: %s", i, route.Converter)
		}
		if err := validateAdvancedCustomConverterPath(i, route.IncomingPath, route.Converter); err != nil {
			return err
		}
		if err := validateAdvancedCustomRouteAuth(i, route.Auth); err != nil {
			return err
		}
		if err := validateAdvancedCustomConverterOptions(i, route.IncomingPath, route.Converter, route.ConverterOptions); err != nil {
			return err
		}
	}

	return nil
}

func validateAdvancedCustomModelFetchURL(index int, fetchURL string) error {
	if strings.HasPrefix(fetchURL, "/") {
		if strings.HasPrefix(fetchURL, "//") {
			return fmt.Errorf("advanced_custom.model_fetch_urls[%d] must be a full URL or a path starting with /", index)
		}
		return nil
	}

	parsedURL, err := url.Parse(fetchURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("advanced_custom.model_fetch_urls[%d] must be a full URL or a path starting with /", index)
	}
	if !strings.EqualFold(parsedURL.Scheme, "http") && !strings.EqualFold(parsedURL.Scheme, "https") {
		return fmt.Errorf("advanced_custom.model_fetch_urls[%d] must use http or https", index)
	}
	return nil
}

func validateAdvancedCustomUpstreamTarget(index int, upstreamPath string) error {
	if strings.HasPrefix(upstreamPath, "/") {
		if strings.HasPrefix(upstreamPath, "//") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path must be a full URL or a path starting with /", index)
		}
		return nil
	}

	parsedURL, err := url.Parse(upstreamPath)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path must be a full URL or a path starting with /", index)
	}
	if !strings.EqualFold(parsedURL.Scheme, "http") && !strings.EqualFold(parsedURL.Scheme, "https") {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path must use http or https", index)
	}
	return nil
}

func validateAdvancedCustomConverterPath(index int, incomingPath string, converter string) error {
	switch converter {
	case AdvancedCustomConverterNone:
		return nil
	case AdvancedCustomConverterAnthropicMessagesToOpenAIChatCompletions:
		if incomingPath == "/v1/messages" {
			return nil
		}
	case AdvancedCustomConverterOpenAIChatCompletionsToAnthropicMessages,
		AdvancedCustomConverterOpenAIChatCompletionsToOpenAIResponses,
		AdvancedCustomConverterOpenAIChatCompletionsToGeminiGenerateContent:
		if incomingPath == "/v1/chat/completions" {
			return nil
		}
	case AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions:
		if incomingPath == "/v1/responses" {
			return nil
		}
	case AdvancedCustomConverterGeminiGenerateContentToOpenAIChatCompletions:
		if strings.Contains(incomingPath, ":generateContent") || strings.Contains(incomingPath, ":streamGenerateContent") {
			return nil
		}
	}
	return fmt.Errorf("advanced_custom.advanced_routes[%d].converter does not match incoming_path: %s", index, converter)
}

func validateAdvancedCustomRouteAuth(index int, auth *AdvancedCustomRouteAuth) error {
	if auth == nil {
		return nil
	}
	authType := strings.TrimSpace(auth.Type)
	switch authType {
	case AdvancedCustomAuthTypeNone:
		return nil
	case AdvancedCustomAuthTypeHeader, AdvancedCustomAuthTypeQuery:
		if strings.TrimSpace(auth.Name) == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].auth.name is required", index)
		}
		if strings.TrimSpace(auth.Value) == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].auth.value is required", index)
		}
		return nil
	default:
		return fmt.Errorf("advanced_custom.advanced_routes[%d].auth.type is invalid: %s", index, auth.Type)
	}
}

var allowedAdvancedCustomResponsesDropFields = map[string]struct{}{
	"stream_options":         {},
	"top_logprobs":           {},
	"store":                  {},
	"metadata":               {},
	"safety_identifier":      {},
	"prompt_cache_key":       {},
	"prompt_cache_retention": {},
	"service_tier":           {},
	"parallel_tool_calls":    {},
	"reasoning":              {},
}

func validateAdvancedCustomConverterOptions(index int, incomingPath string, converter string, options *AdvancedCustomConverterOptions) error {
	if options == nil {
		return nil
	}
	if !advancedCustomConverterOptionsPresent(options) {
		return nil
	}
	if converter != AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions && converter != AdvancedCustomConverterNone {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options is only supported by Responses routes", index)
	}
	if converter == AdvancedCustomConverterNone && incomingPath != "/v1/responses" {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options is only supported by Responses routes", index)
	}

	mode := strings.TrimSpace(options.ResponsesToolsMode)
	if mode != "" {
		if converter == AdvancedCustomConverterNone {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tools_mode is not supported by converter none", index)
		}
		switch mode {
		case AdvancedCustomResponsesToolsModeCompatFlatten, AdvancedCustomResponsesToolsModePreserve:
		default:
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tools_mode is invalid: %s", index, mode)
		}
	}

	allowFlatten := converter == AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions
	if options.ResponsesTools != nil {
		if converter == AdvancedCustomConverterNone && strings.TrimSpace(options.ResponsesTools.Namespace) == AdvancedCustomResponsesToolPolicyFlatten {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options namespace flatten is not supported by converter none", index)
		}
		if err := validateAdvancedCustomResponsesToolPolicy(index, "namespace", options.ResponsesTools.Namespace, allowFlatten); err != nil {
			return err
		}
		if err := validateAdvancedCustomResponsesToolPolicy(index, "custom", options.ResponsesTools.Custom, allowFlatten); err != nil {
			return err
		}
		if err := validateAdvancedCustomResponsesToolPolicy(index, "web_search", options.ResponsesTools.WebSearch, false); err != nil {
			return err
		}
		if converter == AdvancedCustomConverterNone && strings.TrimSpace(options.ResponsesTools.ToolSearch) == AdvancedCustomResponsesToolPolicyFlatten {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options tool_search flatten is not supported by converter none", index)
		}
		if err := validateAdvancedCustomResponsesToolPolicy(index, "tool_search", options.ResponsesTools.ToolSearch, allowFlatten); err != nil {
			return err
		}
		if err := validateAdvancedCustomResponsesToolPolicy(index, "image_generation", options.ResponsesTools.ImageGeneration, false); err != nil {
			return err
		}
		if err := validateAdvancedCustomResponsesToolPolicy(index, "unknown", options.ResponsesTools.Unknown, false); err != nil {
			return err
		}
	}
	if len(options.ResponsesDropFields) > 0 {
		if converter == AdvancedCustomConverterNone {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_drop_fields is not supported by converter none", index)
		}
		seen := make(map[string]struct{}, len(options.ResponsesDropFields))
		for _, raw := range options.ResponsesDropFields {
			field := strings.ToLower(strings.TrimSpace(raw))
			if field == "" {
				continue
			}
			if _, ok := allowedAdvancedCustomResponsesDropFields[field]; !ok {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_drop_fields contains unsupported field: %s", index, raw)
			}
			seen[field] = struct{}{}
		}
		if len(seen) == 0 {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_drop_fields must not be empty", index)
		}
	}
	if err := validateAdvancedCustomResponsesToolConflictPolicy(index, options.ResponsesToolConflictPolicy); err != nil {
		return err
	}
	if err := validateAdvancedCustomResponsesImplicitHostedTools(
		index,
		"converter_options.responses_implicit_hosted_tools",
		options.ResponsesImplicitHostedTools,
	); err != nil {
		return err
	}
	if err := validateAdvancedCustomResponsesImplicitHostedConflictPolicy(index, options); err != nil {
		return err
	}
	if options.ResponsesToolParameters != nil || advancedCustomResponsesModelOverridesHaveToolParameters(options.ResponsesToolModelOverrides) {
		if converter != AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_parameters requires Responses to Chat conversion", index)
		}
	}
	if options.ResponsesToolChoice != nil || advancedCustomResponsesModelOverridesHaveToolChoice(options.ResponsesToolModelOverrides) {
		if converter != AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_choice requires Responses to Chat conversion", index)
		}
	}
	if options.ResponsesToolContinuation != nil || advancedCustomResponsesModelOverridesHaveToolContinuation(options.ResponsesToolModelOverrides) {
		if converter != AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_continuation requires Responses to Chat conversion", index)
		}
	}
	if options.ResponsesToolStateReplay != nil || advancedCustomResponsesModelOverridesHaveToolStateReplay(options.ResponsesToolModelOverrides) {
		if converter != AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tool_state_replay requires Responses to Chat conversion", index)
		}
	}
	if err := validateAdvancedCustomResponsesToolParameters(index, "converter_options.responses_tool_parameters", options.ResponsesToolParameters); err != nil {
		return err
	}
	if err := validateAdvancedCustomResponsesToolChoice(index, "converter_options.responses_tool_choice", options.ResponsesToolChoice); err != nil {
		return err
	}
	if err := validateAdvancedCustomResponsesToolContinuation(index, "converter_options.responses_tool_continuation", options.ResponsesToolContinuation); err != nil {
		return err
	}
	if err := validateAdvancedCustomResponsesToolStateReplay(index, "converter_options.responses_tool_state_replay", options.ResponsesToolStateReplay); err != nil {
		return err
	}
	if err := validateAdvancedCustomResponsesToolModelOverrides(index, allowFlatten, options); err != nil {
		return err
	}
	return nil
}

func advancedCustomConverterOptionsPresent(options *AdvancedCustomConverterOptions) bool {
	if options == nil {
		return false
	}
	return strings.TrimSpace(options.ResponsesToolsMode) != "" ||
		options.ResponsesTools != nil ||
		len(options.ResponsesDropFields) > 0 ||
		strings.TrimSpace(options.ResponsesToolConflictPolicy) != "" ||
		len(options.ResponsesImplicitHostedTools) > 0 ||
		options.ResponsesToolParameters != nil ||
		options.ResponsesToolChoice != nil ||
		options.ResponsesToolContinuation != nil ||
		options.ResponsesToolStateReplay != nil ||
		len(options.ResponsesToolModelOverrides) > 0
}

func advancedCustomResponsesModelOverridesHaveToolParameters(overrides []AdvancedCustomResponsesToolModelOverride) bool {
	for _, override := range overrides {
		if override.ResponsesToolParameters != nil {
			return true
		}
	}
	return false
}

func advancedCustomResponsesModelOverridesHaveToolChoice(overrides []AdvancedCustomResponsesToolModelOverride) bool {
	for _, override := range overrides {
		if override.ResponsesToolChoice != nil {
			return true
		}
	}
	return false
}

func advancedCustomResponsesModelOverridesHaveToolContinuation(overrides []AdvancedCustomResponsesToolModelOverride) bool {
	for _, override := range overrides {
		if override.ResponsesToolContinuation != nil {
			return true
		}
	}
	return false
}

func advancedCustomResponsesModelOverridesHaveToolStateReplay(overrides []AdvancedCustomResponsesToolModelOverride) bool {
	for _, override := range overrides {
		if override.ResponsesToolStateReplay != nil {
			return true
		}
	}
	return false
}

func validateAdvancedCustomResponsesToolPolicy(index int, toolType string, policy string, allowFlatten bool) error {
	policy = strings.TrimSpace(policy)
	if policy == "" {
		return nil
	}
	switch policy {
	case AdvancedCustomResponsesToolPolicyPreserve, AdvancedCustomResponsesToolPolicyDrop, AdvancedCustomResponsesToolPolicyReject:
		return nil
	case AdvancedCustomResponsesToolPolicyFlatten:
		if allowFlatten {
			return nil
		}
	}
	return fmt.Errorf("advanced_custom.advanced_routes[%d].converter_options.responses_tools.%s is invalid: %s", index, toolType, policy)
}
