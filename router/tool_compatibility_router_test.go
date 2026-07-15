package router

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/QuantumNous/new-api/controller"
	"github.com/stretchr/testify/require"
)

func TestToolCompatibilityRoutesEnforceReadAndMutationRoles(t *testing.T) {
	tests := []struct {
		method  string
		path    string
		access  toolCompatibilityRouteAccess
		handler any
	}{
		{http.MethodGet, "/events", toolCompatibilityRouteAccessAdmin, controller.ListToolCompatibilityEvents},
		{http.MethodPatch, "/events/:id/status", toolCompatibilityRouteAccessRoot, controller.UpdateToolCompatibilityEventStatus},
		{http.MethodPost, "/events/:id/apply-suggestion", toolCompatibilityRouteAccessRoot, controller.ApplyToolCompatibilityEventSuggestion},
		{http.MethodPost, "/events/:id/restore-default", toolCompatibilityRouteAccessRoot, controller.RestoreToolCompatibilityEventModelDefault},
	}
	require.Len(t, toolCompatibilityRouteSpecs, len(tests))
	for _, want := range tests {
		found := false
		for _, got := range toolCompatibilityRouteSpecs {
			if got.method != want.method || got.path != want.path {
				continue
			}
			found = true
			require.Equal(t, want.access, got.access)
			require.Equal(t, reflect.ValueOf(want.handler).Pointer(), reflect.ValueOf(got.handler).Pointer())
		}
		require.Truef(t, found, "route %s %s not found", want.method, want.path)
	}
}
