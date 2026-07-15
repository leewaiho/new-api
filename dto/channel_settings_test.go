package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvancedCustomValidateResponsesToChatConverterPath(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
			},
		},
	}
	require.NoError(t, valid.Validate())

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
						Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
					},
				},
			}
			err := config.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "converter does not match incoming_path")
		})
	}
}

func TestAdvancedCustomValidateResponsesToolsMode(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesToolsMode: AdvancedCustomResponsesToolsModePreserve,
				},
			},
		},
	}
	require.NoError(t, valid.Validate())

	invalidMode := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesToolsMode: "bad",
				},
			},
		},
	}
	require.ErrorContains(t, invalidMode.Validate(), "responses_tools_mode is invalid")

	wrongConverter := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterNone,
				ConverterOptions: &AdvancedCustomConverterOptions{
					ResponsesToolsMode: AdvancedCustomResponsesToolsModePreserve,
				},
			},
		},
	}
	require.ErrorContains(t, wrongConverter.Validate(), "only supported")
}

func TestAdvancedCustomValidateResponsesToolPolicies(t *testing.T) {
	valid := &AdvancedCustomConfig{
		Routes: []AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
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
				Converter:    AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
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
