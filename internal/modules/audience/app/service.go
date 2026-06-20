package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/contact"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/exportjob"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/importjob"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/list"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/resolve"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/segment"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	audienceredis "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/infrastructure/redis"
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
	ExportEnabled   bool
	ArtifactSigner  ExportArtifactSigner
	DownloadURLTTL  time.Duration
	IDGen           func() (string, error)
	Logger          *slog.Logger
	RedisCache      *audienceredis.Cache
}

type ExportArtifactSigner interface {
	PresignGetObject(ctx context.Context, key string, expiry time.Duration) (string, error)
}

type Service struct {
	log                        *slog.Logger
	artifactSigner             ExportArtifactSigner
	downloadURLTTL             time.Duration
	createContactH             *contact.CreateHandler
	getContactH                *contact.GetHandler
	listContactsH              *contact.ListHandler
	updateContactH             *contact.UpdateHandler
	archiveContactH            *contact.ArchiveHandler
	createListH                *list.CreateHandler
	listListsH                 *list.ListHandler
	updateListMembershipsH     *list.UpdateMembershipsHandler
	createSegmentH             *segment.CreateHandler
	listSegmentsH              *segment.ListHandler
	updateSegmentH             *segment.UpdateHandler
	startAudienceImportH       *importjob.StartHandler
	listAudienceImportsH       *importjob.ListHandler
	getAudienceImportH         *importjob.GetHandler
	startAudienceExportH       *exportjob.StartHandler
	listAudienceExportsH       *exportjob.ListHandler
	getAudienceExportH         *exportjob.GetHandler
	resolveAudienceSelectionH  *resolve.SelectionHandler
	resolveAudienceRecipientsH *resolve.RecipientsHandler
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = id.NewUUIDGenerator().New
	}
	if opts.DownloadURLTTL <= 0 {
		opts.DownloadURLTTL = 15 * time.Minute
	}
	return &Service{
		log:            opts.Logger,
		artifactSigner: opts.ArtifactSigner,
		downloadURLTTL: opts.DownloadURLTTL,
		createContactH: contact.NewCreate(contact.CreateOptions{
			ContactsWrite: opts.ContactsWrite,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		getContactH: contact.NewGet(contact.GetOptions{
			ContactsRead:  opts.ContactsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		listContactsH: contact.NewList(contact.ListOptions{
			ContactsRead:  opts.ContactsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		updateContactH: contact.NewUpdate(contact.UpdateOptions{
			ContactsWrite: opts.ContactsWrite,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		archiveContactH: contact.NewArchive(contact.ArchiveOptions{
			ContactsWrite: opts.ContactsWrite,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		createListH: list.NewCreate(list.CreateOptions{
			ListsWrite:    opts.ListsWrite,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		listListsH: list.NewList(list.ListOptions{
			ListsRead:     opts.ListsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		updateListMembershipsH: list.NewUpdateMemberships(list.UpdateMembershipsOptions{
			ListsWrite:    opts.ListsWrite,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		createSegmentH: segment.NewCreate(segment.CreateOptions{
			SegmentsWrite: opts.SegmentsWrite,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		listSegmentsH: segment.NewList(segment.ListOptions{
			SegmentsRead:  opts.SegmentsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		updateSegmentH: segment.NewUpdate(segment.UpdateOptions{
			SegmentsWrite: opts.SegmentsWrite,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		startAudienceImportH: importjob.NewStart(importjob.StartOptions{
			ImportJobsWrite: opts.ImportJobsWrite,
			AccessChecker:   opts.AccessChecker,
			IDGen:           opts.IDGen,
			Logger:          opts.Logger,
		}),
		listAudienceImportsH: importjob.NewList(importjob.ListOptions{
			ImportJobsRead: opts.ImportJobsRead,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		getAudienceImportH: importjob.NewGet(importjob.GetOptions{
			ImportJobsRead: opts.ImportJobsRead,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		startAudienceExportH: exportjob.NewStart(exportjob.StartOptions{
			ExportJobsWrite: opts.ExportJobsWrite,
			ContactsWrite:   opts.ContactsWrite,
			SegmentsWrite:   opts.SegmentsWrite,
			AccessChecker:   opts.AccessChecker,
			ExportEnabled:   opts.ExportEnabled,
			IDGen:           opts.IDGen,
			Logger:          opts.Logger,
		}),
		listAudienceExportsH: exportjob.NewList(exportjob.ListOptions{
			ExportJobsRead: opts.ExportJobsRead,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		getAudienceExportH: exportjob.NewGet(exportjob.GetOptions{
			ExportJobsRead: opts.ExportJobsRead,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		resolveAudienceSelectionH: resolve.NewSelection(resolve.SelectionOptions{
			ContactsRead:  opts.ContactsRead,
			SegmentsRead:  opts.SegmentsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
			Cache:         opts.RedisCache,
		}),
		resolveAudienceRecipientsH: resolve.NewRecipients(resolve.RecipientsOptions{
			ContactsRead:  opts.ContactsRead,
			ListsRead:     opts.ListsRead,
			SegmentsRead:  opts.SegmentsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
			Cache:         opts.RedisCache,
		}),
	}
}

// --- Contact facade methods ---

func (s *Service) CreateContact(ctx context.Context, input CreateContactInput) (*ContactResult, error) {
	contactResult, err := s.createContactH.Execute(ctx, contact.CreateCommand{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		Email:       input.Email,
		FirstName:   input.FirstName,
		LastName:    input.LastName,
		Tags:        input.Tags,
		Attributes:  input.Attributes,
		Now:         input.Now,
	})
	if err != nil {
		return nil, err
	}
	return &ContactResult{Contact: contactToDTO(*contactResult)}, nil
}

func (s *Service) ListContacts(ctx context.Context, query ports.ContactListQuery, userID string) (*ContactListResult, error) {
	contacts, cursor, err := s.listContactsH.Execute(ctx, contact.ListCommand{
		Query:  query,
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}
	return &ContactListResult{Contacts: contactSliceToDTOs(contacts), NextCursor: cursor}, nil
}

func (s *Service) GetContact(ctx context.Context, workspaceID, contactID, userID string) (*ContactResult, error) {
	contactResult, err := s.getContactH.Execute(ctx, contact.GetCommand{
		WorkspaceID: workspaceID,
		ContactID:   contactID,
		UserID:      userID,
	})
	if err != nil {
		return nil, err
	}
	return &ContactResult{Contact: contactToDTO(*contactResult)}, nil
}

func (s *Service) UpdateContact(ctx context.Context, input UpdateContactInput) (*ContactResult, error) {
	contactResult, err := s.updateContactH.Execute(ctx, contact.UpdateCommand{
		WorkspaceID: input.WorkspaceID,
		ContactID:   input.ContactID,
		UserID:      input.UserID,
		Email:       input.Email,
		FirstName:   input.FirstName,
		LastName:    input.LastName,
		Status:      input.Status,
		Tags:        input.Tags,
		Attributes:  input.Attributes,
		Now:         input.Now,
	})
	if err != nil {
		return nil, err
	}
	return &ContactResult{Contact: contactToDTO(*contactResult)}, nil
}

func (s *Service) ArchiveContact(ctx context.Context, workspaceID, contactID, userID string, now time.Time) error {
	return s.archiveContactH.Execute(ctx, contact.ArchiveCommand{
		WorkspaceID: workspaceID,
		ContactID:   contactID,
		UserID:      userID,
		Now:         now,
	})
}

// --- List facade methods ---

func (s *Service) CreateList(ctx context.Context, workspaceID, userID, name, description string, metadata map[string]any, now time.Time) (*ListResult, error) {
	listResult, err := s.createListH.Execute(ctx, list.CreateCommand{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Name:        name,
		Description: description,
		Metadata:    metadata,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return &ListResult{List: listToDTO(*listResult)}, nil
}

func (s *Service) ListLists(ctx context.Context, workspaceID, userID string, limit int, cursor string) (*ListListResult, error) {
	result, err := s.listListsH.Execute(ctx, list.ListCommand{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		return nil, err
	}
	return &ListListResult{
		Lists:      listSliceToDTOs(result.Lists),
		Counts:     result.Counts,
		NextCursor: result.NextCursor,
	}, nil
}

func (s *Service) UpdateListMemberships(ctx context.Context, workspaceID, listID, userID, mode string, contactIDs []string, now time.Time) (*MembershipUpdateResult, error) {
	result, err := s.updateListMembershipsH.Execute(ctx, list.UpdateMembershipsCommand{
		WorkspaceID: workspaceID,
		ListID:      listID,
		UserID:      userID,
		Mode:        mode,
		ContactIDs:  contactIDs,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return &MembershipUpdateResult{
		Result: MembershipUpdateResultDTO{
			AddedCount:   result.AddedCount,
			RemovedCount: result.RemovedCount,
			SkippedCount: result.SkippedCount,
		},
	}, nil
}

// --- Segment facade methods ---

func (s *Service) CreateSegment(ctx context.Context, workspaceID, userID, name string, definition map[string]any, now time.Time) (*SegmentResult, error) {
	segmentResult, err := s.createSegmentH.Execute(ctx, segment.CreateCommand{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Name:        name,
		Definition:  definition,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return &SegmentResult{Segment: segmentToDTO(*segmentResult)}, nil
}

func (s *Service) ListSegments(ctx context.Context, workspaceID, userID, status string, limit int, cursor string) (*SegmentListResult, error) {
	segments, nextCursor, err := s.listSegmentsH.Execute(ctx, segment.ListCommand{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Status:      status,
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		return nil, err
	}
	return &SegmentListResult{Segments: segmentSliceToDTOs(segments), NextCursor: nextCursor}, nil
}

func (s *Service) UpdateSegment(ctx context.Context, workspaceID, segmentID, userID, name string, definition map[string]any, status string, now time.Time) (*SegmentResult, error) {
	segmentResult, err := s.updateSegmentH.Execute(ctx, segment.UpdateCommand{
		WorkspaceID: workspaceID,
		SegmentID:   segmentID,
		UserID:      userID,
		Name:        name,
		Definition:  definition,
		Status:      status,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return &SegmentResult{Segment: segmentToDTO(*segmentResult)}, nil
}

// --- Import facade methods ---

func (s *Service) StartAudienceImport(ctx context.Context, workspaceID, userID, sourceURI, dedupeMode string, metadata map[string]any, now time.Time) (*ImportJobResult, error) {
	job, err := s.startAudienceImportH.Execute(ctx, importjob.StartCommand{
		WorkspaceID: workspaceID,
		UserID:      userID,
		SourceURI:   sourceURI,
		DedupeMode:  dedupeMode,
		Metadata:    metadata,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return &ImportJobResult{Job: importJobToDTO(*job)}, nil
}

func (s *Service) ListAudienceImports(ctx context.Context, workspaceID, userID, status string, limit int, cursor string) (*ImportJobListResult, error) {
	jobs, nextCursor, err := s.listAudienceImportsH.Execute(ctx, importjob.ListCommand{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Status:      status,
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		return nil, err
	}
	return &ImportJobListResult{Jobs: importJobSliceToDTOs(jobs), NextCursor: nextCursor}, nil
}

func (s *Service) GetAudienceImport(ctx context.Context, workspaceID, jobID, userID string) (*ImportJobResult, error) {
	job, err := s.getAudienceImportH.Execute(ctx, importjob.GetCommand{
		WorkspaceID: workspaceID,
		JobID:       jobID,
		UserID:      userID,
	})
	if err != nil {
		return nil, err
	}
	return &ImportJobResult{Job: importJobToDTO(*job)}, nil
}

// --- Export facade methods ---

func (s *Service) StartAudienceExport(ctx context.Context, workspaceID, userID, format string, zipOutput bool, filters map[string]any, selectedFields []string, now time.Time) (*ExportJobResult, error) {
	job, err := s.startAudienceExportH.Execute(ctx, exportjob.StartCommand{
		WorkspaceID:    workspaceID,
		UserID:         userID,
		Format:         format,
		ZipOutput:      zipOutput,
		Filters:        filters,
		SelectedFields: selectedFields,
		Now:            now,
	})
	if err != nil {
		return nil, err
	}
	dto := exportJobToDTO(*job)
	return &ExportJobResult{Job: dto}, nil
}

func (s *Service) ListAudienceExports(ctx context.Context, workspaceID, userID, status string, limit int, cursor string) (*ExportJobListResult, error) {
	jobs, nextCursor, err := s.listAudienceExportsH.Execute(ctx, exportjob.ListCommand{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Status:      status,
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		return nil, err
	}
	result := make([]ExportJobDTO, 0, len(jobs))
	for _, job := range jobs {
		dto := exportJobToDTO(job)
		s.enrichExportJob(ctx, &dto)
		result = append(result, dto)
	}
	return &ExportJobListResult{Jobs: result, NextCursor: nextCursor}, nil
}

func (s *Service) GetAudienceExport(ctx context.Context, workspaceID, jobID, userID string) (*ExportJobResult, error) {
	job, err := s.getAudienceExportH.Execute(ctx, exportjob.GetCommand{
		WorkspaceID: workspaceID,
		JobID:       jobID,
		UserID:      userID,
	})
	if err != nil {
		return nil, err
	}
	dto := exportJobToDTO(*job)
	s.enrichExportJob(ctx, &dto)
	return &ExportJobResult{Job: dto}, nil
}

// --- Audience resolution facade methods ---

func (s *Service) ResolveAudienceSelection(ctx context.Context, workspaceID, userID string, ref AudienceSelectionRef) ([]string, error) {
	return s.resolveAudienceSelectionH.Execute(ctx, resolve.SelectionCommand{
		WorkspaceID: workspaceID,
		UserID:      userID,
		ListID:      ref.ListID,
		SegmentID:   ref.SegmentID,
		ContactIDs:  ref.ContactIDs,
	})
}

func (s *Service) enrichExportJob(ctx context.Context, job *ExportJobDTO) {
	if job == nil || s.artifactSigner == nil || job.Status != string(domain.JobStatusCompleted) || job.ArtifactURI == "" {
		return
	}

	url, err := s.artifactSigner.PresignGetObject(ctx, job.ArtifactURI, s.downloadURLTTL)
	if err != nil {
		s.log.Error("failed to presign export artifact", "artifact_uri", job.ArtifactURI, "error", err)
		return
	}

	expiresAt := time.Now().UTC().Add(s.downloadURLTTL)
	job.DownloadURL = url
	job.DownloadURLTTL = &expiresAt
}

func (s *Service) EstimateAudienceSize(ctx context.Context, workspaceID, userID string, ref AudienceSelectionRef) (int, error) {
	ids, err := s.resolveAudienceSelectionH.Execute(ctx, resolve.SelectionCommand{
		WorkspaceID: workspaceID,
		UserID:      userID,
		ListID:      ref.ListID,
		SegmentID:   ref.SegmentID,
		ContactIDs:  ref.ContactIDs,
	})
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (s *Service) ResolveAudienceRecipients(ctx context.Context, workspaceID, userID string, ref AudienceSelectionRef) ([]AudienceRecipient, error) {
	recipients, err := s.resolveAudienceRecipientsH.Execute(ctx, resolve.RecipientsCommand{
		WorkspaceID: workspaceID,
		UserID:      userID,
		ListID:      ref.ListID,
		SegmentID:   ref.SegmentID,
		ContactIDs:  ref.ContactIDs,
	})
	if err != nil {
		return nil, err
	}
	result := make([]AudienceRecipient, len(recipients))
	for i, r := range recipients {
		result[i] = AudienceRecipient{
			ContactID:       r.ContactID,
			Email:           r.Email,
			EmailNormalized: r.EmailNormalized,
			FirstName:       r.FirstName,
			LastName:        r.LastName,
			Tags:            r.Tags,
			Attributes:      r.Attributes,
		}
	}
	return result, nil
}
