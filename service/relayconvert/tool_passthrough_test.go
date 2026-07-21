package relayconvert

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyResponsesToolPoliciesForPassthroughKeepsFunctionTools(t *testing.T) {
	raw := mustRawMessage(t, []map[string]any{
		{"type": "function", "name": "shell"},
	})

	got := ApplyResponsesToolPoliciesForPassthrough(raw, ResponsesToolPolicies{
		Unknown: ResponsesToolPolicyDrop,
	})

	assert.Equal(t, string(raw), string(got), "function-only tools must retain their original bytes")
}

func TestApplyResponsesToolPoliciesForPassthroughKeepsClientDefinedToolTypes(t *testing.T) {
	raw := json.RawMessage(`[
  {"type":"function","name":"shell"},
  {"type":"namespace","name":"mcp__demo__","tools":[{"type":"function","name":"lookup"}]},
  {"type":"custom","name":"apply_patch"},
  {"type":"future_client_tool","name":"future"}
]`)

	got := ApplyResponsesToolPoliciesForPassthrough(raw, ResponsesToolPolicies{
		Namespace:       ResponsesToolPolicyPreserve,
		Custom:          ResponsesToolPolicyPreserve,
		WebSearch:       ResponsesToolPolicyDrop,
		ToolSearch:      ResponsesToolPolicyDrop,
		ImageGeneration: ResponsesToolPolicyDrop,
		Unknown:         ResponsesToolPolicyPreserve,
	})

	assert.Equal(t, raw, got, "client-defined tools must not be rewritten or removed")
}

func TestApplyResponsesToolPoliciesForPassthroughKeepsNamespaceForFlattenPolicy(t *testing.T) {
	raw := json.RawMessage(`[
  {"type":"namespace","name":"mcp__demo__","tools":[{"type":"function","name":"lookup"}]}
]`)

	got := ApplyResponsesToolPoliciesForPassthrough(raw, ResponsesToolPolicies{
		Namespace: ResponsesToolPolicyFlatten,
		Unknown:   ResponsesToolPolicyDrop,
	})

	assert.Equal(t, raw, got, "passthrough cannot flatten namespace tools and must preserve their raw shape")
}

func TestApplyResponsesToolPoliciesForPassthroughDropsHostedTools(t *testing.T) {
	raw := mustRawMessage(t, []map[string]any{
		{"type": "function", "name": "shell"},
		{"type": "image_gen"},
		{"type": "web_search"},
	})

	got := ApplyResponsesToolPoliciesForPassthrough(raw, ResponsesToolPolicies{
		ImageGeneration: ResponsesToolPolicyDrop,
		WebSearch:       ResponsesToolPolicyDrop,
		Unknown:         ResponsesToolPolicyPreserve,
	})

	assert.JSONEq(t, `[{"type":"function","name":"shell"}]`, string(got))
}

func TestApplyResponsesToolPoliciesForPassthroughTreatsRejectAsDrop(t *testing.T) {
	raw := mustRawMessage(t, []map[string]any{{"type": "web_search"}})

	got := ApplyResponsesToolPoliciesForPassthrough(raw, ResponsesToolPolicies{
		WebSearch: ResponsesToolPolicyReject,
	})

	assert.JSONEq(t, `[]`, string(got))
}

func TestApplyResponsesToolPoliciesForPassthroughPreservesAllTools(t *testing.T) {
	raw := json.RawMessage(`[
  {"type":"function","name":"shell"},
  {"type":"image_gen"}
]`)

	got := ApplyResponsesToolPoliciesForPassthrough(raw, ResponsesToolPolicies{
		Namespace:       ResponsesToolPolicyPreserve,
		Custom:          ResponsesToolPolicyPreserve,
		WebSearch:       ResponsesToolPolicyPreserve,
		ToolSearch:      ResponsesToolPolicyPreserve,
		ImageGeneration: ResponsesToolPolicyPreserve,
		Unknown:         ResponsesToolPolicyPreserve,
	})

	assert.Equal(t, raw, got, "unchanged tools must retain their original bytes")
}

func TestDeduplicateConflictingToolsRemovesOnlyActualConflict(t *testing.T) {
	raw := mustRawMessage(t, []map[string]any{
		{"type": "function", "name": "shell"},
		{"type": "function", "name": "image_gen.imagegen"},
		{"type": "image_gen"},
	})

	got := DeduplicateConflictingTools(raw)

	assert.JSONEq(t, `[
		{"type":"function","name":"shell"},
		{"type":"image_gen"}
	]`, string(got))
}

func TestDeduplicateConflictingToolsIgnoresNonHostedToolTypes(t *testing.T) {
	raw := json.RawMessage(`[
  {"type":"function","name":"namespace.lookup"},
  {"type":"function","name":"custom.apply_patch"},
  {"type":"function","name":"future_client_tool.run"},
  {"type":"namespace","name":"namespace"},
  {"type":"custom","name":"custom"},
  {"type":"future_client_tool","name":"future_client_tool"}
]`)

	got := DeduplicateConflictingTools(raw)

	assert.Equal(t, raw, got, "non-hosted tool types must not remove same-prefix functions")
}

func TestDeduplicateConflictingToolsKeepsFunctionWithoutHostedConflict(t *testing.T) {
	raw := json.RawMessage(`[
  {"type":"function","name":"image_gen.imagegen"}
]`)

	got := DeduplicateConflictingTools(raw)

	assert.Equal(t, raw, got, "no actual hosted-tool conflict means no mutation")
}

func TestPassthroughToolHelpersReturnOriginalBytesForInvalidJSON(t *testing.T) {
	raw := json.RawMessage(`[{`)

	filtered := ApplyResponsesToolPoliciesForPassthrough(raw, ResponsesToolPolicies{
		Unknown: ResponsesToolPolicyDrop,
	})
	deduplicated := DeduplicateConflictingTools(raw)

	require.Equal(t, raw, filtered)
	require.Equal(t, raw, deduplicated)
}

func TestApplyResponsesToolPoliciesKeepsFunctionWhenHostedConflictIsDropped(t *testing.T) {
	raw := mustRawMessage(t, []map[string]any{
		{"type": "function", "name": "image_gen.imagegen"},
		{"type": "image_gen"},
	})

	filtered, decisions, err := ApplyResponsesToolPolicies(raw, func(toolType string, toolName string) string {
		if toolType == "image_gen" {
			return ResponsesToolPolicyDrop
		}
		return ResponsesToolPolicyPreserve
	})
	require.NoError(t, err)
	require.Len(t, decisions, 1)

	got, _, err := ApplyResponsesToolConflictPolicy(filtered, dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"type":"function","name":"image_gen.imagegen"}]`, string(got))
}

func TestApplyResponsesToolPoliciesRejectsAllToolsRemoved(t *testing.T) {
	raw := mustRawMessage(t, []map[string]any{{"type": "image_gen"}})

	_, decisions, err := ApplyResponsesToolPolicies(raw, func(toolType string, toolName string) string {
		return ResponsesToolPolicyDrop
	})
	require.ErrorContains(t, err, "All Responses tools were removed")
	require.Len(t, decisions, 1)
}

func TestApplyResponsesToolPoliciesRejectsConfiguredTool(t *testing.T) {
	raw := mustRawMessage(t, []map[string]any{{"type": "image_gen"}})

	_, decisions, err := ApplyResponsesToolPolicies(raw, func(toolType string, toolName string) string {
		return ResponsesToolPolicyReject
	})
	require.ErrorContains(t, err, "rejected by the Advanced Custom route policy")
	require.Len(t, decisions, 1)
}

func TestApplyResponsesToolConflictPolicy(t *testing.T) {
	raw := mustRawMessage(t, []map[string]any{
		{"type": "function", "name": "image_gen.imagegen"},
		{"type": "image_gen"},
	})

	preserved, decisions, err := ApplyResponsesToolConflictPolicy(raw, dto.AdvancedCustomResponsesToolConflictPolicyPreserve)
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(raw), preserved)
	assert.Empty(t, decisions)

	deduplicated, decisions, err := ApplyResponsesToolConflictPolicy(raw, dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"type":"image_gen"}]`, string(deduplicated))
	require.Len(t, decisions, 1)

	_, decisions, err = ApplyResponsesToolConflictPolicy(raw, dto.AdvancedCustomResponsesToolConflictPolicyReject)
	require.ErrorContains(t, err, "conflicts with hosted tool")
	require.Len(t, decisions, 1)
}

func TestApplyResponsesToolConflictPolicyUsesImplicitHostedCapabilities(t *testing.T) {
	raw := mustRawMessage(t, []map[string]any{
		{
			"type":  "namespace",
			"name":  "image_gen",
			"tools": []map[string]any{{"type": "function", "name": "imagegen"}},
		},
		{"type": "function", "name": "image_gen.imagegen"},
		{"type": "namespace", "name": "mcp__demo", "tools": []map[string]any{}},
		{"type": "function", "name": "shell"},
		{"type": "web_search"},
	})

	got, decisions, err := ApplyResponsesToolConflictPolicyWithImplicitHostedTools(
		raw,
		dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate,
		[]string{"image_generation"},
	)
	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"type":"namespace","name":"mcp__demo","tools":[]},
		{"type":"function","name":"shell"},
		{"type":"web_search"}
	]`, string(got))
	require.Len(t, decisions, 2)
	assert.Equal(t, ResponsesToolPolicyDecision{
		ToolType: "namespace",
		ToolName: "image_gen",
		Policy:   dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate,
	}, decisions[0])
	assert.Equal(t, ResponsesToolPolicyDecision{
		ToolType: "function",
		ToolName: "image_gen.imagegen",
		Policy:   dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate,
	}, decisions[1])
}

func TestValidateResponsesToolChoiceAfterPolicy(t *testing.T) {
	choice := mustRawMessage(t, map[string]any{"type": "image_gen"})
	decisions := []ResponsesToolPolicyDecision{
		{ToolType: "image_gen", ToolName: "image_gen", Policy: ResponsesToolPolicyDrop},
	}

	err := ValidateResponsesToolChoiceAfterPolicy(choice, decisions)
	require.ErrorContains(t, err, "tool_choice selects a tool removed by the channel policy")
}

func TestValidateResponsesToolChoiceAfterPolicyMatchesNamespaceFunction(t *testing.T) {
	choice := mustRawMessage(t, map[string]any{"type": "function", "name": "image_gen.imagegen"})
	decisions := []ResponsesToolPolicyDecision{
		{
			ToolType: "namespace",
			ToolName: "image_gen",
			Policy:   dto.AdvancedCustomResponsesToolConflictPolicyDeduplicate,
		},
	}

	err := ValidateResponsesToolChoiceAfterPolicy(choice, decisions)
	require.ErrorContains(t, err, "tool_choice selects a tool removed by the channel policy")
}

func TestApplyResponsesToolPoliciesMatchesUnnamedNativeTypeAndRejectsToolChoice(t *testing.T) {
	raw := mustRawMessage(t, []map[string]any{
		{"type": "shell_command"},
		{"type": "apply_patch"},
	})
	filtered, decisions, err := ApplyResponsesToolPolicies(raw, func(toolType string, toolName string) string {
		if toolType == "shell_command" && toolName == "" {
			return ResponsesToolPolicyDrop
		}
		return ResponsesToolPolicyPreserve
	})
	require.NoError(t, err)
	assert.JSONEq(t, `[{"type":"apply_patch"}]`, string(filtered))
	require.Equal(t, []ResponsesToolPolicyDecision{{
		ToolIndex: 0,
		ToolType:  "shell_command",
		Policy:    ResponsesToolPolicyDrop,
	}}, decisions)

	err = ValidateResponsesToolChoiceAfterPolicy(
		mustRawMessage(t, map[string]any{"type": "shell_command"}),
		decisions,
	)
	require.ErrorContains(t, err, "tool_choice selects a tool removed by the channel policy")
}
