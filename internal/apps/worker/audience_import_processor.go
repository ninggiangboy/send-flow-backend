package worker

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	audiencecontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	audienceports "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/objectstorage"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type AudienceImportProcessor struct {
	name            string
	importJobsWrite audienceports.ImportJobWriteRepository
	contactsWrite   audienceports.ContactWriteRepository
	contactsRead    audienceports.ContactReadRepository
	outboxWriter    audienceports.OutboxWriter
	txManager       transaction.UnitOfWork
	objStorage      objectstorage.ObjectStorage
	idGen           func() (string, error)
	log             *slog.Logger
	pollInterval    time.Duration
	batchSize       int
}

func NewAudienceImportProcessor(
	importJobsWrite audienceports.ImportJobWriteRepository,
	contactsWrite audienceports.ContactWriteRepository,
	contactsRead audienceports.ContactReadRepository,
	outboxWriter audienceports.OutboxWriter,
	txManager transaction.UnitOfWork,
	objStorage objectstorage.ObjectStorage,
	log *slog.Logger,
	pollInterval time.Duration,
	batchSize int,
) *AudienceImportProcessor {
	return &AudienceImportProcessor{
		name:            "audience.import_processor",
		importJobsWrite: importJobsWrite,
		contactsWrite:   contactsWrite,
		contactsRead:    contactsRead,
		outboxWriter:    outboxWriter,
		txManager:       txManager,
		objStorage:      objStorage,
		idGen:           id.NewUUIDGenerator().New,
		log:             log.With("worker", "audience.import_processor"),
		pollInterval:    pollInterval,
		batchSize:       batchSize,
	}
}

func (p *AudienceImportProcessor) Name() string {
	return p.name
}

func (p *AudienceImportProcessor) Run(ctx context.Context) error {
	p.log.Info("starting audience import processor",
		"poll_interval", p.pollInterval,
		"batch_size", p.batchSize,
	)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("audience import processor stopped")
			return nil
		case <-ticker.C:
			p.processOnce(ctx)
		}
	}
}

func (p *AudienceImportProcessor) processOnce(ctx context.Context) {
	jobs, err := p.importJobsWrite.ClaimQueuedImportJobs(ctx, p.batchSize, time.Now().UTC())
	if err != nil {
		p.log.Error("failed to claim queued import jobs", "error", err)
		return
	}

	for _, job := range jobs {
		if err := p.processJob(ctx, job); err != nil {
			p.log.Error("import job failed",
				"import_id", job.ID,
				"workspace_id", job.WorkspaceID,
				"error", err,
			)
		}
	}
}

func (p *AudienceImportProcessor) processJob(ctx context.Context, job domain.AudienceImportJob) error {
	log := p.log.With("import_id", job.ID, "workspace_id", job.WorkspaceID)
	log.Info("processing import job", "source_uri", job.SourceURI)

	now := time.Now().UTC()

	reader, err := p.objStorage.GetObject(ctx, job.SourceURI)
	if err != nil {
		errMsg := fmt.Sprintf("failed to read source from storage: %v", err)
		log.Error(errMsg)
		if markErr := p.importJobsWrite.MarkImportJobFailed(ctx, job.WorkspaceID, job.ID, errMsg, now); markErr != nil {
			log.Error("failed to mark job failed", "error", markErr)
		}
		return fmt.Errorf("read source: %w", err)
	}
	defer reader.Close()

	var counts audienceports.ImportCounts

	switch {
	case strings.HasSuffix(job.SourceURI, ".csv"):
		counts, err = p.processCSV(ctx, job, reader, now)
	case strings.HasSuffix(job.SourceURI, ".json"):
		counts, err = p.processJSON(ctx, job, reader, now)
	default:
		err = fmt.Errorf("unsupported import format: %s", job.SourceURI)
	}

	if err != nil {
		errMsg := fmt.Sprintf("import processing failed: %v", err)
		log.Error(errMsg)
		if markErr := p.importJobsWrite.MarkImportJobFailed(ctx, job.WorkspaceID, job.ID, errMsg, now); markErr != nil {
			log.Error("failed to mark import job failed", "error", markErr)
		}
		return err
	}

	if err := p.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := p.importJobsWrite.MarkImportJobCompleted(txCtx, job.WorkspaceID, job.ID, counts, now); err != nil {
			return fmt.Errorf("mark completed: %w", err)
		}

		if p.outboxWriter != nil {
			eventID, err := p.idGen()
			if err != nil {
				return err
			}

			payload := audiencecontracts.ImportCompletedPayload{
				JobID:        job.ID,
				WorkspaceID:  job.WorkspaceID,
				SourceURI:    job.SourceURI,
				TotalRows:    counts.ProcessedCount,
				CreatedCount: counts.CreatedCount,
				UpdatedCount: counts.UpdatedCount,
				FailedCount:  counts.FailedCount,
			}

			envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
				EventID:       eventID,
				EventType:     audiencecontracts.EventImportCompletedV1,
				EventVersion:  1,
				AggregateType: "audience_import_job",
				AggregateID:   job.ID,
				WorkspaceID:   job.WorkspaceID,
				OccurredAt:    now,
			}, payload)
			if err != nil {
				return err
			}

			payloadBytes, err := events.Marshal(envelope)
			if err != nil {
				return err
			}

			if err := p.outboxWriter.Save(txCtx, audienceports.OutboxEvent{
				ID:            eventID,
				AggregateType: "audience_import_job",
				AggregateID:   job.ID,
				EventType:     audiencecontracts.EventImportCompletedV1,
				Payload:       payloadBytes,
				WorkspaceID:   job.WorkspaceID,
				OccurredAt:    now,
			}); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("transactional completion: %w", err)
	}

	log.Info("import job completed",
		"total_processed", counts.ProcessedCount,
		"created", counts.CreatedCount,
		"updated", counts.UpdatedCount,
		"failed", counts.FailedCount,
	)

	return nil
}

func (p *AudienceImportProcessor) processCSV(ctx context.Context, job domain.AudienceImportJob, reader io.ReadCloser, now time.Time) (audienceports.ImportCounts, error) {
	csvReader := csv.NewReader(reader)
	csvReader.TrimLeadingSpace = true

	headers, err := csvReader.Read()
	if err != nil {
		return audienceports.ImportCounts{}, fmt.Errorf("read csv headers: %w", err)
	}

	var counts audienceports.ImportCounts

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			counts.FailedCount++
			continue
		}

		row := make(map[string]string)
		for i, val := range record {
			if i < len(headers) {
				row[headers[i]] = val
			}
		}

		if err := p.processContactRow(ctx, job, row, &counts, now); err != nil {
			counts.FailedCount++
		}
		counts.ProcessedCount++
	}

	return counts, nil
}

func (p *AudienceImportProcessor) processJSON(ctx context.Context, job domain.AudienceImportJob, reader io.ReadCloser, now time.Time) (audienceports.ImportCounts, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return audienceports.ImportCounts{}, fmt.Errorf("read json: %w", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		return audienceports.ImportCounts{}, fmt.Errorf("unmarshal json: %w", err)
	}

	var counts audienceports.ImportCounts

	for _, row := range rows {
		strRow := make(map[string]string)
		for k, v := range row {
			strRow[k] = fmt.Sprintf("%v", v)
		}

		if err := p.processContactRow(ctx, job, strRow, &counts, now); err != nil {
			counts.FailedCount++
		}
		counts.ProcessedCount++
	}

	return counts, nil
}

func (p *AudienceImportProcessor) processContactRow(ctx context.Context, job domain.AudienceImportJob, row map[string]string, counts *audienceports.ImportCounts, now time.Time) error {
	email := strings.TrimSpace(row["email"])
	if email == "" {
		return fmt.Errorf("missing email in row")
	}

	contactID, err := p.idGen()
	if err != nil {
		return err
	}

	emailNorm := strings.ToLower(strings.TrimSpace(email))
	firstName := row["first_name"]
	lastName := row["last_name"]

	var tags []string
	if t, ok := row["tags"]; ok && strings.TrimSpace(t) != "" {
		tags = strings.Split(t, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	attributes := make(map[string]any)
	for k, v := range row {
		key := strings.ToLower(k)
		if key != "email" && key != "first_name" && key != "last_name" && key != "tags" {
			if strings.TrimSpace(v) != "" {
				attributes[k] = v
			}
		}
	}

	existing, findErr := p.contactsRead.FindContactByEmail(ctx, job.WorkspaceID, emailNorm)

	if job.DedupeMode == domain.DedupeModeByEmail && findErr == nil && existing != nil {
		existing.FirstName = firstName
		existing.LastName = lastName
		existing.Tags = mergeTags(existing.Tags, tags)
		existing.Attributes = mergeAttrs(existing.Attributes, attributes)
		existing.UpdatedAt = now

		if err := p.contactsWrite.UpdateContact(ctx, *existing); err != nil {
			return err
		}
		counts.UpdatedCount++
	} else {
		if findErr != nil && !errors.Is(findErr, domain.ErrContactNotFound) {
			return findErr
		}
		contact := domain.Contact{
			ID:              contactID,
			WorkspaceID:     job.WorkspaceID,
			Email:           email,
			EmailNormalized: emailNorm,
			FirstName:       firstName,
			LastName:        lastName,
			Status:          domain.ContactStatusActive,
			Tags:            tags,
			Attributes:      attributes,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if err := p.contactsWrite.CreateContact(ctx, contact); err != nil {
			return err
		}
		counts.CreatedCount++
	}

	return nil
}

func mergeTags(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	result := make([]string, 0, len(existing)+len(incoming))
	for _, t := range existing {
		t = strings.TrimSpace(t)
		if _, ok := seen[t]; !ok && t != "" {
			seen[t] = struct{}{}
			result = append(result, t)
		}
	}
	for _, t := range incoming {
		t = strings.TrimSpace(t)
		if _, ok := seen[t]; !ok && t != "" {
			seen[t] = struct{}{}
			result = append(result, t)
		}
	}
	return result
}

func mergeAttrs(existing, incoming map[string]any) map[string]any {
	if existing == nil {
		existing = make(map[string]any)
	}
	for k, v := range incoming {
		existing[k] = v
	}
	return existing
}
