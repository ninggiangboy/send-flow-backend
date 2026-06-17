package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type AttachmentRepository struct {
	db platformpostgres.DBTX
}

func NewAttachmentRepository(db platformpostgres.DBTX) *AttachmentRepository {
	return &AttachmentRepository{db: db}
}

func (r *AttachmentRepository) getDB(ctx context.Context) platformpostgres.DBTX {
	if tx := platformpostgres.TxFromCtx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (r *AttachmentRepository) Create(ctx context.Context, attachment domain.AttachmentManifest) error {
	db := r.getDB(ctx)
	_, err := db.Exec(ctx,
		`INSERT INTO transactional_attachments (id, workspace_id, transactional_request_id,
		                                        storage_key, original_filename, content_type,
		                                        byte_size, sha256_digest, disposition,
		                                        content_id, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		attachment.ID, attachment.WorkspaceID, attachment.TransactionalRequestID,
		attachment.StorageKey, attachment.OriginalFilename, attachment.ContentType,
		attachment.ByteSize, attachment.SHA256Digest, attachment.Disposition,
		platformpostgres.Nullable(attachment.ContentID), attachment.CreatedAt,
	)
	return err
}

func (r *AttachmentRepository) CreateMany(ctx context.Context, attachments []domain.AttachmentManifest) error {
	db := r.getDB(ctx)
	if len(attachments) == 0 {
		return nil
	}

	values := make([]string, 0, len(attachments))
	args := make([]any, 0, len(attachments)*11)
	idx := 1

	for _, att := range attachments {
		values = append(values, "($"+
			platformpostgres.Itoa(idx)+",$"+platformpostgres.Itoa(idx+1)+
			",$"+platformpostgres.Itoa(idx+2)+",$"+platformpostgres.Itoa(idx+3)+
			",$"+platformpostgres.Itoa(idx+4)+",$"+platformpostgres.Itoa(idx+5)+
			",$"+platformpostgres.Itoa(idx+6)+",$"+platformpostgres.Itoa(idx+7)+
			",$"+platformpostgres.Itoa(idx+8)+",$"+platformpostgres.Itoa(idx+9)+
			",$"+platformpostgres.Itoa(idx+10)+")")

		args = append(args,
			att.ID, att.WorkspaceID, att.TransactionalRequestID,
			att.StorageKey, att.OriginalFilename, att.ContentType,
			att.ByteSize, att.SHA256Digest, att.Disposition,
			platformpostgres.Nullable(att.ContentID), att.CreatedAt,
		)
		idx += 11
	}

	_, err := db.Exec(ctx,
		`INSERT INTO transactional_attachments (id, workspace_id, transactional_request_id,
		                                        storage_key, original_filename, content_type,
		                                        byte_size, sha256_digest, disposition,
		                                        content_id, created_at)
		 VALUES `+strings.Join(values, ","), args...)
	return err
}

func scanAttachmentRow(row scannable) (*domain.AttachmentManifest, error) {
	var att domain.AttachmentManifest
	var contentID *string

	err := row.Scan(
		&att.ID, &att.WorkspaceID, &att.TransactionalRequestID,
		&att.StorageKey, &att.OriginalFilename, &att.ContentType,
		&att.ByteSize, &att.SHA256Digest, &att.Disposition,
		&contentID, &att.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	if contentID != nil {
		att.ContentID = *contentID
	}

	return &att, nil
}

func scanAttachmentRows(rows pgx.Rows) ([]domain.AttachmentManifest, error) {
	var results []domain.AttachmentManifest
	for rows.Next() {
		att, err := scanAttachmentRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *att)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.AttachmentManifest{}
	}
	return results, nil
}

func (r *AttachmentRepository) ListByRequest(ctx context.Context, workspaceID, requestID string) ([]domain.AttachmentManifest, error) {
	db := r.getDB(ctx)
	rows, err := db.Query(ctx,
		`SELECT id, workspace_id, transactional_request_id,
		        storage_key, original_filename, content_type,
		        byte_size, sha256_digest, disposition,
		        content_id, created_at
		 FROM transactional_attachments
		 WHERE workspace_id = $1 AND transactional_request_id = $2
		 ORDER BY created_at`,
		workspaceID, requestID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanAttachmentRows(rows)
}
