package dto

import (
	"regexp"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvancedCustomValidateResponsesToChatConverterPath(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
			},
		},
	}
	require.NoError(t, valid.Validate())

	validGemini := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
			},
		},
	}
	require.NoError(t, validGemini.Validate())

	tests := []struct {
		name         string
		incomingPath string
	}{
		{name: "chat completions", incomingPath: "/v1/chat/completions"},
		{name: "responses compact", incomingPath: "/v1/responses/compact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdvancedCustomConfig{
				Routes: []AdvancedCustomRoute{
					{
						IncomingPath: tt.incomingPath,
						UpstreamPath: "/v1/chat/completions",
						Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
					},
				},
			}
			err := config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "converter does not match incoming_path")
		})
	}
}

func TestAdvancedCustomValidateModelListRouteConstraints(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: AdvancedCustomModelListPath,
				UpstreamPath: "https://upstream.example/custom/models",
				Converter:    advancedCustomConverterNone,
			},
		},
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name   string
		routes []AdvancedCustomRoute
		want   string
	}{
		{
			name: "model matching rules",
			routes: []AdvancedCustomRoute{
				{
					IncomingPath: AdvancedCustomModelListPath,
					UpstreamPath: "/v1/models",
					Models:       []string{"gpt-4o"},
				},
			},
			want: "models must be empty",
		},
		{
			name: "converter",
			routes: []AdvancedCustomRoute{
				{
					IncomingPath: AdvancedCustomModelListPath,
					UpstreamPath: "/v1/models",
					Converter:    advancedCustomConverterOpenAIChatToOpenAIResponses,
				},
			},
			want: "converter must be none",
		},
		{
			name: "model placeholder",
			routes: []AdvancedCustomRoute{
				{
					IncomingPath: AdvancedCustomModelListPath,
					UpstreamPath: "/v1/models/{model}",
				},
			},
			want: "upstream_path must not contain {model}",
		},
		{
			name: "duplicate routes",
			routes: []AdvancedCustomRoute{
				{IncomingPath: AdvancedCustomModelListPath, UpstreamPath: "/v1/models"},
				{IncomingPath: AdvancedCustomModelListPath, UpstreamPath: "/provider/models"},
			},
			want: "duplicates the /v1/models route",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&AdvancedCustomConfig{Routes: tt.routes}).Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestAdvancedCustomModelListRouteRequiresExactIncomingPath(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/{model}",
				UpstreamPath: "/generic/{model}",
			},
			{
				IncomingPath: AdvancedCustomModelListPath,
				UpstreamPath: "/provider/models",
			},
		},
	}
	require.NoError(t, config.Validate())

	route, ok := config.ModelListRoute()
	require.True(t, ok)
	assert.Equal(t, "/provider/models", route.UpstreamPath)
}

func TestAdvancedCustomValidateDuplicateIncomingPathWithDisjointModels(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"gpt-4o"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"gemini-2.5-flash"},
			},
		},
	}

	require.NoError(t, config.Validate())
}

func TestAdvancedCustomValidateDuplicateIncomingPathRejectsOverlappingModels(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"shared-model"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"shared-model"},
			},
		},
	}

	err := config.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "models overlaps")
}

func TestAdvancedCustomValidateDuplicateIncomingPathRejectsMultipleCatchAllRoutes(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
			},
		},
	}

	err := config.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catch-all already exists")
}

func TestAdvancedCustomValidateDuplicateIncomingPathRequiresCatchAllLast(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"gemini-2.5-flash"},
			},
		},
	}

	err := config.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "catch-all route must be last")
}

func TestAdvancedCustomMatchPathForModel(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"gemini-2.5-flash"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"gpt-4o"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    advancedCustomConverterNone,
			},
		},
	}
	require.NoError(t, config.Validate())

	geminiRoute, ok := config.MatchPathForModel("/v1/responses", "gemini-2.5-flash")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterOpenAIResponsesToGemini, geminiRoute.Converter)

	chatRoute, ok := config.MatchPathForModel("/v1/responses", "gpt-4o")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterOpenAIResponsesToOpenAIChat, chatRoute.Converter)

	fallbackRoute, ok := config.MatchPathForModel("/v1/responses", "unknown-model")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterNone, fallbackRoute.Converter)
}

func TestAdvancedCustomMatchPathForModelRegexRules(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"re:(?i)^OAI-"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    advancedCustomConverterNone,
			},
		},
	}
	require.NoError(t, config.Validate())

	geminiRoute, ok := config.MatchPathForModel("/v1/responses", "gemini-2.5-flash")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterOpenAIResponsesToGemini, geminiRoute.Converter)

	chatRoute, ok := config.MatchPathForModel("/v1/responses", "oai-test")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterOpenAIResponsesToOpenAIChat, chatRoute.Converter)

	fallbackRoute, ok := config.MatchPathForModel("/v1/responses", "gpt-4o")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterNone, fallbackRoute.Converter)
}

func TestAdvancedCustomRouteModelRegexRulesAreCachedCompiled(t *testing.T) {
	require.True(t, matchAdvancedCustomRouteModelRule("re:^cache-probe-", "cache-probe-model"))

	cached, ok := advancedCustomModelRegexCache.Load("^cache-probe-")
	require.True(t, ok)
	require.NotNil(t, cached)
	_, isRegexp := cached.(*regexp.Regexp)
	require.True(t, isRegexp)

	// Invalid patterns never match and are cached as nil so they are not recompiled.
	require.False(t, matchAdvancedCustomRouteModelRule("re:(", "anything"))
	cached, ok = advancedCustomModelRegexCache.Load("(")
	require.True(t, ok)
	re, _ := cached.(*regexp.Regexp)
	require.Nil(t, re)

	// Cached entries keep matching correctly on subsequent calls.
	require.True(t, matchAdvancedCustomRouteModelRule("re:^cache-probe-", "cache-probe-other"))
	require.False(t, matchAdvancedCustomRouteModelRule("re:^cache-probe-", "other-model"))
}

func TestAdvancedCustomMatchPathForModelExactRuleDoesNotMatchPrefix(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"gemini"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    advancedCustomConverterNone,
			},
		},
	}
	require.NoError(t, config.Validate())

	fallbackRoute, ok := config.MatchPathForModel("/v1/responses", "gemini-2.5-flash")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterNone, fallbackRoute.Converter)
}

func TestAdvancedCustomValidateDuplicateIncomingPathRejectsInvalidRegexModels(t *testing.T) {
	tests := []struct {
		name   string
		models []string
		want   string
	}{
		{name: "empty regex", models: []string{"re:"}, want: "regex is empty"},
		{name: "invalid regex", models: []string{"re:["}, want: "regex is invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &AdvancedCustomConfig{
				Routes: []AdvancedCustomRoute{
					{
						IncomingPath: "/v1/responses",
						UpstreamPath: "/v1beta/models/{model}:generateContent",
						Converter:    advancedCustomConverterOpenAIResponsesToGemini,
						Models:       tt.models,
					},
				},
			}

			err := config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestAdvancedCustomValidateDuplicateIncomingPathRejectsDuplicateRegexModels(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"re:^gemini-"},
			},
		},
	}

	err := config.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "models overlaps")
}

func TestAdvancedCustomMatchPathForModelUsesFirstMatchingRegexRoute(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				Models:       []string{"gemini-2.5-flash"},
			},
		},
	}
	require.NoError(t, config.Validate())

	route, ok := config.MatchPathForModel("/v1/responses", "gemini-2.5-flash")
	require.True(t, ok)
	assert.Equal(t, advancedCustomConverterOpenAIResponsesToGemini, route.Converter)
}

func TestAdvancedCustomSupportedEndpointTypesForModel(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Converter:    advancedCustomConverterOpenAIResponsesToGemini,
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1beta/models/{model}:generateContent",
				UpstreamPath: "/v1beta/models/{model}:generateContent",
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1beta/models/{model}:streamGenerateContent",
				UpstreamPath: "/v1beta/models/{model}:streamGenerateContent",
				Models:       []string{"re:^gemini-"},
			},
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/v1/chat/completions",
				Models:       []string{"gpt-4o"},
			},
			{
				IncomingPath: "/v1/messages",
				UpstreamPath: "/v1/messages",
			},
			{
				IncomingPath: "/custom/endpoint",
				UpstreamPath: "/custom/endpoint",
			},
		},
	}
	require.NoError(t, config.Validate())

	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAIResponse,
		constant.EndpointTypeGemini,
		constant.EndpointTypeAnthropic,
	}, config.SupportedEndpointTypesForModel("gemini-2.5-flash"))
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAI,
		constant.EndpointTypeAnthropic,
	}, config.SupportedEndpointTypesForModel("gpt-4o"))
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeAnthropic,
	}, config.SupportedEndpointTypesForModel("other-model"))
}

func TestAdvancedCustomValidateResponsesToolsMode(t *testing.T) {
	config := &AdvancedCustomConfig{Routes: []AdvancedCustomRoute{{
		IncomingPath: "/v1/responses",
		UpstreamPath: "/v1/chat/completions",
		Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
		ConverterOptions: &AdvancedCustomConverterOptions{
			ResponsesToolsMode: AdvancedCustomResponsesToolsModePreserve,
		},
	}}}
	require.NoError(t, config.Validate())

	config.Routes[0].ConverterOptions.ResponsesToolsMode = "bad"
	require.ErrorContains(t, config.Validate(), "responses_tools_mode is invalid")
}

func TestAdvancedCustomValidateResponsesToolPolicies(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesTools: &AdvancedCustomResponsesToolsOptions{
						Namespace: AdvancedCustomResponsesToolPolicyPreserve,
						Custom:    AdvancedCustomResponsesToolPolicyDrop,
						WebSearch: AdvancedCustomResponsesToolPolicyReject,
						Unknown:   AdvancedCustomResponsesToolPolicyDrop,
					},
				},
			},
		},
	}
	require.NoError(t, valid.Validate())

	flattenNonNamespace := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesTools: &AdvancedCustomResponsesToolsOptions{WebSearch: AdvancedCustomResponsesToolPolicyFlatten},
				},
			},
		},
	}
	require.ErrorContains(t, flattenNonNamespace.Validate(), "responses_tools.web_search is invalid")

	flattenUnknown := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesTools: &AdvancedCustomResponsesToolsOptions{Unknown: AdvancedCustomResponsesToolPolicyFlatten},
				},
			},
		},
	}
	require.ErrorContains(t, flattenUnknown.Validate(), "responses_tools.unknown is invalid")
}

func TestAdvancedCustomValidateResponsesDropFields(t *testing.T) {
	valid := &AdvancedCustomConfig{Routes: []AdvancedCustomRoute{{
		IncomingPath: "/v1/responses",
		UpstreamPath: "/v1/chat/completions",
		Converter:    advancedCustomConverterOpenAIResponsesToOpenAIChat,
		ConverterOptions: &AdvancedCustomConverterOptions{
			ResponsesDropFields: []string{"metadata", "store"},
		},
	}}}
	require.NoError(t, valid.Validate())

	valid.Routes[0].ConverterOptions.ResponsesDropFields = []string{"unsupported"}
	require.ErrorContains(t, valid.Validate(), "responses_drop_fields contains unsupported field")
}

func TestAdvancedCustomValidateResponsesToolPoliciesForPassthrough(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    AdvancedCustomConverterNone,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesTools: &AdvancedCustomResponsesToolsOptions{
						ImageGeneration: AdvancedCustomResponsesToolPolicyDrop,
					},
					ResponsesToolConflictPolicy: AdvancedCustomResponsesToolConflictPolicyDeduplicate,
				},
			},
		},
	}
	require.NoError(t, valid.Validate())

	flatten := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    AdvancedCustomConverterNone,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesTools: &AdvancedCustomResponsesToolsOptions{
						Namespace: AdvancedCustomResponsesToolPolicyFlatten,
					},
				},
			},
		},
	}
	require.ErrorContains(t, flatten.Validate(), "namespace flatten is not supported by converter none")

	dropFields := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    AdvancedCustomConverterNone,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesDropFields: []string{"metadata"},
				},
			},
		},
	}
	require.ErrorContains(t, dropFields.Validate(), "responses_drop_fields is not supported by converter none")
}

func TestAdvancedCustomValidateResponsesToolModelOverrides(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    AdvancedCustomConverterNone,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesToolModelOverrides: []AdvancedCustomResponsesToolModelOverride{
						{
							Models: []string{"glm-5.2"},
							ResponsesTools: &AdvancedCustomResponsesToolsOptions{
								ImageGeneration: AdvancedCustomResponsesToolPolicyDrop,
							},
							ToolNames: []AdvancedCustomResponsesToolNamePolicy{
								{
									ToolType: "custom",
									ToolName: "apply_patch",
									Policy:   AdvancedCustomResponsesToolPolicyReject,
								},
							},
						},
					},
				},
			},
		},
	}
	require.NoError(t, valid.Validate())

	duplicateModel := valid
	duplicateModel.Routes[0].ConverterOptions.ResponsesToolModelOverrides = append(
		duplicateModel.Routes[0].ConverterOptions.ResponsesToolModelOverrides,
		AdvancedCustomResponsesToolModelOverride{
			Models: []string{"glm-5.2"},
			ResponsesTools: &AdvancedCustomResponsesToolsOptions{
				WebSearch: AdvancedCustomResponsesToolPolicyDrop,
			},
		},
	)
	require.ErrorContains(t, duplicateModel.Validate(), "model is already configured")

	functionPolicy := valid
	functionPolicy.Routes[0].ConverterOptions.ResponsesToolModelOverrides[0].ToolNames = []AdvancedCustomResponsesToolNamePolicy{
		{
			ToolType: "function",
			ToolName: "shell",
			Policy:   AdvancedCustomResponsesToolPolicyDrop,
		},
	}
	require.ErrorContains(t, functionPolicy.Validate(), "function tools are always preserved")
}

func TestResolveAdvancedCustomResponsesToolPolicy(t *testing.T) {
	options := &AdvancedCustomConverterOptions{
		ResponsesTools: &AdvancedCustomResponsesToolsOptions{
			WebSearch:       AdvancedCustomResponsesToolPolicyReject,
			ImageGeneration: AdvancedCustomResponsesToolPolicyDrop,
		},
		ResponsesToolModelOverrides: []AdvancedCustomResponsesToolModelOverride{
			{
				Models: []string{"glm-5.2"},
				ResponsesTools: &AdvancedCustomResponsesToolsOptions{
					ImageGeneration: AdvancedCustomResponsesToolPolicyPreserve,
				},
				ToolNames: []AdvancedCustomResponsesToolNamePolicy{
					{
						ToolType: "image_generation",
						ToolName: "image_gen",
						Policy:   AdvancedCustomResponsesToolPolicyReject,
					},
				},
			},
		},
	}

	tests := []struct {
		name           string
		requestedModel string
		upstreamModel  string
		toolType       string
		toolName       string
		wantPolicy     string
		wantSource     string
	}{
		{
			name:       "function is protected",
			toolType:   "function",
			toolName:   "shell",
			wantPolicy: AdvancedCustomResponsesToolPolicyPreserve,
			wantSource: AdvancedCustomResponsesToolPolicySourceProtected,
		},
		{
			name:           "model tool name wins",
			requestedModel: "glm-5.2",
			toolType:       "image_gen",
			toolName:       "image_gen",
			wantPolicy:     AdvancedCustomResponsesToolPolicyReject,
			wantSource:     AdvancedCustomResponsesToolPolicySourceModelToolName,
		},
		{
			name:          "upstream model type override",
			upstreamModel: "glm-5.2",
			toolType:      "image_generation",
			wantPolicy:    AdvancedCustomResponsesToolPolicyPreserve,
			wantSource:    AdvancedCustomResponsesToolPolicySourceModelToolType,
		},
		{
			name:           "other model stays on route default",
			requestedModel: "glm-5.1",
			toolType:       "image_gen",
			wantPolicy:     AdvancedCustomResponsesToolPolicyDrop,
			wantSource:     AdvancedCustomResponsesToolPolicySourceRoute,
		},
		{
			name:       "unconfigured known type defaults preserve",
			toolType:   "tool_search",
			wantPolicy: AdvancedCustomResponsesToolPolicyPreserve,
			wantSource: AdvancedCustomResponsesToolPolicySourceSystemDefault,
		},
		{
			name:       "unknown type defaults preserve",
			toolType:   "future_tool",
			wantPolicy: AdvancedCustomResponsesToolPolicyPreserve,
			wantSource: AdvancedCustomResponsesToolPolicySourceSystemDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAdvancedCustomResponsesToolPolicy(
				options,
				tt.requestedModel,
				tt.upstreamModel,
				tt.toolType,
				tt.toolName,
			)
			assert.Equal(t, tt.wantPolicy, got.Policy)
			assert.Equal(t, tt.wantSource, got.Source)
		})
	}
}

func TestResolveAdvancedCustomResponsesToolConflictPolicy(t *testing.T) {
	assert.Equal(
		t,
		AdvancedCustomResponsesToolConflictPolicyDeduplicate,
		ResolveAdvancedCustomResponsesToolConflictPolicy(nil),
	)
	assert.Equal(
		t,
		AdvancedCustomResponsesToolConflictPolicyReject,
		ResolveAdvancedCustomResponsesToolConflictPolicy(&AdvancedCustomConverterOptions{
			ResponsesToolConflictPolicy: AdvancedCustomResponsesToolConflictPolicyReject,
		}),
	)
}

func TestResolveAdvancedCustomResponsesImplicitHostedTools(t *testing.T) {
	options := &AdvancedCustomConverterOptions{
		ResponsesImplicitHostedTools: []string{"web_search_preview", "image_gen"},
		ResponsesToolModelOverrides: []AdvancedCustomResponsesToolModelOverride{
			{
				Models:                       []string{"gpt-5.6-sol"},
				ResponsesImplicitHostedTools: []string{"image_generation", "future_hosted_tool"},
			},
		},
	}

	assert.Equal(
		t,
		[]string{"web_search", "image_generation", "future_hosted_tool"},
		ResolveAdvancedCustomResponsesImplicitHostedTools(options, "gpt-5.6-sol", ""),
	)
	assert.Equal(
		t,
		[]string{"web_search", "image_generation", "future_hosted_tool"},
		ResolveAdvancedCustomResponsesImplicitHostedTools(options, "alias-model", "gpt-5.6-sol"),
	)
	assert.Equal(
		t,
		[]string{"web_search", "image_generation"},
		ResolveAdvancedCustomResponsesImplicitHostedTools(options, "gpt-5.6-terra", ""),
	)
}

func TestAdvancedCustomValidateResponsesImplicitHostedTools(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    AdvancedCustomConverterNone,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesToolModelOverrides: []AdvancedCustomResponsesToolModelOverride{
						{
							Models:                       []string{"gpt-5.6-sol"},
							ResponsesImplicitHostedTools: []string{"image_generation"},
						},
					},
				},
			},
		},
	}
	require.NoError(t, valid.Validate())

	duplicateAlias := *valid
	duplicateRoute := valid.Routes[0]
	duplicateOptions := *valid.Routes[0].ConverterOptions
	duplicateOverride := duplicateOptions.ResponsesToolModelOverrides[0]
	duplicateOverride.ResponsesImplicitHostedTools = []string{"image_gen", "image_generation"}
	duplicateOptions.ResponsesToolModelOverrides = []AdvancedCustomResponsesToolModelOverride{duplicateOverride}
	duplicateRoute.ConverterOptions = &duplicateOptions
	duplicateAlias.Routes = []AdvancedCustomRoute{duplicateRoute}
	require.ErrorContains(t, duplicateAlias.Validate(), "duplicate capability: image_generation")

	preserve := *valid
	preserveRoute := valid.Routes[0]
	preserveOptions := *valid.Routes[0].ConverterOptions
	preserveOptions.ResponsesToolConflictPolicy = AdvancedCustomResponsesToolConflictPolicyPreserve
	preserveRoute.ConverterOptions = &preserveOptions
	preserve.Routes = []AdvancedCustomRoute{preserveRoute}
	require.ErrorContains(t, preserve.Validate(), "requires responses_tool_conflict_policy deduplicate or reject")
}

func TestAdvancedCustomValidateResponsesToolModelOverrideRequiresDifference(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    AdvancedCustomConverterNone,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesTools: &AdvancedCustomResponsesToolsOptions{
						ImageGeneration: AdvancedCustomResponsesToolPolicyDrop,
					},
					ResponsesToolModelOverrides: []AdvancedCustomResponsesToolModelOverride{
						{
							Models: []string{"glm-5.2"},
							ResponsesTools: &AdvancedCustomResponsesToolsOptions{
								ImageGeneration: AdvancedCustomResponsesToolPolicyDrop,
							},
						},
					},
				},
			},
		},
	}
	require.ErrorContains(t, config.Validate(), "must differ from the route policy")
}

func TestAdvancedCustomValidateResponsesToolConflictPolicy(t *testing.T) {
	config := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    AdvancedCustomConverterNone,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesToolConflictPolicy: "invalid",
				},
			},
		},
	}
	require.ErrorContains(t, config.Validate(), "responses_tool_conflict_policy is invalid")
}

func TestResolveAdvancedCustomResponsesToolPolicyKeepsLegacyMode(t *testing.T) {
	options := &AdvancedCustomConverterOptions{
		ResponsesToolsMode: AdvancedCustomResponsesToolsModeCompatFlatten,
	}

	namespace := ResolveAdvancedCustomResponsesToolPolicy(options, "", "", "namespace", "mcp__demo__")
	assert.Equal(t, AdvancedCustomResponsesToolPolicyFlatten, namespace.Policy)
	assert.Equal(t, AdvancedCustomResponsesToolPolicySourceRoute, namespace.Source)

	image := ResolveAdvancedCustomResponsesToolPolicy(options, "", "", "image_gen", "")
	assert.Equal(t, AdvancedCustomResponsesToolPolicyDrop, image.Policy)
	assert.Equal(t, AdvancedCustomResponsesToolPolicySourceRoute, image.Source)
}

func TestResolveAdvancedCustomResponsesToolPolicyPrefersRequestedModelOverride(t *testing.T) {
	options := &AdvancedCustomConverterOptions{
		ResponsesToolModelOverrides: []AdvancedCustomResponsesToolModelOverride{
			{
				Models:         []string{"glm-5.2"},
				ResponsesTools: &AdvancedCustomResponsesToolsOptions{ImageGeneration: AdvancedCustomResponsesToolPolicyDrop},
			},
			{
				Models:         []string{"alias-model"},
				ResponsesTools: &AdvancedCustomResponsesToolsOptions{ImageGeneration: AdvancedCustomResponsesToolPolicyReject},
			},
		},
	}

	resolution := ResolveAdvancedCustomResponsesToolPolicy(options, "alias-model", "glm-5.2", "image_gen", "")
	require.Equal(t, AdvancedCustomResponsesToolPolicyReject, resolution.Policy)
	require.Equal(t, AdvancedCustomResponsesToolPolicySourceModelToolType, resolution.Source)
	require.Equal(t, "alias-model", resolution.MatchedModel)
}
