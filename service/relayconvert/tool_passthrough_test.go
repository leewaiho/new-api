package relayconvert

import (
	"encoding/json"
	"testing"

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
