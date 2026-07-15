package model

import (
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var toolCompatibilityEventTestMu sync.Mutex

func withToolCompatibilityEventTestDB(t *testing.T) {
	t.Helper()
	toolCompatibilityEventTestMu.Lock()
	previousDB := DB
	previousType := common.MainDatabaseType()
	db, err := gorm.Open(sqlite.Open("file:tool_compatibility_event_"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		toolCompatibilityEventTestMu.Unlock()
		t.Fatalf("open sqlite test database: %v", err)
	}
	if err := db.AutoMigrate(&ToolCompatibilityEvent{}); err != nil {
		toolCompatibilityEventTestMu.Unlock()
		t.Fatalf("migrate test database: %v", err)
	}
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
		toolCompatibilityEventTestMu.Unlock()
	})
}

func validToolCompatibilityEventInput() ToolCompatibilityEventInput {
	return ToolCompatibilityEventInput{
		ChannelId:      16,
		Route:          "/v1/responses",
		RequestedModel: "glm-5.2",
		UpstreamModel:  "glm-5.2",
		ToolType:       "image_generation",
		EventType:      ToolCompatibilityEventTypePolicyDrop,
		CurrentPolicy:  "drop",
		ErrorMessage:   "Responses tool image_generation was removed by the configured policy",
	}
}

func TestToolCompatibilityEventKeyIsStableAndModelScoped(t *testing.T) {
	input := validToolCompatibilityEventInput()
	fingerprint := ToolCompatibilityErrorFingerprint("same error")
	first := ToolCompatibilityEventKey(input, fingerprint)
	second := ToolCompatibilityEventKey(input, fingerprint)
	if first != second {
		t.Fatalf("event key is unstable: %q != %q", first, second)
	}
	input.RequestedModel = "glm-5.3"
	if first == ToolCompatibilityEventKey(input, fingerprint) {
		t.Fatal("event key merged distinct requested models")
	}
}

func TestSanitizeToolCompatibilityErrorRedactsSecretsAndStructuredPayloads(t *testing.T) {
	structured := `upstream error Authorization: Bearer sk-secret token=abc body={"tools":[{"name":"shell"}]}`
	if got := SanitizeToolCompatibilityError(structured); got != "[redacted structured error]" {
		t.Fatalf("structured error = %q", got)
	}
	plain := "upstream rejected Authorization: Bearer sk-secret-token api_key=abc123 password=hunter2"
	got := SanitizeToolCompatibilityError(plain)
	for _, forbidden := range []string{"sk-secret-token", "abc123", "hunter2"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("sanitized error leaked %q: %q", forbidden, got)
		}
	}
}

func TestSanitizeToolCompatibilityErrorTruncatesUnicodeWithoutInvalidUTF8(t *testing.T) {
	message := strings.Repeat("错", 600)
	got := SanitizeToolCompatibilityError(message)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("sanitized error was not truncated: %q", got)
	}
	if len([]rune(strings.TrimSuffix(got, "…"))) != 512 {
		t.Fatalf("sanitized rune count = %d, want 512", len([]rune(strings.TrimSuffix(got, "…"))))
	}
	if !utf8.ValidString(got) {
		t.Fatal("sanitized error contains invalid UTF-8")
	}
}

func TestRecordToolCompatibilityEventAggregatesMatchingEvents(t *testing.T) {
	withToolCompatibilityEventTestDB(t)
	input := validToolCompatibilityEventInput()
	first, err := RecordToolCompatibilityEvent(input)
	if err != nil {
		t.Fatalf("record first event: %v", err)
	}
	second, err := RecordToolCompatibilityEvent(input)
	if err != nil {
		t.Fatalf("record second event: %v", err)
	}
	if first.Id != second.Id || second.OccurrenceCount != 2 {
		t.Fatalf("aggregate result = %#v, want same row with occurrence_count=2", second)
	}
	var count int64
	if err := DB.Model(&ToolCompatibilityEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 1 {
		t.Fatalf("event row count = %d, want 1", count)
	}
}

func TestRecordToolCompatibilityEventDoesNotMergeDifferentModels(t *testing.T) {
	withToolCompatibilityEventTestDB(t)
	input := validToolCompatibilityEventInput()
	if _, err := RecordToolCompatibilityEvent(input); err != nil {
		t.Fatalf("record first event: %v", err)
	}
	input.RequestedModel = "glm-5.3"
	if _, err := RecordToolCompatibilityEvent(input); err != nil {
		t.Fatalf("record second model event: %v", err)
	}
	var count int64
	if err := DB.Model(&ToolCompatibilityEvent{}).Count(&count).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 2 {
		t.Fatalf("event row count = %d, want 2", count)
	}
}

func TestRecordToolCompatibilityEventRejectsInvalidInput(t *testing.T) {
	withToolCompatibilityEventTestDB(t)
	input := validToolCompatibilityEventInput()
	input.EventType = "unsupported_value"
	if _, err := RecordToolCompatibilityEvent(input); err == nil || !strings.Contains(err.Error(), "unknown event_type") {
		t.Fatalf("invalid event type error = %v", err)
	}
}

func TestRecordToolCompatibilityEventAggregatesConcurrentWrites(t *testing.T) {
	withToolCompatibilityEventTestDB(t)
	input := validToolCompatibilityEventInput()
	const writers = 12

	errors := make(chan error, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := RecordToolCompatibilityEvent(input)
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("record concurrent event: %v", err)
		}
	}

	var events []ToolCompatibilityEvent
	if err := DB.Find(&events).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event row count = %d, want 1", len(events))
	}
	if events[0].OccurrenceCount != writers {
		t.Fatalf("occurrence_count = %d, want %d", events[0].OccurrenceCount, writers)
	}
}

func TestRecordToolCompatibilityEventRefreshesPoliciesAndReopensResolvedEvent(t *testing.T) {
	withToolCompatibilityEventTestDB(t)
	input := validToolCompatibilityEventInput()
	input.CurrentPolicy = "preserve"
	input.SuggestedPolicy = "drop"
	first, err := RecordToolCompatibilityEvent(input)
	if err != nil {
		t.Fatalf("record first event: %v", err)
	}
	if _, err := UpdateToolCompatibilityEventResolutionStatus(first.Id, ToolCompatibilityResolutionStatusResolved); err != nil {
		t.Fatalf("resolve event: %v", err)
	}

	input.CurrentPolicy = "reject"
	input.SuggestedPolicy = "preserve"
	second, err := RecordToolCompatibilityEvent(input)
	if err != nil {
		t.Fatalf("record recurring event: %v", err)
	}
	if second.Id != first.Id {
		t.Fatalf("recurring event id = %d, want %d", second.Id, first.Id)
	}
	if second.OccurrenceCount != 2 {
		t.Fatalf("occurrence_count = %d, want 2", second.OccurrenceCount)
	}
	if second.CurrentPolicy != "reject" || second.SuggestedPolicy != "preserve" {
		t.Fatalf("latest policies not refreshed: current=%q suggested=%q", second.CurrentPolicy, second.SuggestedPolicy)
	}
	if second.ResolutionStatus != ToolCompatibilityResolutionStatusOpen {
		t.Fatalf("recurring resolved event status = %q, want open", second.ResolutionStatus)
	}
}

func TestToolCompatibilityEventMigrationCreatesLookupIndexes(t *testing.T) {
	withToolCompatibilityEventTestDB(t)
	migrator := DB.Migrator()
	for _, field := range []string{"EventKey", "ChannelId", "RequestedModel", "EventType", "LastSeenAt", "ResolutionStatus"} {
		if !migrator.HasIndex(&ToolCompatibilityEvent{}, field) {
			t.Fatalf("missing index for %s", field)
		}
	}
}
