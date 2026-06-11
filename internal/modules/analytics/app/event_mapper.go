package app

import (
	"errors"
	"time"
)

var ErrMalformedPayload = errors.New("malformed event payload")

type MappedEvent struct {
	Input IngestEmailEventFactInput
}

func ParseTimestamp(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
