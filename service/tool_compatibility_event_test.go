package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRecordToolCompatibilityEventUsesModelSanitization(t *testing.T) {
	previousDB := model.DB
	t.Cleanup(func() { model.DB = previousDB })

	db, err := gorm.Open(sqlite.Open("file:service_tool_compatibility_event?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}
	if err := db.AutoMigrate(&model.ToolCompatibilityEvent{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	model.DB = db

	event, err := RecordToolCompatibilityEvent(ToolCompatibilityEventInput{
		ChannelId:      16,
		Route:          "/v1/responses",
		RequestedModel: "glm-5.2",
		ToolType:       "image_generation",
		EventType:      model.ToolCompatibilityEventTypePolicyDrop,
		ErrorMessage:   "token=do-not-store",
	})
	if err != nil {
		t.Fatalf("record event: %v", err)
	}
	if strings.Contains(event.SanitizedError, "do-not-store") {
		t.Fatalf("service wrapper leaked raw error: %q", event.SanitizedError)
	}
}
