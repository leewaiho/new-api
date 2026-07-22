package relay

import (
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// upstreamErrorFullBodyEnabled controls whether the full upstream request body
// (including conversation content in input/messages) is logged verbatim on
// upstream errors. When disabled (default) the conversation payload is redacted
// while tools, model and other diagnostic fields are preserved, so tool
// compatibility issues can be diagnosed without persisting user prompts.
var upstreamErrorFullBodyEnabled = os.Getenv("UPSTREAM_ERROR_FULL_BODY") == "true"

const upstreamErrorResponsePreviewLimit = 8192

// CaptureUpstreamError logs the converted upstream request body together with
// the upstream error response when the upstream returns a non-2xx status. It is
// intended for diagnosing request-conversion and tool-compatibility failures
// such as "tools[n].type is illegal". Conversation content (input/messages) is
// redacted unless UPSTREAM_ERROR_FULL_BODY=true.
func CaptureUpstreamError(info *relaycommon.RelayInfo, statusCode int, requestJSON []byte, responseBody []byte) {
	if len(requestJSON) == 0 && len(responseBody) == 0 {
		return
	}
	requestPreview := sanitizeUpstreamRequestBody(requestJSON)
	responsePreview := responseBody
	if len(responsePreview) > upstreamErrorResponsePreviewLimit {
		responsePreview = responsePreview[:upstreamErrorResponsePreviewLimit]
	}
	common.SysError(fmt.Sprintf(
		"upstream error captured | requestId=%s channelId=%d model=%s status=%d | request=%s | response=%s",
		info.RequestId, info.ChannelId, info.OriginModelName, statusCode, requestPreview, responsePreview,
	))
}

// sanitizeUpstreamRequestBody redacts conversation content (input/messages) from
// a converted upstream request body while preserving tools and other diagnostic
// metadata. It returns the original body verbatim when full-body capture is
// enabled.
func sanitizeUpstreamRequestBody(requestJSON []byte) []byte {
	if upstreamErrorFullBodyEnabled || len(requestJSON) == 0 {
		return requestJSON
	}
	var obj map[string]any
	if err := common.Unmarshal(requestJSON, &obj); err != nil {
		return []byte(fmt.Sprintf("<unparseable upstream request body, %d bytes>", len(requestJSON)))
	}
	for _, key := range []string{"input", "messages"} {
		if v, ok := obj[key]; ok {
			obj[key] = redactConversationValue(v)
		}
	}
	out, err := common.Marshal(obj)
	if err != nil {
		return []byte("<failed to re-marshal sanitized upstream request body>")
	}
	return out
}

func redactConversationValue(v any) any {
	switch val := v.(type) {
	case []any:
		return fmt.Sprintf("<redacted: %d items>", len(val))
	case string:
		return fmt.Sprintf("<redacted: %d chars>", len(val))
	default:
		return "<redacted>"
	}
}
