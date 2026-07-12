package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func issueSessionCookie(t *testing.T, applyDashboardLifetime bool) *http.Cookie {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	store := cookie.NewStore([]byte("dashboard-session-lifetime-test-secret"))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	})
	router.Use(sessions.Sessions("session", store))
	router.GET("/", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", 1)
		if applyDashboardLifetime {
			applyDashboardSessionOptions(session)
		}
		if err := session.Save(); err != nil {
			t.Fatalf("save session: %v", err)
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookie count = %d, want 1", len(cookies))
	}
	return cookies[0]
}

func TestCompletedDashboardLoginUsesConfiguredLifetime(t *testing.T) {
	previousDays := common.DashboardSessionLifetimeDays
	common.DashboardSessionLifetimeDays = 90
	t.Cleanup(func() { common.DashboardSessionLifetimeDays = previousDays })

	cookie := issueSessionCookie(t, true)
	if cookie.MaxAge != 90*24*60*60 {
		t.Fatalf("MaxAge = %d, want %d", cookie.MaxAge, 90*24*60*60)
	}
	if !cookie.HttpOnly || cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("dashboard cookie security attributes changed: %+v", cookie)
	}
}

func TestTemporarySessionDoesNotUseDashboardLifetime(t *testing.T) {
	previousDays := common.DashboardSessionLifetimeDays
	common.DashboardSessionLifetimeDays = 3650
	t.Cleanup(func() { common.DashboardSessionLifetimeDays = previousDays })

	cookie := issueSessionCookie(t, false)
	if cookie.MaxAge != 30*24*60*60 {
		t.Fatalf("temporary session MaxAge = %d, want existing store default %d", cookie.MaxAge, 30*24*60*60)
	}
}
