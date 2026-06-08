package domain

import "errors"

var (
	ErrPayloadInvalid               = errors.New("delivery payload invalid")
	ErrMessageNotFound              = errors.New("delivery message not found")
	ErrAttemptNotFound              = errors.New("delivery attempt not found")
	ErrTransactionalRequestNotFound = errors.New("delivery transactional request not found")
	ErrInvalidStateTransition       = errors.New("delivery invalid state transition")
	ErrDuplicateMessage             = errors.New("delivery duplicate message")
	ErrCampaignScheduleInvalid      = errors.New("delivery campaign schedule invalid")
	ErrCampaignCandidatesNotFound   = errors.New("delivery campaign candidates not found")
	ErrSuppressedRecipient          = errors.New("delivery suppressed recipient")
	ErrSenderNotReady               = errors.New("delivery sender not ready")
	ErrSenderDomainNotFound         = errors.New("delivery sender domain not found")
	ErrSenderDomainNotVerified      = errors.New("delivery sender domain not verified")
	ErrTemplateNotFound             = errors.New("delivery template not found")
	ErrTemplateRenderPayloadInvalid = errors.New("delivery template render payload invalid")
	ErrTemplateRenderFailed         = errors.New("delivery template render failed")
	ErrProviderTemporaryFailure     = errors.New("delivery provider temporary failure")
	ErrProviderPermanentFailure     = errors.New("delivery provider permanent failure")
	ErrReadDenied                   = errors.New("delivery read denied")
	ErrRequestBodyInvalid           = errors.New("delivery request body invalid")
	ErrIdempotencyKeyConflict       = errors.New("delivery idempotency key conflict")
	ErrRecipientInvalid             = errors.New("delivery recipient invalid")
	ErrTemporarilyUnavailable       = errors.New("delivery temporarily unavailable")
)
