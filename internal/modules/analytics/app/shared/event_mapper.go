package shared

import (
	"errors"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/ingestion"
)

var ErrMalformedPayload = errors.New("malformed event payload")

type MappedEvent struct {
	Input ingestion.IngestFactCommand
}

func ParseTimestamp(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
