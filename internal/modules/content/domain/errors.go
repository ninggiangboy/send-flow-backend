package domain

import "errors"

var (
	ErrReadDenied              = errors.New("template read denied")
	ErrWriteDenied             = errors.New("template write denied")
	ErrRenderDenied            = errors.New("template render denied")
	ErrTemplateNotFound        = errors.New("template not found")
	ErrTemplateVersionNotFound = errors.New("template version not found")
	ErrPublishConflict         = errors.New("template publish conflict")
	ErrVersionConflict         = errors.New("template version conflict")
	ErrSourceInvalid           = errors.New("template source invalid")
	ErrPublishPayloadInvalid   = errors.New("template publish payload invalid")
	ErrRenderPayloadInvalid    = errors.New("template render payload invalid")
	ErrRenderContextInvalid    = errors.New("template render context invalid")
	ErrTemplateNameConflict    = errors.New("content template name conflict")
)
