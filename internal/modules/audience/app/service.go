package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
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
		opts.IDGen = id.NewUUIDGenerator().New
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

type ContactDTO struct {
	ID              string
	WorkspaceID     string
	Email           string
	EmailNormalized string
	FirstName       string
	LastName        string
	Status          string
	Tags            []string
	Attributes      map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ArchivedAt      *time.Time
}

func contactToDTO(c domain.Contact) ContactDTO {
	return ContactDTO{
		ID:              c.ID,
		WorkspaceID:     c.WorkspaceID,
		Email:           c.Email,
		EmailNormalized: c.EmailNormalized,
		FirstName:       c.FirstName,
		LastName:        c.LastName,
		Status:          string(c.Status),
		Tags:            c.Tags,
		Attributes:      c.Attributes,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
		ArchivedAt:      c.ArchivedAt,
	}
}

func contactSliceToDTOs(contacts []domain.Contact) []ContactDTO {
	result := make([]ContactDTO, len(contacts))
	for i, c := range contacts {
		result[i] = contactToDTO(c)
	}
	return result
}

type ContactResult struct {
	Contact ContactDTO
}

type ContactListResult struct {
	Contacts   []ContactDTO
	NextCursor string
}

type AudienceListDTO struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ArchivedAt  *time.Time
}

func listToDTO(l domain.AudienceList) AudienceListDTO {
	return AudienceListDTO{
		ID:          l.ID,
		WorkspaceID: l.WorkspaceID,
		Name:        l.Name,
		Description: l.Description,
		Metadata:    l.Metadata,
		CreatedAt:   l.CreatedAt,
		UpdatedAt:   l.UpdatedAt,
		ArchivedAt:  l.ArchivedAt,
	}
}

func listSliceToDTOs(lists []domain.AudienceList) []AudienceListDTO {
	result := make([]AudienceListDTO, len(lists))
	for i, l := range lists {
		result[i] = listToDTO(l)
	}
	return result
}

type ListResult struct {
	List AudienceListDTO
}

type ListListResult struct {
	Lists      []AudienceListDTO
	Counts     map[string]int64
	NextCursor string
}

type MembershipUpdateResultDTO struct {
	AddedCount   int
	RemovedCount int
	SkippedCount int
}

type MembershipUpdateResult struct {
	Result MembershipUpdateResultDTO
}

type SegmentDTO struct {
	ID             string
	WorkspaceID    string
	Name           string
	DefinitionJSON map[string]any
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func segmentToDTO(s domain.Segment) SegmentDTO {
	return SegmentDTO{
		ID:             s.ID,
		WorkspaceID:    s.WorkspaceID,
		Name:           s.Name,
		DefinitionJSON: s.DefinitionJSON,
		Status:         string(s.Status),
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

func segmentSliceToDTOs(segments []domain.Segment) []SegmentDTO {
	result := make([]SegmentDTO, len(segments))
	for i, s := range segments {
		result[i] = segmentToDTO(s)
	}
	return result
}

type SegmentResult struct {
	Segment SegmentDTO
}

type SegmentListResult struct {
	Segments   []SegmentDTO
	NextCursor string
}

type ImportJobDTO struct {
	ID             string
	WorkspaceID    string
	SourceURI      string
	DedupeMode     string
	Status         string
	ProcessedCount int64
	CreatedCount   int64
	UpdatedCount   int64
	FailedCount    int64
	ErrorSummary   string
	Metadata       map[string]any
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

func importJobToDTO(j domain.AudienceImportJob) ImportJobDTO {
	return ImportJobDTO{
		ID:             j.ID,
		WorkspaceID:    j.WorkspaceID,
		SourceURI:      j.SourceURI,
		DedupeMode:     string(j.DedupeMode),
		Status:         string(j.Status),
		ProcessedCount: j.ProcessedCount,
		CreatedCount:   j.CreatedCount,
		UpdatedCount:   j.UpdatedCount,
		FailedCount:    j.FailedCount,
		ErrorSummary:   j.ErrorSummary,
		Metadata:       j.Metadata,
		CreatedAt:      j.CreatedAt,
		UpdatedAt:      j.UpdatedAt,
		CompletedAt:    j.CompletedAt,
	}
}

func importJobSliceToDTOs(jobs []domain.AudienceImportJob) []ImportJobDTO {
	result := make([]ImportJobDTO, len(jobs))
	for i, j := range jobs {
		result[i] = importJobToDTO(j)
	}
	return result
}

type ImportJobResult struct {
	Job ImportJobDTO
}

type ImportJobListResult struct {
	Jobs       []ImportJobDTO
	NextCursor string
}

type ExportJobDTO struct {
	ID             string
	WorkspaceID    string
	FiltersJSON    map[string]any
	SelectedFields []string
	Format         string
	Status         string
	ArtifactURI    string
	ErrorSummary   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

func exportJobToDTO(j domain.AudienceExportJob) ExportJobDTO {
	return ExportJobDTO{
		ID:             j.ID,
		WorkspaceID:    j.WorkspaceID,
		FiltersJSON:    j.FiltersJSON,
		SelectedFields: j.SelectedFields,
		Format:         string(j.Format),
		Status:         string(j.Status),
		ArtifactURI:    j.ArtifactURI,
		ErrorSummary:   j.ErrorSummary,
		CreatedAt:      j.CreatedAt,
		UpdatedAt:      j.UpdatedAt,
		CompletedAt:    j.CompletedAt,
	}
}

type ExportJobResult struct {
	Job ExportJobDTO
}

type AudienceSelectionRef struct {
	ListID     string
	SegmentID  string
	ContactIDs []string
}

type AudienceRecipient struct {
	ContactID       string
	Email           string
	EmailNormalized string
	FirstName       string
	LastName        string
	Tags            []string
	Attributes      map[string]any
}

func (s *Service) ResolveAudienceSelection(ctx context.Context, workspaceID, userID string, ref AudienceSelectionRef) ([]string, error) {
	log := s.log.With("usecase", "resolve_audience_selection", "workspace_id", workspaceID)

	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience:resolve"); err != nil {
		return nil, err
	}

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
			if errors.Is(err, domain.ErrSegmentNotFound) {
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

func (s *Service) EstimateAudienceSize(ctx context.Context, workspaceID, userID string, ref AudienceSelectionRef) (int, error) {
	ids, err := s.ResolveAudienceSelection(ctx, workspaceID, userID, ref)
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (s *Service) ResolveAudienceRecipients(ctx context.Context, workspaceID, userID string, ref AudienceSelectionRef) ([]AudienceRecipient, error) {
	log := s.log.With("usecase", "resolve_audience_recipients", "workspace_id", workspaceID)

	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience:resolve"); err != nil {
		return nil, err
	}

	const maxAudienceRecipients = 100000

	recipients := make(map[string]*AudienceRecipient)
	order := make([]string, 0)

	addContact := func(c domain.Contact) bool {
		if _, ok := recipients[c.ID]; !ok {
			if len(order) >= maxAudienceRecipients {
				return false
			}
			recipients[c.ID] = &AudienceRecipient{
				ContactID:       c.ID,
				Email:           c.Email,
				EmailNormalized: c.EmailNormalized,
				FirstName:       c.FirstName,
				LastName:        c.LastName,
				Tags:            c.Tags,
				Attributes:      c.Attributes,
			}
			order = append(order, c.ID)
		}
		return true
	}

	if ref.ListID != "" {
		list, err := s.listsRead.FindListByID(ctx, workspaceID, ref.ListID)
		if err != nil {
			if errors.Is(err, domain.ErrListNotFound) {
				return nil, err
			}
			log.Error("failed to find list", "error", err)
			return nil, err
		}
		_ = list

		query := ports.ContactListQuery{
			WorkspaceID: workspaceID,
			ListID:      ref.ListID,
			Status:      string(domain.ContactStatusActive),
			Limit:       10000,
		}
		for {
			contacts, cursor, err := s.contactsRead.ListContacts(ctx, query)
			if err != nil {
				log.Error("failed to list contacts by list", "error", err)
				return nil, err
			}
			for _, c := range contacts {
				if c.Status == domain.ContactStatusActive {
					if !addContact(c) {
						break
					}
				}
			}
			if cursor == "" || len(order) >= maxAudienceRecipients {
				break
			}
			query.Cursor = cursor
		}
	}

	if ref.SegmentID != "" {
		segment, err := s.segmentsRead.FindSegmentByID(ctx, workspaceID, ref.SegmentID)
		if err != nil {
			if errors.Is(err, domain.ErrSegmentNotFound) {
				return nil, err
			}
			log.Error("failed to find segment", "error", err)
			return nil, err
		}
		if segment.Status != domain.SegmentStatusReady {
			return nil, domain.ErrSegmentDefinitionInvalid
		}

		rules, err := domain.ExtractRules(segment.DefinitionJSON)
		if err != nil {
			return nil, domain.ErrSegmentDefinitionInvalid
		}

		query := ports.ContactListQuery{
			WorkspaceID: workspaceID,
			Status:      string(domain.ContactStatusActive),
			Limit:       10000,
		}
		for {
			contacts, cursor, err := s.contactsRead.ListContacts(ctx, query)
			if err != nil {
				log.Error("failed to list contacts for segment", "error", err)
				return nil, err
			}
			for _, c := range contacts {
				if domain.MatchesSegment(c, rules) {
					if !addContact(c) {
						break
					}
				}
			}
			if cursor == "" || len(order) >= maxAudienceRecipients {
				break
			}
			query.Cursor = cursor
		}
	}

	for _, cid := range ref.ContactIDs {
		if len(order) >= maxAudienceRecipients {
			break
		}
		contact, err := s.contactsRead.FindContactByID(ctx, workspaceID, cid)
		if err != nil {
			if errors.Is(err, domain.ErrContactNotFound) {
				return nil, err
			}
			log.Error("failed to find contact", "error", err)
			return nil, err
		}
		addContact(*contact)
	}

	if len(order) >= maxAudienceRecipients {
		log.Warn("audience recipient limit reached, results truncated", "count", len(order), "limit", maxAudienceRecipients)
	}

	result := make([]AudienceRecipient, 0, len(order))
	for _, id := range order {
		if r, ok := recipients[id]; ok {
			result = append(result, *r)
		}
	}
	if result == nil {
		result = []AudienceRecipient{}
	}
	return result, nil
}
