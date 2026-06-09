package domain

import "errors"

var (
	ErrQueueReadDenied          = errors.New("operations queue read denied")
	ErrDLQReadDenied            = errors.New("operations dlq read denied")
	ErrReplayManageDenied       = errors.New("operations replay manage denied")
	ErrOutboxRecordNotFound     = errors.New("outbox record not found")
	ErrDeadLetterRecordNotFound = errors.New("dead letter record not found")
	ErrReplayJobNotFound        = errors.New("replay job not found")
	ErrReplayTargetInvalid      = errors.New("replay target invalid")
	ErrReplayConflict           = errors.New("replay conflict")
	ErrFilterInvalid            = errors.New("operations filter invalid")
	ErrWorkspaceRequired        = errors.New("workspace id required")
)
