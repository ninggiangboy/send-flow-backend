package startaudienceexport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	ExportJobsWrite ports.ExportJobWriteRepository
	ContactsWrite   ports.ContactWriteRepository
	SegmentsWrite   ports.SegmentWriteRepository
	AccessChecker   ports.WorkspaceAccessChecker
	ExportEnabled   bool
	IDGen           func() (string, error)
	Logger          *slog.Logger
}

type Command struct {
	WorkspaceID    string
	UserID         string
	Format         string
	ZipOutput      bool
	Filters        map[string]any
	SelectedFields []string
	Now            time.Time
}

type Handler struct {
	exportJobsWrite ports.ExportJobWriteRepository
	contactsWrite   ports.ContactWriteRepository
	segmentsWrite   ports.SegmentWriteRepository
	accessChecker   ports.WorkspaceAccessChecker
	exportEnabled   bool
	idGen           func() (string, error)
	log             *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		exportJobsWrite: opts.ExportJobsWrite,
		contactsWrite:   opts.ContactsWrite,
		segmentsWrite:   opts.SegmentsWrite,
		accessChecker:   opts.AccessChecker,
		exportEnabled:   opts.ExportEnabled,
		idGen:           opts.IDGen,
		log:             opts.Logger.With("usecase", "start_audience_export"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.AudienceExportJob, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.export"); err != nil {
		if errors.Is(err, domain.ErrExportDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrExportDenied
		}
		return nil, err
	}

	if !domain.ValidExportFormat(cmd.Format) {
		return nil, domain.ErrExportFormatInvalid
	}
	if !h.exportEnabled {
		return nil, domain.ErrExportUnavailable
	}
	if err := validateExportFilters(cmd.Filters); err != nil {
		return nil, err
	}
	if err := validateSelectedFields(cmd.SelectedFields); err != nil {
		return nil, err
	}

	estimatedTotalCount, err := h.estimateTotalCount(ctx, cmd)
	if err != nil {
		h.log.Error("failed to estimate export total count", "error", err)
		return nil, err
	}

	id, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate id", "error", err)
		return nil, err
	}

	job := domain.AudienceExportJob{
		ID:                  id,
		WorkspaceID:         cmd.WorkspaceID,
		FiltersJSON:         cmd.Filters,
		SelectedFields:      cmd.SelectedFields,
		Format:              domain.ExportFormat(cmd.Format),
		ZipOutput:           cmd.ZipOutput,
		Status:              domain.JobStatusQueued,
		EstimatedTotalCount: estimatedTotalCount,
		CreatedAt:           cmd.Now,
		UpdatedAt:           cmd.Now,
	}

	if err := h.exportJobsWrite.CreateExportJob(ctx, job); err != nil {
		h.log.Error("failed to create export job", "error", err)
		return nil, err
	}

	h.log.Info("export job created", "export_id", id)
	return &job, nil
}

func (h *Handler) estimateTotalCount(ctx context.Context, cmd Command) (int64, error) {
	query := ports.ContactListQuery{
		WorkspaceID: cmd.WorkspaceID,
		Limit:       1000,
	}
	var segmentRules []domain.SegmentRule

	if cmd.Filters != nil {
		if status, ok := cmd.Filters["status"].(string); ok && status != "" {
			query.Status = status
		} else {
			query.Status = string(domain.ContactStatusActive)
		}
		if listID, ok := cmd.Filters["list_id"].(string); ok && listID != "" {
			query.ListID = listID
		}
		if segmentID, ok := cmd.Filters["segment_id"].(string); ok && segmentID != "" {
			segment, err := h.segmentsWrite.FindSegmentByID(ctx, cmd.WorkspaceID, segmentID)
			if err != nil {
				return 0, err
			}
			if segment.Status != domain.SegmentStatusReady {
				return 0, domain.ErrSegmentDefinitionInvalid
			}
			rules, err := domain.ExtractRules(segment.DefinitionJSON)
			if err != nil {
				return 0, domain.ErrSegmentDefinitionInvalid
			}
			segmentRules = rules
		}
		if q, ok := cmd.Filters["query"].(string); ok && q != "" {
			query.Q = q
		}
	} else {
		query.Status = string(domain.ContactStatusActive)
	}

	var total int64
	for {
		contacts, cursor, err := h.contactsWrite.ListContacts(ctx, query)
		if err != nil {
			return 0, err
		}
		for _, c := range contacts {
			if len(segmentRules) > 0 && !domain.MatchesSegment(c, segmentRules) {
				continue
			}
			total++
		}
		if cursor == "" {
			break
		}
		query.Cursor = cursor
	}

	return total, nil
}

var allowedExportFilters = map[string]struct{}{
	"status":     {},
	"list_id":    {},
	"segment_id": {},
	"query":      {},
}

func validateExportFilters(filters map[string]any) error {
	for key, value := range filters {
		if _, ok := allowedExportFilters[key]; !ok {
			return fmt.Errorf("%w: unsupported filter %q", domain.ErrExportFilterInvalid, key)
		}
		switch key {
		case "status":
			status, ok := value.(string)
			if !ok || !domain.ValidContactStatus(status) {
				return fmt.Errorf("%w: invalid status filter", domain.ErrExportFilterInvalid)
			}
		case "list_id", "segment_id", "query":
			if _, ok := value.(string); !ok {
				return fmt.Errorf("%w: invalid %s filter", domain.ErrExportFilterInvalid, key)
			}
		}
	}
	return nil
}

func validateSelectedFields(fields []string) error {
	const maxSelectedFields = 64
	if len(fields) > maxSelectedFields {
		return fmt.Errorf("%w: too many selected fields", domain.ErrExportFilterInvalid)
	}
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || !domain.IsPrintable(field) {
			return fmt.Errorf("%w: invalid selected field", domain.ErrExportFilterInvalid)
		}
		if _, ok := seen[field]; ok {
			return fmt.Errorf("%w: duplicate selected field", domain.ErrExportFilterInvalid)
		}
		seen[field] = struct{}{}
	}
	return nil
}
