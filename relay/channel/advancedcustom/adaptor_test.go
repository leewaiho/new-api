package advancedcustom

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAdaptorUsesExactRouteAndQueryAuth(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/messages",
				UpstreamPath: "https://upstream.example/v1/chat/completions?existing=1",
				Converter:    dto.AdvancedCustomConverterAnthropicMessagesToOpenAIChatCompletions,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeQuery,
					Name:  "api_key",
					Value: "{api_key}",
				},
			},
		},
	})
	info.RequestURLPath = "/v1/messages?client=1"

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "https", parsedURL.Scheme)
	assert.Equal(t, "upstream.example", parsedURL.Host)
	assert.Equal(t, "/v1/chat/completions", parsedURL.Path)
	assert.Equal(t, "1", parsedURL.Query().Get("existing"))
	assert.Equal(t, "sk-test", parsedURL.Query().Get("api_key"))
}

func TestAdaptorJoinsUpstreamPathWithChannelBaseURL(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/proxy/v1/chat/completions?existing=1",
				Converter:    dto.AdvancedCustomConverterNone,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeQuery,
					Name:  "api_key",
					Value: "{api_key}",
				},
			},
		},
	})
	info.ChannelBaseUrl = "https://gateway.example/base"

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "https", parsedURL.Scheme)
	assert.Equal(t, "gateway.example", parsedURL.Host)
	assert.Equal(t, "/base/proxy/v1/chat/completions", parsedURL.Path)
	assert.Equal(t, "1", parsedURL.Query().Get("existing"))
	assert.Equal(t, "sk-test", parsedURL.Query().Get("api_key"))
}

func TestAdaptorReturnsErrorWhenUpstreamPathNeedsMissingBaseURL(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterNone,
			},
		},
	})
	info.ChannelBaseUrl = ""

	_, err := adaptor.GetRequestURL(info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base URL is required")
}

func TestAdaptorSetupRequestHeaderUsesDefaultBearerAuth(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "https://upstream.example/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterNone,
			},
		},
	})
	c := advancedCustomGinContext("/v1/chat/completions")
	header := http.Header{}

	require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))
	assert.Equal(t, "Bearer sk-test", header.Get("Authorization"))
}

func TestAdaptorSetupRequestHeaderUsesConfiguredHeaderAuth(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "https://upstream.example/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterNone,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeHeader,
					Name:  "x-api-key",
					Value: "{api_key}",
				},
			},
		},
	})
	c := advancedCustomGinContext("/v1/chat/completions")
	header := http.Header{}

	require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))
	assert.Empty(t, header.Get("Authorization"))
	assert.Equal(t, "sk-test", header.Get("x-api-key"))
}

func TestAdaptorSetupRequestHeaderAddsClaudeDefaultHeaders(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/messages",
				UpstreamPath: "https://api.anthropic.com/v1/messages",
				Converter:    dto.AdvancedCustomConverterNone,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeHeader,
					Name:  "x-api-key",
					Value: "{api_key}",
				},
			},
		},
	})
	info.RelayFormat = types.RelayFormatClaude
	c := advancedCustomGinContext("/v1/messages")
	header := http.Header{}

	require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))
	assert.Equal(t, "sk-test", header.Get("x-api-key"))
	assert.Equal(t, "2023-06-01", header.Get("anthropic-version"))
}

func TestAdaptorReturnsErrorWhenNoRouteMatchesPath(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/messages",
				UpstreamPath: "https://upstream.example/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterAnthropicMessagesToOpenAIChatCompletions,
			},
		},
	})
	info.RequestURLPath = "/v1/chat/completions"

	_, err := adaptor.GetRequestURL(info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support request path")
}

func TestAdaptorReplacesModelPlaceholderInRouteURL(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent",
				Converter:    dto.AdvancedCustomConverterOpenAIChatCompletionsToGeminiGenerateContent,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeQuery,
					Name:  "key",
					Value: "{api_key}",
				},
			},
		},
	})
	info.UpstreamModelName = "gemini-2.5-flash"

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "/v1beta/models/gemini-2.5-flash:generateContent", parsedURL.Path)
	assert.Equal(t, "sk-test", parsedURL.Query().Get("key"))
	assert.Empty(t, parsedURL.Query().Get("alt"))
}

func TestAdaptorSwitchesGeminiGenerateContentURLForStream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/chat/completions",
				UpstreamPath: "https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent?existing=1",
				Converter:    dto.AdvancedCustomConverterOpenAIChatCompletionsToGeminiGenerateContent,
				Auth: &dto.AdvancedCustomRouteAuth{
					Type:  dto.AdvancedCustomAuthTypeQuery,
					Name:  "key",
					Value: "{api_key}",
				},
			},
		},
	})
	info.UpstreamModelName = "gemini-2.5-pro"
	info.IsStream = true

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "/v1beta/models/gemini-2.5-pro:streamGenerateContent", parsedURL.Path)
	assert.Equal(t, "sse", parsedURL.Query().Get("alt"))
	assert.Equal(t, "1", parsedURL.Query().Get("existing"))
	assert.Equal(t, "sk-test", parsedURL.Query().Get("key"))
}

func TestAdaptorMatchesGeminiIncomingPathTemplate(t *testing.T) {
	tests := []struct {
		name            string
		requestURLPath  string
		wantRequestPath string
	}{
		{
			name:            "generate content",
			requestURLPath:  "/v1beta/models/gemini-2.5-flash:generateContent",
			wantRequestPath: "/v1/chat/completions",
		},
		{
			name:            "stream generate content",
			requestURLPath:  "/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse",
			wantRequestPath: "/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adaptor := &Adaptor{}
			info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
				Routes: []dto.AdvancedCustomRoute{
					{
						IncomingPath: "/v1beta/models/{model}:generateContent",
						UpstreamPath: "https://upstream.example/v1/chat/completions",
						Converter:    dto.AdvancedCustomConverterGeminiGenerateContentToOpenAIChatCompletions,
					},
				},
			})
			info.RequestURLPath = tt.requestURLPath

			requestURL, err := adaptor.GetRequestURL(info)
			require.NoError(t, err)

			parsedURL, err := url.Parse(requestURL)
			require.NoError(t, err)
			assert.Equal(t, tt.wantRequestPath, parsedURL.Path)
		})
	}
}

func TestAdaptorConvertsResponsesRequestToOpenAIChatUpstream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	c := advancedCustomGinContext("/v1/responses")

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model:        "gpt-test",
		Instructions: mustAdvancedCustomRawMessage(t, "system rules"),
		Input:        mustAdvancedCustomRawMessage(t, "hello"),
	})
	require.NoError(t, err)

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Equal(t, "gpt-test", chatReq.Model)
	require.Len(t, chatReq.Messages, 2)
	assert.Equal(t, "system", chatReq.Messages[0].Role)
	assert.Equal(t, "system rules", chatReq.Messages[0].StringContent())
	assert.Equal(t, "user", chatReq.Messages[1].Role)
	assert.Equal(t, "hello", chatReq.Messages[1].StringContent())

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "/v1/chat/completions", parsedURL.Path)
}

func TestAdaptorResponsesToolsDefaultPreservesAllToolTypes(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	c := advancedCustomGinContext("/v1/responses")

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustAdvancedCustomRawMessage(t, "hello"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{
				"type": "namespace",
				"name": "mcp__demo__",
				"tools": []map[string]any{
					{
						"type":       "function",
						"name":       "lookup_order",
						"parameters": map[string]any{"type": "object"},
					},
				},
			},
			{"type": "web_search"},
		}),
	})
	require.NoError(t, err)

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, chatReq.Tools, 2)
	assert.Equal(t, "namespace", chatReq.Tools[0].Type)
	assert.Equal(t, "web_search", chatReq.Tools[1].Type)
	assert.Empty(t, chatReq.Tools[1].WebSearch)
	assert.Empty(t, chatReq.Tools[1].Custom)
	assert.Empty(t, info.ResponsesToolNameMappings)
}

func TestAdaptorResponsesToolsNamespaceOnlyPolicyFlattensNamespaceAndPreservesWebSearch(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: &dto.AdvancedCustomConverterOptions{
					ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{
						Namespace: dto.AdvancedCustomResponsesToolPolicyFlatten,
					},
				},
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	c := advancedCustomGinContext("/v1/responses")

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustAdvancedCustomRawMessage(t, "hello"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{
				"type": "namespace",
				"name": "mcp__demo__",
				"tools": []map[string]any{
					{
						"type":       "function",
						"name":       "lookup_order",
						"parameters": map[string]any{"type": "object"},
					},
				},
			},
			{"type": "web_search"},
		}),
	})
	require.NoError(t, err)

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, chatReq.Tools, 2)
	assert.Equal(t, "function", chatReq.Tools[0].Type)
	assert.Equal(t, "mcp__demo__lookup_order", chatReq.Tools[0].Function.Name)
	assert.Equal(t, "web_search", chatReq.Tools[1].Type)
	assert.Empty(t, chatReq.Tools[1].WebSearch)
	assert.Empty(t, chatReq.Tools[1].Custom)
}

func TestAdaptorResponsesToolsFlattensNativeCodingToolsForChatUpstream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: &dto.AdvancedCustomConverterOptions{
					ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{{
						Models: []string{"glm-5.2"},
						ToolNames: []dto.AdvancedCustomResponsesToolNamePolicy{
							{ToolType: "custom", ToolName: "apply_patch", Policy: dto.AdvancedCustomResponsesToolPolicyFlatten},
							{ToolType: "shell_command", Policy: dto.AdvancedCustomResponsesToolPolicyFlatten},
						},
					}},
				},
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"

	converted, err := adaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustAdvancedCustomRawMessage(t, "update the workspace"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "custom", "name": "apply_patch", "description": "Apply a patch."},
			{"type": "shell_command"},
		}),
		ToolChoice: mustAdvancedCustomRawMessage(t, map[string]any{"type": "custom", "name": "apply_patch"}),
	})
	require.NoError(t, err)

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, chatReq.Tools, 2)
	assert.Equal(t, "function", chatReq.Tools[0].Type)
	assert.Equal(t, "apply_patch", chatReq.Tools[0].Function.Name)
	assert.Equal(t, "function", chatReq.Tools[1].Type)
	assert.Equal(t, "shell_command", chatReq.Tools[1].Function.Name)
	choice, ok := chatReq.ToolChoice.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "function", choice["type"])
	assert.Equal(t, "apply_patch", choice["function"].(map[string]any)["name"])
	assert.Equal(t, "custom", info.ResponsesToolNameMappings["apply_patch"].NativeToolType)
	assert.Equal(t, "shell_command", info.ResponsesToolNameMappings["shell_command"].NativeToolType)
}

func TestAdaptorResponsesToolsModelOverrideFlattensToolSearchForChatUpstream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1/chat/completions",
			Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
			ConverterOptions: &dto.AdvancedCustomConverterOptions{
				ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{{
					Models: []string{"glm-5.2"},
					ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{
						ToolSearch: dto.AdvancedCustomResponsesToolPolicyFlatten,
					},
				}},
			},
		}},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"

	converted, err := adaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustAdvancedCustomRawMessage(t, "find a tool"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "tool_search"},
		}),
		ToolChoice: mustAdvancedCustomRawMessage(t, map[string]any{"type": "tool_search"}),
	})
	require.NoError(t, err)
	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, chatReq.Tools, 1)
	assert.Equal(t, "function", chatReq.Tools[0].Type)
	assert.Equal(t, "tool_search", chatReq.Tools[0].Function.Name)
	assert.Empty(t, chatReq.Tools[0].Custom)
	choice := chatReq.ToolChoice.(map[string]any)
	assert.Equal(t, "tool_search", choice["function"].(map[string]any)["name"])
	assert.Equal(t, "tool_search", info.ResponsesToolNameMappings["tool_search"].NativeToolType)
}

func TestAdaptorRejectsUnregisteredNativeToolBeforeChatUpstreamAndRecordsCompatibilityEvent(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_unregistered_native_tool?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1/chat/completions",
			Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
			ConverterOptions: &dto.AdvancedCustomConverterOptions{
				ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{{
					Models: []string{"glm-5.2"},
					ToolNames: []dto.AdvancedCustomResponsesToolNamePolicy{{
						ToolType: "computer",
						Policy:   dto.AdvancedCustomResponsesToolPolicyFlatten,
					}},
				}},
			},
		}},
	})
	info.ChannelId = 98
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	info.OriginModelName = "glm-5.2"

	_, err = adaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustAdvancedCustomRawMessage(t, "use the computer"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "computer"},
		}),
	})
	require.ErrorContains(t, err, `responses tool "computer" has no registered Chat function adapter`)

	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, "computer", events[0].ToolType)
	assert.Equal(t, model.ToolCompatibilityEventTypeInvalidToolSchema, events[0].EventType)
}

func TestAdaptorRejectsComputerByDefaultAndRecordsCompatibilityEvent(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_default_computer_reject?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{
		IncomingPath: "/v1/responses",
		UpstreamPath: "/v1/chat/completions",
		Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
	}}})
	info.ChannelId = 98
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"

	_, err = adaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustAdvancedCustomRawMessage(t, "use the computer"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "computer_use_preview"},
		}),
	})
	require.ErrorContains(t, err, "was rejected by the Advanced Custom route policy")

	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, "computer_use_preview", events[0].ToolType)
	assert.Equal(t, dto.AdvancedCustomResponsesToolPolicyReject, events[0].CurrentPolicy)
	assert.Equal(t, model.ToolCompatibilityEventTypePolicyReject, events[0].EventType)
}

func TestAdaptorResponsesToChatCompatibilityEventUsesConvertedToolIndex(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_converted_tool_index?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: &dto.AdvancedCustomConverterOptions{
					ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{
						Namespace: dto.AdvancedCustomResponsesToolPolicyFlatten,
					},
				},
			},
		},
	})
	info.ChannelMeta.ChannelId = 97
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"

	converted, err := adaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustAdvancedCustomRawMessage(t, "hello"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{
				"type": "namespace",
				"name": "mcp__demo__",
				"tools": []map[string]any{{
					"type":       "function",
					"name":       "lookup_order",
					"parameters": map[string]any{"type": "object"},
				}, {
					"type":       "function",
					"name":       "lookup_customer",
					"parameters": map[string]any{"type": "object"},
				}},
			},
			{"type": "shell_command"},
		}),
	})
	require.NoError(t, err)
	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, chatReq.Tools, 3)
	require.Equal(t, "function", chatReq.Tools[0].Type)
	require.Equal(t, "function", chatReq.Tools[1].Type)
	require.Equal(t, "shell_command", chatReq.Tools[2].Type)

	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info,
		dto.AdvancedCustomRoute{IncomingPath: "/v1/responses"},
		"glm-5.2",
		"glm-5.2",
		adaptor.compatibilityTools,
		types.NewErrorWithStatusCode(errors.New("tools[2].type: type is illegal"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)
	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "shell_command", events[0].ToolType)
	require.Empty(t, events[0].ToolName)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, events[0].SuggestedPolicy)
}

func TestAdaptorResponsesToChatDoResponseRecordsFlattenedToolSearchByOriginalType(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_tool_search_do_response?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{
		IncomingPath: "/v1/responses",
		UpstreamPath: "/v1/chat/completions",
		Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
		ConverterOptions: &dto.AdvancedCustomConverterOptions{ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{
			ToolSearch: dto.AdvancedCustomResponsesToolPolicyFlatten,
		}},
	}}})
	info.ChannelId = 97
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"

	_, err = adaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustAdvancedCustomRawMessage(t, "find a tool"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "tool_search"},
		}),
	})
	require.NoError(t, err)

	response := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"tools[0].type: type is illegal","type":"upstream_error","code":1214}}`)),
	}
	_, responseErr := adaptor.DoResponse(advancedCustomGinContext("/v1/responses"), response, info)
	require.Error(t, responseErr)

	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	assert.Equal(t, "tool_search", events[0].ToolType)
	assert.Equal(t, model.ToolCompatibilityEventTypeUpstreamUnsupported, events[0].EventType)
}

func TestAdaptorResponsesToChatCompatibilityEventPreservesCustomToolName(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_converted_custom_tool_name?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{{
			IncomingPath: "/v1/responses",
			UpstreamPath: "/v1/chat/completions",
			Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
		}},
	})
	info.ChannelMeta.ChannelId = 97
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"

	converted, err := adaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustAdvancedCustomRawMessage(t, "hello"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "custom", "name": "apply_patch", "format": map[string]any{"type": "text"}},
		}),
	})
	require.NoError(t, err)
	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, chatReq.Tools, 1)
	require.Equal(t, "custom", chatReq.Tools[0].Type)
	require.Contains(t, string(chatReq.Tools[0].Custom), `"name":"apply_patch"`)

	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info,
		dto.AdvancedCustomRoute{IncomingPath: "/v1/responses"},
		"glm-5.2",
		"glm-5.2",
		adaptor.compatibilityTools,
		types.NewErrorWithStatusCode(errors.New("tools[0].type: type is illegal"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)
	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "custom", events[0].ToolType)
	require.Equal(t, "apply_patch", events[0].ToolName)
	require.Equal(t, model.ToolCompatibilityEventTypeUpstreamUnsupported, events[0].EventType)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, events[0].SuggestedPolicy)
}

func TestAdaptorResponsesToolsPerToolPolicyPreservesNamespaceAndDropsWebSearch(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: &dto.AdvancedCustomConverterOptions{
					ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{
						Namespace: dto.AdvancedCustomResponsesToolPolicyPreserve,
						WebSearch: dto.AdvancedCustomResponsesToolPolicyDrop,
					},
				},
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	c := advancedCustomGinContext("/v1/responses")

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustAdvancedCustomRawMessage(t, "hello"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "namespace", "name": "mcp__demo__", "tools": []map[string]any{{"type": "function", "name": "lookup"}}},
			{"type": "web_search"},
		}),
	})
	require.NoError(t, err)

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, chatReq.Tools, 1)
	assert.Equal(t, "namespace", chatReq.Tools[0].Type)
}

func TestAdaptorResponsesToolsModePreserveRejectsUnsupportedComputerUse(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: &dto.AdvancedCustomConverterOptions{
					ResponsesToolsMode: dto.AdvancedCustomResponsesToolsModePreserve,
				},
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	c := advancedCustomGinContext("/v1/responses")

	_, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustAdvancedCustomRawMessage(t, "hello"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{
				"type": "namespace",
				"name": "mcp__demo__",
				"tools": []map[string]any{
					{
						"type":       "function",
						"name":       "lookup_order",
						"parameters": map[string]any{"type": "object"},
					},
				},
			},
			{"type": "computer_use"},
		}),
	})
	require.ErrorContains(t, err, `responses tool "computer_use" has no registered Chat function adapter`)
}

func TestAdaptorResponsesToolsModelOverrideDropsUnsupportedComputerPreviewBeforeChatUpstream(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/chat/completions",
				Converter:    dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: &dto.AdvancedCustomConverterOptions{
					ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{{
						Models: []string{"glm-5.2"},
						ToolNames: []dto.AdvancedCustomResponsesToolNamePolicy{{
							ToolType: "computer_use_preview",
							Policy:   dto.AdvancedCustomResponsesToolPolicyDrop,
						}},
					}},
				},
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"

	converted, err := adaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustAdvancedCustomRawMessage(t, "use the fallback"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "computer_use_preview"},
			{"type": "function", "name": "lookup", "parameters": map[string]any{"type": "object"}},
		}),
	})
	require.NoError(t, err)

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	require.Len(t, chatReq.Tools, 1)
	assert.Equal(t, "function", chatReq.Tools[0].Type)
	assert.Equal(t, "lookup", chatReq.Tools[0].Function.Name)
}

func TestAdaptorResponsesPassthroughSendsExpectedToolsToUpstream(t *testing.T) {
	service.InitHttpClient()
	var upstreamBody []byte
	var upstreamReadErr error
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamBody, upstreamReadErr = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response","output":[]}`))
	}))
	defer upstream.Close()

	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: upstream.URL + "/v1/responses",
				Converter:    dto.AdvancedCustomConverterNone,
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	c := advancedCustomGinContext("/v1/responses")

	request := dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustAdvancedCustomRawMessage(t, "use a tool"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "function", "name": "shell", "description": "run shell", "parameters": map[string]any{"type": "object"}},
			{"type": "function", "name": "apply_patch", "description": "apply patch", "parameters": map[string]any{"type": "object"}},
			{"type": "function", "name": "image_gen.imagegen", "description": "generate image", "parameters": map[string]any{"type": "object"}},
			{"type": "namespace", "name": "mcp__demo__", "tools": []map[string]any{{"type": "function", "name": "lookup"}}},
			{"type": "custom", "name": "apply_patch_custom"},
			{"type": "future_client_tool", "name": "future"},
			{"type": "image_gen"},
			{"type": "web_search"},
			{"type": "tool_search"},
		}),
		ToolChoice: mustAdvancedCustomRawMessage(t, map[string]any{
			"type": "function",
			"name": "shell",
		}),
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, request)
	require.NoError(t, err)
	body, err := common.Marshal(converted)
	require.NoError(t, err)
	info.UpstreamRequestBodySize = int64(len(body))

	response, err := adaptor.DoRequest(c, info, bytes.NewReader(body))
	require.NoError(t, err)
	resp, ok := response.(*http.Response)
	require.True(t, ok)
	defer resp.Body.Close()

	require.NoError(t, upstreamReadErr)
	var upstreamRequest dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(upstreamBody, &upstreamRequest))
	var tools []map[string]any
	require.NoError(t, common.Unmarshal(upstreamRequest.Tools, &tools))
	require.Len(t, tools, 8, "upstream must preserve all tools except the actual hosted/function conflict")
	assert.Equal(t, "function", tools[0]["type"])
	assert.Equal(t, "shell", tools[0]["name"])
	assert.Equal(t, "function", tools[1]["type"])
	assert.Equal(t, "apply_patch", tools[1]["name"])
	assert.Equal(t, "namespace", tools[2]["type"])
	assert.Equal(t, "custom", tools[3]["type"])
	assert.Equal(t, "future_client_tool", tools[4]["type"])
	assert.Equal(t, "image_gen", tools[5]["type"])
	assert.Equal(t, "web_search", tools[6]["type"])
	assert.Equal(t, "tool_search", tools[7]["type"])
	assert.JSONEq(t, `{"type":"function","name":"shell"}`, string(upstreamRequest.ToolChoice))
}

func TestAdaptorResponsesPassthroughKeepsImageGenFunctionWithoutHostedConflict(t *testing.T) {
	service.InitHttpClient()
	var upstreamBody []byte
	var upstreamReadErr error
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamBody, upstreamReadErr = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response","output":[]}`))
	}))
	defer upstream.Close()

	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: upstream.URL + "/v1/responses",
				Converter:    dto.AdvancedCustomConverterNone,
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	c := advancedCustomGinContext("/v1/responses")

	request := dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustAdvancedCustomRawMessage(t, "generate an image"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "function", "name": "image_gen.imagegen", "description": "generate image", "parameters": map[string]any{"type": "object"}},
		}),
	}
	converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, request)
	require.NoError(t, err)
	body, err := common.Marshal(converted)
	require.NoError(t, err)
	info.UpstreamRequestBodySize = int64(len(body))

	response, err := adaptor.DoRequest(c, info, bytes.NewReader(body))
	require.NoError(t, err)
	resp, ok := response.(*http.Response)
	require.True(t, ok)
	defer resp.Body.Close()

	require.NoError(t, upstreamReadErr)
	var upstreamRequest dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(upstreamBody, &upstreamRequest))
	var tools []map[string]any
	require.NoError(t, common.Unmarshal(upstreamRequest.Tools, &tools))
	require.Len(t, tools, 1, "function tool must not be removed without an actual hosted-tool conflict")
	assert.Equal(t, "image_gen.imagegen", tools[0]["name"])
}

func TestAdvancedCustomPopulateChatWebSearchOptionsUsesRouteConfigAtIndex419(t *testing.T) {
	enabled := true
	searchResult := true
	options := &dto.AdvancedCustomConverterOptions{
		ResponsesToolParameters: &dto.AdvancedCustomResponsesToolParameters{
			WebSearch: &dto.AdvancedCustomWebSearchParameterCompatibility{
				WhenNestedOptionsMissing: dto.AdvancedCustomResponsesToolMissingOptionsPopulateDefaults,
				Defaults: dto.AdvancedCustomWebSearchParameterDefaults{
					Enable:       &enabled,
					SearchResult: &searchResult,
					SearchEngine: "search_std",
				},
			},
		},
	}
	tools := make([]dto.ToolCallRequest, 420)
	for i := 0; i < len(tools)-1; i++ {
		tools[i] = dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:       fmt.Sprintf("tool_%03d", i),
				Parameters: map[string]any{"type": "object"},
			},
		}
	}
	tools[419] = dto.ToolCallRequest{Type: "web_search_preview"}
	request := &dto.GeneralOpenAIRequest{Model: "provider-model", Tools: tools}

	advancedCustomPopulateChatWebSearchOptions(options, &relaycommon.RelayInfo{}, request)

	require.Len(t, request.Tools, 420)
	assert.Equal(t, "web_search", request.Tools[419].Type)
	assert.Equal(t, true, request.Tools[419].WebSearch["enable"])
	assert.Equal(t, true, request.Tools[419].WebSearch["search_result"])
	assert.Equal(t, "search_std", request.Tools[419].WebSearch["search_engine"])
}

func TestAdvancedCustomPopulateChatWebSearchOptionsUsesModelOverrideAndPreservesExplicitValues(t *testing.T) {
	routeEnabled := false
	modelEnabled := true
	modelSearchResult := true
	options := &dto.AdvancedCustomConverterOptions{
		ResponsesToolParameters: &dto.AdvancedCustomResponsesToolParameters{
			WebSearch: &dto.AdvancedCustomWebSearchParameterCompatibility{
				WhenNestedOptionsMissing: dto.AdvancedCustomResponsesToolMissingOptionsPopulateDefaults,
				Defaults:                 dto.AdvancedCustomWebSearchParameterDefaults{Enable: &routeEnabled},
			},
		},
		ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{
			{
				Models: []string{"glm-5.2"},
				ResponsesToolParameters: &dto.AdvancedCustomResponsesToolParameters{
					WebSearch: &dto.AdvancedCustomWebSearchParameterCompatibility{
						WhenNestedOptionsMissing: dto.AdvancedCustomResponsesToolMissingOptionsPopulateDefaults,
						Defaults: dto.AdvancedCustomWebSearchParameterDefaults{
							Enable:       &modelEnabled,
							SearchResult: &modelSearchResult,
						},
					},
				},
			},
		},
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "glm-5.2",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "provider-glm"},
	}
	request := &dto.GeneralOpenAIRequest{
		Model: "provider-glm",
		Tools: []dto.ToolCallRequest{
			{Type: "web_search"},
			{Type: "web_search", WebSearch: map[string]any{"enable": false, "search_result": false}},
		},
	}

	advancedCustomPopulateChatWebSearchOptions(options, info, request)

	assert.Equal(t, true, request.Tools[0].WebSearch["enable"])
	assert.Equal(t, true, request.Tools[0].WebSearch["search_result"])
	assert.Equal(t, false, request.Tools[1].WebSearch["enable"])
	assert.Equal(t, false, request.Tools[1].WebSearch["search_result"])
}

func TestAdvancedCustomPopulateChatWebSearchOptionsDoesNothingWithoutConfig(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "glm-5.2",
		Tools: []dto.ToolCallRequest{{Type: "web_search"}},
	}

	advancedCustomPopulateChatWebSearchOptions(nil, &relaycommon.RelayInfo{}, request)

	require.Len(t, request.Tools, 1)
	assert.Equal(t, "web_search", request.Tools[0].Type)
	assert.Empty(t, request.Tools[0].WebSearch)
}

func advancedCustomRelayInfo(config *dto.AdvancedCustomConfig) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat:    types.RelayFormatOpenAI,
		RelayMode:      relayconstant.RelayModeChatCompletions,
		RequestURLPath: "/v1/chat/completions",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey:         "sk-test",
			ChannelBaseUrl: "https://fallback.example",
			ChannelType:    constant.ChannelTypeAdvancedCustom,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AdvancedCustom: config,
			},
		},
	}
}

func advancedCustomGinContext(path string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func mustAdvancedCustomRawMessage(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := common.Marshal(value)
	require.NoError(t, err)
	return raw
}

func TestAdaptorResponsesPassthroughModelOverrideDropsOnlyTargetModelImageTool(t *testing.T) {
	service.InitHttpClient()
	var upstreamBodies [][]byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		upstreamBodies = append(upstreamBodies, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response","output":[]}`))
	}))
	defer upstream.Close()

	config := &dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: upstream.URL + "/v1/responses",
				Converter:    dto.AdvancedCustomConverterNone,
				ConverterOptions: &dto.AdvancedCustomConverterOptions{
					ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{
						{
							Models: []string{"glm-5.2"},
							ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{
								ImageGeneration: dto.AdvancedCustomResponsesToolPolicyDrop,
							},
						},
					},
				},
			},
		},
	}

	for _, model := range []string{"glm-5.2", "glm-5.1"} {
		adaptor := &Adaptor{}
		info := advancedCustomRelayInfo(config)
		info.RelayMode = relayconstant.RelayModeResponses
		info.RequestURLPath = "/v1/responses"
		info.OriginModelName = model
		c := advancedCustomGinContext("/v1/responses")
		request := dto.OpenAIResponsesRequest{
			Model: model,
			Input: mustAdvancedCustomRawMessage(t, "use tools"),
			Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
				{"type": "function", "name": "image_gen.imagegen"},
				{"type": "image_gen"},
				{"type": "function", "name": "shell"},
			}),
		}

		converted, err := adaptor.ConvertOpenAIResponsesRequest(c, info, request)
		require.NoError(t, err)
		body, err := common.Marshal(converted)
		require.NoError(t, err)
		info.UpstreamRequestBodySize = int64(len(body))
		response, err := adaptor.DoRequest(c, info, bytes.NewReader(body))
		require.NoError(t, err)
		resp := response.(*http.Response)
		_ = resp.Body.Close()
	}

	require.Len(t, upstreamBodies, 2)
	var targetRequest dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(upstreamBodies[0], &targetRequest))
	var targetTools []map[string]any
	require.NoError(t, common.Unmarshal(targetRequest.Tools, &targetTools))
	require.Len(t, targetTools, 2)
	assert.Equal(t, "image_gen.imagegen", targetTools[0]["name"], "dropping hosted image tool must preserve its function fallback")
	assert.Equal(t, "shell", targetTools[1]["name"])

	var otherRequest dto.OpenAIResponsesRequest
	require.NoError(t, common.Unmarshal(upstreamBodies[1], &otherRequest))
	var otherTools []map[string]any
	require.NoError(t, common.Unmarshal(otherRequest.Tools, &otherTools))
	require.Len(t, otherTools, 2, "other models keep hosted image support and only deduplicate the conflicting function")
	assert.Equal(t, "image_gen", otherTools[0]["type"])
	assert.Equal(t, "shell", otherTools[1]["name"])
}

func TestAdaptorResponsesPassthroughImplicitHostedToolDeduplicatesTargetModelNamespace(t *testing.T) {
	config := &dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    dto.AdvancedCustomConverterNone,
				ConverterOptions: &dto.AdvancedCustomConverterOptions{
					ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{
						{
							Models:                       []string{"gpt-5.6-sol"},
							ResponsesImplicitHostedTools: []string{"image_generation"},
						},
					},
				},
			},
		},
	}

	for _, test := range []struct {
		model     string
		toolCount int
	}{
		{model: "gpt-5.6-sol", toolCount: 2},
		{model: "gpt-5.6-terra", toolCount: 3},
	} {
		t.Run(test.model, func(t *testing.T) {
			adaptor := &Adaptor{}
			info := advancedCustomRelayInfo(config)
			info.RelayMode = relayconstant.RelayModeResponses
			info.RequestURLPath = "/v1/responses"
			info.OriginModelName = test.model
			converted, err := adaptor.ConvertOpenAIResponsesRequest(
				advancedCustomGinContext("/v1/responses"),
				info,
				dto.OpenAIResponsesRequest{
					Model: test.model,
					Input: mustAdvancedCustomRawMessage(t, "use tools"),
					Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
						{
							"type":  "namespace",
							"name":  "image_gen",
							"tools": []map[string]any{{"type": "function", "name": "imagegen"}},
						},
						{"type": "namespace", "name": "mcp__demo", "tools": []map[string]any{}},
						{"type": "web_search"},
					}),
				},
			)
			require.NoError(t, err)
			body, err := common.Marshal(converted)
			require.NoError(t, err)
			var request dto.OpenAIResponsesRequest
			require.NoError(t, common.Unmarshal(body, &request))
			var tools []map[string]any
			require.NoError(t, common.Unmarshal(request.Tools, &tools))
			require.Len(t, tools, test.toolCount)
			if test.model == "gpt-5.6-sol" {
				assert.Equal(t, "mcp__demo", tools[0]["name"])
				assert.Equal(t, "web_search", tools[1]["type"])
			} else {
				assert.Equal(t, "image_gen", tools[0]["name"])
			}
		})
	}
}

func TestAdaptorResponsesPassthroughRejectsAllToolsRemovedWithAdminPath(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    dto.AdvancedCustomConverterNone,
				ConverterOptions: &dto.AdvancedCustomConverterOptions{
					ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{
						ImageGeneration: dto.AdvancedCustomResponsesToolPolicyDrop,
					},
				},
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	info.OriginModelName = "glm-5.2"
	c := advancedCustomGinContext("/v1/responses")

	_, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustAdvancedCustomRawMessage(t, "generate image"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{{"type": "image_gen"}}),
	})
	require.ErrorContains(t, err, "All Responses tools were removed")
	require.ErrorContains(t, err, "Channel > Advanced Custom > Tool Handling > Model Tool Capabilities")
	require.ErrorContains(t, err, "requested_model=glm-5.2")
}

func TestAdaptorResponsesPassthroughRejectsToolChoiceForDroppedTool(t *testing.T) {
	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{
		Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/responses",
				UpstreamPath: "/v1/responses",
				Converter:    dto.AdvancedCustomConverterNone,
				ConverterOptions: &dto.AdvancedCustomConverterOptions{
					ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{
						ImageGeneration: dto.AdvancedCustomResponsesToolPolicyDrop,
					},
				},
			},
		},
	})
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	c := advancedCustomGinContext("/v1/responses")

	_, err := adaptor.ConvertOpenAIResponsesRequest(c, info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustAdvancedCustomRawMessage(t, "use selected tool"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{
			{"type": "image_gen"},
			{"type": "function", "name": "shell"},
		}),
		ToolChoice: mustAdvancedCustomRawMessage(t, map[string]any{"type": "image_gen"}),
	})
	require.ErrorContains(t, err, "tool_choice selects a tool removed by the channel policy")
}

func TestClassifyAdvancedCustomUpstreamToolErrorOnlySuggestsDropForExplicitUnsupportedTool(t *testing.T) {
	eventType, suggestion := classifyAdvancedCustomUpstreamToolError(http.StatusBadRequest, "Unsupported tool type: image_generation", "image_gen")
	require.Equal(t, model.ToolCompatibilityEventTypeUpstreamUnsupported, eventType)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, suggestion)

	eventType, suggestion = classifyAdvancedCustomUpstreamToolError(http.StatusBadRequest, "Bad Request", "web_search")
	require.Equal(t, model.ToolCompatibilityEventTypeUnclassified, eventType)
	require.Empty(t, suggestion)

	eventType, suggestion = classifyAdvancedCustomUpstreamToolError(http.StatusBadRequest, "tools[297].web_search cannot be empty", "web_search")
	require.Equal(t, model.ToolCompatibilityEventTypeUnclassified, eventType)
	require.Empty(t, suggestion)
}

func TestAdaptorRecordsPolicyDropCompatibilityEvent(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_event_policy_drop?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	adaptor := &Adaptor{}
	info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{
		IncomingPath: "/v1/responses", UpstreamPath: "/v1/responses", Converter: dto.AdvancedCustomConverterNone,
		ConverterOptions: &dto.AdvancedCustomConverterOptions{ResponsesTools: &dto.AdvancedCustomResponsesToolsOptions{
			ImageGeneration: dto.AdvancedCustomResponsesToolPolicyDrop,
		}},
	}}})
	info.ChannelId = 97
	info.RelayMode = relayconstant.RelayModeResponses
	info.RequestURLPath = "/v1/responses"
	info.OriginModelName = "glm-5.2"

	_, err = adaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2", Input: mustAdvancedCustomRawMessage(t, "generate"),
		Tools: mustAdvancedCustomRawMessage(t, []map[string]any{{"type": "image_gen"}, {"type": "function", "name": "shell"}}),
	})
	require.NoError(t, err)
	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, model.ToolCompatibilityEventTypePolicyDrop, events[0].EventType)
	require.Equal(t, "image_gen", events[0].ToolType)
	require.Equal(t, "glm-5.2", events[0].RequestedModel)
	require.False(t, strings.Contains(events[0].SanitizedError, "generate"))
}

func TestAdaptorSuggestsDropForSystemDefaultComputerPolicyReject(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_event_computer_policy_reject?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	for _, tt := range []struct {
		name           string
		options        *dto.AdvancedCustomConverterOptions
		wantSuggestion string
	}{
		{
			name:           "system default",
			wantSuggestion: dto.AdvancedCustomResponsesToolPolicyDrop,
		},
		{
			name: "explicit model rule",
			options: &dto.AdvancedCustomConverterOptions{
				ResponsesToolModelOverrides: []dto.AdvancedCustomResponsesToolModelOverride{{
					Models: []string{"glm-5.2"},
					ToolNames: []dto.AdvancedCustomResponsesToolNamePolicy{{
						ToolType: "computer",
						Policy:   dto.AdvancedCustomResponsesToolPolicyReject,
					}},
				}},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, db.Exec("DELETE FROM tool_compatibility_events").Error)
			adaptor := &Adaptor{}
			info := advancedCustomRelayInfo(&dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{{
				IncomingPath: "/v1/responses", UpstreamPath: "/v1/chat/completions",
				Converter:        dto.AdvancedCustomConverterOpenAIResponsesToOpenAIChatCompletions,
				ConverterOptions: tt.options,
			}}})
			info.ChannelId = 97
			info.RelayMode = relayconstant.RelayModeResponses
			info.RequestURLPath = "/v1/responses"
			info.OriginModelName = "glm-5.2"

			_, err := adaptor.ConvertOpenAIResponsesRequest(advancedCustomGinContext("/v1/responses"), info, dto.OpenAIResponsesRequest{
				Model: "glm-5.2", Input: mustAdvancedCustomRawMessage(t, "use computer"),
				Tools: mustAdvancedCustomRawMessage(t, []map[string]any{{"type": "computer"}}),
			})
			require.ErrorContains(t, err, "responses tool computer/ was rejected by the Advanced Custom route policy")

			var events []model.ToolCompatibilityEvent
			require.NoError(t, db.Find(&events).Error)
			require.Len(t, events, 1)
			require.Equal(t, model.ToolCompatibilityEventTypePolicyReject, events[0].EventType)
			require.Equal(t, "computer", events[0].ToolType)
			require.Empty(t, events[0].ToolName)
			require.Equal(t, tt.wantSuggestion, events[0].SuggestedPolicy)
		})
	}
}

func TestRecordAdvancedCustomUpstreamToolCompatibilityEventsRequiresIndexedToolAttribution(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_upstream_event_attribution?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	info := advancedCustomRelayInfo(nil)
	info.ChannelMeta.ChannelId = 97
	route := dto.AdvancedCustomRoute{IncomingPath: "/v1/responses"}
	tools := []relayconvert.ResponsesToolPolicyDecision{
		{ToolIndex: 0, ToolType: "image_gen"},
		{ToolIndex: 297, ToolType: "web_search"},
	}

	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info, route, "glm-5.2", "glm-5.2", tools,
		types.NewErrorWithStatusCode(errors.New("Unsupported tool type: image_generation"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)
	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Empty(t, events[0].ToolType)
	require.Equal(t, model.ToolCompatibilityEventTypeUnclassified, events[0].EventType)
	require.Empty(t, events[0].SuggestedPolicy)

	require.NoError(t, db.Exec("DELETE FROM tool_compatibility_events").Error)
	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info, route, "glm-5.2", "glm-5.2", tools,
		types.NewErrorWithStatusCode(errors.New("tools[297].web_search cannot be empty"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)
	events = nil
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "web_search", events[0].ToolType)
	require.Equal(t, model.ToolCompatibilityEventTypeUnclassified, events[0].EventType)
	require.Empty(t, events[0].SuggestedPolicy)

	require.NoError(t, db.Exec("DELETE FROM tool_compatibility_events").Error)
	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info, route, "glm-5.2", "glm-5.2", tools,
		types.NewErrorWithStatusCode(errors.New("Bad Request"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)
	events = nil
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Empty(t, events[0].ToolType)
	require.Equal(t, model.ToolCompatibilityEventTypeUnclassified, events[0].EventType)
	require.Empty(t, events[0].SuggestedPolicy)
}

func TestRecordAdvancedCustomUpstreamToolCompatibilityEventsUsesToolIndexForNativeTypes(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_upstream_native_type_attribution?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	info := advancedCustomRelayInfo(nil)
	info.ChannelMeta.ChannelId = 97
	route := dto.AdvancedCustomRoute{IncomingPath: "/v1/responses"}
	tools := []relayconvert.ResponsesToolPolicyDecision{
		{ToolIndex: 0, ToolType: "shell_command"},
		{ToolIndex: 1, ToolType: "apply_patch"},
	}

	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info, route, "glm-5.2", "glm-5.2", tools,
		types.NewErrorWithStatusCode(errors.New("tools[1].type: type is illegal"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)
	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "apply_patch", events[0].ToolType)
	require.Empty(t, events[0].ToolName)
	require.Equal(t, model.ToolCompatibilityEventTypeUpstreamUnsupported, events[0].EventType)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, events[0].SuggestedPolicy)

	require.NoError(t, db.Exec("DELETE FROM tool_compatibility_events").Error)
	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info, route, "glm-5.2", "glm-5.2", tools,
		types.NewErrorWithStatusCode(errors.New("tools[0].type: type is illegal"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)
	events = nil
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "shell_command", events[0].ToolType)
	require.Equal(t, model.ToolCompatibilityEventTypeUpstreamUnsupported, events[0].EventType)
}

func TestRecordAdvancedCustomUpstreamToolCompatibilityEventsMatchesUniqueExplicitTypeWithoutIndex(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_upstream_no_index?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	info := advancedCustomRelayInfo(nil)
	info.ChannelMeta.ChannelId = 97
	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info,
		dto.AdvancedCustomRoute{IncomingPath: "/v1/responses"},
		"glm-5.2",
		"glm-5.2",
		[]relayconvert.ResponsesToolPolicyDecision{{ToolIndex: 0, ToolType: "shell_command"}},
		types.NewErrorWithStatusCode(errors.New("Unsupported tool type: shell_command"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)

	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "shell_command", events[0].ToolType)
	require.Equal(t, model.ToolCompatibilityEventTypeUpstreamUnsupported, events[0].EventType)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, events[0].SuggestedPolicy)
}

func TestRecordAdvancedCustomUpstreamToolCompatibilityEventsRejectsAmbiguousOrMissingExplicitTypeWithoutIndex(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_upstream_explicit_type_attribution?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	info := advancedCustomRelayInfo(nil)
	info.ChannelMeta.ChannelId = 97
	route := dto.AdvancedCustomRoute{IncomingPath: "/v1/responses"}
	uniqueTools := []relayconvert.ResponsesToolPolicyDecision{
		{ToolIndex: 0, ToolType: "shell_command"},
		{ToolIndex: 1, ToolType: "apply_patch"},
	}

	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info, route, "glm-5.2", "glm-5.2", uniqueTools,
		types.NewErrorWithStatusCode(errors.New("Unsupported tool type: shell_command"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)
	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "shell_command", events[0].ToolType)
	require.Equal(t, model.ToolCompatibilityEventTypeUpstreamUnsupported, events[0].EventType)
	require.Equal(t, dto.AdvancedCustomResponsesToolPolicyDrop, events[0].SuggestedPolicy)

	require.NoError(t, db.Exec("DELETE FROM tool_compatibility_events").Error)
	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info, route, "glm-5.2", "glm-5.2", uniqueTools,
		types.NewErrorWithStatusCode(errors.New("Unsupported tool type"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)
	events = nil
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Empty(t, events[0].ToolType)
	require.Equal(t, model.ToolCompatibilityEventTypeUnclassified, events[0].EventType)
	require.Empty(t, events[0].SuggestedPolicy)

	require.NoError(t, db.Exec("DELETE FROM tool_compatibility_events").Error)
	duplicateTools := []relayconvert.ResponsesToolPolicyDecision{
		{ToolIndex: 0, ToolType: "shell_command"},
		{ToolIndex: 1, ToolType: "shell_command"},
	}
	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info, route, "glm-5.2", "glm-5.2", duplicateTools,
		types.NewErrorWithStatusCode(errors.New("Unsupported tool type: shell_command"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)
	events = nil
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Empty(t, events[0].ToolType)
	require.Equal(t, model.ToolCompatibilityEventTypeUnclassified, events[0].EventType)
	require.Empty(t, events[0].SuggestedPolicy)
}

func TestRecordAdvancedCustomUpstreamToolCompatibilityEventsRecordsIndexedWebSearchParameterError(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_upstream_web_search_parameter?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	info := advancedCustomRelayInfo(nil)
	info.ChannelMeta.ChannelId = 97
	recordAdvancedCustomUpstreamToolCompatibilityEvents(
		info,
		dto.AdvancedCustomRoute{IncomingPath: "/v1/responses"},
		"glm-5.2",
		"glm-5.2",
		[]relayconvert.ResponsesToolPolicyDecision{{ToolIndex: 0, ToolType: "web_search"}},
		types.NewErrorWithStatusCode(errors.New("tools[0].web_search cannot be empty"), types.ErrorCodeBadResponse, http.StatusBadRequest),
	)

	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "web_search", events[0].ToolType)
	require.Equal(t, model.ToolCompatibilityEventTypeUnclassified, events[0].EventType)
	require.Empty(t, events[0].SuggestedPolicy)
}

func TestRecordAdvancedCustomToolCompatibilityEventsRecordsInvalidSchema(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:advanced_custom_invalid_schema_event?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ToolCompatibilityEvent{}))
	model.DB = db
	t.Cleanup(func() { model.DB = previousDB })

	info := advancedCustomRelayInfo(nil)
	info.ChannelMeta.ChannelId = 97
	recordAdvancedCustomToolCompatibilityEvents(
		info,
		dto.AdvancedCustomRoute{IncomingPath: "/v1/responses"},
		"glm-5.2",
		"glm-5.2",
		[]relayconvert.ResponsesToolPolicyDecision{{ToolType: "namespace", ToolName: "mcp__broken__"}},
		errors.New("namespace tool has invalid nested tools"),
	)

	var events []model.ToolCompatibilityEvent
	require.NoError(t, db.Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, model.ToolCompatibilityEventTypeInvalidToolSchema, events[0].EventType)
	require.Equal(t, "namespace", events[0].ToolType)
	require.Equal(t, "mcp__broken__", events[0].ToolName)
	require.Empty(t, events[0].SuggestedPolicy)
}
