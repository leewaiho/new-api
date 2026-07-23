package relayconvert

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeResponsesInputToolCallItemIDsUsesResponsesPrefixes(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{
		Input: mustRawMessage(t, []map[string]any{
			{
				"type":      "function_call",
				"id":        "call_shell",
				"call_id":   "call_shell",
				"name":      "shell_command",
				"arguments": `{"command":"pwd"}`,
			},
			{
				"type":    "custom_tool_call",
				"id":      "call_patch",
				"call_id": "call_patch",
				"name":    "apply_patch",
				"input":   "*** Begin Patch\n*** End Patch",
			},
			{
				"type":      "function_call",
				"id":        "fc_existing",
				"call_id":   "call_existing",
				"name":      "lookup",
				"arguments": `{}`,
			},
			{
				"type":    "custom_tool_call",
				"id":      "ctc_existing",
				"call_id": "call_existing_custom",
				"name":    "apply_patch",
				"input":   "*** Begin Patch\n*** End Patch",
			},
			{
				"type":      "function_call",
				"call_id":   "call_missing_item_id",
				"name":      "lookup",
				"arguments": `{}`,
			},
			{
				"type":    "function_call_output",
				"call_id": "call_shell",
				"output":  "/repo",
			},
		}),
	}

	require.NoError(t, NormalizeResponsesInputToolCallItemIDs(request))

	var input []map[string]any
	require.NoError(t, common.Unmarshal(request.Input, &input))
	assert.Equal(t, "fc_call_shell", input[0]["id"])
	assert.Equal(t, "call_shell", input[0]["call_id"])
	assert.Equal(t, "ctc_call_patch", input[1]["id"])
	assert.Equal(t, "call_patch", input[1]["call_id"])
	assert.Equal(t, "fc_existing", input[2]["id"])
	assert.Equal(t, "ctc_existing", input[3]["id"])
	assert.Equal(t, "fc_call_missing_item_id", input[4]["id"])
	assert.NotContains(t, input[5], "id")
	assert.Equal(t, "call_shell", input[5]["call_id"])
}

func TestNormalizeResponsesInputToolCallItemIDsPreservesNonArrayInput(t *testing.T) {
	request := &dto.OpenAIResponsesRequest{Input: mustRawMessage(t, "hello")}

	require.NoError(t, NormalizeResponsesInputToolCallItemIDs(request))
	assert.JSONEq(t, `"hello"`, string(request.Input))
}
