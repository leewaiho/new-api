package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ToolCompatibilityEventTypeUpstreamUnsupported = "upstream_unsupported"
	ToolCompatibilityEventTypeNameConflict        = "name_conflict"
	ToolCompatibilityEventTypePolicyDrop          = "policy_drop"
	ToolCompatibilityEventTypePolicyReject        = "policy_reject"
	ToolCompatibilityEventTypeInvalidToolSchema   = "invalid_tool_schema"
	ToolCompatibilityEventTypeUnclassified        = "unclassified"
	ToolCompatibilityEventTypeAcceptedDefinition  = "accepted_definition"
	ToolCompatibilityEventTypeInvoked             = "invoked"

	ToolCompatibilityResolutionStatusOpen     = "open"
	ToolCompatibilityResolutionStatusIgnored  = "ignored"
	ToolCompatibilityResolutionStatusResolved = "resolved"
)

const toolCompatibilityEventMaxRecordAttempts = 10

var (
	ErrInvalidToolCompatibilityEvent = errors.New("invalid tool compatibility event")

	toolCompatibilitySensitiveAssignmentPattern = regexp.MustCompile(`(?i)\b(authorization|api[_-]?key|token|secret|password)\b\s*[:=]\s*[^\s,;]+`)
	toolCompatibilityBearerPattern              = regexp.MustCompile(`(?i)\bbearer\s+[^\s,;]+`)
	toolCompatibilityLongTokenPattern           = regexp.MustCompile(`\b(?:sk|sess|token|key)-[A-Za-z0-9._-]{8,}\b`)
	toolCompatibilityWhitespacePattern          = regexp.MustCompile(`\s+`)
)

type ToolCompatibilityEvent struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	EventKey         string `json:"event_key" gorm:"type:char(64);uniqueIndex;not null"`
	ChannelId        int    `json:"channel_id" gorm:"index;not null"`
	Route            string `json:"route" gorm:"type:varchar(255);not null"`
	RequestedModel   string `json:"requested_model" gorm:"type:varchar(255);index;not null;default:''"`
	UpstreamModel    string `json:"upstream_model" gorm:"type:varchar(255);not null;default:''"`
	ToolType         string `json:"tool_type" gorm:"type:varchar(128);not null;default:''"`
	ToolName         string `json:"tool_name" gorm:"type:varchar(255);not null;default:''"`
	EventType        string `json:"event_type" gorm:"type:varchar(64);index;not null"`
	CurrentPolicy    string `json:"current_policy" gorm:"type:varchar(32);not null;default:''"`
	SuggestedPolicy  string `json:"suggested_policy" gorm:"type:varchar(32);not null;default:''"`
	ErrorFingerprint string `json:"error_fingerprint" gorm:"type:char(64);not null"`
	SanitizedError   string `json:"sanitized_error" gorm:"type:text;not null;default:''"`
	OccurrenceCount  int    `json:"occurrence_count" gorm:"not null;default:1"`
	FirstSeenAt      int64  `json:"first_seen_at" gorm:"bigint;not null"`
	LastSeenAt       int64  `json:"last_seen_at" gorm:"bigint;index;not null"`
	ResolutionStatus string `json:"resolution_status" gorm:"type:varchar(32);index;not null;default:'open'"`
}

type ToolCompatibilityEventInput struct {
	ChannelId       int
	Route           string
	RequestedModel  string
	UpstreamModel   string
	ToolType        string
	ToolName        string
	EventType       string
	CurrentPolicy   string
	SuggestedPolicy string
	ErrorMessage    string
}

func (input ToolCompatibilityEventInput) normalized() (ToolCompatibilityEventInput, error) {
	input.Route = strings.TrimSpace(input.Route)
	input.RequestedModel = strings.TrimSpace(input.RequestedModel)
	input.UpstreamModel = strings.TrimSpace(input.UpstreamModel)
	input.ToolType = strings.TrimSpace(input.ToolType)
	input.ToolName = strings.TrimSpace(input.ToolName)
	input.EventType = strings.TrimSpace(input.EventType)
	input.CurrentPolicy = strings.TrimSpace(input.CurrentPolicy)
	input.SuggestedPolicy = strings.TrimSpace(input.SuggestedPolicy)

	if input.ChannelId <= 0 || input.Route == "" || input.EventType == "" {
		return ToolCompatibilityEventInput{}, fmt.Errorf("%w: channel_id, route, and event_type are required", ErrInvalidToolCompatibilityEvent)
	}
	if !isToolCompatibilityEventType(input.EventType) {
		return ToolCompatibilityEventInput{}, fmt.Errorf("%w: unknown event_type %q", ErrInvalidToolCompatibilityEvent, input.EventType)
	}
	return input, nil
}

func isToolCompatibilityEventType(eventType string) bool {
	switch eventType {
	case ToolCompatibilityEventTypeUpstreamUnsupported,
		ToolCompatibilityEventTypeNameConflict,
		ToolCompatibilityEventTypePolicyDrop,
		ToolCompatibilityEventTypePolicyReject,
		ToolCompatibilityEventTypeInvalidToolSchema,
		ToolCompatibilityEventTypeUnclassified,
		ToolCompatibilityEventTypeAcceptedDefinition,
		ToolCompatibilityEventTypeInvoked:
		return true
	default:
		return false
	}
}

func SanitizeToolCompatibilityError(raw string) string {
	message := strings.TrimSpace(raw)
	if message == "" {
		return ""
	}
	// Structured payloads can include user messages, tool schemas, or arguments. Do not persist them.
	if strings.ContainsAny(message, "{}[]") {
		return "[redacted structured error]"
	}
	message = toolCompatibilityBearerPattern.ReplaceAllString(message, "Bearer [redacted]")
	message = toolCompatibilitySensitiveAssignmentPattern.ReplaceAllString(message, "$1=[redacted]")
	message = toolCompatibilityLongTokenPattern.ReplaceAllString(message, "[redacted]")
	message = toolCompatibilityWhitespacePattern.ReplaceAllString(message, " ")
	const maxLength = 512
	if len(message) > maxLength {
		message = message[:maxLength] + "…"
	}
	return message
}

func ToolCompatibilityErrorFingerprint(sanitizedError string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(sanitizedError))))
	return hex.EncodeToString(sum[:])
}

func ToolCompatibilityEventKey(input ToolCompatibilityEventInput, errorFingerprint string) string {
	parts := []string{
		fmt.Sprintf("%d", input.ChannelId),
		strings.ToLower(strings.TrimSpace(input.Route)),
		strings.ToLower(strings.TrimSpace(input.RequestedModel)),
		strings.ToLower(strings.TrimSpace(input.UpstreamModel)),
		strings.ToLower(strings.TrimSpace(input.ToolType)),
		strings.ToLower(strings.TrimSpace(input.ToolName)),
		strings.ToLower(strings.TrimSpace(input.EventType)),
		strings.ToLower(strings.TrimSpace(errorFingerprint)),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(sum[:])
}

func RecordToolCompatibilityEvent(input ToolCompatibilityEventInput) (*ToolCompatibilityEvent, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}
	normalized, err := input.normalized()
	if err != nil {
		return nil, err
	}
	sanitizedError := SanitizeToolCompatibilityError(normalized.ErrorMessage)
	errorFingerprint := ToolCompatibilityErrorFingerprint(sanitizedError)
	eventKey := ToolCompatibilityEventKey(normalized, errorFingerprint)
	now := common.GetTimestamp()

	for attempt := 0; attempt < toolCompatibilityEventMaxRecordAttempts; attempt++ {
		event, err := recordToolCompatibilityEventAttempt(normalized, eventKey, errorFingerprint, sanitizedError, now)
		if err == nil {
			return event, nil
		}
		if !isToolCompatibilityEventRetryableError(err) || attempt == toolCompatibilityEventMaxRecordAttempts-1 {
			return nil, err
		}
		backoff := 5 * time.Millisecond << min(attempt, 4)
		time.Sleep(backoff)
	}
	return nil, errors.New("tool compatibility event retry exhausted")
}

func recordToolCompatibilityEventAttempt(input ToolCompatibilityEventInput, eventKey, errorFingerprint, sanitizedError string, now int64) (*ToolCompatibilityEvent, error) {
	var result ToolCompatibilityEvent
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing ToolCompatibilityEvent
		err := tx.Where("event_key = ?", eventKey).First(&existing).Error
		if err == nil {
			updates := map[string]any{
				"occurrence_count": gorm.Expr("occurrence_count + ?", 1),
				"last_seen_at":     now,
				"sanitized_error":  sanitizedError,
			}
			if err := tx.Model(&ToolCompatibilityEvent{}).Where("id = ?", existing.Id).Updates(updates).Error; err != nil {
				return err
			}
			return tx.First(&result, existing.Id).Error
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		created := ToolCompatibilityEvent{
			EventKey:         eventKey,
			ChannelId:        input.ChannelId,
			Route:            input.Route,
			RequestedModel:   input.RequestedModel,
			UpstreamModel:    input.UpstreamModel,
			ToolType:         input.ToolType,
			ToolName:         input.ToolName,
			EventType:        input.EventType,
			CurrentPolicy:    input.CurrentPolicy,
			SuggestedPolicy:  input.SuggestedPolicy,
			ErrorFingerprint: errorFingerprint,
			SanitizedError:   sanitizedError,
			OccurrenceCount:  1,
			FirstSeenAt:      now,
			LastSeenAt:       now,
			ResolutionStatus: ToolCompatibilityResolutionStatusOpen,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		result = created
		return nil
	})
	return &result, err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isToolCompatibilityEventRetryableError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate") || strings.Contains(message, "locked") || strings.Contains(message, "deadlock")
}
