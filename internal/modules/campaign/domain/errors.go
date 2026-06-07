package domain

import "errors"

var (
	ErrReadDenied              = errors.New("campaign read denied")
	ErrWriteDenied             = errors.New("campaign write denied")
	ErrSendDenied              = errors.New("campaign send denied")
	ErrCampaignNotFound        = errors.New("campaign not found")
	ErrPayloadInvalid          = errors.New("campaign payload invalid")
	ErrAudienceNotReady        = errors.New("campaign audience not ready")
	ErrSenderNotVerified       = errors.New("campaign sender not verified")
	ErrTemplatePublishRequired = errors.New("campaign template publish required")
	ErrInvalidStateTransition  = errors.New("campaign invalid state transition")
	ErrCandidateNotFound       = errors.New("campaign candidate not found")
)
