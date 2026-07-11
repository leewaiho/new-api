package model

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/auth_setting"
)

func TestDashboardSessionLifetimeOptionMap(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	previousDays := common.DashboardSessionLifetimeDays
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
		common.DashboardSessionLifetimeDays = previousDays
	})

	if got := common.DashboardSessionLifetimeDays; got != auth_setting.DefaultDashboardSessionLifetimeDays {
		t.Fatalf("default DashboardSessionLifetimeDays = %d, want %d", got, auth_setting.DefaultDashboardSessionLifetimeDays)
	}

	if err := updateOptionMap(auth_setting.OptionDashboardSessionLifetimeDays, "90"); err != nil {
		t.Fatalf("updateOptionMap(valid lifetime): %v", err)
	}
	if got := common.DashboardSessionLifetimeDays; got != 90 {
		t.Fatalf("DashboardSessionLifetimeDays = %d, want 90", got)
	}

	common.OptionMapRWMutex.RLock()
	stored := common.OptionMap[auth_setting.OptionDashboardSessionLifetimeDays]
	common.OptionMapRWMutex.RUnlock()
	if stored != strconv.Itoa(90) {
		t.Fatalf("OptionMap value = %q, want %q", stored, "90")
	}
}

func TestDashboardSessionLifetimeOptionMapRejectsInvalidValue(t *testing.T) {
	previousDays := common.DashboardSessionLifetimeDays
	t.Cleanup(func() { common.DashboardSessionLifetimeDays = previousDays })

	if err := updateOptionMap(auth_setting.OptionDashboardSessionLifetimeDays, "0"); err == nil {
		t.Fatal("updateOptionMap accepted zero-day dashboard session lifetime")
	}
	if got := common.DashboardSessionLifetimeDays; got != previousDays {
		t.Fatalf("DashboardSessionLifetimeDays changed to %d after invalid update, want %d", got, previousDays)
	}
}
