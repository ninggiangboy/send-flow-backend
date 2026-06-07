package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	ContactsRead    ports.ContactReadRepository
	ContactsWrite   ports.ContactWriteRepository
	ListsRead       ports.ListReadRepository
	ListsWrite      ports.ListWriteRepository
	SegmentsRead    ports.SegmentReadRepository
	SegmentsWrite   ports.SegmentWriteRepository
	ImportJobsRead  ports.ImportJobReadRepository
	ImportJobsWrite ports.ImportJobWriteRepository
	ExportJobsRead  ports.ExportJobReadRepository
	ExportJobsWrite ports.ExportJobWriteRepository
	AccessChecker   ports.WorkspaceAccessChecker
	IDGen           func() (string, error)
	Logger          *slog.Logger
}

type Service struct {
	contactsRead    ports.ContactReadRepository
	contactsWrite   ports.ContactWriteRepository
	listsRead       ports.ListReadRepository
	listsWrite      ports.ListWriteRepository
	segmentsRead    ports.SegmentReadRepository
	segmentsWrite   ports.SegmentWriteRepository
	importJobsRead  ports.ImportJobReadRepository
	importJobsWrite ports.ImportJobWriteRepository
	exportJobsRead  ports.ExportJobReadRepository
	exportJobsWrite ports.ExportJobWriteRepository
	accessChecker   ports.WorkspaceAccessChecker
	idGen           func() (string, error)
	log             *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) { return "", nil }
	}
	return &Service{
		contactsRead:    opts.ContactsRead,
		contactsWrite:   opts.ContactsWrite,
		listsRead:       opts.ListsRead,
		listsWrite:      opts.ListsWrite,
		segmentsRead:    opts.SegmentsRead,
		segmentsWrite:   opts.SegmentsWrite,
		importJobsRead:  opts.ImportJobsRead,
		importJobsWrite: opts.ImportJobsWrite,
		exportJobsRead:  opts.ExportJobsRead,
		exportJobsWrite: opts.ExportJobsWrite,
		accessChecker:   opts.AccessChecker,
		idGen:           opts.IDGen,
		log:             opts.Logger.With("module", "audience"),
	}
}

type CreateContactInput struct {
	WorkspaceID string
	UserID      string
	Email       string
	FirstName   string
	LastName    string
	Tags        []string
	Attributes  map[string]any
	Now         time.Time
}

type UpdateContactInput struct {
	WorkspaceID string
	ContactID   string
	UserID      string
	Email       *string
	FirstName   *string
	LastName    *string
	Status      *string
	Tags        []string
	Attributes  map[string]any
	Now         time.Time
}

type ContactResult struct {
	Contact domain.Contact
}

type ContactListResult struct {
	Contacts   []domain.Contact
	NextCursor string
}

type ListResult struct {
	List domain.AudienceList
}

type ListListResult struct {
	Lists      []domain.AudienceList
	Counts     map[string]int64
	NextCursor string
}

type MembershipUpdateResult struct {
	Result domain.MembershipUpdateResult
}

type SegmentResult struct {
	Segment domain.Segment
}

type SegmentListResult struct {
	Segments   []domain.Segment
	NextCursor string
}

type ImportJobResult struct {
	Job domain.AudienceImportJob
}

type ImportJobListResult struct {
	Jobs       []domain.AudienceImportJob
	NextCursor string
}

type ExportJobResult struct {
	Job domain.AudienceExportJob
}

func (s *Service) CreateContact(ctx context.Context, input CreateContactInput) (*ContactResult, error) {
	log := s.log.With("usecase", "create_contact", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	email := input.Email
	if email == "" {
		return nil, domain.ErrContactPayloadInvalid
	}
	normalized := domain.NormalizeEmail(email)

	if !domain.ValidContactStatus(string(domain.ContactStatusActive)) {
		return nil, domain.ErrContactStatusInvalid
	}

	if normalized == "" {
		return nil, domain.ErrContactPayloadInvalid
	}

	existing, _ := s.contactsRead.FindContactByEmail(ctx, input.WorkspaceID, normalized)
	if existing != nil {
		return nil, domain.ErrContactEmailConflict
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	contact := domain.Contact{
		ID:              id,
		WorkspaceID:     input.WorkspaceID,
		Email:           email,
		EmailNormalized: normalized,
		FirstName:       input.FirstName,
		LastName:        input.LastName,
		Status:          domain.ContactStatusActive,
		Tags:            input.Tags,
		Attributes:      input.Attributes,
		CreatedAt:       input.Now,
		UpdatedAt:       input.Now,
	}

	if err := s.contactsWrite.CreateContact(ctx, contact); err != nil {
		log.Error("failed to create contact", "error", err)
		return nil, err
	}

	log.Info("contact created", "contact_id", id)
	return &ContactResult{Contact: contact}, nil
}

func (s *Service) ListContacts(ctx context.Context, query ports.ContactListQuery, userID string) (*ContactListResult, error) {
	log := s.log.With("usecase", "list_contacts", "workspace_id", query.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, query.WorkspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 50
	}

	contacts, cursor, err := s.contactsRead.ListContacts(ctx, query)
	if err != nil {
		log.Error("failed to list contacts", "error", err)
		return nil, err
	}

	return &ContactListResult{Contacts: contacts, NextCursor: cursor}, nil
}

func (s *Service) GetContact(ctx context.Context, workspaceID, contactID, userID string) (*ContactResult, error) {
	log := s.log.With("usecase", "get_contact", "workspace_id", workspaceID, "contact_id", contactID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	contact, err := s.contactsRead.FindContactByID(ctx, workspaceID, contactID)
	if err != nil {
		if errors.Is(err, domain.ErrContactNotFound) {
			return nil, err
		}
		log.Error("failed to find contact", "error", err)
		return nil, err
	}

	return &ContactResult{Contact: *contact}, nil
}

func (s *Service) UpdateContact(ctx context.Context, input UpdateContactInput) (*ContactResult, error) {
	log := s.log.With("usecase", "update_contact", "workspace_id", input.WorkspaceID, "contact_id", input.ContactID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	contact, err := s.contactsRead.FindContactByID(ctx, input.WorkspaceID, input.ContactID)
	if err != nil {
		if errors.Is(err, domain.ErrContactNotFound) {
			return nil, err
		}
		log.Error("failed to find contact", "error", err)
		return nil, err
	}

	if input.Email != nil {
		normalized := domain.NormalizeEmail(*input.Email)
		if normalized == "" {
			return nil, domain.ErrContactPayloadInvalid
		}
		existing, _ := s.contactsRead.FindContactByEmail(ctx, input.WorkspaceID, normalized)
		if existing != nil && existing.ID != input.ContactID {
			return nil, domain.ErrContactEmailConflict
		}
		contact.Email = *input.Email
		contact.EmailNormalized = normalized
	}
	if input.FirstName != nil {
		contact.FirstName = *input.FirstName
	}
	if input.LastName != nil {
		contact.LastName = *input.LastName
	}
	if input.Status != nil {
		if !domain.ValidContactStatus(*input.Status) {
			return nil, domain.ErrContactStatusInvalid
		}
		contact.Status = domain.ContactStatus(*input.Status)
	}
	if input.Tags != nil {
		contact.Tags = input.Tags
	}
	if input.Attributes != nil {
		contact.Attributes = input.Attributes
	}
	contact.UpdatedAt = input.Now

	if err := s.contactsWrite.UpdateContact(ctx, *contact); err != nil {
		log.Error("failed to update contact", "error", err)
		return nil, err
	}

	log.Info("contact updated")
	return &ContactResult{Contact: *contact}, nil
}

func (s *Service) ArchiveContact(ctx context.Context, workspaceID, contactID, userID string, now time.Time) error {
	log := s.log.With("usecase", "archive_contact", "workspace_id", workspaceID, "contact_id", contactID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return domain.ErrWriteDenied
		}
		return err
	}

	if _, err := s.contactsRead.FindContactByID(ctx, workspaceID, contactID); err != nil {
		if errors.Is(err, domain.ErrContactNotFound) {
			return err
		}
		log.Error("failed to find contact for archive", "error", err)
		return err
	}

	if err := s.contactsWrite.ArchiveContact(ctx, workspaceID, contactID, now); err != nil {
		log.Error("failed to archive contact", "error", err)
		return err
	}

	log.Info("contact archived")
	return nil
}

func (s *Service) CreateList(ctx context.Context, workspaceID, userID, name, description string, metadata map[string]any, now time.Time) (*ListResult, error) {
	log := s.log.With("usecase", "create_list", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if name == "" {
		return nil, domain.ErrContactPayloadInvalid
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	list := domain.AudienceList{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        name,
		Description: description,
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.listsWrite.CreateList(ctx, list); err != nil {
		if errors.Is(err, domain.ErrListNameConflict) {
			return nil, err
		}
		log.Error("failed to create list", "error", err)
		return nil, err
	}

	log.Info("list created", "list_id", id)
	return &ListResult{List: list}, nil
}

func (s *Service) ListLists(ctx context.Context, workspaceID, userID string, limit int, cursor string) (*ListListResult, error) {
	log := s.log.With("usecase", "list_lists", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := ports.ListListQuery{WorkspaceID: workspaceID, Limit: limit, Cursor: cursor}
	lists, nextCursor, err := s.listsRead.ListLists(ctx, query)
	if err != nil {
		log.Error("failed to list lists", "error", err)
		return nil, err
	}

	listIDs := make([]string, len(lists))
	for i, l := range lists {
		listIDs[i] = l.ID
	}

	counts := map[string]int64{}
	if len(listIDs) > 0 {
		var countErr error
		counts, countErr = s.listsRead.CountContactsByList(ctx, workspaceID, listIDs)
		if countErr != nil {
			log.Error("failed to count contacts by list", "error", countErr)
		}
	}

	return &ListListResult{Lists: lists, Counts: counts, NextCursor: nextCursor}, nil
}

func (s *Service) UpdateListMemberships(ctx context.Context, workspaceID, listID, userID, mode string, contactIDs []string, now time.Time) (*MembershipUpdateResult, error) {
	log := s.log.With("usecase", "update_list_memberships", "workspace_id", workspaceID, "list_id", listID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if !domain.ValidListMembershipMode(mode) {
		return nil, domain.ErrListMembershipPayloadInvalid
	}

	list, err := s.listsRead.FindListByID(ctx, workspaceID, listID)
	if err != nil {
		if errors.Is(err, domain.ErrListNotFound) {
			return nil, err
		}
		log.Error("failed to find list", "error", err)
		return nil, err
	}
	_ = list

	deduplicated := make([]string, 0, len(contactIDs))
	seen := map[string]struct{}{}
	for _, id := range contactIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			deduplicated = append(deduplicated, id)
		}
	}

	var result domain.MembershipUpdateResult
	switch domain.ListMembershipMode(mode) {
	case domain.ListMembershipModeReplace:
		result, err = s.listsWrite.ReplaceListMemberships(ctx, workspaceID, listID, deduplicated, now)
	case domain.ListMembershipModeMerge:
		result, err = s.listsWrite.MergeListMemberships(ctx, workspaceID, listID, deduplicated, now)
	}
	if err != nil {
		log.Error("failed to update list memberships", "error", err)
		return nil, err
	}

	log.Info("list memberships updated", "list_id", listID, "added", result.AddedCount, "removed", result.RemovedCount, "skipped", result.SkippedCount)
	return &MembershipUpdateResult{Result: result}, nil
}

func (s *Service) CreateSegment(ctx context.Context, workspaceID, userID, name string, definition map[string]any, now time.Time) (*SegmentResult, error) {
	log := s.log.With("usecase", "create_segment", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if name == "" {
		return nil, domain.ErrSegmentDefinitionInvalid
	}
	if definition == nil || len(definition) == 0 {
		return nil, domain.ErrSegmentDefinitionInvalid
	}

	if rules, ok := definition["rules"]; ok {
		if _, ok := rules.([]any); !ok {
			return nil, domain.ErrSegmentDefinitionInvalid
		}
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	segment := domain.Segment{
		ID:             id,
		WorkspaceID:    workspaceID,
		Name:           name,
		DefinitionJSON: definition,
		Status:         domain.SegmentStatusReady,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.segmentsWrite.CreateSegment(ctx, segment); err != nil {
		if errors.Is(err, domain.ErrSegmentNameConflict) {
			return nil, err
		}
		log.Error("failed to create segment", "error", err)
		return nil, err
	}

	log.Info("segment created", "segment_id", id)
	return &SegmentResult{Segment: segment}, nil
}

func (s *Service) ListSegments(ctx context.Context, workspaceID, userID, status string, limit int, cursor string) (*SegmentListResult, error) {
	log := s.log.With("usecase", "list_segments", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := ports.SegmentListQuery{WorkspaceID: workspaceID, Status: status, Limit: limit, Cursor: cursor}
	segments, nextCursor, err := s.segmentsRead.ListSegments(ctx, query)
	if err != nil {
		log.Error("failed to list segments", "error", err)
		return nil, err
	}

	return &SegmentListResult{Segments: segments, NextCursor: nextCursor}, nil
}

func (s *Service) UpdateSegment(ctx context.Context, workspaceID, segmentID, userID, name string, definition map[string]any, status string, now time.Time) (*SegmentResult, error) {
	log := s.log.With("usecase", "update_segment", "workspace_id", workspaceID, "segment_id", segmentID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	segment, err := s.segmentsRead.FindSegmentByID(ctx, workspaceID, segmentID)
	if err != nil {
		if errors.Is(err, domain.ErrSegmentNotFound) {
			return nil, err
		}
		log.Error("failed to find segment", "error", err)
		return nil, err
	}

	if name != "" {
		segment.Name = name
	}
	if definition != nil {
		if len(definition) == 0 {
			return nil, domain.ErrSegmentDefinitionInvalid
		}
		if rules, ok := definition["rules"]; ok {
			if _, ok := rules.([]any); !ok {
				return nil, domain.ErrSegmentDefinitionInvalid
			}
		}
		segment.DefinitionJSON = definition
	}
	if status != "" {
		if !domain.ValidSegmentStatus(status) {
			return nil, domain.ErrContactPayloadInvalid
		}
		segment.Status = domain.SegmentStatus(status)
	}
	segment.UpdatedAt = now

	if err := s.segmentsWrite.UpdateSegment(ctx, *segment); err != nil {
		if errors.Is(err, domain.ErrSegmentNameConflict) {
			return nil, err
		}
		log.Error("failed to update segment", "error", err)
		return nil, err
	}

	log.Info("segment updated")
	return &SegmentResult{Segment: *segment}, nil
}

func (s *Service) StartAudienceImport(ctx context.Context, workspaceID, userID, sourceURI, dedupeMode string, metadata map[string]any, now time.Time) (*ImportJobResult, error) {
	log := s.log.With("usecase", "start_audience_import", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.import"); err != nil {
		if errors.Is(err, domain.ErrImportDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrImportDenied
		}
		return nil, err
	}

	if sourceURI == "" {
		return nil, domain.ErrImportSourceInvalid
	}

	if !domain.ValidDedupeMode(dedupeMode) {
		return nil, domain.ErrImportSourceInvalid
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	job := domain.AudienceImportJob{
		ID:          id,
		WorkspaceID: workspaceID,
		SourceURI:   sourceURI,
		DedupeMode:  domain.DedupeMode(dedupeMode),
		Status:      domain.JobStatusQueued,
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.importJobsWrite.CreateImportJob(ctx, job); err != nil {
		log.Error("failed to create import job", "error", err)
		return nil, err
	}

	log.Info("import job created", "import_id", id)
	return &ImportJobResult{Job: job}, nil
}

func (s *Service) ListAudienceImports(ctx context.Context, workspaceID, userID, status string, limit int, cursor string) (*ImportJobListResult, error) {
	log := s.log.With("usecase", "list_audience_imports", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := ports.ImportJobListQuery{WorkspaceID: workspaceID, Status: status, Limit: limit, Cursor: cursor}
	jobs, nextCursor, err := s.importJobsRead.ListImportJobs(ctx, query)
	if err != nil {
		log.Error("failed to list import jobs", "error", err)
		return nil, err
	}

	return &ImportJobListResult{Jobs: jobs, NextCursor: nextCursor}, nil
}

func (s *Service) GetAudienceImport(ctx context.Context, workspaceID, jobID, userID string) (*ImportJobResult, error) {
	log := s.log.With("usecase", "get_audience_import", "workspace_id", workspaceID, "import_id", jobID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	job, err := s.importJobsRead.FindImportJobByID(ctx, workspaceID, jobID)
	if err != nil {
		if errors.Is(err, domain.ErrImportJobNotFound) {
			return nil, err
		}
		log.Error("failed to find import job", "error", err)
		return nil, err
	}

	return &ImportJobResult{Job: *job}, nil
}

func (s *Service) StartAudienceExport(ctx context.Context, workspaceID, userID, format string, filters map[string]any, selectedFields []string, now time.Time) (*ExportJobResult, error) {
	log := s.log.With("usecase", "start_audience_export", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.export"); err != nil {
		if errors.Is(err, domain.ErrExportDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrExportDenied
		}
		return nil, err
	}

	if !domain.ValidExportFormat(format) {
		return nil, domain.ErrExportFormatInvalid
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	job := domain.AudienceExportJob{
		ID:             id,
		WorkspaceID:    workspaceID,
		FiltersJSON:    filters,
		SelectedFields: selectedFields,
		Format:         domain.ExportFormat(format),
		Status:         domain.JobStatusQueued,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.exportJobsWrite.CreateExportJob(ctx, job); err != nil {
		log.Error("failed to create export job", "error", err)
		return nil, err
	}

	log.Info("export job created", "export_id", id)
	return &ExportJobResult{Job: job}, nil
}

func (s *Service) GetAudienceExport(ctx context.Context, workspaceID, jobID, userID string) (*ExportJobResult, error) {
	log := s.log.With("usecase", "get_audience_export", "workspace_id", workspaceID, "export_id", jobID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	job, err := s.exportJobsRead.FindExportJobByID(ctx, workspaceID, jobID)
	if err != nil {
		if errors.Is(err, domain.ErrExportJobNotFound) {
			return nil, err
		}
		log.Error("failed to find export job", "error", err)
		return nil, err
	}

	return &ExportJobResult{Job: *job}, nil
}

type AudienceSelectionRef struct {
	ListID     string
	SegmentID  string
	ContactIDs []string
}

func (s *Service) ResolveAudienceSelection(ctx context.Context, workspaceID string, ref AudienceSelectionRef) ([]string, error) {
	log := s.log.With("usecase", "resolve_audience_selection", "workspace_id", workspaceID)

	contactIDs := make(map[string]struct{})

	if ref.ListID != "" {
		query := ports.ContactListQuery{
			WorkspaceID: workspaceID,
			ListID:      ref.ListID,
			Limit:       10000,
		}
		for {
			contacts, cursor, err := s.contactsRead.ListContacts(ctx, query)
			if err != nil {
				log.Error("failed to list contacts by list", "error", err)
				return nil, err
			}
			for _, c := range contacts {
				contactIDs[c.ID] = struct{}{}
			}
			if cursor == "" {
				break
			}
			query.Cursor = cursor
		}
	}

	if ref.SegmentID != "" {
		_, err := s.segmentsRead.FindSegmentByID(ctx, workspaceID, ref.SegmentID)
		if err != nil {
			if err == domain.ErrSegmentNotFound {
				return nil, err
			}
			log.Error("failed to find segment", "error", err)
			return nil, err
		}
	}

	for _, id := range ref.ContactIDs {
		contactIDs[id] = struct{}{}
	}

	result := make([]string, 0, len(contactIDs))
	for id := range contactIDs {
		result = append(result, id)
	}
	if result == nil {
		result = []string{}
	}
	return result, nil
}

func (s *Service) EstimateAudienceSize(ctx context.Context, workspaceID string, ref AudienceSelectionRef) (int, error) {
	ids, err := s.ResolveAudienceSelection(ctx, workspaceID, ref)
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}
