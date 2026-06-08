package domain

import "errors"

var (
	ErrProviderNotSupported    = errors.New("ingestion provider not supported")
	ErrInvalidSignature        = errors.New("ingestion invalid signature")
	ErrPayloadInvalid          = errors.New("ingestion payload invalid")
	ErrDuplicateEventConflict  = errors.New("ingestion duplicate event conflict")
	ErrRawEventNotFound        = errors.New("ingestion raw event not found")
	ErrNormalizedEventNotFound = errors.New("ingestion normalized event not found")
	ErrTemporarilyUnavailable  = errors.New("ingestion temporarily unavailable")
)
