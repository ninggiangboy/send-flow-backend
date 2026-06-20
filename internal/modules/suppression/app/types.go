package app

import (
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app/entry"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
)

// Input type aliases — backward compatible with API and worker callers.
type (
	CreateEntryInput       = entry.CreateInput
	CreateSystemEntryInput = entry.CreateSystemInput
)

// CheckSuppressionResult alias — backward compatible with the facade return type.
type CheckSuppressionResult = entry.CheckResult

// ListEntriesResult is a concrete DTO because its exported field names differ
// from the internal list return values.
type ListEntriesResult struct {
	Entries    []domain.SuppressionEntry
	NextCursor string
}
