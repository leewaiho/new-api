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
