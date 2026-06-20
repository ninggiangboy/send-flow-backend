package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type TemplateReadRepository struct {
	db platformpostgres.DBTX
}

type TemplateWriteRepository struct {
	db platformpostgres.DBTX
}

func NewTemplateReadRepository(db platformpostgres.DBTX) *TemplateReadRepository {
	return &TemplateReadRepository{db: db}
}

func NewTemplateWriteRepository(db platformpostgres.DBTX) *TemplateWriteRepository {
	return &TemplateWriteRepository{db: db}
}

func (r *TemplateReadRepository) FindTemplateByID(ctx context.Context, workspaceID, templateID string) (*domain.Template, error) {
	var t domain.Template
	var metadataJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, name, status, subject, source_html, COALESCE(source_text, ''), metadata, COALESCE(current_version_id, ''), created_at, updated_at, archived_at FROM templates WHERE id = $1 AND workspace_id = $2`,
		templateID, workspaceID,
	).Scan(&t.ID, &t.WorkspaceID, &t.Name, &t.Status, &t.Subject, &t.SourceHTML, &t.SourceText, &metadataJSON, &t.CurrentVersionID, &t.CreatedAt, &t.UpdatedAt, &t.ArchivedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTemplateNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(metadataJSON, &t.Metadata); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TemplateReadRepository) ListTemplates(ctx context.Context, workspaceID, status, q, cursor string, limit int) ([]domain.Template, string, error) {
	args := []any{workspaceID}
	where := "WHERE workspace_id = $1"
	argIdx := 2

	if status != "" {
		where += " AND status = $" + platformpostgres.Itoa(argIdx)
		args = append(args, status)
		argIdx++
	}

	if q != "" {
		where += " AND (name ILIKE $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, "%"+q+"%")
		argIdx++
	}

	if cursor != "" {
		where += " AND (created_at, id) < (SELECT created_at, id FROM templates WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, cursor)
		argIdx++
	}

	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	where += " ORDER BY created_at DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx,
		`SELECT id, workspace_id, name, status, subject, source_html, COALESCE(source_text, ''), metadata, COALESCE(current_version_id, ''), created_at, updated_at, archived_at FROM templates `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var results []domain.Template
	for rows.Next() {
		var t domain.Template
		var metadataJSON []byte
		if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.Name, &t.Status, &t.Subject, &t.SourceHTML, &t.SourceText, &metadataJSON, &t.CurrentVersionID, &t.CreatedAt, &t.UpdatedAt, &t.ArchivedAt); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(metadataJSON, &t.Metadata); err != nil {
			return nil, "", err
		}
		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(results) > limit {
		nextCursor = results[limit-1].ID
		results = results[:limit]
	}
	if results == nil {
		results = []domain.Template{}
	}
	return results, nextCursor, nil
}

func (r *TemplateReadRepository) ListTemplateVersions(ctx context.Context, workspaceID, templateID, cursor string, limit int) ([]domain.TemplateVersion, string, error) {
	args := []any{workspaceID, templateID}
	where := "WHERE workspace_id = $1 AND template_id = $2"
	argIdx := 3

	if cursor != "" {
		where += " AND (version_number, id) < (SELECT version_number, id FROM template_versions WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, cursor)
		argIdx++
	}

	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	where += " ORDER BY version_number DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx,
		`SELECT id, workspace_id, template_id, version_number, subject, source_html, COALESCE(source_text, ''), metadata, published_at, created_at FROM template_versions `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var results []domain.TemplateVersion
	for rows.Next() {
		var v domain.TemplateVersion
		var metadataJSON []byte
		if err := rows.Scan(&v.ID, &v.WorkspaceID, &v.TemplateID, &v.VersionNumber, &v.Subject, &v.SourceHTML, &v.SourceText, &metadataJSON, &v.PublishedAt, &v.CreatedAt); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(metadataJSON, &v.Metadata); err != nil {
			return nil, "", err
		}
		results = append(results, v)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(results) > limit {
		nextCursor = results[limit-1].ID
		results = results[:limit]
	}
	if results == nil {
		results = []domain.TemplateVersion{}
	}
	return results, nextCursor, nil
}

func (r *TemplateReadRepository) FindTemplateVersionByID(ctx context.Context, workspaceID, versionID string) (*domain.TemplateVersion, error) {
	var v domain.TemplateVersion
	var metadataJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, template_id, version_number, subject, source_html, COALESCE(source_text, ''), metadata, published_at, created_at FROM template_versions WHERE id = $1 AND workspace_id = $2`,
		versionID, workspaceID,
	).Scan(&v.ID, &v.WorkspaceID, &v.TemplateID, &v.VersionNumber, &v.Subject, &v.SourceHTML, &v.SourceText, &metadataJSON, &v.PublishedAt, &v.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTemplateVersionNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(metadataJSON, &v.Metadata); err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *TemplateReadRepository) FindCurrentVersion(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error) {
	var v domain.TemplateVersion
	var metadataJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT tv.id, tv.workspace_id, tv.template_id, tv.version_number, tv.subject, tv.source_html, COALESCE(tv.source_text, ''), tv.metadata, tv.published_at, tv.created_at FROM template_versions tv JOIN templates t ON t.current_version_id = tv.id WHERE t.id = $1 AND t.workspace_id = $2`,
		templateID, workspaceID,
	).Scan(&v.ID, &v.WorkspaceID, &v.TemplateID, &v.VersionNumber, &v.Subject, &v.SourceHTML, &v.SourceText, &metadataJSON, &v.PublishedAt, &v.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTemplateVersionNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(metadataJSON, &v.Metadata); err != nil {
		return nil, err
	}
	return &v, nil
}

func (w *TemplateWriteRepository) CreateTemplate(ctx context.Context, t domain.Template) error {
	metadataJSON, err := json.Marshal(t.Metadata)
	if err != nil {
		return err
	}
	_, err = w.db.Exec(ctx,
		`INSERT INTO templates (id, workspace_id, name, status, subject, source_html, source_text, metadata, current_version_id, created_at, updated_at, archived_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		t.ID, t.WorkspaceID, t.Name, string(t.Status), t.Subject, t.SourceHTML, platformpostgres.Nullable(t.SourceText), metadataJSON, platformpostgres.Nullable(t.CurrentVersionID), t.CreatedAt, t.UpdatedAt, t.ArchivedAt,
	)
	if err != nil {
		if platformpostgres.IsUniqueViolation(err) {
			return domain.ErrTemplateNameConflict
		}
		return err
	}
	return nil
}

func (w *TemplateWriteRepository) UpdateTemplate(ctx context.Context, t domain.Template) error {
	metadataJSON, err := json.Marshal(t.Metadata)
	if err != nil {
		return err
	}
	tag, err := w.db.Exec(ctx,
		`UPDATE templates SET name=$1, status=$2, subject=$3, source_html=$4, source_text=$5, metadata=$6, current_version_id=$7, updated_at=$8 WHERE id=$9 AND workspace_id=$10`,
		t.Name, string(t.Status), t.Subject, t.SourceHTML, platformpostgres.Nullable(t.SourceText), metadataJSON, platformpostgres.Nullable(t.CurrentVersionID), t.UpdatedAt, t.ID, t.WorkspaceID,
	)
	if err != nil {
		if platformpostgres.IsUniqueViolation(err) {
			return domain.ErrTemplateNameConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTemplateNotFound
	}
	return nil
}

func (w *TemplateWriteRepository) PublishTemplateVersion(ctx context.Context, t domain.Template, v domain.TemplateVersion) error {
	tx, err := w.beginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	metadataJSON, err := json.Marshal(v.Metadata)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO template_versions (id, workspace_id, template_id, version_number, subject, source_html, source_text, metadata, published_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		v.ID, v.WorkspaceID, v.TemplateID, v.VersionNumber, v.Subject, v.SourceHTML, platformpostgres.Nullable(v.SourceText), metadataJSON, v.PublishedAt, v.CreatedAt,
	); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx,
		`UPDATE templates SET status=$1, current_version_id=$2, updated_at=$3 WHERE id=$4 AND workspace_id=$5`,
		string(t.Status), t.CurrentVersionID, t.UpdatedAt, t.ID, t.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrTemplateNotFound
	}

	return tx.Commit(ctx)
}

func (w *TemplateWriteRepository) CreateRenderSnapshot(ctx context.Context, s domain.RenderedTemplateSnapshot) error {
	warningsJSON, err := json.Marshal(s.Warnings)
	if err != nil {
		return err
	}
	_, err = w.db.Exec(ctx,
		`INSERT INTO template_render_snapshots (id, workspace_id, template_id, template_version_id, render_input_hash, subject, rendered_html, rendered_text, warnings, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		s.ID, s.WorkspaceID, platformpostgres.Nullable(s.TemplateID), platformpostgres.Nullable(s.TemplateVersionID), s.RenderInputHash, platformpostgres.Nullable(s.Subject), platformpostgres.Nullable(s.RenderedHTML), platformpostgres.Nullable(s.RenderedText), warningsJSON, s.CreatedAt,
	)
	return err
}

func (w *TemplateWriteRepository) beginTx(ctx context.Context) (pgx.Tx, error) {
	if conn, ok := w.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	}); ok {
		return conn.Begin(ctx)
	}
	return nil, errors.New("write repository requires a pool or conn that supports Begin")
}

// Combined repository — satisfies the merged TemplateWriteRepository interface.

type TemplateRepository struct {
	*TemplateReadRepository
	*TemplateWriteRepository
}

func NewTemplateRepository(read *TemplateReadRepository, write *TemplateWriteRepository) *TemplateRepository {
	return &TemplateRepository{read, write}
}
