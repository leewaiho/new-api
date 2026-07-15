package auth_setting

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
)

const (
	OptionDashboardSessionLifetimeDays  = "DashboardSessionLifetimeDays"
	DefaultDashboardSessionLifetimeDays = 30
	MaxDashboardSessionLifetimeDays     = 3650
	secondsPerDay                       = 24 * 60 * 60
)

type dashboardSessionCookieStore interface {
	Options(sessions.Options)
}

type sessionCookieStoreMaxAgeSetter interface {
	MaxAge(int)
}

func ParseDashboardSessionLifetimeDays(raw string) (int, error) {
	days, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("dashboard session lifetime must be an integer number of days")
	}
	if days < 1 || days > MaxDashboardSessionLifetimeDays {
		return 0, fmt.Errorf("dashboard session lifetime must be between 1 and %d days", MaxDashboardSessionLifetimeDays)
	}
	return days, nil
}

func NormalizeDashboardSessionLifetimeDays(days int) int {
	if days < 1 || days > MaxDashboardSessionLifetimeDays {
		return DefaultDashboardSessionLifetimeDays
	}
	return days
}

func DashboardSessionCodecMaxAgeSeconds() int {
	return MaxDashboardSessionLifetimeDays * secondsPerDay
}

func ConfigureDashboardSessionCookieStore(store dashboardSessionCookieStore) error {
	maxAgeSetter, ok := store.(sessionCookieStoreMaxAgeSetter)
	if !ok {
		return fmt.Errorf("session cookie store does not support codec max age configuration")
	}

	// Gorilla CookieStore validates the signed cookie timestamp using a
	// store-level codec MaxAge. Keep that decoder window large enough for the
	// longest supported dashboard session, then restore the default cookie
	// options used by temporary OAuth/2FA sessions.
	maxAgeSetter.MaxAge(DashboardSessionCodecMaxAgeSeconds())
	store.Options(DashboardSessionOptions(DefaultDashboardSessionLifetimeDays))
	return nil
}

func DashboardSessionOptions(days int) sessions.Options {
	days = NormalizeDashboardSessionLifetimeDays(days)
	return sessions.Options{
		Path:     "/",
		MaxAge:   days * secondsPerDay,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	}
}
