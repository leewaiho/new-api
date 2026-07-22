package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustMarshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	out, err := common.Marshal(v)
	require.NoError(t, err)
	return out
}

func TestSanitizeUpstreamRequestBodyRedactsMessagesAndPreservesTools(t *testing.T) {
	upstreamErrorFullBodyEnabled = false

	body := mustMarshalJSON(t, map[string]any{
		"model": "glm-5.2",
		"messages": []any{
			map[string]any{"role": "user", "content": "secret prompt"},
		},
		"tools": []any{
			map[string]any{"type": "function", "function": map[string]any{"name": "view_image"}},
			map[string]any{"type": "shell_command"},
		},
		"stream": true,
	})

	out := sanitizeUpstreamRequestBody(body)

	var parsed map[string]any
	require.NoError(t, common.Unmarshal(out, &parsed))

	assert.Equal(t, "glm-5.2", parsed["model"])
	assert.Equal(t, true, parsed["stream"])

	tools, ok := parsed["tools"].([]any)
	require.True(t, ok, "tools must be preserved verbatim")
	require.Len(t, tools, 2)

	messages, ok := parsed["messages"].(string)
	require.True(t, ok, "messages must be redacted to a string placeholder")
	assert.Contains(t, messages, "redacted")
	assert.Contains(t, messages, "1 items")
}

func TestSanitizeUpstreamRequestBodyRedactsResponsesInput(t *testing.T) {
	upstreamErrorFullBodyEnabled = false

	body := mustMarshalJSON(t, map[string]any{
		"model": "glm-5.2",
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "secret"},
		},
		"tools": []any{
			map[string]any{"type": "web_search"},
		},
	})

	out := sanitizeUpstreamRequestBody(body)

	var parsed map[string]any
	require.NoError(t, common.Unmarshal(out, &parsed))

	tools, ok := parsed["tools"].([]any)
	require.True(t, ok)
	require.Len(t, tools, 1)

	input, ok := parsed["input"].(string)
	require.True(t, ok, "input must be redacted to a string placeholder")
	assert.Contains(t, input, "redacted")
}

func TestSanitizeUpstreamRequestBodyFullModeReturnsVerbatim(t *testing.T) {
	upstreamErrorFullBodyEnabled = true

	body := mustMarshalJSON(t, map[string]any{
		"model":    "glm-5.2",
		"messages": []any{"secret"},
	})

	out := sanitizeUpstreamRequestBody(body)
	assert.Equal(t, body, out)
}

func TestSanitizeUpstreamRequestBodyHandlesEmptyAndInvalid(t *testing.T) {
	upstreamErrorFullBodyEnabled = false

	assert.Empty(t, sanitizeUpstreamRequestBody(nil))

	out := sanitizeUpstreamRequestBody([]byte("not valid json"))
	assert.Contains(t, string(out), "unparseable")
}
