package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type ReadRepository struct {
	db DBTX
}

type WriteRepository struct {
	db DBTX
}

func NewReadRepository(db DBTX) *ReadRepository {
	return &ReadRepository{db: db}
}

func NewWriteRepository(db DBTX) *WriteRepository {
	return &WriteRepository{db: db}
}

func (r *ReadRepository) FindByID(ctx context.Context, workspaceID, domainID string) (*domain.SenderDomain, []domain.DNSRecord, error) {
	sd, err := r.findDomainByID(ctx, workspaceID, domainID)
	if err != nil {
		return nil, nil, err
	}
	records, err := r.findRecordsByDomainID(ctx, domainID)
	if err != nil {
		return nil, nil, err
	}
	return sd, records, nil
}

func (r *ReadRepository) FindByDomain(ctx context.Context, workspaceID, normalizedDomain string) (*domain.SenderDomain, error) {
	return r.findDomainByDomain(ctx, workspaceID, normalizedDomain)
}

func (r *ReadRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.SenderDomain, error) {
	rows, err := r.db.Query(ctx, `SELECT id, workspace_id, domain, provider, status, verified_at, disabled_at, created_at, updated_at FROM sender_domains WHERE workspace_id = $1 ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.SenderDomain
	for rows.Next() {
		var sd domain.SenderDomain
		if err := rows.Scan(&sd.ID, &sd.WorkspaceID, &sd.Domain, &sd.Provider, &sd.Status, &sd.VerifiedAt, &sd.DisabledAt, &sd.CreatedAt, &sd.UpdatedAt); err != nil {
			return nil, err
		}
		results = append(results, sd)
	}
	if results == nil {
		results = []domain.SenderDomain{}
	}
	return results, rows.Err()
}

func (r *ReadRepository) findDomainByID(ctx context.Context, workspaceID, domainID string) (*domain.SenderDomain, error) {
	var sd domain.SenderDomain
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, domain, provider, status, verified_at, disabled_at, created_at, updated_at FROM sender_domains WHERE id = $1 AND workspace_id = $2`,
		domainID, workspaceID,
	).Scan(&sd.ID, &sd.WorkspaceID, &sd.Domain, &sd.Provider, &sd.Status, &sd.VerifiedAt, &sd.DisabledAt, &sd.CreatedAt, &sd.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDomainNotFound
		}
		return nil, err
	}
	return &sd, nil
}

func (r *ReadRepository) findDomainByDomain(ctx context.Context, workspaceID, normalizedDomain string) (*domain.SenderDomain, error) {
	var sd domain.SenderDomain
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, domain, provider, status, verified_at, disabled_at, created_at, updated_at FROM sender_domains WHERE workspace_id = $1 AND domain = $2`,
		workspaceID, normalizedDomain,
	).Scan(&sd.ID, &sd.WorkspaceID, &sd.Domain, &sd.Provider, &sd.Status, &sd.VerifiedAt, &sd.DisabledAt, &sd.CreatedAt, &sd.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDomainNotFound
		}
		return nil, err
	}
	return &sd, nil
}

func (r *ReadRepository) findRecordsByDomainID(ctx context.Context, senderDomainID string) ([]domain.DNSRecord, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, sender_domain_id, record_type, host, expected_value, COALESCE(current_value, ''), status, last_checked_at, COALESCE(failure_reason, ''), created_at, updated_at FROM sender_domain_dns_records WHERE sender_domain_id = $1 ORDER BY record_type, host`,
		senderDomainID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []domain.DNSRecord
	for rows.Next() {
		var rec domain.DNSRecord
		if err := rows.Scan(&rec.ID, &rec.SenderDomainID, &rec.RecordType, &rec.Host, &rec.ExpectedValue, &rec.CurrentValue, &rec.Status, &rec.LastCheckedAt, &rec.FailureReason, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	if records == nil {
		records = []domain.DNSRecord{}
	}
	return records, rows.Err()
}

func (w *WriteRepository) Create(ctx context.Context, senderDomain domain.SenderDomain, records []domain.DNSRecord) error {
	tx, err := w.beginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`INSERT INTO sender_domains (id, workspace_id, domain, provider, status, verified_at, disabled_at, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		senderDomain.ID, senderDomain.WorkspaceID, senderDomain.Domain, senderDomain.Provider, senderDomain.Status, senderDomain.VerifiedAt, senderDomain.DisabledAt, senderDomain.CreatedAt, senderDomain.UpdatedAt,
	); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDomainConflict
		}
		return err
	}

	for _, rec := range records {
		if _, err := tx.Exec(ctx,
			`INSERT INTO sender_domain_dns_records (id, sender_domain_id, record_type, host, expected_value, current_value, status, last_checked_at, failure_reason, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			rec.ID, rec.SenderDomainID, rec.RecordType, rec.Host, rec.ExpectedValue, nullable(rec.CurrentValue), rec.Status, rec.LastCheckedAt, nullable(rec.FailureReason), rec.CreatedAt, rec.UpdatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (w *WriteRepository) UpdateDomain(ctx context.Context, senderDomain domain.SenderDomain) error {
	tag, err := w.db.Exec(ctx,
		`UPDATE sender_domains SET domain = $1, provider = $2, status = $3, verified_at = $4, disabled_at = $5, updated_at = $6 WHERE id = $7 AND workspace_id = $8`,
		senderDomain.Domain, senderDomain.Provider, senderDomain.Status, senderDomain.VerifiedAt, senderDomain.DisabledAt, senderDomain.UpdatedAt, senderDomain.ID, senderDomain.WorkspaceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDomainNotFound
	}
	return nil
}

func (w *WriteRepository) ReplaceDNSRecordStatuses(ctx context.Context, senderDomainID string, records []domain.DNSRecord) error {
	tx, err := w.beginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM sender_domain_dns_records WHERE sender_domain_id = $1`, senderDomainID); err != nil {
		return err
	}

	for _, rec := range records {
		if _, err := tx.Exec(ctx,
			`INSERT INTO sender_domain_dns_records (id, sender_domain_id, record_type, host, expected_value, current_value, status, last_checked_at, failure_reason, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			rec.ID, rec.SenderDomainID, rec.RecordType, rec.Host, rec.ExpectedValue, nullable(rec.CurrentValue), rec.Status, rec.LastCheckedAt, nullable(rec.FailureReason), rec.CreatedAt, rec.UpdatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (w *WriteRepository) beginTx(ctx context.Context) (pgx.Tx, error) {
	if conn, ok := w.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	}); ok {
		return conn.Begin(ctx)
	}
	return nil, errors.New("write repository requires a pool or conn that supports Begin")
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
