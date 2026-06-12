package worker

import (
	"context"
	"encoding/csv"
	"encoding/json"
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

type AudienceExportProcessor struct {
	name            string
	exportJobsWrite audienceports.ExportJobWriteRepository
	contactsRead    audienceports.ContactReadRepository
	outboxWriter    audienceports.OutboxWriter
	txManager       transaction.UnitOfWork
	objStorage      objectstorage.ObjectStorage
	idGen           func() (string, error)
	log             *slog.Logger
	pollInterval    time.Duration
	batchSize       int
}

func NewAudienceExportProcessor(
	exportJobsWrite audienceports.ExportJobWriteRepository,
	contactsRead audienceports.ContactReadRepository,
	outboxWriter audienceports.OutboxWriter,
	txManager transaction.UnitOfWork,
	objStorage objectstorage.ObjectStorage,
	log *slog.Logger,
	pollInterval time.Duration,
	batchSize int,
) *AudienceExportProcessor {
	return &AudienceExportProcessor{
		name:            "audience.export_processor",
		exportJobsWrite: exportJobsWrite,
		contactsRead:    contactsRead,
		outboxWriter:    outboxWriter,
		txManager:       txManager,
		objStorage:      objStorage,
		idGen:           id.NewUUIDGenerator().New,
		log:             log.With("worker", "audience.export_processor"),
		pollInterval:    pollInterval,
		batchSize:       batchSize,
	}
}

func (p *AudienceExportProcessor) Name() string {
	return p.name
}

func (p *AudienceExportProcessor) Run(ctx context.Context) error {
	p.log.Info("starting audience export processor",
		"poll_interval", p.pollInterval,
		"batch_size", p.batchSize,
	)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("audience export processor stopped")
			return nil
		case <-ticker.C:
			p.processOnce(ctx)
		}
	}
}

func (p *AudienceExportProcessor) processOnce(ctx context.Context) {
	jobs, err := p.exportJobsWrite.ClaimQueuedExportJobs(ctx, p.batchSize, time.Now().UTC())
	if err != nil {
		p.log.Error("failed to claim queued export jobs", "error", err)
		return
	}

	for _, job := range jobs {
		if err := p.processJob(ctx, job); err != nil {
			p.log.Error("export job failed",
				"export_id", job.ID,
				"workspace_id", job.WorkspaceID,
				"error", err,
			)
		}
	}
}

func (p *AudienceExportProcessor) processJob(ctx context.Context, job domain.AudienceExportJob) error {
	log := p.log.With("export_id", job.ID, "workspace_id", job.WorkspaceID)
	log.Info("processing export job", "format", job.Format)

	now := time.Now().UTC()

	query := buildContactQuery(job)
	artifactKey := fmt.Sprintf("exports/%s/%s/%s.%s", job.WorkspaceID, job.ID, job.ID, job.Format)

	var contentType string
	switch job.Format {
	case domain.ExportFormatCSV:
		contentType = "text/csv"
	case domain.ExportFormatJSON:
		contentType = "application/json"
	default:
		err := fmt.Errorf("unsupported export format: %s", job.Format)
		if markErr := p.exportJobsWrite.MarkExportJobFailed(ctx, job.WorkspaceID, job.ID, err.Error(), now); markErr != nil {
			log.Error("failed to mark export job failed", "error", markErr)
		}
		return err
	}

	pr, pw := io.Pipe()

	streamCtx, streamCancel := context.WithCancel(ctx)

	type writeResult struct {
		count int
		err   error
	}
	resultCh := make(chan writeResult, 1)
	go func() {
		var res writeResult
		switch job.Format {
		case domain.ExportFormatCSV:
			res.count, res.err = p.writeCSVStream(streamCtx, pw, job.WorkspaceID, query, job.SelectedFields)
		case domain.ExportFormatJSON:
			res.count, res.err = p.writeJSONStream(streamCtx, pw, job.WorkspaceID, query, job.SelectedFields)
		}
		if res.err != nil {
			pw.CloseWithError(res.err)
		} else {
			pw.Close()
		}
		resultCh <- res
	}()

	if err := p.objStorage.PutObject(ctx, artifactKey, pr, contentType); err != nil {
		pr.Close()
		streamCancel()
		pw.CloseWithError(err)
		<-resultCh
		errMsg := fmt.Sprintf("failed to store artifact: %v", err)
		log.Error(errMsg)
		if markErr := p.exportJobsWrite.MarkExportJobFailed(ctx, job.WorkspaceID, job.ID, errMsg, now); markErr != nil {
			log.Error("failed to mark export job failed", "error", markErr)
		}
		return err
	}
	streamCancel()

	res := <-resultCh
	if res.err != nil {
		errMsg := fmt.Sprintf("export generation failed: %v", res.err)
		log.Error(errMsg)
		if markErr := p.exportJobsWrite.MarkExportJobFailed(ctx, job.WorkspaceID, job.ID, errMsg, now); markErr != nil {
			log.Error("failed to mark export job failed", "error", markErr)
		}
		return res.err
	}

	totalRows := int64(res.count)

	if err := p.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := p.exportJobsWrite.MarkExportJobCompleted(txCtx, job.WorkspaceID, job.ID, artifactKey, now); err != nil {
			return fmt.Errorf("mark completed: %w", err)
		}

		if p.outboxWriter != nil {
			eventID, err := p.idGen()
			if err != nil {
				return err
			}

			payload := audiencecontracts.ExportCompletedPayload{
				JobID:       job.ID,
				WorkspaceID: job.WorkspaceID,
				Format:      string(job.Format),
				ArtifactURI: artifactKey,
				TotalRows:   totalRows,
			}

			envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
				EventID:       eventID,
				EventType:     audiencecontracts.EventExportCompletedV1,
				EventVersion:  1,
				AggregateType: "audience_export_job",
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
				AggregateType: "audience_export_job",
				AggregateID:   job.ID,
				EventType:     audiencecontracts.EventExportCompletedV1,
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

	log.Info("export job completed",
		"artifact_uri", artifactKey,
		"total_rows", totalRows,
	)

	return nil
}

func buildContactQuery(job domain.AudienceExportJob) audienceports.ContactListQuery {
	query := audienceports.ContactListQuery{
		WorkspaceID: job.WorkspaceID,
		Limit:       1000,
	}

	if job.FiltersJSON != nil {
		if status, ok := job.FiltersJSON["status"].(string); ok && status != "" {
			query.Status = status
		} else {
			query.Status = string(domain.ContactStatusActive)
		}
		if listID, ok := job.FiltersJSON["list_id"].(string); ok && listID != "" {
			query.ListID = listID
		}
		if segmentID, ok := job.FiltersJSON["segment_id"].(string); ok && segmentID != "" {
			query.SegmentID = segmentID
		}
		if q, ok := job.FiltersJSON["query"].(string); ok && q != "" {
			query.Q = q
		}
	} else {
		query.Status = string(domain.ContactStatusActive)
	}

	return query
}

func (p *AudienceExportProcessor) writeCSVStream(ctx context.Context, w io.Writer, workspaceID string, query audienceports.ContactListQuery, selectedFields []string) (int, error) {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	fields := selectedFields
	if len(fields) == 0 {
		fields = []string{"email", "first_name", "last_name", "status", "tags", "created_at"}
	}

	if err := writer.Write(fields); err != nil {
		return 0, err
	}

	rowCount := 0

	for {
		contacts, cursor, err := p.contactsRead.ListContacts(ctx, query)
		if err != nil {
			return rowCount, err
		}

		for _, c := range contacts {
			record := make([]string, len(fields))
			for i, f := range fields {
				switch strings.ToLower(f) {
				case "email":
					record[i] = c.Email
				case "first_name":
					record[i] = c.FirstName
				case "last_name":
					record[i] = c.LastName
				case "status":
					record[i] = string(c.Status)
				case "tags":
					record[i] = strings.Join(c.Tags, ",")
				case "created_at":
					record[i] = c.CreatedAt.Format(time.RFC3339)
				case "updated_at":
					record[i] = c.UpdatedAt.Format(time.RFC3339)
				default:
					if v, ok := c.Attributes[f]; ok {
						record[i] = fmt.Sprintf("%v", v)
					}
				}
			}
			if err := writer.Write(record); err != nil {
				return rowCount, err
			}
			rowCount++
		}

		if cursor == "" {
			break
		}
		query.Cursor = cursor
	}

	return rowCount, nil
}

func (p *AudienceExportProcessor) writeJSONStream(ctx context.Context, w io.Writer, workspaceID string, query audienceports.ContactListQuery, selectedFields []string) (int, error) {
	fields := selectedFields
	if len(fields) == 0 {
		fields = []string{"email", "first_name", "last_name", "status", "tags", "created_at"}
	}

	writer := json.NewEncoder(w)

	if _, err := w.Write([]byte("[")); err != nil {
		return 0, err
	}

	rowCount := 0
	first := true

	for {
		contacts, cursor, err := p.contactsRead.ListContacts(ctx, query)
		if err != nil {
			return rowCount, err
		}

		for _, c := range contacts {
			row := make(map[string]any)
			for _, f := range fields {
				switch strings.ToLower(f) {
				case "email":
					row[f] = c.Email
				case "first_name":
					row[f] = c.FirstName
				case "last_name":
					row[f] = c.LastName
				case "status":
					row[f] = string(c.Status)
				case "tags":
					row[f] = c.Tags
				case "created_at":
					row[f] = c.CreatedAt.Format(time.RFC3339)
				case "updated_at":
					row[f] = c.UpdatedAt.Format(time.RFC3339)
				default:
					if v, ok := c.Attributes[f]; ok {
						row[f] = v
					}
				}
			}

			if first {
				first = false
			} else {
				if _, err := w.Write([]byte(",")); err != nil {
					return rowCount, err
				}
			}

			if err := writer.Encode(row); err != nil {
				return rowCount, err
			}
			rowCount++
		}

		if cursor == "" {
			break
		}
		query.Cursor = cursor
	}

	if _, err := w.Write([]byte("]")); err != nil {
		return rowCount, err
	}

	return rowCount, nil
}
