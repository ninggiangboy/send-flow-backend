package worker

import (
	"archive/zip"
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
	segmentsRead    audienceports.SegmentReadRepository
	outboxWriter    audienceports.OutboxWriter
	txManager       transaction.UnitOfWork
	objStorage      objectstorage.ObjectStorage
	idGen           func() (string, error)
	log             *slog.Logger
	batchSize       int
	guard           *PollingGuard
}

func NewAudienceExportProcessor(
	exportJobsWrite audienceports.ExportJobWriteRepository,
	contactsRead audienceports.ContactReadRepository,
	segmentsRead audienceports.SegmentReadRepository,
	outboxWriter audienceports.OutboxWriter,
	txManager transaction.UnitOfWork,
	objStorage objectstorage.ObjectStorage,
	log *slog.Logger,
	pollInterval time.Duration,
	batchSize int,
) *AudienceExportProcessor {
	name := "audience.export_processor"
	return &AudienceExportProcessor{
		name:            name,
		exportJobsWrite: exportJobsWrite,
		contactsRead:    contactsRead,
		segmentsRead:    segmentsRead,
		outboxWriter:    outboxWriter,
		txManager:       txManager,
		objStorage:      objStorage,
		idGen:           id.NewUUIDGenerator().New,
		log:             log.With("worker", name),
		batchSize:       batchSize,
		guard:           NewPollingGuard(name, pollInterval, 3, 0, log),
	}
}

func (p *AudienceExportProcessor) Name() string {
	return p.name
}

func (p *AudienceExportProcessor) Run(ctx context.Context) error {
	p.log.Info("starting audience export processor",
		"batch_size", p.batchSize,
	)
	return p.guard.Run(ctx, p)
}

func (p *AudienceExportProcessor) Poll(ctx context.Context) (bool, error) {
	jobs, err := p.exportJobsWrite.ClaimQueuedExportJobs(ctx, p.batchSize, time.Now().UTC())
	if err != nil {
		p.log.Error("failed to claim queued export jobs", "error", err)
		return false, nil
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

	return len(jobs) > 0, nil
}

func (p *AudienceExportProcessor) processJob(ctx context.Context, job domain.AudienceExportJob) error {
	log := p.log.With("export_id", job.ID, "workspace_id", job.WorkspaceID)
	log.Info("processing export job", "format", job.Format)

	now := time.Now().UTC()

	query, segmentRules, err := p.buildExportQuery(ctx, job)
	if err != nil {
		errMsg := fmt.Sprintf("invalid export job: %v", err)
		log.Error(errMsg)
		if failErr := p.failJob(ctx, job, errMsg, now); failErr != nil {
			log.Error("failed to persist export failure", "error", failErr)
		}
		return err
	}
	artifactExt := string(job.Format)
	if job.ZipOutput {
		artifactExt += ".zip"
	}
	artifactKey := fmt.Sprintf("exports/%s/%s/%s.%s", job.WorkspaceID, job.ID, job.ID, artifactExt)

	var contentType string
	if job.ZipOutput {
		contentType = "application/zip"
	} else {
		switch job.Format {
		case domain.ExportFormatCSV:
			contentType = "text/csv"
		case domain.ExportFormatJSON:
			contentType = "application/json"
		default:
			err := fmt.Errorf("unsupported export format: %s", job.Format)
			if failErr := p.failJob(ctx, job, err.Error(), now); failErr != nil {
				log.Error("failed to persist export failure", "error", failErr)
			}
			return err
		}
	}

	pr, pw := io.Pipe()

	progress := func(processed int) error {
		return p.exportJobsWrite.UpdateExportJobProgress(ctx, job.WorkspaceID, job.ID, int64(processed), time.Now().UTC())
	}

	streamCtx, streamCancel := context.WithCancel(ctx)

	type writeResult struct {
		count int
		err   error
	}
	resultCh := make(chan writeResult, 1)
	go func() {
		var res writeResult
		if job.ZipOutput {
			res.count, res.err = p.writeZIPStream(streamCtx, pw, job, query, segmentRules, progress)
		} else {
			switch job.Format {
			case domain.ExportFormatCSV:
				res.count, res.err = p.writeCSVStream(streamCtx, pw, job, query, segmentRules, progress)
			case domain.ExportFormatJSON:
				res.count, res.err = p.writeJSONStream(streamCtx, pw, job, query, segmentRules, progress)
			}
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
		if failErr := p.failJob(ctx, job, errMsg, now); failErr != nil {
			log.Error("failed to persist export failure", "error", failErr)
		}
		return err
	}
	streamCancel()

	res := <-resultCh
	if res.err != nil {
		errMsg := fmt.Sprintf("export generation failed: %v", res.err)
		log.Error(errMsg)
		if failErr := p.failJob(ctx, job, errMsg, now); failErr != nil {
			log.Error("failed to persist export failure", "error", failErr)
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
		"zip_output", job.ZipOutput,
	)

	return nil
}

func (p *AudienceExportProcessor) writeZIPStream(ctx context.Context, w io.Writer, job domain.AudienceExportJob, query audienceports.ContactListQuery, segmentRules []domain.SegmentRule, progress func(int) error) (int, error) {
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	entry, err := zipWriter.Create(fmt.Sprintf("%s.%s", job.ID, job.Format))
	if err != nil {
		return 0, err
	}

	switch job.Format {
	case domain.ExportFormatCSV:
		return p.writeCSVStream(ctx, entry, job, query, segmentRules, progress)
	case domain.ExportFormatJSON:
		return p.writeJSONStream(ctx, entry, job, query, segmentRules, progress)
	default:
		err := fmt.Errorf("unsupported export format: %s", job.Format)
		return 0, err
	}
}

func (p *AudienceExportProcessor) buildExportQuery(ctx context.Context, job domain.AudienceExportJob) (audienceports.ContactListQuery, []domain.SegmentRule, error) {
	query := audienceports.ContactListQuery{
		WorkspaceID: job.WorkspaceID,
		Limit:       1000,
	}
	var segmentRules []domain.SegmentRule

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
			segment, err := p.segmentsRead.FindSegmentByID(ctx, job.WorkspaceID, segmentID)
			if err != nil {
				return query, nil, err
			}
			if segment.Status != domain.SegmentStatusReady {
				return query, nil, domain.ErrSegmentDefinitionInvalid
			}
			segmentRules, err = domain.ExtractRules(segment.DefinitionJSON)
			if err != nil {
				return query, nil, domain.ErrSegmentDefinitionInvalid
			}
		}
		if q, ok := job.FiltersJSON["query"].(string); ok && q != "" {
			query.Q = q
		}
	} else {
		query.Status = string(domain.ContactStatusActive)
	}

	return query, segmentRules, nil
}

func (p *AudienceExportProcessor) writeCSVStream(ctx context.Context, w io.Writer, job domain.AudienceExportJob, query audienceports.ContactListQuery, segmentRules []domain.SegmentRule, progress func(int) error) (int, error) {
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return 0, err
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	fields := job.SelectedFields
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
			if len(segmentRules) > 0 && !domain.MatchesSegment(c, segmentRules) {
				continue
			}
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
		writer.Flush()
		if err := writer.Error(); err != nil {
			return rowCount, err
		}
		if err := progress(rowCount); err != nil {
			return rowCount, err
		}

		if cursor == "" {
			break
		}
		query.Cursor = cursor
	}

	return rowCount, nil
}

func (p *AudienceExportProcessor) writeJSONStream(ctx context.Context, w io.Writer, job domain.AudienceExportJob, query audienceports.ContactListQuery, segmentRules []domain.SegmentRule, progress func(int) error) (int, error) {
	fields := job.SelectedFields
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
			if len(segmentRules) > 0 && !domain.MatchesSegment(c, segmentRules) {
				continue
			}
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
		if err := progress(rowCount); err != nil {
			return rowCount, err
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

func (p *AudienceExportProcessor) failJob(ctx context.Context, job domain.AudienceExportJob, errorSummary string, now time.Time) error {
	return p.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := p.exportJobsWrite.MarkExportJobFailed(txCtx, job.WorkspaceID, job.ID, errorSummary, now); err != nil {
			return err
		}
		if p.outboxWriter == nil {
			return nil
		}

		eventID, err := p.idGen()
		if err != nil {
			return err
		}

		payload := audiencecontracts.ExportFailedPayload{
			JobID:        job.ID,
			WorkspaceID:  job.WorkspaceID,
			ErrorMessage: errorSummary,
		}

		envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
			EventID:       eventID,
			EventType:     audiencecontracts.EventExportFailedV1,
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

		return p.outboxWriter.Save(txCtx, audienceports.OutboxEvent{
			ID:            eventID,
			AggregateType: "audience_export_job",
			AggregateID:   job.ID,
			EventType:     audiencecontracts.EventExportFailedV1,
			Payload:       payloadBytes,
			WorkspaceID:   job.WorkspaceID,
			OccurredAt:    now,
		})
	})
}
