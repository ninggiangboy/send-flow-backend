package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/archivecontact"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/createcontact"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/createlist"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/createsegment"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/getaudienceexport"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/getaudienceimport"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/getcontact"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/listaudienceexports"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/listaudienceimports"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/listcontacts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/listlists"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/listsegments"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/resolveaudiencerecipients"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/resolveaudienceselection"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/startaudienceexport"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/startaudienceimport"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/updatecontact"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/updatelistmemberships"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app/updatesegment"
	audienceredis "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/infrastructure/redis"
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
	createContactH             *createcontact.Handler
	getContactH                *getcontact.Handler
	listContactsH              *listcontacts.Handler
	updateContactH             *updatecontact.Handler
	archiveContactH            *archivecontact.Handler
	createListH                *createlist.Handler
	listListsH                 *listlists.Handler
	updateListMembershipsH     *updatelistmemberships.Handler
	createSegmentH             *createsegment.Handler
	listSegmentsH              *listsegments.Handler
	updateSegmentH             *updatesegment.Handler
	startAudienceImportH       *startaudienceimport.Handler
	listAudienceImportsH       *listaudienceimports.Handler
	getAudienceImportH         *getaudienceimport.Handler
	startAudienceExportH       *startaudienceexport.Handler
	listAudienceExportsH       *listaudienceexports.Handler
	getAudienceExportH         *getaudienceexport.Handler
	resolveAudienceSelectionH  *resolveaudienceselection.Handler
	resolveAudienceRecipientsH *resolveaudiencerecipients.Handler
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
		createContactH: createcontact.New(createcontact.Options{
			ContactsRead:  opts.ContactsRead,
			ContactsWrite: opts.ContactsWrite,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		getContactH: getcontact.New(getcontact.Options{
			ContactsRead:  opts.ContactsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		listContactsH: listcontacts.New(listcontacts.Options{
			ContactsRead:  opts.ContactsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		updateContactH: updatecontact.New(updatecontact.Options{
			ContactsRead:  opts.ContactsRead,
			ContactsWrite: opts.ContactsWrite,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		archiveContactH: archivecontact.New(archivecontact.Options{
			ContactsRead:  opts.ContactsRead,
			ContactsWrite: opts.ContactsWrite,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		createListH: createlist.New(createlist.Options{
			ListsWrite:    opts.ListsWrite,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		listListsH: listlists.New(listlists.Options{
			ListsRead:     opts.ListsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		updateListMembershipsH: updatelistmemberships.New(updatelistmemberships.Options{
			ListsRead:     opts.ListsRead,
			ListsWrite:    opts.ListsWrite,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		createSegmentH: createsegment.New(createsegment.Options{
			SegmentsWrite: opts.SegmentsWrite,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		listSegmentsH: listsegments.New(listsegments.Options{
			SegmentsRead:  opts.SegmentsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		updateSegmentH: updatesegment.New(updatesegment.Options{
			SegmentsRead:  opts.SegmentsRead,
			SegmentsWrite: opts.SegmentsWrite,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		startAudienceImportH: startaudienceimport.New(startaudienceimport.Options{
			ImportJobsWrite: opts.ImportJobsWrite,
			AccessChecker:   opts.AccessChecker,
			IDGen:           opts.IDGen,
			Logger:          opts.Logger,
		}),
		listAudienceImportsH: listaudienceimports.New(listaudienceimports.Options{
			ImportJobsRead: opts.ImportJobsRead,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		getAudienceImportH: getaudienceimport.New(getaudienceimport.Options{
			ImportJobsRead: opts.ImportJobsRead,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		startAudienceExportH: startaudienceexport.New(startaudienceexport.Options{
			ExportJobsWrite: opts.ExportJobsWrite,
			ContactsRead:    opts.ContactsRead,
			SegmentsRead:    opts.SegmentsRead,
			AccessChecker:   opts.AccessChecker,
			ExportEnabled:   opts.ExportEnabled,
			IDGen:           opts.IDGen,
			Logger:          opts.Logger,
		}),
		listAudienceExportsH: listaudienceexports.New(listaudienceexports.Options{
			ExportJobsRead: opts.ExportJobsRead,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		getAudienceExportH: getaudienceexport.New(getaudienceexport.Options{
			ExportJobsRead: opts.ExportJobsRead,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		resolveAudienceSelectionH: resolveaudienceselection.New(resolveaudienceselection.Options{
			ContactsRead:  opts.ContactsRead,
			SegmentsRead:  opts.SegmentsRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
			Cache:         opts.RedisCache,
		}),
		resolveAudienceRecipientsH: resolveaudiencerecipients.New(resolveaudiencerecipients.Options{
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
	contact, err := s.createContactH.Execute(ctx, createcontact.Command{
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
	return &ContactResult{Contact: contactToDTO(*contact)}, nil
}

func (s *Service) ListContacts(ctx context.Context, query ports.ContactListQuery, userID string) (*ContactListResult, error) {
	contacts, cursor, err := s.listContactsH.Execute(ctx, listcontacts.Command{
		Query:  query,
		UserID: userID,
	})
	if err != nil {
		return nil, err
	}
	return &ContactListResult{Contacts: contactSliceToDTOs(contacts), NextCursor: cursor}, nil
}

func (s *Service) GetContact(ctx context.Context, workspaceID, contactID, userID string) (*ContactResult, error) {
	contact, err := s.getContactH.Execute(ctx, getcontact.Command{
		WorkspaceID: workspaceID,
		ContactID:   contactID,
		UserID:      userID,
	})
	if err != nil {
		return nil, err
	}
	return &ContactResult{Contact: contactToDTO(*contact)}, nil
}

func (s *Service) UpdateContact(ctx context.Context, input UpdateContactInput) (*ContactResult, error) {
	contact, err := s.updateContactH.Execute(ctx, updatecontact.Command{
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
	return &ContactResult{Contact: contactToDTO(*contact)}, nil
}

func (s *Service) ArchiveContact(ctx context.Context, workspaceID, contactID, userID string, now time.Time) error {
	return s.archiveContactH.Execute(ctx, archivecontact.Command{
		WorkspaceID: workspaceID,
		ContactID:   contactID,
		UserID:      userID,
		Now:         now,
	})
}

// --- List facade methods ---

func (s *Service) CreateList(ctx context.Context, workspaceID, userID, name, description string, metadata map[string]any, now time.Time) (*ListResult, error) {
	list, err := s.createListH.Execute(ctx, createlist.Command{
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
	return &ListResult{List: listToDTO(*list)}, nil
}

func (s *Service) ListLists(ctx context.Context, workspaceID, userID string, limit int, cursor string) (*ListListResult, error) {
	result, err := s.listListsH.Execute(ctx, listlists.Command{
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
	result, err := s.updateListMembershipsH.Execute(ctx, updatelistmemberships.Command{
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
	segment, err := s.createSegmentH.Execute(ctx, createsegment.Command{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Name:        name,
		Definition:  definition,
		Now:         now,
	})
	if err != nil {
		return nil, err
	}
	return &SegmentResult{Segment: segmentToDTO(*segment)}, nil
}

func (s *Service) ListSegments(ctx context.Context, workspaceID, userID, status string, limit int, cursor string) (*SegmentListResult, error) {
	segments, nextCursor, err := s.listSegmentsH.Execute(ctx, listsegments.Command{
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
	segment, err := s.updateSegmentH.Execute(ctx, updatesegment.Command{
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
	return &SegmentResult{Segment: segmentToDTO(*segment)}, nil
}

// --- Import facade methods ---

func (s *Service) StartAudienceImport(ctx context.Context, workspaceID, userID, sourceURI, dedupeMode string, metadata map[string]any, now time.Time) (*ImportJobResult, error) {
	job, err := s.startAudienceImportH.Execute(ctx, startaudienceimport.Command{
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
	jobs, nextCursor, err := s.listAudienceImportsH.Execute(ctx, listaudienceimports.Command{
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
	job, err := s.getAudienceImportH.Execute(ctx, getaudienceimport.Command{
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
	job, err := s.startAudienceExportH.Execute(ctx, startaudienceexport.Command{
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
	jobs, nextCursor, err := s.listAudienceExportsH.Execute(ctx, listaudienceexports.Command{
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
	job, err := s.getAudienceExportH.Execute(ctx, getaudienceexport.Command{
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
	return s.resolveAudienceSelectionH.Execute(ctx, resolveaudienceselection.Command{
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
	ids, err := s.resolveAudienceSelectionH.Execute(ctx, resolveaudienceselection.Command{
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
	recipients, err := s.resolveAudienceRecipientsH.Execute(ctx, resolveaudiencerecipients.Command{
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
