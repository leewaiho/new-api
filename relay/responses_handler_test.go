package relay

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/types"
)

func TestNewResponsesConvertRequestErrorReturnsBadRequest(t *testing.T) {
	err := newResponsesConvertRequestError(errors.New("invalid Responses tool choice"))

	if err.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, err.StatusCode)
	}
	if err.GetErrorCode() != types.ErrorCodeConvertRequestFailed {
		t.Fatalf("expected error code %q, got %q", types.ErrorCodeConvertRequestFailed, err.GetErrorCode())
	}
	if !types.IsSkipRetryError(err) {
		t.Fatal("expected conversion errors to skip retry")
	}
}
