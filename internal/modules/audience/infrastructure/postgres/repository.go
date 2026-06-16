package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
	platformpostgres "github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
)

type ContactReadRepository struct {
	db platformpostgres.DBTX
}

type ContactWriteRepository struct {
	db platformpostgres.DBTX
}

type ListReadRepository struct {
	db platformpostgres.DBTX
}

type ListWriteRepository struct {
	db platformpostgres.DBTX
}

type SegmentReadRepository struct {
	db platformpostgres.DBTX
}

type SegmentWriteRepository struct {
	db platformpostgres.DBTX
}

type ImportJobReadRepository struct {
	db platformpostgres.DBTX
}

type ImportJobWriteRepository struct {
	db platformpostgres.DBTX
}

type ExportJobReadRepository struct {
	db platformpostgres.DBTX
}

type ExportJobWriteRepository struct {
	db platformpostgres.DBTX
}

func NewContactReadRepository(db platformpostgres.DBTX) *ContactReadRepository {
	return &ContactReadRepository{db: db}
}

func NewContactWriteRepository(db platformpostgres.DBTX) *ContactWriteRepository {
	return &ContactWriteRepository{db: db}
}

func NewListReadRepository(db platformpostgres.DBTX) *ListReadRepository {
	return &ListReadRepository{db: db}
}

func NewListWriteRepository(db platformpostgres.DBTX) *ListWriteRepository {
	return &ListWriteRepository{db: db}
}

func NewSegmentReadRepository(db platformpostgres.DBTX) *SegmentReadRepository {
	return &SegmentReadRepository{db: db}
}

func NewSegmentWriteRepository(db platformpostgres.DBTX) *SegmentWriteRepository {
	return &SegmentWriteRepository{db: db}
}

func NewImportJobReadRepository(db platformpostgres.DBTX) *ImportJobReadRepository {
	return &ImportJobReadRepository{db: db}
}

func NewImportJobWriteRepository(db platformpostgres.DBTX) *ImportJobWriteRepository {
	return &ImportJobWriteRepository{db: db}
}

func NewExportJobReadRepository(db platformpostgres.DBTX) *ExportJobReadRepository {
	return &ExportJobReadRepository{db: db}
}

func NewExportJobWriteRepository(db platformpostgres.DBTX) *ExportJobWriteRepository {
	return &ExportJobWriteRepository{db: db}
}

// --- Contact Read ---

func (r *ContactReadRepository) FindContactByID(ctx context.Context, workspaceID, contactID string) (*domain.Contact, error) {
	var c domain.Contact
	var tagsJSON, attrsJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, email, email_normalized, COALESCE(first_name, ''), COALESCE(last_name, ''), status, tags, attributes, created_at, updated_at, archived_at FROM contacts WHERE id = $1 AND workspace_id = $2`,
		contactID, workspaceID,
	).Scan(&c.ID, &c.WorkspaceID, &c.Email, &c.EmailNormalized, &c.FirstName, &c.LastName, &c.Status, &tagsJSON, &attrsJSON, &c.CreatedAt, &c.UpdatedAt, &c.ArchivedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrContactNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(tagsJSON, &c.Tags); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(attrsJSON, &c.Attributes); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ContactReadRepository) FindContactByEmail(ctx context.Context, workspaceID, emailNormalized string) (*domain.Contact, error) {
	var c domain.Contact
	var tagsJSON, attrsJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, email, email_normalized, COALESCE(first_name, ''), COALESCE(last_name, ''), status, tags, attributes, created_at, updated_at, archived_at FROM contacts WHERE workspace_id = $1 AND email_normalized = $2`,
		workspaceID, emailNormalized,
	).Scan(&c.ID, &c.WorkspaceID, &c.Email, &c.EmailNormalized, &c.FirstName, &c.LastName, &c.Status, &tagsJSON, &attrsJSON, &c.CreatedAt, &c.UpdatedAt, &c.ArchivedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrContactNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(tagsJSON, &c.Tags); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(attrsJSON, &c.Attributes); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ContactReadRepository) ListContacts(ctx context.Context, query ports.ContactListQuery) ([]domain.Contact, string, error) {
	args := []any{query.WorkspaceID}
	where := "WHERE workspace_id = $1"
	argIdx := 2

	if query.Status != "" {
		where += " AND status = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.Status)
		argIdx++
	}

	if query.Q != "" {
		pattern := strings.ReplaceAll(query.Q, "%", "\\%")
		pattern = strings.ReplaceAll(pattern, "_", "\\_")
		likePattern := "%" + strings.ToLower(pattern) + "%"
		where += " AND (email_normalized LIKE $" + platformpostgres.Itoa(argIdx) + " ESCAPE '\\' OR COALESCE(first_name, '') LIKE $" + platformpostgres.Itoa(argIdx) + " ESCAPE '\\' OR COALESCE(last_name, '') LIKE $" + platformpostgres.Itoa(argIdx) + " ESCAPE '\\')"
		args = append(args, likePattern)
		argIdx++
	}

	if query.ListID != "" {
		where += " AND id IN (SELECT contact_id FROM audience_list_memberships WHERE workspace_id = $1 AND list_id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, query.ListID)
		argIdx++
	}

	if query.Cursor != "" {
		where += " AND (created_at, id) < (SELECT created_at, id FROM contacts WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, query.Cursor)
		argIdx++
	}

	limit := query.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	where += " ORDER BY created_at DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	sql := "SELECT id, workspace_id, email, email_normalized, COALESCE(first_name, ''), COALESCE(last_name, ''), status, tags, attributes, created_at, updated_at, archived_at FROM contacts " + where

	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var results []domain.Contact
	for rows.Next() {
		var c domain.Contact
		var tagsJSON, attrsJSON []byte
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.Email, &c.EmailNormalized, &c.FirstName, &c.LastName, &c.Status, &tagsJSON, &attrsJSON, &c.CreatedAt, &c.UpdatedAt, &c.ArchivedAt); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(tagsJSON, &c.Tags); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(attrsJSON, &c.Attributes); err != nil {
			return nil, "", err
		}
		results = append(results, c)
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
		results = []domain.Contact{}
	}

	return results, nextCursor, nil
}

// --- Contact Write ---

func (w *ContactWriteRepository) CreateContact(ctx context.Context, c domain.Contact) error {
	tagsJSON, err := json.Marshal(c.Tags)
	if err != nil {
		return err
	}
	attrsJSON, err := json.Marshal(c.Attributes)
	if err != nil {
		return err
	}
	_, err = w.db.Exec(ctx,
		`INSERT INTO contacts (id, workspace_id, email, email_normalized, first_name, last_name, status, tags, attributes, created_at, updated_at, archived_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		c.ID, c.WorkspaceID, c.Email, c.EmailNormalized, platformpostgres.Nullable(c.FirstName), platformpostgres.Nullable(c.LastName), string(c.Status), tagsJSON, attrsJSON, c.CreatedAt, c.UpdatedAt, c.ArchivedAt,
	)
	if err != nil {
		if platformpostgres.IsUniqueViolation(err) {
			return domain.ErrContactEmailConflict
		}
		return err
	}
	return nil
}

func (w *ContactWriteRepository) UpdateContact(ctx context.Context, c domain.Contact) error {
	tagsJSON, err := json.Marshal(c.Tags)
	if err != nil {
		return err
	}
	attrsJSON, err := json.Marshal(c.Attributes)
	if err != nil {
		return err
	}
	tag, err := w.db.Exec(ctx,
		`UPDATE contacts SET email=$1, email_normalized=$2, first_name=$3, last_name=$4, status=$5, tags=$6, attributes=$7, updated_at=$8 WHERE id=$9 AND workspace_id=$10`,
		c.Email, c.EmailNormalized, platformpostgres.Nullable(c.FirstName), platformpostgres.Nullable(c.LastName), string(c.Status), tagsJSON, attrsJSON, c.UpdatedAt, c.ID, c.WorkspaceID,
	)
	if err != nil {
		if platformpostgres.IsUniqueViolation(err) {
			return domain.ErrContactEmailConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrContactNotFound
	}
	return nil
}

func (w *ContactWriteRepository) ArchiveContact(ctx context.Context, workspaceID, contactID string, archivedAt time.Time) error {
	tag, err := w.db.Exec(ctx,
		`UPDATE contacts SET status='archived', archived_at=$1, updated_at=$1 WHERE id=$2 AND workspace_id=$3`,
		archivedAt, contactID, workspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrContactNotFound
	}
	return nil
}

// --- List Read ---

func (r *ListReadRepository) FindListByID(ctx context.Context, workspaceID, listID string) (*domain.AudienceList, error) {
	var l domain.AudienceList
	var metadataJSON []byte
	var desc *string
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, name, description, metadata, created_at, updated_at, archived_at FROM audience_lists WHERE id = $1 AND workspace_id = $2`,
		listID, workspaceID,
	).Scan(&l.ID, &l.WorkspaceID, &l.Name, &desc, &metadataJSON, &l.CreatedAt, &l.UpdatedAt, &l.ArchivedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrListNotFound
		}
		return nil, err
	}
	if desc != nil {
		l.Description = *desc
	}
	if err := json.Unmarshal(metadataJSON, &l.Metadata); err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *ListReadRepository) ListLists(ctx context.Context, query ports.ListListQuery) ([]domain.AudienceList, string, error) {
	args := []any{query.WorkspaceID}
	where := "WHERE workspace_id = $1 AND archived_at IS NULL"
	argIdx := 2

	if query.Cursor != "" {
		where += " AND (created_at, id) < (SELECT created_at, id FROM audience_lists WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, query.Cursor)
		argIdx++
	}

	limit := query.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	where += " ORDER BY created_at DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx,
		`SELECT id, workspace_id, name, COALESCE(description, ''), metadata, created_at, updated_at, archived_at FROM audience_lists `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var results []domain.AudienceList
	for rows.Next() {
		var l domain.AudienceList
		var metadataJSON []byte
		if err := rows.Scan(&l.ID, &l.WorkspaceID, &l.Name, &l.Description, &metadataJSON, &l.CreatedAt, &l.UpdatedAt, &l.ArchivedAt); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(metadataJSON, &l.Metadata); err != nil {
			return nil, "", err
		}
		results = append(results, l)
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
		results = []domain.AudienceList{}
	}
	return results, nextCursor, nil
}

func (r *ListReadRepository) CountContactsByList(ctx context.Context, workspaceID string, listIDs []string) (map[string]int64, error) {
	if len(listIDs) == 0 {
		return map[string]int64{}, nil
	}

	args := []any{workspaceID}
	placeholders := make([]string, len(listIDs))
	for i, id := range listIDs {
		placeholders[i] = "$" + platformpostgres.Itoa(i+2)
		args = append(args, id)
	}

	rows, err := r.db.Query(ctx,
		`SELECT list_id, COUNT(*) FROM audience_list_memberships WHERE workspace_id = $1 AND list_id IN (`+strings.Join(placeholders, ",")+`) GROUP BY list_id`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int64, len(listIDs))
	for _, id := range listIDs {
		counts[id] = 0
	}
	for rows.Next() {
		var listID string
		var count int64
		if err := rows.Scan(&listID, &count); err != nil {
			return nil, err
		}
		counts[listID] = count
	}
	return counts, rows.Err()
}

// --- List Write ---

func (w *ListWriteRepository) CreateList(ctx context.Context, l domain.AudienceList) error {
	metadataJSON, err := json.Marshal(l.Metadata)
	if err != nil {
		return err
	}
	_, err = w.db.Exec(ctx,
		`INSERT INTO audience_lists (id, workspace_id, name, description, metadata, created_at, updated_at, archived_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		l.ID, l.WorkspaceID, l.Name, platformpostgres.Nullable(l.Description), metadataJSON, l.CreatedAt, l.UpdatedAt, l.ArchivedAt,
	)
	if err != nil {
		if platformpostgres.IsUniqueViolation(err) {
			return domain.ErrListNameConflict
		}
		return err
	}
	return nil
}

func (w *ListWriteRepository) ReplaceListMemberships(ctx context.Context, workspaceID, listID string, contactIDs []string, now time.Time) (domain.MembershipUpdateResult, error) {
	tx, err := w.beginTx(ctx)
	if err != nil {
		return domain.MembershipUpdateResult{}, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `DELETE FROM audience_list_memberships WHERE workspace_id = $1 AND list_id = $2`, workspaceID, listID)
	if err != nil {
		return domain.MembershipUpdateResult{}, err
	}
	removedCount := int(tag.RowsAffected())

	addedCount := 0
	skippedCount := 0
	seen := map[string]struct{}{}
	for _, contactID := range contactIDs {
		if _, ok := seen[contactID]; ok {
			skippedCount++
			continue
		}
		seen[contactID] = struct{}{}

		var exists int
		if err := tx.QueryRow(ctx, `SELECT 1 FROM contacts WHERE id = $1 AND workspace_id = $2`, contactID, workspaceID).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return domain.MembershipUpdateResult{}, err
		}

		_, err := tx.Exec(ctx,
			`INSERT INTO audience_list_memberships (workspace_id, list_id, contact_id, created_at) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			workspaceID, listID, contactID, now,
		)
		if err != nil {
			return domain.MembershipUpdateResult{}, err
		}
		addedCount++
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.MembershipUpdateResult{}, err
	}

	return domain.MembershipUpdateResult{AddedCount: addedCount, RemovedCount: removedCount, SkippedCount: skippedCount}, nil
}

func (w *ListWriteRepository) MergeListMemberships(ctx context.Context, workspaceID, listID string, contactIDs []string, now time.Time) (domain.MembershipUpdateResult, error) {
	tx, err := w.beginTx(ctx)
	if err != nil {
		return domain.MembershipUpdateResult{}, err
	}
	defer tx.Rollback(ctx)

	addedCount := 0
	skippedCount := 0
	seen := map[string]struct{}{}
	for _, contactID := range contactIDs {
		if _, ok := seen[contactID]; ok {
			skippedCount++
			continue
		}
		seen[contactID] = struct{}{}

		var exists int
		if err := tx.QueryRow(ctx, `SELECT 1 FROM contacts WHERE id = $1 AND workspace_id = $2`, contactID, workspaceID).Scan(&exists); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return domain.MembershipUpdateResult{}, err
		}

		tag, err := tx.Exec(ctx,
			`INSERT INTO audience_list_memberships (workspace_id, list_id, contact_id, created_at) VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
			workspaceID, listID, contactID, now,
		)
		if err != nil {
			return domain.MembershipUpdateResult{}, err
		}
		if tag.RowsAffected() > 0 {
			addedCount++
		} else {
			skippedCount++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.MembershipUpdateResult{}, err
	}

	return domain.MembershipUpdateResult{AddedCount: addedCount, SkippedCount: skippedCount}, nil
}

// --- Segment Read ---

func (r *SegmentReadRepository) FindSegmentByID(ctx context.Context, workspaceID, segmentID string) (*domain.Segment, error) {
	var s domain.Segment
	var defJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, name, definition_json, status, created_at, updated_at FROM segments WHERE id = $1 AND workspace_id = $2`,
		segmentID, workspaceID,
	).Scan(&s.ID, &s.WorkspaceID, &s.Name, &defJSON, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSegmentNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(defJSON, &s.DefinitionJSON); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SegmentReadRepository) ListSegments(ctx context.Context, query ports.SegmentListQuery) ([]domain.Segment, string, error) {
	args := []any{query.WorkspaceID}
	where := "WHERE workspace_id = $1"
	argIdx := 2

	if query.Status != "" {
		where += " AND status = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.Status)
		argIdx++
	}

	if query.Cursor != "" {
		where += " AND (created_at, id) < (SELECT created_at, id FROM segments WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, query.Cursor)
		argIdx++
	}

	limit := query.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	where += " ORDER BY created_at DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx,
		`SELECT id, workspace_id, name, definition_json, status, created_at, updated_at FROM segments `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var results []domain.Segment
	for rows.Next() {
		var s domain.Segment
		var defJSON []byte
		if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.Name, &defJSON, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(defJSON, &s.DefinitionJSON); err != nil {
			return nil, "", err
		}
		results = append(results, s)
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
		results = []domain.Segment{}
	}
	return results, nextCursor, nil
}

// --- Segment Write ---

func (w *SegmentWriteRepository) CreateSegment(ctx context.Context, s domain.Segment) error {
	defJSON, err := json.Marshal(s.DefinitionJSON)
	if err != nil {
		return err
	}
	_, err = w.db.Exec(ctx,
		`INSERT INTO segments (id, workspace_id, name, definition_json, status, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		s.ID, s.WorkspaceID, s.Name, defJSON, string(s.Status), s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		if platformpostgres.IsUniqueViolation(err) {
			return domain.ErrSegmentNameConflict
		}
		return err
	}
	return nil
}

func (w *SegmentWriteRepository) UpdateSegment(ctx context.Context, s domain.Segment) error {
	defJSON, err := json.Marshal(s.DefinitionJSON)
	if err != nil {
		return err
	}
	tag, err := w.db.Exec(ctx,
		`UPDATE segments SET name=$1, definition_json=$2, status=$3, updated_at=$4 WHERE id=$5 AND workspace_id=$6`,
		s.Name, defJSON, string(s.Status), s.UpdatedAt, s.ID, s.WorkspaceID,
	)
	if err != nil {
		if platformpostgres.IsUniqueViolation(err) {
			return domain.ErrSegmentNameConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSegmentNotFound
	}
	return nil
}

// --- Import Job Read ---

func (r *ImportJobReadRepository) FindImportJobByID(ctx context.Context, workspaceID, jobID string) (*domain.AudienceImportJob, error) {
	var j domain.AudienceImportJob
	var metadataJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, source_uri, dedupe_mode, status, processed_count, created_count, updated_count, failed_count, COALESCE(error_summary, ''), metadata, created_at, updated_at, completed_at FROM audience_import_jobs WHERE id = $1 AND workspace_id = $2`,
		jobID, workspaceID,
	).Scan(&j.ID, &j.WorkspaceID, &j.SourceURI, &j.DedupeMode, &j.Status, &j.ProcessedCount, &j.CreatedCount, &j.UpdatedCount, &j.FailedCount, &j.ErrorSummary, &metadataJSON, &j.CreatedAt, &j.UpdatedAt, &j.CompletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrImportJobNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(metadataJSON, &j.Metadata); err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *ImportJobReadRepository) ListImportJobs(ctx context.Context, query ports.ImportJobListQuery) ([]domain.AudienceImportJob, string, error) {
	args := []any{query.WorkspaceID}
	where := "WHERE workspace_id = $1"
	argIdx := 2

	if query.Status != "" {
		where += " AND status = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.Status)
		argIdx++
	}

	if query.Cursor != "" {
		where += " AND (created_at, id) < (SELECT created_at, id FROM audience_import_jobs WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, query.Cursor)
		argIdx++
	}

	limit := query.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	where += " ORDER BY created_at DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx,
		`SELECT id, workspace_id, source_uri, dedupe_mode, status, processed_count, created_count, updated_count, failed_count, COALESCE(error_summary, ''), metadata, created_at, updated_at, completed_at FROM audience_import_jobs `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var results []domain.AudienceImportJob
	for rows.Next() {
		var j domain.AudienceImportJob
		var metadataJSON []byte
		if err := rows.Scan(&j.ID, &j.WorkspaceID, &j.SourceURI, &j.DedupeMode, &j.Status, &j.ProcessedCount, &j.CreatedCount, &j.UpdatedCount, &j.FailedCount, &j.ErrorSummary, &metadataJSON, &j.CreatedAt, &j.UpdatedAt, &j.CompletedAt); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(metadataJSON, &j.Metadata); err != nil {
			return nil, "", err
		}
		results = append(results, j)
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
		results = []domain.AudienceImportJob{}
	}
	return results, nextCursor, nil
}

// --- Import Job Write ---

func (w *ImportJobWriteRepository) CreateImportJob(ctx context.Context, j domain.AudienceImportJob) error {
	metadataJSON, err := json.Marshal(j.Metadata)
	if err != nil {
		return err
	}
	_, err = w.db.Exec(ctx,
		`INSERT INTO audience_import_jobs (id, workspace_id, source_uri, dedupe_mode, status, processed_count, created_count, updated_count, failed_count, error_summary, metadata, created_at, updated_at, completed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		j.ID, j.WorkspaceID, j.SourceURI, string(j.DedupeMode), string(j.Status), j.ProcessedCount, j.CreatedCount, j.UpdatedCount, j.FailedCount, platformpostgres.Nullable(j.ErrorSummary), metadataJSON, j.CreatedAt, j.UpdatedAt, j.CompletedAt,
	)
	return err
}

func (w *ImportJobWriteRepository) UpdateImportJob(ctx context.Context, j domain.AudienceImportJob) error {
	metadataJSON, err := json.Marshal(j.Metadata)
	if err != nil {
		return err
	}
	tag, err := w.db.Exec(ctx,
		`UPDATE audience_import_jobs SET status=$1, processed_count=$2, created_count=$3, updated_count=$4, failed_count=$5, error_summary=$6, metadata=$7, updated_at=$8, completed_at=$9 WHERE id=$10 AND workspace_id=$11`,
		string(j.Status), j.ProcessedCount, j.CreatedCount, j.UpdatedCount, j.FailedCount, platformpostgres.Nullable(j.ErrorSummary), metadataJSON, j.UpdatedAt, j.CompletedAt, j.ID, j.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrImportJobNotFound
	}
	return nil
}

func (w *ImportJobWriteRepository) ClaimQueuedImportJobs(ctx context.Context, limit int, now time.Time) ([]domain.AudienceImportJob, error) {
	rows, err := w.db.Query(ctx,
		`UPDATE audience_import_jobs SET status='running', updated_at=$1
		 WHERE id IN (SELECT id FROM audience_import_jobs WHERE status='queued' ORDER BY created_at ASC LIMIT $2 FOR UPDATE SKIP LOCKED)
		 RETURNING id, workspace_id, source_uri, dedupe_mode, status, processed_count, created_count, updated_count, failed_count, COALESCE(error_summary, ''), metadata, created_at, updated_at, completed_at`,
		now, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []domain.AudienceImportJob
	for rows.Next() {
		var j domain.AudienceImportJob
		var metadataJSON []byte
		if err := rows.Scan(&j.ID, &j.WorkspaceID, &j.SourceURI, &j.DedupeMode, &j.Status, &j.ProcessedCount, &j.CreatedCount, &j.UpdatedCount, &j.FailedCount, &j.ErrorSummary, &metadataJSON, &j.CreatedAt, &j.UpdatedAt, &j.CompletedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(metadataJSON, &j.Metadata); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if jobs == nil {
		jobs = []domain.AudienceImportJob{}
	}
	return jobs, nil
}

func (w *ImportJobWriteRepository) MarkImportJobRunning(ctx context.Context, workspaceID, jobID string, now time.Time) error {
	tag, err := w.db.Exec(ctx,
		`UPDATE audience_import_jobs SET status='running', updated_at=$1 WHERE id=$2 AND workspace_id=$3 AND status='queued'`,
		now, jobID, workspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrImportJobNotFound
	}
	return nil
}

func (w *ImportJobWriteRepository) MarkImportJobCompleted(ctx context.Context, workspaceID, jobID string, counts ports.ImportCounts, now time.Time) error {
	tag, err := w.db.Exec(ctx,
		`UPDATE audience_import_jobs SET status='completed', processed_count=$1, created_count=$2, updated_count=$3, failed_count=$4, completed_at=$5, updated_at=$5 WHERE id=$6 AND workspace_id=$7 AND status='running'`,
		counts.ProcessedCount, counts.CreatedCount, counts.UpdatedCount, counts.FailedCount, now, jobID, workspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrImportJobNotFound
	}
	return nil
}

func (w *ImportJobWriteRepository) MarkImportJobFailed(ctx context.Context, workspaceID, jobID string, errorSummary string, now time.Time) error {
	tag, err := w.db.Exec(ctx,
		`UPDATE audience_import_jobs SET status='failed', error_summary=$1, completed_at=$2, updated_at=$2 WHERE id=$3 AND workspace_id=$4`,
		errorSummary, now, jobID, workspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrImportJobNotFound
	}
	return nil
}

// --- Export Job Read ---

func (r *ExportJobReadRepository) FindExportJobByID(ctx context.Context, workspaceID, jobID string) (*domain.AudienceExportJob, error) {
	var j domain.AudienceExportJob
	var filtersJSON, fieldsJSON []byte
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, filters_json, selected_fields, format, zip_output, status, processed_count, estimated_total_count, COALESCE(artifact_uri, ''), COALESCE(error_summary, ''), created_at, updated_at, completed_at FROM audience_export_jobs WHERE id = $1 AND workspace_id = $2`,
		jobID, workspaceID,
	).Scan(&j.ID, &j.WorkspaceID, &filtersJSON, &fieldsJSON, &j.Format, &j.ZipOutput, &j.Status, &j.ProcessedCount, &j.EstimatedTotalCount, &j.ArtifactURI, &j.ErrorSummary, &j.CreatedAt, &j.UpdatedAt, &j.CompletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrExportJobNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(filtersJSON, &j.FiltersJSON); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(fieldsJSON, &j.SelectedFields); err != nil {
		return nil, err
	}
	return &j, nil
}

func (r *ExportJobReadRepository) ListExportJobs(ctx context.Context, query ports.ExportJobListQuery) ([]domain.AudienceExportJob, string, error) {
	args := []any{query.WorkspaceID}
	where := "WHERE workspace_id = $1"
	argIdx := 2

	if query.Status != "" {
		where += " AND status = $" + platformpostgres.Itoa(argIdx)
		args = append(args, query.Status)
		argIdx++
	}

	if query.Cursor != "" {
		where += " AND (created_at, id) < (SELECT created_at, id FROM audience_export_jobs WHERE id = $" + platformpostgres.Itoa(argIdx) + ")"
		args = append(args, query.Cursor)
		argIdx++
	}

	limit := query.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	where += " ORDER BY created_at DESC, id DESC LIMIT $" + platformpostgres.Itoa(argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Query(ctx,
		`SELECT id, workspace_id, filters_json, selected_fields, format, zip_output, status, processed_count, estimated_total_count, COALESCE(artifact_uri, ''), COALESCE(error_summary, ''), created_at, updated_at, completed_at FROM audience_export_jobs `+where, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var results []domain.AudienceExportJob
	for rows.Next() {
		var j domain.AudienceExportJob
		var filtersJSON, fieldsJSON []byte
		if err := rows.Scan(&j.ID, &j.WorkspaceID, &filtersJSON, &fieldsJSON, &j.Format, &j.ZipOutput, &j.Status, &j.ProcessedCount, &j.EstimatedTotalCount, &j.ArtifactURI, &j.ErrorSummary, &j.CreatedAt, &j.UpdatedAt, &j.CompletedAt); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(filtersJSON, &j.FiltersJSON); err != nil {
			return nil, "", err
		}
		if err := json.Unmarshal(fieldsJSON, &j.SelectedFields); err != nil {
			return nil, "", err
		}
		results = append(results, j)
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
		results = []domain.AudienceExportJob{}
	}
	return results, nextCursor, nil
}

// --- Export Job Write ---

func (w *ExportJobWriteRepository) CreateExportJob(ctx context.Context, j domain.AudienceExportJob) error {
	filtersJSON, err := json.Marshal(j.FiltersJSON)
	if err != nil {
		return err
	}
	fieldsJSON, err := json.Marshal(j.SelectedFields)
	if err != nil {
		return err
	}
	_, err = w.db.Exec(ctx,
		`INSERT INTO audience_export_jobs (id, workspace_id, filters_json, selected_fields, format, zip_output, status, processed_count, estimated_total_count, artifact_uri, error_summary, created_at, updated_at, completed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		j.ID, j.WorkspaceID, filtersJSON, fieldsJSON, string(j.Format), j.ZipOutput, string(j.Status), j.ProcessedCount, j.EstimatedTotalCount, platformpostgres.Nullable(j.ArtifactURI), platformpostgres.Nullable(j.ErrorSummary), j.CreatedAt, j.UpdatedAt, j.CompletedAt,
	)
	return err
}

func (w *ExportJobWriteRepository) UpdateExportJob(ctx context.Context, j domain.AudienceExportJob) error {
	filtersJSON, err := json.Marshal(j.FiltersJSON)
	if err != nil {
		return err
	}
	fieldsJSON, err := json.Marshal(j.SelectedFields)
	if err != nil {
		return err
	}
	tag, err := w.db.Exec(ctx,
		`UPDATE audience_export_jobs SET filters_json=$1, selected_fields=$2, format=$3, zip_output=$4, status=$5, processed_count=$6, estimated_total_count=$7, artifact_uri=$8, error_summary=$9, updated_at=$10, completed_at=$11 WHERE id=$12 AND workspace_id=$13`,
		filtersJSON, fieldsJSON, string(j.Format), j.ZipOutput, string(j.Status), j.ProcessedCount, j.EstimatedTotalCount, platformpostgres.Nullable(j.ArtifactURI), platformpostgres.Nullable(j.ErrorSummary), j.UpdatedAt, j.CompletedAt, j.ID, j.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrExportJobNotFound
	}
	return nil
}

func (w *ExportJobWriteRepository) ClaimQueuedExportJobs(ctx context.Context, limit int, now time.Time) ([]domain.AudienceExportJob, error) {
	rows, err := w.db.Query(ctx,
		`UPDATE audience_export_jobs SET status='running', updated_at=$1
		 WHERE id IN (SELECT id FROM audience_export_jobs WHERE status='queued' ORDER BY created_at ASC LIMIT $2 FOR UPDATE SKIP LOCKED)
		 RETURNING id, workspace_id, filters_json, selected_fields, format, zip_output, status, processed_count, estimated_total_count, COALESCE(artifact_uri, ''), COALESCE(error_summary, ''), created_at, updated_at, completed_at`,
		now, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []domain.AudienceExportJob
	for rows.Next() {
		var j domain.AudienceExportJob
		var filtersJSON, fieldsJSON []byte
		if err := rows.Scan(&j.ID, &j.WorkspaceID, &filtersJSON, &fieldsJSON, &j.Format, &j.ZipOutput, &j.Status, &j.ProcessedCount, &j.EstimatedTotalCount, &j.ArtifactURI, &j.ErrorSummary, &j.CreatedAt, &j.UpdatedAt, &j.CompletedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(filtersJSON, &j.FiltersJSON); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(fieldsJSON, &j.SelectedFields); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if jobs == nil {
		jobs = []domain.AudienceExportJob{}
	}
	return jobs, nil
}

func (w *ExportJobWriteRepository) MarkExportJobRunning(ctx context.Context, workspaceID, jobID string, now time.Time) error {
	tag, err := w.db.Exec(ctx,
		`UPDATE audience_export_jobs SET status='running', updated_at=$1 WHERE id=$2 AND workspace_id=$3 AND status='queued'`,
		now, jobID, workspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrExportJobNotFound
	}
	return nil
}

func (w *ExportJobWriteRepository) UpdateExportJobProgress(ctx context.Context, workspaceID, jobID string, processedCount int64, now time.Time) error {
	tag, err := w.db.Exec(ctx,
		`UPDATE audience_export_jobs SET processed_count=$1, updated_at=$2 WHERE id=$3 AND workspace_id=$4 AND status='running'`,
		processedCount, now, jobID, workspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrExportJobNotFound
	}
	return nil
}

func (w *ExportJobWriteRepository) MarkExportJobCompleted(ctx context.Context, workspaceID, jobID string, artifactURI string, now time.Time) error {
	tag, err := w.db.Exec(ctx,
		`UPDATE audience_export_jobs SET status='completed', artifact_uri=$1, completed_at=$2, updated_at=$2 WHERE id=$3 AND workspace_id=$4 AND status='running'`,
		artifactURI, now, jobID, workspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrExportJobNotFound
	}
	return nil
}

func (w *ExportJobWriteRepository) MarkExportJobFailed(ctx context.Context, workspaceID, jobID string, errorSummary string, now time.Time) error {
	tag, err := w.db.Exec(ctx,
		`UPDATE audience_export_jobs SET status='failed', error_summary=$1, completed_at=$2, updated_at=$2 WHERE id=$3 AND workspace_id=$4`,
		errorSummary, now, jobID, workspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrExportJobNotFound
	}
	return nil
}

// --- Helpers ---

func (w *ListWriteRepository) beginTx(ctx context.Context) (pgx.Tx, error) {
	if conn, ok := w.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	}); ok {
		return conn.Begin(ctx)
	}
	return nil, errors.New("write repository requires a pool or conn that supports Begin")
}
