package auth_setting

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	OptionDashboardSessionLifetimeDays  = "DashboardSessionLifetimeDays"
	DefaultDashboardSessionLifetimeDays = 30
	MaxDashboardSessionLifetimeDays     = 3650
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
