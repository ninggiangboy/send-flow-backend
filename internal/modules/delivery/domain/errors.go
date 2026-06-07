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
	ErrTemplateRenderFailed         = errors.New("delivery template render failed")
	ErrProviderTemporaryFailure     = errors.New("delivery provider temporary failure")
	ErrProviderPermanentFailure     = errors.New("delivery provider permanent failure")
	ErrReadDenied                   = errors.New("delivery read denied")
)
