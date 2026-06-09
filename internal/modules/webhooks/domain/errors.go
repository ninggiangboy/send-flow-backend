package domain

import "errors"

var (
	ErrConfigNotFound      = errors.New("webhook config not found")
	ErrDeliveryNotFound    = errors.New("webhook delivery not found")
	ErrManageDenied        = errors.New("webhook manage denied")
	ErrDeliveryReadDenied  = errors.New("webhook delivery read denied")
	ErrDeliveryRetryDenied = errors.New("webhook delivery retry denied")
	ErrTargetURLInvalid    = errors.New("webhook target URL invalid")
	ErrSubscriptionInvalid = errors.New("webhook subscription invalid")
	ErrRotateConflict      = errors.New("webhook secret rotate conflict")
	ErrRetryConflict       = errors.New("webhook retry conflict")
	ErrConfigInvalid       = errors.New("webhook config invalid")
	ErrDeliveryInvalid     = errors.New("webhook delivery invalid")
	ErrConfigNameConflict  = errors.New("webhook config name conflict")
	ErrConfigDisabled      = errors.New("webhook config disabled")
)
