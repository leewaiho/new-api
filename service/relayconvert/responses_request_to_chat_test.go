package relayconvert

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesRequestToChatCompletionsRequestInstructionsAndScalarInput(t *testing.T) {
	stream := true
	temperature := 0.0
	topP := 0.9
	maxOutputTokens := uint(128)
	parallelToolCalls := true

	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model:                "gpt-test",
		Instructions:         mustRawMessage(t, "system rules"),
		Input:                mustRawMessage(t, "hello"),
		Stream:               &stream,
		StreamOptions:        &dto.StreamOptions{IncludeUsage: true},
		MaxOutputTokens:      &maxOutputTokens,
		Temperature:          &temperature,
		TopP:                 &topP,
		User:                 mustRawMessage(t, "user-1"),
		Store:                mustRawMessage(t, false),
		Metadata:             mustRawMessage(t, map[string]any{"trace": "abc"}),
		ParallelToolCalls:    mustRawMessage(t, parallelToolCalls),
		PromptCacheKey:       mustRawMessage(t, "cache-key"),
		PromptCacheRetention: mustRawMessage(t, "24h"),
		Reasoning:            &dto.Reasoning{Effort: "medium"},
	})
	require.NoError(t, err)

	assert.Equal(t, "gpt-test", got.Model)
	require.Len(t, got.Messages, 2)
	assert.Equal(t, dto.Message{Role: "system", Content: "system rules"}, got.Messages[0])
	assert.Equal(t, dto.Message{Role: "user", Content: "hello"}, got.Messages[1])
	assert.Same(t, &stream, got.Stream)
	require.NotNil(t, got.StreamOptions)
	assert.True(t, got.StreamOptions.IncludeUsage)
	assert.Equal(t, maxOutputTokens, lo.FromPtr(got.MaxCompletionTokens))
	assert.Equal(t, 0.0, lo.FromPtr(got.Temperature))
	assert.Equal(t, 0.9, lo.FromPtr(got.TopP))
	assert.True(t, lo.FromPtr(got.ParallelTooCalls))
	assert.Equal(t, "cache-key", got.PromptCacheKey)
	assert.Equal(t, "medium", got.ReasoningEffort)
	assert.Equal(t, `"user-1"`, string(got.User))
	assert.Equal(t, `false`, string(got.Store))
	assert.Equal(t, "abc", gjson.GetBytes(got.Metadata, "trace").String())
}

func TestResponsesRequestToChatCompletionsRequestMultimodalInput(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "look"},
					{"type": "input_image", "image_url": "https://example.test/a.png", "detail": "low"},
					{"type": "input_file", "file_id": "file_1", "filename": "a.txt"},
					{"type": "input_audio", "input_audio": map[string]any{"data": "abc", "format": "wav"}},
					{"type": "input_video", "video_url": map[string]any{"url": "https://example.test/v.mp4"}},
				},
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 1)
	assert.Equal(t, "user", got.Messages[0].Role)
	parts := got.Messages[0].ParseContent()
	require.Len(t, parts, 5)
	assert.Equal(t, dto.ContentTypeText, parts[0].Type)
	assert.Equal(t, "look", parts[0].Text)
	assert.Equal(t, dto.ContentTypeImageURL, parts[1].Type)
	assert.Equal(t, "https://example.test/a.png", parts[1].GetImageMedia().Url)
	assert.Equal(t, dto.ContentTypeFile, parts[2].Type)
	assert.Equal(t, "file_1", parts[2].GetFile().FileId)
	assert.Equal(t, dto.ContentTypeInputAudio, parts[3].Type)
	assert.Equal(t, "wav", parts[3].GetInputAudio().Format)
	assert.Equal(t, dto.ContentTypeVideoUrl, parts[4].Type)
	assert.Equal(t, "https://example.test/v.mp4", parts[4].GetVideoUrl().Url)
}

func TestResponsesRequestToChatCompletionsRequestAssistantTextAndFunctionCallCoexist(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "output_text", "text": "I will call."},
				},
			},
			{
				"type":      "function_call",
				"call_id":   "call_1",
				"name":      "lookup",
				"arguments": map[string]any{"q": "x"},
			},
			{
				"type":    "function_call_output",
				"call_id": "call_1",
				"output":  map[string]any{"ok": true},
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 2)
	assert.Equal(t, "assistant", got.Messages[0].Role)
	assert.Equal(t, "I will call.", got.Messages[0].StringContent())
	toolCalls := got.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_1", toolCalls[0].ID)
	assert.Equal(t, "function", toolCalls[0].Type)
	assert.Equal(t, "lookup", toolCalls[0].Function.Name)
	assert.JSONEq(t, `{"q":"x"}`, toolCalls[0].Function.Arguments)
	assert.Equal(t, "tool", got.Messages[1].Role)
	assert.Equal(t, "call_1", got.Messages[1].ToolCallId)
	assert.JSONEq(t, `{"ok":true}`, got.Messages[1].StringContent())
}

func TestResponsesRequestToChatCompletionsRequestOnlyFunctionCallCreatesAssistant(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{
				"type":      "function_call",
				"call_id":   "call_1",
				"name":      "lookup",
				"arguments": `{"q":"x"}`,
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 1)
	assert.Equal(t, "assistant", got.Messages[0].Role)
	assert.Nil(t, got.Messages[0].Content)
	toolCalls := got.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, `{"q":"x"}`, toolCalls[0].Function.Arguments)
}

func TestResponsesRequestToChatCompletionsRequestToolsToolChoiceAndTextFormat(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, "hello"),
		Tools: mustRawMessage(t, []map[string]any{
			{
				"type":        "function",
				"name":        "lookup",
				"description": "Lookup data",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"q": map[string]any{"type": "string"},
					},
				},
			},
		}),
		ToolChoice: mustRawMessage(t, map[string]any{
			"type": "function",
			"name": "lookup",
		}),
		Text: mustRawMessage(t, map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "answer",
				"schema": map[string]any{"type": "object"},
				"strict": true,
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Tools, 1)
	assert.Equal(t, "function", got.Tools[0].Type)
	assert.Equal(t, "lookup", got.Tools[0].Function.Name)
	assert.Equal(t, "Lookup data", got.Tools[0].Function.Description)
	assert.Equal(t, "object", got.Tools[0].Function.Parameters.(map[string]any)["type"])
	assert.Equal(t, map[string]any{
		"type": "function",
		"function": map[string]any{
			"name": "lookup",
		},
	}, got.ToolChoice)
	require.NotNil(t, got.ResponseFormat)
	assert.Equal(t, "json_schema", got.ResponseFormat.Type)
	assert.Equal(t, "answer", gjson.GetBytes(got.ResponseFormat.JsonSchema, "name").String())
	assert.True(t, gjson.GetBytes(got.ResponseFormat.JsonSchema, "strict").Bool())
}

func TestResponsesRequestToChatCompletionsRequestFlattensNamespaceToolsWithMapping(t *testing.T) {
	mappings := map[string]dto.ResponsesToolNameMapping{}
	got, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, "hello"),
		Tools: mustRawMessage(t, []map[string]any{
			{
				"type":        "namespace",
				"name":        "mcp__demo__",
				"description": "Demo tools",
				"tools": []map[string]any{
					{
						"type":        "function",
						"name":        "lookup_order",
						"description": "Look up an order",
						"parameters": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"order_id": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		}),
	}, ResponsesRequestToChatOptions{
		FlattenNamespaceTools: true,
		ToolNameMappings:      mappings,
	})
	require.NoError(t, err)

	require.Len(t, got.Tools, 1)
	assert.Equal(t, "function", got.Tools[0].Type)
	assert.Equal(t, "mcp__demo__lookup_order", got.Tools[0].Function.Name)
	assert.Equal(t, "Look up an order", got.Tools[0].Function.Description)
	assert.Equal(t, "object", got.Tools[0].Function.Parameters.(map[string]any)["type"])
	assert.Equal(t, dto.ResponsesToolNameMapping{Namespace: "mcp__demo__", Name: "lookup_order"}, mappings["mcp__demo__lookup_order"])
}

func TestResponsesRequestToChatCompletionsRequestDropsUnsupportedToolsWhenRequested(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, "hello"),
		Tools: mustRawMessage(t, []map[string]any{
			{"type": "web_search"},
			{"type": "custom", "name": "apply_patch"},
			{"type": "function", "name": "lookup", "parameters": map[string]any{"type": "object"}},
		}),
	}, ResponsesRequestToChatOptions{DropUnsupportedTools: true})
	require.NoError(t, err)

	require.Len(t, got.Tools, 1)
	assert.Equal(t, "function", got.Tools[0].Type)
	assert.Equal(t, "lookup", got.Tools[0].Function.Name)
}

func TestResponsesRequestToChatCompletionsRequestAppliesPerToolPolicies(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, "hello"),
		Tools: mustRawMessage(t, []map[string]any{
			{
				"type": "namespace",
				"name": "mcp__demo__",
				"tools": []map[string]any{
					{"type": "function", "name": "lookup", "parameters": map[string]any{"type": "object"}},
				},
			},
			{"type": "web_search"},
			{"type": "custom", "name": "apply_patch"},
		}),
	}, ResponsesRequestToChatOptions{
		ToolPolicies: ResponsesToolPolicies{
			Namespace: ResponsesToolPolicyPreserve,
			WebSearch: ResponsesToolPolicyDrop,
			Custom:    ResponsesToolPolicyPreserve,
		},
	})
	require.NoError(t, err)

	require.Len(t, got.Tools, 2)
	assert.Equal(t, "namespace", got.Tools[0].Type)
	assert.Contains(t, string(got.Tools[0].Custom), `"type":"namespace"`)
	assert.Equal(t, "custom", got.Tools[1].Type)
}

func TestResponsesRequestToChatCompletionsRequestRejectsToolByPolicy(t *testing.T) {
	_, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, "hello"),
		Tools: mustRawMessage(t, []map[string]any{{"type": "web_search"}}),
	}, ResponsesRequestToChatOptions{
		ToolPolicies: ResponsesToolPolicies{WebSearch: ResponsesToolPolicyReject},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `responses tool "web_search" is not supported`)
}

func TestResponsesRequestToChatCompletionsRequestRejectsDroppedToolChoice(t *testing.T) {
	_, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model:      "gpt-test",
		Input:      mustRawMessage(t, "hello"),
		Tools:      mustRawMessage(t, []map[string]any{{"type": "web_search"}}),
		ToolChoice: mustRawMessage(t, map[string]any{"type": "web_search"}),
	}, ResponsesRequestToChatOptions{
		ToolPolicies: ResponsesToolPolicies{WebSearch: ResponsesToolPolicyDrop},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool_choice")
	assert.Contains(t, err.Error(), "web_search")
}

func TestResponsesRequestToChatCompletionsRequestRewritesFlattenedNamespaceFunctionToolChoice(t *testing.T) {
	mappings := map[string]dto.ResponsesToolNameMapping{}
	got, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, "hello"),
		Tools: mustRawMessage(t, []map[string]any{
			{
				"type": "namespace",
				"name": "mcp__demo__",
				"tools": []map[string]any{
					{"type": "function", "name": "lookup"},
				},
			},
		}),
		ToolChoice: mustRawMessage(t, map[string]any{"type": "function", "name": "lookup"}),
	}, ResponsesRequestToChatOptions{
		ToolPolicies:     ResponsesToolPolicies{Namespace: ResponsesToolPolicyFlatten},
		ToolNameMappings: mappings,
	})
	require.NoError(t, err)
	choice, ok := got.ToolChoice.(map[string]any)
	require.True(t, ok)
	function, ok := choice["function"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "mcp__demo__lookup", function["name"])
}

func TestResponsesRequestToChatCompletionsRequestConvertsPreservedCustomToolChoice(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model:      "gpt-test",
		Input:      mustRawMessage(t, "hello"),
		Tools:      mustRawMessage(t, []map[string]any{{"type": "custom", "name": "apply_patch"}}),
		ToolChoice: mustRawMessage(t, map[string]any{"type": "custom", "name": "apply_patch"}),
	}, ResponsesRequestToChatOptions{
		ToolPolicies: ResponsesToolPolicies{Custom: ResponsesToolPolicyPreserve},
	})
	require.NoError(t, err)
	choice, ok := got.ToolChoice.(map[string]any)
	require.True(t, ok)
	custom, ok := choice["custom"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "apply_patch", custom["name"])
}

func TestResponsesRequestToChatCompletionsRequestRejectsAllowedToolsChoiceWhenPoliciesMutateTools(t *testing.T) {
	_, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, "hello"),
		ToolChoice: mustRawMessage(t, map[string]any{
			"type":  "allowed_tools",
			"mode":  "auto",
			"tools": []map[string]any{{"type": "web_search"}},
		}),
	}, ResponsesRequestToChatOptions{
		ToolPolicies: ResponsesToolPolicies{WebSearch: ResponsesToolPolicyDrop},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allowed_tools")
	assert.Contains(t, err.Error(), "tool policies")
}

func TestResponsesRequestToChatCompletionsRequestDropsUnknownToolByCompatDefault(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, "hello"),
		Tools: mustRawMessage(t, []map[string]any{{"type": "computer_use"}}),
	}, ResponsesRequestToChatOptions{
		ToolPolicies: ResponsesToolPolicies{Namespace: ResponsesToolPolicyFlatten},
	})
	require.NoError(t, err)
	assert.Empty(t, got.Tools)
}

func TestResponsesRequestToChatCompletionsRequestCustomToolCallPreservesRawShape(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{
				"type":    "custom_tool_call",
				"call_id": "call_custom",
				"name":    "apply_patch",
				"input":   "patch body",
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 1)
	toolCalls := got.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, dto.CustomType, toolCalls[0].Type)
	assert.Equal(t, "call_custom", toolCalls[0].ID)
	assert.Equal(t, "apply_patch", toolCalls[0].Function.Name)
	assert.Equal(t, "patch body", toolCalls[0].Function.Arguments)
	assert.Equal(t, "custom_tool_call", gjson.GetBytes(toolCalls[0].Custom, "type").String())
	assert.Equal(t, "patch body", gjson.GetBytes(toolCalls[0].Custom, "input").String())
}

func TestResponsesRequestToChatCompletionsRequestFlattensNativeCodingTools(t *testing.T) {
	mappings := map[string]dto.ResponsesToolNameMapping{}
	got, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustRawMessage(t, "update the file"),
		Tools: mustRawMessage(t, []map[string]any{
			{
				"type":        "custom",
				"name":        "apply_patch",
				"description": "Apply a patch to the workspace.",
			},
			{
				"type": "shell_command",
			},
		}),
		ToolChoice: mustRawMessage(t, map[string]any{
			"type": "custom",
			"name": "apply_patch",
		}),
	}, ResponsesRequestToChatOptions{
		ToolPolicyResolver: func(toolType string, toolName string) string {
			if (toolType == "custom" && toolName == "apply_patch") || toolType == "shell_command" {
				return ResponsesToolPolicyFlatten
			}
			return ResponsesToolPolicyPreserve
		},
		ToolNameMappings: mappings,
	})
	require.NoError(t, err)
	require.Len(t, got.Tools, 2)

	assert.Equal(t, "function", got.Tools[0].Type)
	assert.Equal(t, "apply_patch", got.Tools[0].Function.Name)
	assert.Equal(t, "Apply a patch to the workspace.", got.Tools[0].Function.Description)
	assert.Equal(t, map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"input"},
		"properties": map[string]any{
			"input": map[string]any{"type": "string"},
		},
	}, got.Tools[0].Function.Parameters)

	assert.Equal(t, "function", got.Tools[1].Type)
	assert.Equal(t, "shell_command", got.Tools[1].Function.Name)
	assert.Equal(t, map[string]any{
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
	}, got.Tools[1].Function.Parameters)

	choice, ok := got.ToolChoice.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "function", choice["type"])
	assert.Equal(t, "apply_patch", choice["function"].(map[string]any)["name"])
	assert.Equal(t, dto.ResponsesToolNameMapping{
		Name:           "apply_patch",
		NativeToolType: "custom",
		ArgumentsCodec: "custom_input",
	}, mappings["apply_patch"])
	assert.Equal(t, dto.ResponsesToolNameMapping{
		Name:           "shell_command",
		NativeToolType: "shell_command",
	}, mappings["shell_command"])

	shellChoice, err := responsesRequestToolChoiceToChat(
		mustRawMessage(t, map[string]any{"type": "shell_command"}),
		ResponsesRequestToChatOptions{
			ToolPolicyResolver: func(toolType string, toolName string) string {
				if (toolType == "custom" && toolName == "apply_patch") || toolType == "shell_command" {
					return ResponsesToolPolicyFlatten
				}
				return ResponsesToolPolicyPreserve
			},
			ToolNameMappings: mappings,
		},
		got.Tools,
	)
	require.NoError(t, err)
	assert.Equal(t, "function", shellChoice.(map[string]any)["type"])
	assert.Equal(t, "shell_command", shellChoice.(map[string]any)["function"].(map[string]any)["name"])
}

func TestResponsesRequestToChatCompletionsRequestConvertsNativeToolCallHistoryAndOutput(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Tools: mustRawMessage(t, []map[string]any{
			{"type": "custom", "name": "apply_patch"},
		}),
		Input: mustRawMessage(t, []map[string]any{
			{
				"type":    "custom_tool_call",
				"call_id": "call_patch",
				"name":    "apply_patch",
				"input":   "*** Begin Patch\n*** End Patch",
			},
			{
				"type":    "custom_tool_call_output",
				"call_id": "call_patch",
				"name":    "apply_patch",
				"output":  "Done.",
			},
		}),
	}, ResponsesRequestToChatOptions{
		ToolPolicies:     ResponsesToolPolicies{Custom: ResponsesToolPolicyFlatten},
		ToolNameMappings: map[string]dto.ResponsesToolNameMapping{},
	})
	require.NoError(t, err)
	require.Len(t, got.Messages, 2)

	toolCalls := got.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "function", toolCalls[0].Type)
	assert.Equal(t, "call_patch", toolCalls[0].ID)
	assert.Equal(t, "apply_patch", toolCalls[0].Function.Name)
	assert.JSONEq(t, `{"input":"*** Begin Patch\n*** End Patch"}`, toolCalls[0].Function.Arguments)

	assert.Equal(t, "tool", got.Messages[1].Role)
	assert.Equal(t, "call_patch", got.Messages[1].ToolCallId)
	assert.Equal(t, "Done.", got.Messages[1].StringContent())
}

func TestResponsesRequestToChatCompletionsRequestRejectsUnregisteredNativeToolFlatten(t *testing.T) {
	_, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustRawMessage(t, "use the computer"),
		Tools: mustRawMessage(t, []map[string]any{
			{"type": "computer"},
		}),
	}, ResponsesRequestToChatOptions{
		ToolPolicies: ResponsesToolPolicies{Unknown: ResponsesToolPolicyFlatten},
	})
	require.ErrorContains(t, err, `responses tool "computer" has no registered Chat function adapter`)
}

func TestResponsesRequestToChatCompletionsRequestRejectsStatefulFields(t *testing.T) {
	tests := []struct {
		name string
		req  *dto.OpenAIResponsesRequest
		want string
	}{
		{
			name: "conversation",
			req:  &dto.OpenAIResponsesRequest{Model: "gpt-test", Conversation: mustRawMessage(t, "conv_1")},
			want: "conversation",
		},
		{
			name: "previous response",
			req:  &dto.OpenAIResponsesRequest{Model: "gpt-test", PreviousResponseID: "resp_1"},
			want: "previous_response_id",
		},
		{
			name: "prompt",
			req:  &dto.OpenAIResponsesRequest{Model: "gpt-test", Prompt: mustRawMessage(t, map[string]any{"id": "pmpt_1"})},
			want: "prompt",
		},
		{
			name: "context management",
			req:  &dto.OpenAIResponsesRequest{Model: "gpt-test", ContextManagement: mustRawMessage(t, map[string]any{"type": "auto"})},
			want: "context_management",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResponsesRequestToChatCompletionsRequest(tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.Contains(t, err.Error(), "stateful fields")
		})
	}
}

func mustRawMessage(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := common.Marshal(value)
	require.NoError(t, err)
	return raw
}

func TestResponsesRequestToChatCompletionsRequestDropsResponseFieldsByName(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model:                "gpt-test",
		Input:                mustRawMessage(t, "hello"),
		Metadata:             mustRawMessage(t, map[string]any{"codex": "trace"}),
		Store:                mustRawMessage(t, true),
		TopLogProbs:          intPtr(2),
		ServiceTier:          "auto",
		SafetyIdentifier:     mustRawMessage(t, "user-1"),
		PromptCacheRetention: mustRawMessage(t, "24h"),
		PromptCacheKey:       mustRawMessage(t, "cache-key"),
		ParallelToolCalls:    mustRawMessage(t, true),
		StreamOptions:        &dto.StreamOptions{IncludeUsage: true},
		Reasoning:            &dto.Reasoning{Effort: "high"},
	}, ResponsesRequestToChatOptions{
		DropResponseFields: map[string]struct{}{
			"metadata":               {},
			"store":                  {},
			"service_tier":           {},
			"safety_identifier":      {},
			"prompt_cache_retention": {},
			"prompt_cache_key":       {},
			"parallel_tool_calls":    {},
			"stream_options":         {},
			"reasoning":              {},
			"top_logprobs":           {},
		},
	})
	require.NoError(t, err)
	assert.Nil(t, got.Metadata)
	assert.Nil(t, got.Store)
	assert.Nil(t, got.SafetyIdentifier)
	assert.Nil(t, got.PromptCacheRetention)
	assert.Empty(t, got.ServiceTier)
	assert.Empty(t, got.PromptCacheKey)
	assert.Nil(t, got.ParallelTooCalls)
	assert.Nil(t, got.StreamOptions)
	assert.Empty(t, got.ReasoningEffort)
	assert.Nil(t, got.TopLogProbs)
}

func TestResponsesRequestToChatCompletionsRequestKeepsFieldsByDefault(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model:          "gpt-test",
		Input:          mustRawMessage(t, "hello"),
		Metadata:       mustRawMessage(t, map[string]any{"codex": "trace"}),
		Store:          mustRawMessage(t, false),
		ServiceTier:    "auto",
		PromptCacheKey: mustRawMessage(t, "cache-key"),
		Reasoning:      &dto.Reasoning{Effort: "high"},
	}, ResponsesRequestToChatOptions{})
	require.NoError(t, err)
	assert.Equal(t, `{"codex":"trace"}`, string(got.Metadata))
	assert.Equal(t, "auto", strings.Trim(string(got.ServiceTier), `"`))
	assert.Equal(t, "cache-key", got.PromptCacheKey)
	assert.Equal(t, "high", got.ReasoningEffort)
}

func TestApplyResponsesToolConflictPolicyDeduplicatesAlias(t *testing.T) {
	mkTools := func(t *testing.T, items ...map[string]any) json.RawMessage {
		t.Helper()
		raw, err := common.Marshal(items)
		require.NoError(t, err)
		return json.RawMessage(raw)
	}

	// Regression: Codex registers a built-in function tool "image_gen.imagegen"
	// while the upstream advertises hosted "image_generation". Without alias
	// normalization the function tool survives and the upstream rejects with
	// "Function 'image_gen.imagegen' conflicts with a hosted tool".
	t.Run("drops function image_gen.imagegen conflicting with hosted image_generation", func(t *testing.T) {
		in := mkTools(t,
			map[string]any{"type": "function", "name": "image_gen.imagegen", "parameters": map[string]any{"type": "object"}},
			map[string]any{"type": "image_generation"},
		)
		out, decisions, err := ApplyResponsesToolConflictPolicy(in, dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate)
		require.NoError(t, err)
		var got []map[string]any
		require.NoError(t, common.Unmarshal(out, &got))
		require.Len(t, got, 1, "conflicting function tool dropped, hosted image_generation kept")
		assert.Equal(t, "image_generation", got[0]["type"])
		require.Len(t, decisions, 1)
		assert.Equal(t, "image_gen.imagegen", decisions[0].ToolName)
	})

	t.Run("drops function web_search.* conflicting with hosted web_search_preview", func(t *testing.T) {
		in := mkTools(t,
			map[string]any{"type": "function", "name": "web_search.search", "parameters": map[string]any{"type": "object"}},
			map[string]any{"type": "web_search_preview"},
		)
		out, _, err := ApplyResponsesToolConflictPolicy(in, dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate)
		require.NoError(t, err)
		var got []map[string]any
		require.NoError(t, common.Unmarshal(out, &got))
		require.Len(t, got, 1)
		assert.Equal(t, "web_search_preview", got[0]["type"])
	})

	t.Run("keeps unrelated function tool alongside hosted tool", func(t *testing.T) {
		in := mkTools(t,
			map[string]any{"type": "function", "name": "calculator", "parameters": map[string]any{"type": "object"}},
			map[string]any{"type": "image_generation"},
		)
		out, decisions, err := ApplyResponsesToolConflictPolicy(in, dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate)
		require.NoError(t, err)
		var got []map[string]any
		require.NoError(t, common.Unmarshal(out, &got))
		require.Len(t, got, 2, "unrelated function tool must be preserved")
		require.Empty(t, decisions)
	})
}

func TestResponsesRequestToolsToChatPreservesEmptyWebSearchForChatUpstream(t *testing.T) {
	tools := mustRawMessage(t, []map[string]any{{"type": "web_search"}, {"type": "web_search_preview"}})
	converted, err := responsesRequestToolsToChat(tools, ResponsesRequestToChatOptions{
		ToolPolicies: ResponsesToolPolicies{WebSearch: ResponsesToolPolicyPreserve},
	})
	require.NoError(t, err)
	require.Len(t, converted, 2)
	for _, tool := range converted {
		require.Equal(t, "web_search", tool.Type)
		require.Empty(t, tool.WebSearch)
		require.Empty(t, tool.Custom)
	}
}

func TestResponsesRequestToolsToChatPreservesExplicitWebSearchOptions(t *testing.T) {
	tools := mustRawMessage(t, []map[string]any{{
		"type":       "web_search",
		"web_search": map[string]any{"enable": true, "search_result": false, "search_engine": "search_std"},
	}})
	converted, err := responsesRequestToolsToChat(tools, ResponsesRequestToChatOptions{
		ToolPolicies: ResponsesToolPolicies{WebSearch: ResponsesToolPolicyPreserve},
	})
	require.NoError(t, err)
	require.Len(t, converted, 1)
	require.Equal(t, false, converted[0].WebSearch["search_result"])
	require.Equal(t, "search_std", converted[0].WebSearch["search_engine"])
}

func TestResponsesRequestToChatCompletionsRequestLeavesWebSearchOptionsUnset(t *testing.T) {
	request, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustRawMessage(t, "search for a fact"),
		Tools: mustRawMessage(t, []map[string]any{{"type": "web_search"}}),
	}, ResponsesRequestToChatOptions{ToolPolicies: ResponsesToolPolicies{WebSearch: ResponsesToolPolicyPreserve}})
	require.NoError(t, err)
	encoded, err := common.Marshal(request)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(encoded, "tools.0.web_search").Exists())
	require.False(t, gjson.GetBytes(encoded, "tools.0.custom").Exists())
}

func TestResponsesRequestToChatCompletionsRequestLeavesWebSearchOptionsUnsetAtIndex297(t *testing.T) {
	tools := make([]map[string]any, 298)
	for i := 0; i < 297; i++ {
		tools[i] = map[string]any{
			"type":       "function",
			"name":       fmt.Sprintf("tool_%d", i),
			"parameters": map[string]any{"type": "object"},
		}
	}
	tools[297] = map[string]any{"type": "web_search"}

	request, err := ResponsesRequestToChatCompletionsRequestWithOptions(&dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustRawMessage(t, "search for a fact"),
		Tools: mustRawMessage(t, tools),
	}, ResponsesRequestToChatOptions{ToolPolicies: ResponsesToolPolicies{WebSearch: ResponsesToolPolicyPreserve}})
	require.NoError(t, err)
	encoded, err := common.Marshal(request)
	require.NoError(t, err)
	require.Equal(t, "web_search", gjson.GetBytes(encoded, "tools.297.type").String())
	require.False(t, gjson.GetBytes(encoded, "tools.297.web_search").Exists())
}
