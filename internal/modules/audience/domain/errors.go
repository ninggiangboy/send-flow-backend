package domain

import "errors"

var (
	ErrReadDenied                   = errors.New("audience read denied")
	ErrWriteDenied                  = errors.New("audience write denied")
	ErrImportDenied                 = errors.New("audience import denied")
	ErrExportDenied                 = errors.New("audience export denied")
	ErrExportUnavailable            = errors.New("audience export unavailable")
	ErrContactNotFound              = errors.New("contact not found")
	ErrListNotFound                 = errors.New("audience list not found")
	ErrSegmentNotFound              = errors.New("segment not found")
	ErrImportJobNotFound            = errors.New("import job not found")
	ErrExportJobNotFound            = errors.New("export job not found")
	ErrContactEmailConflict         = errors.New("contact email conflict")
	ErrListNameConflict             = errors.New("list name conflict")
	ErrSegmentNameConflict          = errors.New("segment name conflict")
	ErrImportDuplicateSubmission    = errors.New("import duplicate submission")
	ErrContactPayloadInvalid        = errors.New("contact payload invalid")
	ErrContactStatusInvalid         = errors.New("contact status invalid")
	ErrSegmentDefinitionInvalid     = errors.New("segment definition invalid")
	ErrListMembershipPayloadInvalid = errors.New("list membership payload invalid")
	ErrImportSourceInvalid          = errors.New("import source invalid")
	ErrExportFilterInvalid          = errors.New("export filter invalid")
	ErrExportFormatInvalid          = errors.New("export format invalid")
)
