package advancedcustom

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

const ChannelName = "advanced_custom"

const advancedCustomModelPlaceholder = "{model}"

type Adaptor struct {
	openaiAdaptor openai.Adaptor
	claudeAdaptor claude.Adaptor
	geminiAdaptor gemini.Adaptor

	resolved  bool
	converted bool
	route     dto.AdvancedCustomRoute
	converter string

	compatibilityRequestedModel string
	compatibilityUpstreamModel  string
	compatibilityTools          []relayconvert.ResponsesToolPolicyDecision
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.openaiAdaptor.Init(info)
	a.claudeAdaptor.Init(info)
	a.geminiAdaptor.Init(info)
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter == relayconvert.ConverterNone {
		return a.convertOpenAICompatibleRequest(c, info, request)
	}

	switch converter {
	case relayconvert.ConverterOpenAIChatToClaudeMessages,
		relayconvert.ConverterOpenAIChatToOpenAIResponses,
		relayconvert.ConverterOpenAIChatToGeminiContent:
		result, err := service.ConvertRequestByID(c, info, converter, request)
		if err != nil {
			return nil, err
		}
		return result.Value, nil
	default:
		return nil, fmt.Errorf("converter %q does not support OpenAI chat completions requests", converter)
	}
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}

	switch converter {
	case relayconvert.ConverterNone:
		return a.claudeAdaptor.ConvertClaudeRequest(c, info, request)
	case relayconvert.ConverterClaudeMessagesToOpenAIChat:
		result, err := service.ConvertRequestByID(c, info, converter, request)
		if err != nil {
			return nil, err
		}
		chatRequest, ok := result.Value.(*dto.GeneralOpenAIRequest)
		if !ok {
			return nil, fmt.Errorf("expected OpenAI chat completions request, got %T", result.Value)
		}
		return a.convertOpenAICompatibleRequest(c, info, chatRequest)
	default:
		return nil, fmt.Errorf("converter %q does not support Anthropic Messages requests", converter)
	}
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}

	switch converter {
	case relayconvert.ConverterNone:
		return a.geminiAdaptor.ConvertGeminiRequest(c, info, request)
	case relayconvert.ConverterGeminiContentToOpenAIChat:
		result, err := service.ConvertRequestByID(c, info, converter, request)
		if err != nil {
			return nil, err
		}
		chatRequest, ok := result.Value.(*dto.GeneralOpenAIRequest)
		if !ok {
			return nil, fmt.Errorf("expected OpenAI chat completions request, got %T", result.Value)
		}
		return a.convertOpenAICompatibleRequest(c, info, chatRequest)
	default:
		return nil, fmt.Errorf("converter %q does not support Gemini generateContent requests", converter)
	}
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	requestedModel, upstreamModel := advancedCustomResponsesModelNames(info, request.Model)
	policyResolver := func(toolType string, toolName string) string {
		return dto.ResolveAdvancedCustomResponsesToolPolicy(
			a.route.ConverterOptions,
			requestedModel,
			upstreamModel,
			toolType,
			toolName,
		).Policy
	}
	filteredTools, policyDecisions, err := relayconvert.ApplyResponsesToolPolicies(request.Tools, policyResolver)
	if err != nil {
		recordAdvancedCustomToolCompatibilityEvents(info, a.route, requestedModel, upstreamModel, policyDecisions, err)
		return nil, advancedCustomResponsesToolPolicyError(info, a.route, requestedModel, upstreamModel, policyDecisions, err)
	}
	filteredTools, conflictDecisions, err := relayconvert.ApplyResponsesToolConflictPolicyWithImplicitHostedTools(
		filteredTools,
		dto.ResolveAdvancedCustomResponsesToolConflictPolicy(a.route.ConverterOptions),
		dto.ResolveAdvancedCustomResponsesImplicitHostedTools(
			a.route.ConverterOptions,
			requestedModel,
			upstreamModel,
		),
	)
	if err != nil {
		recordAdvancedCustomToolCompatibilityEvents(info, a.route, requestedModel, upstreamModel, conflictDecisions, err)
		return nil, advancedCustomResponsesToolPolicyError(info, a.route, requestedModel, upstreamModel, conflictDecisions, err)
	}
	decisions := append(policyDecisions, conflictDecisions...)
	if err := relayconvert.ValidateResponsesToolChoiceAfterPolicy(request.ToolChoice, decisions); err != nil {
		recordAdvancedCustomToolCompatibilityEvents(info, a.route, requestedModel, upstreamModel, decisions, err)
		return nil, advancedCustomResponsesToolPolicyError(info, a.route, requestedModel, upstreamModel, decisions, err)
	}
	recordAdvancedCustomToolCompatibilityEvents(info, a.route, requestedModel, upstreamModel, decisions, nil)
	a.compatibilityRequestedModel = requestedModel
	a.compatibilityUpstreamModel = upstreamModel
	a.compatibilityTools = summarizeAdvancedCustomResponsesTools(filteredTools)
	request.Tools = filteredTools
	switch converter {
	case relayconvert.ConverterNone:
		return a.convertOpenAICompatibleResponsesRequest(c, info, request)
	case relayconvert.ConverterOpenAIResponsesToOpenAIChat:
		mappings := map[string]dto.ResponsesToolNameMapping{}
		chatOptions := relayconvert.ResponsesRequestToChatOptions{
			ToolPolicies:       advancedCustomResponsesToolPolicies(a.route.ConverterOptions, requestedModel, upstreamModel),
			ToolNameMappings:   mappings,
			DropResponseFields: advancedCustomResponsesDropFields(a.route.ConverterOptions),
		}
		chatReq, err := service.ResponsesRequestToChatCompletionsRequestWithOptions(&request, chatOptions)
		if err != nil {
			if isAdvancedCustomToolConversionError(err) {
				recordAdvancedCustomToolCompatibilityEvents(info, a.route, requestedModel, upstreamModel, summarizeAdvancedCustomResponsesTools(filteredTools), err)
			}
			return nil, err
		}
		if len(mappings) > 0 {
			info.ResponsesToolNameMappings = mappings
		}
		return a.convertOpenAICompatibleRequest(c, info, chatReq)
	case relayconvert.ConverterOpenAIResponsesToGemini:
		result, err := service.ConvertRequestByID(c, info, converter, request)
		if err != nil {
			return nil, err
		}
		geminiRequest, ok := result.Value.(*dto.GeminiChatRequest)
		if !ok {
			return nil, fmt.Errorf("expected Gemini generateContent request, got %T", result.Value)
		}
		return geminiRequest, nil
	default:
		return nil, fmt.Errorf("converter %q does not support OpenAI Responses requests", converter)
	}
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter != relayconvert.ConverterNone {
		return nil, fmt.Errorf("converter %q does not support embedding requests", converter)
	}
	return a.convertOpenAICompatibleEmbeddingRequest(c, info, request)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter != relayconvert.ConverterNone {
		return nil, fmt.Errorf("converter %q does not support audio requests", converter)
	}
	return a.convertOpenAICompatibleAudioRequest(c, info, request)
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter != relayconvert.ConverterNone {
		return nil, fmt.Errorf("converter %q does not support image requests", converter)
	}
	return a.convertOpenAICompatibleImageRequest(c, info, request)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	a.converted = true
	return a.openaiAdaptor.ConvertRerankRequest(c, relayMode, request)
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if err := a.resolve(nil, info); err != nil {
		return "", err
	}
	return a.routeURL(info)
}

func (a *Adaptor) BuildModelListRequest(info *relaycommon.RelayInfo) (string, http.Header, error) {
	if info == nil {
		return "", nil, errors.New("missing relay info")
	}
	config := info.ChannelOtherSettings.AdvancedCustom
	if config == nil {
		return "", nil, errors.New("advanced_custom is required")
	}
	if err := config.Validate(); err != nil {
		return "", nil, err
	}
	route, ok := config.ModelListRoute()
	if !ok {
		return "", nil, errors.New("advanced custom channel does not configure a /v1/models route")
	}
	converter := strings.TrimSpace(route.Converter)
	if converter == "" {
		converter = relayconvert.ConverterNone
	}
	if converter != relayconvert.ConverterNone {
		return "", nil, fmt.Errorf("converter %q does not support model list requests", converter)
	}

	requestURL, err := buildRouteURL(route, converter, info)
	if err != nil {
		return "", nil, err
	}

	header := http.Header{}
	auth := route.Auth
	if auth == nil {
		header.Set("Authorization", "Bearer "+info.ApiKey)
		return requestURL, header, nil
	}

	switch strings.TrimSpace(auth.Type) {
	case dto.AdvancedCustomAuthTypeNone, dto.AdvancedCustomAuthTypeQuery:
	case dto.AdvancedCustomAuthTypeHeader:
		header.Set(strings.TrimSpace(auth.Name), applyAuthTemplate(auth.Value, info.ApiKey))
	default:
		return "", nil, fmt.Errorf("invalid advanced custom auth type: %s", auth.Type)
	}
	return requestURL, header, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	if err := a.resolve(c, info); err != nil {
		return err
	}

	channel.SetupApiRequestHeader(info, c, header)
	auth := a.route.Auth
	if auth == nil {
		header.Set("Authorization", "Bearer "+info.ApiKey)
	} else {
		switch strings.TrimSpace(auth.Type) {
		case dto.AdvancedCustomAuthTypeNone:
		case dto.AdvancedCustomAuthTypeHeader:
			header.Set(strings.TrimSpace(auth.Name), applyAuthTemplate(auth.Value, info.ApiKey))
		case dto.AdvancedCustomAuthTypeQuery:
		default:
			return fmt.Errorf("invalid advanced custom auth type: %s", auth.Type)
		}
	}

	if shouldApplyClaudeHeaders(a.converter, info) {
		applyClaudeHeaders(c, header, info)
	}

	return nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if err := a.resolve(c, info); err != nil {
		return nil, err
	}
	if !a.converted && a.converter != relayconvert.ConverterNone {
		return nil, errors.New("advanced custom converter routes cannot be used with pass-through request body")
	}

	if info.RelayMode == relayconstant.RelayModeAudioTranscription ||
		info.RelayMode == relayconstant.RelayModeAudioTranslation ||
		(info.RelayMode == relayconstant.RelayModeImagesEdits && !isJSONRequest(c)) {
		return channel.DoFormRequest(a, c, info, requestBody)
	}
	if info.RelayMode == relayconstant.RelayModeRealtime {
		return channel.DoWssRequest(a, c, info, requestBody)
	}
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if resolveErr := a.resolve(c, info); resolveErr != nil {
		return nil, types.NewOpenAIError(resolveErr, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	switch a.converter {
	case relayconvert.ConverterNone:
		usage, err = a.doNativeResponse(c, resp, info)
	case relayconvert.ConverterClaudeMessagesToOpenAIChat,
		relayconvert.ConverterGeminiContentToOpenAIChat:
		usage, err = a.openaiAdaptor.DoResponse(c, resp, info)
	case relayconvert.ConverterOpenAIChatToClaudeMessages:
		usage, err = a.claudeAdaptor.DoResponse(c, resp, info)
	case relayconvert.ConverterOpenAIChatToGeminiContent,
		relayconvert.ConverterOpenAIResponsesToGemini:
		usage, err = a.geminiAdaptor.DoResponse(c, resp, info)
	case relayconvert.ConverterOpenAIChatToOpenAIResponses:
		if info.IsStream {
			usage, err = openai.OaiResponsesToChatStreamHandler(c, info, resp)
		} else {
			usage, err = openai.OaiResponsesToChatHandler(c, info, resp)
		}
	case relayconvert.ConverterOpenAIResponsesToOpenAIChat:
		if info.IsStream {
			usage, err = openai.OaiChatToResponsesStreamHandler(c, info, resp)
		} else {
			usage, err = openai.OaiChatToResponsesHandler(c, info, resp)
		}
	default:
		err = types.NewOpenAIError(fmt.Errorf("unsupported advanced custom converter: %s", a.converter), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if err != nil {
		recordAdvancedCustomUpstreamToolCompatibilityEvents(info, a.route, a.compatibilityRequestedModel, a.compatibilityUpstreamModel, a.compatibilityTools, err)
	}
	return usage, err
}

func (a *Adaptor) GetModelList() []string {
	models := make([]string, 0, len(openai.ModelList)+len(claude.ModelList)+len(gemini.ModelList))
	models = append(models, openai.ModelList...)
	models = append(models, claude.ModelList...)
	models = append(models, gemini.ModelList...)
	return lo.Uniq(models)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) doNativeResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		return a.claudeAdaptor.DoResponse(c, resp, info)
	case types.RelayFormatGemini:
		return a.geminiAdaptor.DoResponse(c, resp, info)
	default:
		return a.openaiAdaptor.DoResponse(c, resp, info)
	}
}

func (a *Adaptor) resolveForConversion(c *gin.Context, info *relaycommon.RelayInfo) (string, error) {
	if err := a.resolve(c, info); err != nil {
		return "", err
	}
	a.converted = true
	return a.converter, nil
}

func (a *Adaptor) resolve(c *gin.Context, info *relaycommon.RelayInfo) error {
	if a.resolved {
		return nil
	}
	if info == nil {
		return errors.New("missing relay info")
	}
	config := info.ChannelOtherSettings.AdvancedCustom
	if config == nil {
		return errors.New("advanced_custom is required")
	}
	if err := config.Validate(); err != nil {
		return err
	}

	incomingPath := incomingRequestPath(c, info)
	route, ok := config.MatchPathForModel(incomingPath, info.OriginModelName)
	if ok {
		route.Converter = strings.TrimSpace(route.Converter)
		if route.Converter == "" {
			route.Converter = relayconvert.ConverterNone
		}
		a.route = route
		a.converter = route.Converter
		a.resolved = true
		return nil
	}
	return fmt.Errorf("advanced custom channel does not support request path %s for model %s", incomingPath, info.OriginModelName)
}

func incomingRequestPath(c *gin.Context, info *relaycommon.RelayInfo) string {
	if c != nil && c.Request != nil && c.Request.URL != nil {
		return c.Request.URL.Path
	}
	if info == nil {
		return ""
	}
	return strings.Split(info.RequestURLPath, "?")[0]
}

func (a *Adaptor) routeURL(info *relaycommon.RelayInfo) (string, error) {
	return buildRouteURL(a.route, a.converter, info)
}

func buildRouteURL(route dto.AdvancedCustomRoute, converter string, info *relaycommon.RelayInfo) (string, error) {
	parsedURL, err := resolveUpstreamTargetURL(applyUpstreamPathTemplate(strings.TrimSpace(route.UpstreamPath), info), info)
	if err != nil {
		return "", err
	}
	if shouldUseGeminiStreamURL(converter, info) {
		useGeminiStreamGenerateContentURL(parsedURL)
	}
	if info != nil && info.RelayMode == relayconstant.RelayModeRealtime {
		switch parsedURL.Scheme {
		case "https":
			parsedURL.Scheme = "wss"
		case "http":
			parsedURL.Scheme = "ws"
		}
	}
	if route.Auth != nil && strings.TrimSpace(route.Auth.Type) == dto.AdvancedCustomAuthTypeQuery {
		query := parsedURL.Query()
		query.Set(strings.TrimSpace(route.Auth.Name), applyAuthTemplate(route.Auth.Value, info.ApiKey))
		parsedURL.RawQuery = query.Encode()
	}
	return parsedURL.String(), nil
}

func resolveUpstreamTargetURL(upstreamPath string, info *relaycommon.RelayInfo) (*url.URL, error) {
	if strings.HasPrefix(upstreamPath, "/") {
		if strings.HasPrefix(upstreamPath, "//") {
			return nil, errors.New("advanced custom upstream path must be a full URL or a path starting with /")
		}
		if info == nil || strings.TrimSpace(info.ChannelBaseUrl) == "" {
			return nil, errors.New("channel base URL is required when advanced custom upstream path is relative")
		}
		return joinBaseURLAndUpstreamPath(info.ChannelBaseUrl, upstreamPath)
	}

	parsedURL, err := url.Parse(upstreamPath)
	if err != nil {
		return nil, err
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, errors.New("advanced custom upstream path must be a full URL or a path starting with /")
	}
	if !strings.EqualFold(parsedURL.Scheme, "http") && !strings.EqualFold(parsedURL.Scheme, "https") {
		return nil, errors.New("advanced custom upstream path must use http or https")
	}
	return parsedURL, nil
}

func joinBaseURLAndUpstreamPath(baseURL string, upstreamPath string) (*url.URL, error) {
	parsedBaseURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, err
	}
	if parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return nil, errors.New("channel base URL must be a full URL when advanced custom upstream path is relative")
	}
	if !strings.EqualFold(parsedBaseURL.Scheme, "http") && !strings.EqualFold(parsedBaseURL.Scheme, "https") {
		return nil, errors.New("channel base URL must use http or https when advanced custom upstream path is relative")
	}

	parsedPath, err := url.Parse(upstreamPath)
	if err != nil {
		return nil, err
	}
	parsedBaseURL.Path = strings.TrimRight(parsedBaseURL.Path, "/") + "/" + strings.TrimLeft(parsedPath.Path, "/")
	parsedBaseURL.RawPath = ""
	parsedBaseURL.RawQuery = parsedPath.RawQuery
	parsedBaseURL.Fragment = parsedPath.Fragment
	return parsedBaseURL, nil
}

func applyUpstreamPathTemplate(upstreamPath string, info *relaycommon.RelayInfo) string {
	if info == nil {
		return upstreamPath
	}
	return strings.ReplaceAll(upstreamPath, advancedCustomModelPlaceholder, info.UpstreamModelName)
}

func shouldUseGeminiStreamURL(converter string, info *relaycommon.RelayInfo) bool {
	return info != nil &&
		info.IsStream &&
		(converter == relayconvert.ConverterOpenAIChatToGeminiContent ||
			converter == relayconvert.ConverterOpenAIResponsesToGemini)
}

func useGeminiStreamGenerateContentURL(parsedURL *url.URL) {
	if strings.Contains(parsedURL.Path, ":generateContent") {
		parsedURL.Path = strings.Replace(parsedURL.Path, ":generateContent", ":streamGenerateContent", 1)
	}
	if strings.Contains(parsedURL.Path, ":streamGenerateContent") {
		query := parsedURL.Query()
		query.Set("alt", "sse")
		parsedURL.RawQuery = query.Encode()
	}
}

func shouldApplyClaudeHeaders(converter string, info *relaycommon.RelayInfo) bool {
	return converter == relayconvert.ConverterOpenAIChatToClaudeMessages ||
		(converter == relayconvert.ConverterNone && info != nil && info.RelayFormat == types.RelayFormatClaude)
}

func applyClaudeHeaders(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) {
	anthropicVersion := ""
	if c != nil && c.Request != nil {
		anthropicVersion = c.Request.Header.Get("anthropic-version")
	}
	if anthropicVersion == "" {
		anthropicVersion = "2023-06-01"
	}
	header.Set("anthropic-version", anthropicVersion)
	if c != nil {
		claude.CommonClaudeHeadersOperation(c, header, info)
	}
}

func applyAuthTemplate(template string, apiKey string) string {
	return strings.ReplaceAll(template, "{api_key}", apiKey)
}

func isJSONRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	return strings.Contains(strings.ToLower(c.Request.Header.Get("Content-Type")), "application/json")
}

func (a *Adaptor) convertOpenAICompatibleRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	advancedCustomPopulateChatWebSearchOptions(a.route.ConverterOptions, info, request)

	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertOpenAIRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func advancedCustomPopulateChatWebSearchOptions(options *dto.AdvancedCustomConverterOptions, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) {
	if request == nil {
		return
	}
	requestedModel, upstreamModel := advancedCustomResponsesModelNames(info, request.Model)
	compatibility := dto.ResolveAdvancedCustomResponsesWebSearchParameters(options, requestedModel, upstreamModel)
	defaults := map[string]any(nil)
	if compatibility != nil {
		if strings.TrimSpace(compatibility.WhenNestedOptionsMissing) != dto.AdvancedCustomResponsesToolMissingOptionsPopulateDefaults {
			return
		}
		defaults = make(map[string]any, 3)
		if compatibility.Defaults.Enable != nil {
			defaults["enable"] = *compatibility.Defaults.Enable
		}
		if compatibility.Defaults.SearchResult != nil {
			defaults["search_result"] = *compatibility.Defaults.SearchResult
		}
		if searchEngine := strings.TrimSpace(compatibility.Defaults.SearchEngine); searchEngine != "" {
			defaults["search_engine"] = searchEngine
		}
		if len(defaults) == 0 {
			return
		}
	} else if advancedCustomUsesGLMChatWebSearchSchema(info, request.Model) {
		defaults = map[string]any{"enable": true, "search_result": true}
	} else {
		return
	}
	for i := range request.Tools {
		toolType := strings.TrimSpace(request.Tools[i].Type)
		if toolType != "web_search" && toolType != "web_search_preview" {
			continue
		}
		if len(request.Tools[i].WebSearch) > 0 {
			continue
		}
		request.Tools[i].Type = "web_search"
		request.Tools[i].WebSearch = maps.Clone(defaults)
	}
}

func advancedCustomUsesGLMChatWebSearchSchema(info *relaycommon.RelayInfo, requestModel string) bool {
	models := []string{requestModel}
	if info != nil {
		models = append(models, info.OriginModelName)
		if info.ChannelMeta != nil {
			models = append(models, info.UpstreamModelName)
		}
	}
	for _, modelName := range models {
		normalized := strings.ToLower(strings.TrimSpace(modelName))
		if strings.HasPrefix(normalized, "glm-") {
			return true
		}
	}
	return false
}

func (a *Adaptor) convertClaudeToOpenAICompatibleRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertClaudeRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertGeminiToOpenAICompatibleRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertGeminiRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}
func (a *Adaptor) convertOpenAICompatibleResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertOpenAIResponsesRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertOpenAICompatibleEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertEmbeddingRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertOpenAICompatibleAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertAudioRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertOpenAICompatibleImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertImageRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func summarizeAdvancedCustomResponsesTools(raw json.RawMessage) []relayconvert.ResponsesToolPolicyDecision {
	var tools []map[string]any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil
	}
	out := make([]relayconvert.ResponsesToolPolicyDecision, 0, len(tools))
	for _, tool := range tools {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		if toolType == "" {
			continue
		}
		out = append(out, relayconvert.ResponsesToolPolicyDecision{ToolType: toolType, ToolName: strings.TrimSpace(common.Interface2String(tool["name"]))})
	}
	return out
}

func recordAdvancedCustomUpstreamToolCompatibilityEvents(info *relaycommon.RelayInfo, route dto.AdvancedCustomRoute, requestedModel string, upstreamModel string, tools []relayconvert.ResponsesToolPolicyDecision, cause *types.NewAPIError) {
	channelID := 0
	if info != nil {
		channelID = info.ChannelId
	}
	if channelID <= 0 || len(tools) == 0 || cause == nil {
		return
	}
	message := cause.ErrorWithStatusCode()
	matchedTools := make([]relayconvert.ResponsesToolPolicyDecision, 0, len(tools))
	for _, tool := range tools {
		if advancedCustomUpstreamErrorMentionsTool(message, tool.ToolType, tool.ToolName) {
			matchedTools = append(matchedTools, tool)
		}
	}
	if len(matchedTools) == 0 {
		matchedTools = []relayconvert.ResponsesToolPolicyDecision{{}}
	}
	for _, tool := range matchedTools {
		eventType, suggestion := classifyAdvancedCustomUpstreamToolError(cause.StatusCode, message, tool.ToolType)
		if eventType == "" {
			continue
		}
		if _, err := service.RecordToolCompatibilityEvent(service.ToolCompatibilityEventInput{
			ChannelId: channelID, Route: route.IncomingPath, RequestedModel: requestedModel, UpstreamModel: upstreamModel,
			ToolType: tool.ToolType, ToolName: tool.ToolName, EventType: eventType, SuggestedPolicy: suggestion, ErrorMessage: message,
		}); err != nil {
			common.SysError("record upstream tool compatibility event failed: " + err.Error())
		}
	}
}

func classifyAdvancedCustomUpstreamToolError(statusCode int, message string, toolType string) (string, string) {
	if statusCode < http.StatusBadRequest {
		return "", ""
	}
	lower := strings.ToLower(message)
	normalizedType := strings.ToLower(strings.TrimSpace(toolType))
	// Only explicit unsupported-tool wording may recommend a policy change. A
	// generic 400 remains evidence for review, never an automatic Drop proposal.
	if strings.Contains(lower, "unsupported tool type") || strings.Contains(lower, "tool type is not supported") || strings.Contains(lower, "does not support tool") {
		if normalizedType == "" || normalizedType == "function" {
			return model.ToolCompatibilityEventTypeUpstreamUnsupported, ""
		}
		return model.ToolCompatibilityEventTypeUpstreamUnsupported, dto.AdvancedCustomResponsesToolPolicyDrop
	}
	return model.ToolCompatibilityEventTypeUnclassified, ""
}

func advancedCustomUpstreamErrorMentionsTool(message string, toolType string, toolName string) bool {
	lower := strings.ToLower(message)
	if name := strings.ToLower(strings.TrimSpace(toolName)); name != "" && strings.Contains(lower, name) {
		return true
	}
	for _, alias := range advancedCustomToolTypeAliases(toolType) {
		if strings.Contains(lower, alias) {
			return true
		}
	}
	return false
}

func advancedCustomToolTypeAliases(toolType string) []string {
	switch strings.ToLower(strings.TrimSpace(toolType)) {
	case "web_search", "web_search_preview":
		return []string{"web_search", "web_search_preview"}
	case "image_gen", "image_generation":
		return []string{"image_gen", "image_generation"}
	case "tool_search":
		return []string{"tool_search"}
	case "namespace":
		return []string{"namespace"}
	case "custom":
		return []string{"custom"}
	case "function":
		return []string{"function"}
	default:
		if normalized := strings.ToLower(strings.TrimSpace(toolType)); normalized != "" {
			return []string{normalized}
		}
		return nil
	}
}

func isAdvancedCustomToolConversionError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "tool") || strings.Contains(message, "namespace")
}

func recordAdvancedCustomToolCompatibilityEvents(
	info *relaycommon.RelayInfo,
	route dto.AdvancedCustomRoute,
	requestedModel string,
	upstreamModel string,
	decisions []relayconvert.ResponsesToolPolicyDecision,
	cause error,
) {
	channelID := 0
	if info != nil {
		channelID = info.ChannelId
	}
	if channelID <= 0 || strings.TrimSpace(route.IncomingPath) == "" {
		return
	}
	if len(decisions) == 0 && cause != nil {
		decisions = []relayconvert.ResponsesToolPolicyDecision{{}}
	}
	for _, decision := range decisions {
		eventType := model.ToolCompatibilityEventTypeInvalidToolSchema
		suggestedPolicy := ""
		switch decision.Policy {
		case dto.AdvancedCustomResponsesToolPolicyDrop:
			eventType = model.ToolCompatibilityEventTypePolicyDrop
		case dto.AdvancedCustomResponsesToolPolicyReject:
			eventType = model.ToolCompatibilityEventTypePolicyReject
		case dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate:
			eventType = model.ToolCompatibilityEventTypeNameConflict
			suggestedPolicy = dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate
		default:
			if cause == nil {
				continue
			}
		}
		_, err := service.RecordToolCompatibilityEvent(service.ToolCompatibilityEventInput{
			ChannelId: channelID, Route: route.IncomingPath,
			RequestedModel: requestedModel, UpstreamModel: upstreamModel,
			ToolType: decision.ToolType, ToolName: decision.ToolName,
			EventType: eventType, CurrentPolicy: decision.Policy,
			SuggestedPolicy: suggestedPolicy,
			ErrorMessage: func() string {
				if cause == nil {
					return ""
				}
				return cause.Error()
			}(),
		})
		if err != nil {
			common.SysError("record tool compatibility event failed: " + err.Error())
		}
	}
}

func advancedCustomResponsesToolPolicies(options *dto.AdvancedCustomConverterOptions, requestedModel string, upstreamModel string) relayconvert.ResponsesToolPolicies {
	resolve := func(toolType string) string {
		return dto.ResolveAdvancedCustomResponsesToolPolicy(options, requestedModel, upstreamModel, toolType, "").Policy
	}
	return relayconvert.ResponsesToolPolicies{
		Namespace:       resolve("namespace"),
		Custom:          resolve("custom"),
		WebSearch:       resolve("web_search"),
		ToolSearch:      resolve("tool_search"),
		ImageGeneration: resolve("image_generation"),
		Unknown:         resolve("unknown"),
	}
}

func advancedCustomResponsesModelNames(info *relaycommon.RelayInfo, requestModel string) (string, string) {
	requestedModel := strings.TrimSpace(info.OriginModelName)
	if requestedModel == "" {
		requestedModel = strings.TrimSpace(requestModel)
	}
	upstreamModel := strings.TrimSpace(requestModel)
	if info.ChannelMeta != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		upstreamModel = strings.TrimSpace(info.UpstreamModelName)
	}
	return requestedModel, upstreamModel
}

func advancedCustomResponsesToolPolicyError(
	info *relaycommon.RelayInfo,
	route dto.AdvancedCustomRoute,
	requestedModel string,
	upstreamModel string,
	decisions []relayconvert.ResponsesToolPolicyDecision,
	cause error,
) error {
	dropped := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		dropped = append(dropped, fmt.Sprintf("%s/%s=%s", decision.ToolType, decision.ToolName, decision.Policy))
	}
	channelID := 0
	if info.ChannelMeta != nil {
		channelID = info.ChannelId
	}
	return fmt.Errorf(
		"advanced custom Responses tool handling failed: channel_id=%d route=%s requested_model=%s upstream_model=%s decisions=[%s]; configure Channel > Advanced Custom > Tool Handling > Model Tool Capabilities: %w",
		channelID,
		route.IncomingPath,
		requestedModel,
		upstreamModel,
		strings.Join(dropped, ", "),
		cause,
	)
}

func advancedCustomResponsesDropFields(options *dto.AdvancedCustomConverterOptions) map[string]struct{} {
	if options == nil || len(options.ResponsesDropFields) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(options.ResponsesDropFields))
	for _, raw := range options.ResponsesDropFields {
		field := strings.ToLower(strings.TrimSpace(raw))
		if field == "" {
			continue
		}
		out[field] = struct{}{}
	}
	return out
}
