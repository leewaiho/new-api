package auth_setting

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-contrib/sessions/cookie"
)

func TestParseDashboardSessionLifetimeDays(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "minimum", raw: "1", want: 1},
		{name: "default", raw: "30", want: 30},
		{name: "maximum", raw: "3650", want: 3650},
		{name: "zero", raw: "0", wantErr: true},
		{name: "negative", raw: "-1", wantErr: true},
		{name: "too large", raw: "3651", wantErr: true},
		{name: "non numeric", raw: "forever", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDashboardSessionLifetimeDays(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseDashboardSessionLifetimeDays(%q) unexpectedly succeeded with %d", tt.raw, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("ParseDashboardSessionLifetimeDays(%q) = (%d, %v), want (%d, nil)", tt.raw, got, err, tt.want)
			}
		})
	}
}

func TestDashboardSessionOptionsPreservesCookieSecurityAttributes(t *testing.T) {
	options := DashboardSessionOptions(90)
	if options.MaxAge != 90*24*60*60 {
		t.Fatalf("MaxAge = %d, want %d", options.MaxAge, 90*24*60*60)
	}
	if options.Path != "/" || !options.HttpOnly || options.Secure || options.SameSite != http.SameSiteStrictMode {
		t.Fatalf("dashboard cookie security options changed: %+v", options)
	}
}

func TestDashboardSessionCodecMaxAgeCoversMaximumLifetime(t *testing.T) {
	want := MaxDashboardSessionLifetimeDays * 24 * 60 * 60
	if got := DashboardSessionCodecMaxAgeSeconds(); got != want {
		t.Fatalf("DashboardSessionCodecMaxAgeSeconds() = %d, want %d", got, want)
	}
}

func TestConfigureDashboardSessionCookieStoreUpdatesCodecMaxAge(t *testing.T) {
	store := cookie.NewStore([]byte("dashboard-session-codec-test-secret"))
	maxAgeSetter, ok := store.(sessionCookieStoreMaxAgeSetter)
	if !ok {
		t.Fatal("cookie store does not expose MaxAge")
	}

	// Reproduce the original mismatch with a short decoder lifetime. The
	// configuration under test must extend the codec lifetime while preserving
	// the default 30-day options for temporary sessions.
	maxAgeSetter.MaxAge(1)
	if err := ConfigureDashboardSessionCookieStore(store); err != nil {
		t.Fatalf("ConfigureDashboardSessionCookieStore() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	session, err := store.New(request, "session")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defaultMaxAge := DefaultDashboardSessionLifetimeDays * 24 * 60 * 60
	if session.Options.MaxAge != defaultMaxAge {
		t.Fatalf("temporary session MaxAge = %d, want %d", session.Options.MaxAge, defaultMaxAge)
	}

	session.Values["id"] = 1
	session.Options.MaxAge = 10
	recorder := httptest.NewRecorder()
	if err := store.Save(request, recorder, session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookie count = %d, want 1", len(cookies))
	}

	time.Sleep(2200 * time.Millisecond)
	reloadRequest := httptest.NewRequest(http.MethodGet, "http://example.test/", nil)
	reloadRequest.AddCookie(cookies[0])
	reloaded, err := store.New(reloadRequest, "session")
	if err != nil {
		t.Fatalf("decode session after original codec limit: %v", err)
	}
	if got := reloaded.Values["id"]; got != 1 {
		t.Fatalf("reloaded session id = %v, want 1", got)
	}
}
