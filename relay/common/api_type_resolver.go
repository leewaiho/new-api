package common

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// RequestSignature captures request-level signals that can be used to infer
// the desired upstream API type. It is intentionally decoupled from gin.Context
// so the inference rules can be unit-tested and reused outside HTTP handlers.
type RequestSignature struct {
	Path      string
	UserAgent string
	Headers   http.Header
}

// ExpectedAPITypeRule is one link in the inference chain.
// Returning (_, false) means "I cannot decide; pass to the next rule".
type ExpectedAPITypeRule interface {
	Infer(sig RequestSignature) (apiType int, ok bool)
}

// ExpectedAPITypeChain runs rules in order and returns the first confident result.
type ExpectedAPITypeChain []ExpectedAPITypeRule

func (chain ExpectedAPITypeChain) Infer(sig RequestSignature) (int, bool) {
	for _, rule := range chain {
		if apiType, ok := rule.Infer(sig); ok {
			return apiType, true
		}
	}
	return 0, false
}

// CurrentAPITypeInferenceChain can be replaced or appended to for custom behavior
// or testing. By default it uses path-first, then User-Agent.
var CurrentAPITypeInferenceChain ExpectedAPITypeChain = DefaultAPITypeInferenceChain()

// ResolveExpectedAPIType runs the current inference chain.
func ResolveExpectedAPIType(sig RequestSignature) (int, bool) {
	return CurrentAPITypeInferenceChain.Infer(sig)
}

// DefaultAPITypeInferenceChain returns the built-in rule chain.
func DefaultAPITypeInferenceChain() ExpectedAPITypeChain {
	return ExpectedAPITypeChain{
		PathAPITypeRule{},
		UserAgentAPITypeRule{},
	}
}

// PathAPITypeRule infers the expected API type from the request path.
// It is intentionally conservative: only paths with an unambiguous wire
// protocol produce a result.
type PathAPITypeRule struct{}

func (PathAPITypeRule) Infer(sig RequestSignature) (int, bool) {
	switch {
	case strings.HasPrefix(sig.Path, "/v1/messages"):
		return constant.APITypeAnthropic, true
	case strings.HasPrefix(sig.Path, "/v1/chat/completions"),
		strings.HasPrefix(sig.Path, "/v1/completions"),
		strings.HasPrefix(sig.Path, "/v1/responses"),
		strings.HasPrefix(sig.Path, "/v1/embeddings"),
		strings.HasPrefix(sig.Path, "/v1/rerank"):
		return constant.APITypeOpenAI, true
	case strings.HasPrefix(sig.Path, "/v1beta/models/"):
		return constant.APITypeGemini, true
	default:
		return 0, false
	}
}

// UserAgentAPITypeRule infers the expected API type from the User-Agent header.
// It is used as a fallback when the path is ambiguous.
type UserAgentAPITypeRule struct{}

func (UserAgentAPITypeRule) Infer(sig RequestSignature) (int, bool) {
	ua := strings.ToLower(sig.UserAgent)
	if strings.Contains(ua, "claude-cli") || strings.Contains(ua, "anthropic") {
		return constant.APITypeAnthropic, true
	}
	if strings.Contains(ua, "codex") || strings.Contains(ua, "openai") {
		return constant.APITypeOpenAI, true
	}
	return 0, false
}
