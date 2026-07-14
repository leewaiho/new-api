package service

import "github.com/QuantumNous/new-api/model"

// ToolCompatibilityEventInput is intentionally shared with model so future relay callers do not
// copy raw upstream errors into a second event DTO.
type ToolCompatibilityEventInput = model.ToolCompatibilityEventInput

func RecordToolCompatibilityEvent(input ToolCompatibilityEventInput) (*model.ToolCompatibilityEvent, error) {
	return model.RecordToolCompatibilityEvent(input)
}
