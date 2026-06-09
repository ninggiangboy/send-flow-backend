package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type UserWriteRepository struct{ db DBTX }
type UserReadRepository struct{ db DBTX }

func NewUserWriteRepository(db DBTX) *UserWriteRepository {
	return &UserWriteRepository{db: db}
}
func NewUserReadRepository(db DBTX) *UserReadRepository { return &UserReadRepository{db: db} }

func (r *UserWriteRepository) Create(ctx context.Context, user domain.User) error {
	_, err := r.getDB(ctx).Exec(ctx, `INSERT INTO users (id,email,hashed_password,status,primary_auth_method,email_verified_at,mfa_enabled_at,created_at,updated_at) VALUES ($1,$2,$3,'active',$4,$5,$6,$7,$8)`, user.ID, user.Email, nullable(user.HashedPassword), user.PrimaryAuthMethod, user.EmailVerifiedAt, user.MFAEnabledAt, user.CreatedAt, user.UpdatedAt)
	return err
}

func (r *UserWriteRepository) UpdatePassword(ctx context.Context, userID, hashedPassword string, at time.Time) error {
	_, err := r.getDB(ctx).Exec(ctx, `UPDATE users SET hashed_password=$2, updated_at=$3 WHERE id=$1`, userID, hashedPassword, at)
	return err
}

func (r *UserWriteRepository) MarkEmailVerified(ctx context.Context, userID string, at time.Time) error {
	_, err := r.getDB(ctx).Exec(ctx, `UPDATE users SET email_verified_at=COALESCE(email_verified_at,$2), updated_at=$2 WHERE id=$1`, userID, at)
	return err
}

func (r *UserWriteRepository) SetMFAEnabledAt(ctx context.Context, userID string, enabledAt *time.Time, at time.Time) error {
	_, err := r.getDB(ctx).Exec(ctx, `UPDATE users SET mfa_enabled_at=$2, updated_at=$3 WHERE id=$1`, userID, enabledAt, at)
	return err
}

func (r *UserReadRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.getDB(ctx).QueryRow(ctx, `SELECT id,email,COALESCE(hashed_password,''),primary_auth_method,email_verified_at,mfa_enabled_at,created_at,updated_at FROM users WHERE email=$1`, email)
	return scanUser(row)
}

func (r *UserReadRepository) FindByID(ctx context.Context, userID string) (*domain.User, error) {
	row := r.getDB(ctx).QueryRow(ctx, `SELECT id,email,COALESCE(hashed_password,''),primary_auth_method,email_verified_at,mfa_enabled_at,created_at,updated_at FROM users WHERE id=$1`, userID)
	return scanUser(row)
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	if err := row.Scan(&u.ID, &u.Email, &u.HashedPassword, &u.PrimaryAuthMethod, &u.EmailVerifiedAt, &u.MFAEnabledAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

type ExternalAccountWriteRepository struct{ db DBTX }
type ExternalAccountReadRepository struct{ db DBTX }

func NewExternalAccountWriteRepository(db DBTX) *ExternalAccountWriteRepository {
	return &ExternalAccountWriteRepository{db: db}
}
func NewExternalAccountReadRepository(db DBTX) *ExternalAccountReadRepository {
	return &ExternalAccountReadRepository{db: db}
}

func (r *ExternalAccountReadRepository) FindByProviderIdentity(ctx context.Context, provider, providerUserID string) (*domain.ExternalAuthAccount, error) {
	var a domain.ExternalAuthAccount
	var lastLoginAt *time.Time
	err := r.getDB(ctx).QueryRow(ctx, `SELECT id,user_id,provider,provider_user_id,COALESCE(provider_email,''),provider_email_verified,linked_at,last_login_at FROM external_auth_accounts WHERE provider=$1 AND provider_user_id=$2`, provider, providerUserID).Scan(&a.ID, &a.UserID, &a.Provider, &a.ProviderUserID, &a.ProviderEmail, &a.ProviderEmailVerified, &a.LinkedAt, &lastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.LastLoginAt = lastLoginAt
	return &a, nil
}

func (r *ExternalAccountWriteRepository) Create(ctx context.Context, a domain.ExternalAuthAccount) error {
	_, err := r.getDB(ctx).Exec(ctx, `INSERT INTO external_auth_accounts (id,user_id,provider,provider_user_id,provider_email,provider_email_verified,linked_at,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$7,$7)`, a.ID, a.UserID, a.Provider, a.ProviderUserID, a.ProviderEmail, a.ProviderEmailVerified, a.LinkedAt)
	return err
}

func (r *ExternalAccountWriteRepository) TouchLogin(ctx context.Context, accountID string, at time.Time) error {
	_, err := r.getDB(ctx).Exec(ctx, `UPDATE external_auth_accounts SET last_login_at=$2, updated_at=$2 WHERE id=$1`, accountID, at)
	return err
}

type SessionWriteRepository struct{ db DBTX }
type SessionReadRepository struct{ db DBTX }

func NewSessionWriteRepository(db DBTX) *SessionWriteRepository {
	return &SessionWriteRepository{db: db}
}
func NewSessionReadRepository(db DBTX) *SessionReadRepository {
	return &SessionReadRepository{db: db}
}

func (r *SessionWriteRepository) Create(ctx context.Context, s domain.Session) error {
	_, err := r.getDB(ctx).Exec(ctx, `INSERT INTO sessions (id,user_id,auth_method,access_jti,refresh_jti,expires_at,ip_address,user_agent,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, s.ID, s.UserID, s.AuthMethod, s.AccessJTI, s.RefreshJTI, s.ExpiresAt, nullable(s.IPAddress), nullable(s.UserAgent), s.CreatedAt)
	return err
}

func (r *SessionReadRepository) FindByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	return r.findOne(ctx, `SELECT id,user_id,auth_method,access_jti,refresh_jti,expires_at,revoked_at,COALESCE(ip_address,''),COALESCE(user_agent,''),created_at FROM sessions WHERE id=$1`, sessionID)
}

func (r *SessionReadRepository) FindByAccessJTI(ctx context.Context, jti string) (*domain.Session, error) {
	return r.findOne(ctx, `SELECT id,user_id,auth_method,access_jti,refresh_jti,expires_at,revoked_at,COALESCE(ip_address,''),COALESCE(user_agent,''),created_at FROM sessions WHERE access_jti=$1`, jti)
}

func (r *SessionReadRepository) ListByUser(ctx context.Context, userID string, now time.Time) ([]domain.Session, error) {
	rows, err := r.getDB(ctx).Query(ctx, `SELECT id,user_id,auth_method,access_jti,refresh_jti,expires_at,revoked_at,COALESCE(ip_address,''),COALESCE(user_agent,''),created_at FROM sessions WHERE user_id=$1 AND (revoked_at IS NULL) AND expires_at > $2 ORDER BY created_at DESC`, userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Session
	for rows.Next() {
		var s domain.Session
		if err := rows.Scan(&s.ID, &s.UserID, &s.AuthMethod, &s.AccessJTI, &s.RefreshJTI, &s.ExpiresAt, &s.RevokedAt, &s.IPAddress, &s.UserAgent, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *SessionWriteRepository) RevokeByID(ctx context.Context, sessionID string, now time.Time) error {
	_, err := r.getDB(ctx).Exec(ctx, `UPDATE sessions SET revoked_at=$2 WHERE id=$1 AND revoked_at IS NULL`, sessionID, now)
	return err
}

func (r *SessionWriteRepository) RevokeByUser(ctx context.Context, userID string, now time.Time) error {
	_, err := r.getDB(ctx).Exec(ctx, `UPDATE sessions SET revoked_at=$2 WHERE user_id=$1 AND revoked_at IS NULL`, userID, now)
	return err
}

func (r *SessionWriteRepository) RotateTokens(ctx context.Context, sessionID, accessJTI, refreshJTI string, expiresAt, now time.Time) error {
	_, err := r.getDB(ctx).Exec(ctx, `UPDATE sessions SET access_jti=$2, refresh_jti=$3, expires_at=$4 WHERE id=$1`, sessionID, accessJTI, refreshJTI, expiresAt)
	return err
}

func (r *SessionReadRepository) findOne(ctx context.Context, sql string, arg string) (*domain.Session, error) {
	var s domain.Session
	err := r.getDB(ctx).QueryRow(ctx, sql, arg).Scan(&s.ID, &s.UserID, &s.AuthMethod, &s.AccessJTI, &s.RefreshJTI, &s.ExpiresAt, &s.RevokedAt, &s.IPAddress, &s.UserAgent, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

type AuthTokenRepository struct{ db DBTX }

func NewAuthTokenRepository(db DBTX) *AuthTokenRepository {
	return &AuthTokenRepository{db: db}
}

func (r *AuthTokenRepository) Create(ctx context.Context, token domain.AuthToken) error {
	_, err := r.getDB(ctx).Exec(ctx, `INSERT INTO auth_tokens (id,user_id,purpose,token_hash,expires_at,consumed_at,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`, token.ID, token.UserID, token.Purpose, token.TokenHash, token.ExpiresAt, token.ConsumedAt, token.CreatedAt)
	return err
}

func (r *AuthTokenRepository) FindByHash(ctx context.Context, purpose, tokenHash string) (*domain.AuthToken, error) {
	var token domain.AuthToken
	err := r.getDB(ctx).QueryRow(ctx, `SELECT id,user_id,purpose,token_hash,expires_at,consumed_at,created_at FROM auth_tokens WHERE purpose=$1 AND token_hash=$2`, purpose, tokenHash).Scan(&token.ID, &token.UserID, &token.Purpose, &token.TokenHash, &token.ExpiresAt, &token.ConsumedAt, &token.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *AuthTokenRepository) Consume(ctx context.Context, tokenID string, at time.Time) error {
	_, err := r.getDB(ctx).Exec(ctx, `UPDATE auth_tokens SET consumed_at=$2 WHERE id=$1 AND consumed_at IS NULL`, tokenID, at)
	return err
}

func (r *AuthTokenRepository) DeleteByUserAndPurpose(ctx context.Context, userID, purpose string) error {
	_, err := r.getDB(ctx).Exec(ctx, `DELETE FROM auth_tokens WHERE user_id=$1 AND purpose=$2`, userID, purpose)
	return err
}

type TOTPRepository struct {
	db   DBTX
	pool *pgxpool.Pool
}

func NewTOTPRepository(db DBTX, pool *pgxpool.Pool) *TOTPRepository {
	return &TOTPRepository{db: db, pool: pool}
}

func (r *TOTPRepository) UpsertSecret(ctx context.Context, secret domain.TOTPSecret) error {
	_, err := r.getDB(ctx).Exec(ctx, `
		INSERT INTO user_mfa_totp (user_id,secret,created_at,updated_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (user_id) DO UPDATE SET secret=EXCLUDED.secret, updated_at=EXCLUDED.updated_at
	`, secret.UserID, secret.Secret, secret.CreatedAt, secret.UpdatedAt)
	return err
}

func (r *TOTPRepository) FindSecretByUser(ctx context.Context, userID string) (*domain.TOTPSecret, error) {
	var secret domain.TOTPSecret
	err := r.getDB(ctx).QueryRow(ctx, `SELECT user_id,secret,created_at,updated_at FROM user_mfa_totp WHERE user_id=$1`, userID).Scan(&secret.UserID, &secret.Secret, &secret.CreatedAt, &secret.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &secret, nil
}

func (r *TOTPRepository) DeleteSecret(ctx context.Context, userID string) error {
	_, err := r.getDB(ctx).Exec(ctx, `DELETE FROM user_mfa_totp WHERE user_id=$1`, userID)
	return err
}

func (r *TOTPRepository) ReplaceRecoveryCodes(ctx context.Context, userID string, codes []domain.RecoveryCode) error {
	if r.pool != nil {
		return r.replaceRecoveryCodesInTx(ctx, userID, codes)
	}
	if _, err := r.getDB(ctx).Exec(ctx, `DELETE FROM user_mfa_recovery_codes WHERE user_id=$1`, userID); err != nil {
		return err
	}
	for _, code := range codes {
		if _, err := r.getDB(ctx).Exec(ctx, `INSERT INTO user_mfa_recovery_codes (id,user_id,code_hash,consumed_at,created_at) VALUES ($1,$2,$3,$4,$5)`, code.ID, code.UserID, code.CodeHash, code.ConsumedAt, code.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *TOTPRepository) replaceRecoveryCodesInTx(ctx context.Context, userID string, codes []domain.RecoveryCode) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM user_mfa_recovery_codes WHERE user_id=$1`, userID); err != nil {
		return err
	}
	for _, code := range codes {
		if _, err := tx.Exec(ctx, `INSERT INTO user_mfa_recovery_codes (id,user_id,code_hash,consumed_at,created_at) VALUES ($1,$2,$3,$4,$5)`, code.ID, code.UserID, code.CodeHash, code.ConsumedAt, code.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *TOTPRepository) ListRecoveryCodes(ctx context.Context, userID string) ([]domain.RecoveryCode, error) {
	rows, err := r.getDB(ctx).Query(ctx, `SELECT id,user_id,code_hash,consumed_at,created_at FROM user_mfa_recovery_codes WHERE user_id=$1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.RecoveryCode
	for rows.Next() {
		var code domain.RecoveryCode
		if err := rows.Scan(&code.ID, &code.UserID, &code.CodeHash, &code.ConsumedAt, &code.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

func (r *TOTPRepository) ConsumeRecoveryCode(ctx context.Context, codeID string, at time.Time) error {
	_, err := r.getDB(ctx).Exec(ctx, `UPDATE user_mfa_recovery_codes SET consumed_at=$2 WHERE id=$1 AND consumed_at IS NULL`, codeID, at)
	return err
}

// ---- Workspace Repositories ----

type WorkspaceWriteRepository struct{ db DBTX }
type WorkspaceReadRepository struct{ db DBTX }
type RoleWriteRepository struct{ db DBTX }
type RoleReadRepository struct{ db DBTX }

func NewWorkspaceWriteRepository(db DBTX) *WorkspaceWriteRepository {
	return &WorkspaceWriteRepository{db: db}
}
func NewWorkspaceReadRepository(db DBTX) *WorkspaceReadRepository {
	return &WorkspaceReadRepository{db: db}
}
func NewRoleWriteRepository(db DBTX) *RoleWriteRepository {
	return &RoleWriteRepository{db: db}
}
func NewRoleReadRepository(db DBTX) *RoleReadRepository {
	return &RoleReadRepository{db: db}
}

func (r *WorkspaceWriteRepository) Create(ctx context.Context, w domain.Workspace) error {
	_, err := r.getDB(ctx).Exec(ctx, `INSERT INTO workspaces (id,name,plan,logo_icon,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		w.ID, w.Name, w.Plan, w.LogoIcon, w.CreatedAt, w.UpdatedAt)
	return err
}

func (r *WorkspaceReadRepository) FindByID(ctx context.Context, workspaceID string) (*domain.Workspace, error) {
	row := r.getDB(ctx).QueryRow(ctx, `SELECT id,name,plan,logo_icon,created_at,updated_at FROM workspaces WHERE id=$1`, workspaceID)
	return scanWorkspace(row)
}

func (r *WorkspaceReadRepository) ListByUser(ctx context.Context, userID string) ([]domain.Workspace, error) {
	rows, err := r.getDB(ctx).Query(ctx, `
		SELECT w.id,w.name,w.plan,w.logo_icon,w.created_at,w.updated_at
		FROM workspaces w
		JOIN workspace_memberships m ON m.workspace_id = w.id
		WHERE m.user_id=$1 AND m.status='active'
		ORDER BY w.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Workspace
	for rows.Next() {
		var w domain.Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.Plan, &w.LogoIcon, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func scanWorkspace(row pgx.Row) (*domain.Workspace, error) {
	var w domain.Workspace
	if err := row.Scan(&w.ID, &w.Name, &w.Plan, &w.LogoIcon, &w.CreatedAt, &w.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrWorkspaceNotFound
		}
		return nil, err
	}
	return &w, nil
}

// ---- Role Repositories ----

func (r *RoleWriteRepository) Create(ctx context.Context, role domain.Role) error {
	_, err := r.getDB(ctx).Exec(ctx, `INSERT INTO roles (id,workspace_id,name,permissions_mask,builtin,type,status,version,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		role.ID, role.WorkspaceID, role.Name, role.PermissionsMask, role.Builtin, string(role.Type), string(role.Status), role.Version, role.CreatedAt, role.UpdatedAt)
	return err
}

func (r *RoleWriteRepository) Update(ctx context.Context, role domain.Role) error {
	_, err := r.getDB(ctx).Exec(ctx, `UPDATE roles SET name=$2, permissions_mask=$3, builtin=$4, type=$5, status=$6, version=$7, updated_at=$8 WHERE id=$1 AND workspace_id=$9`,
		role.ID, role.Name, role.PermissionsMask, role.Builtin, string(role.Type), string(role.Status), role.Version, role.UpdatedAt, role.WorkspaceID)
	return err
}

func (r *RoleWriteRepository) ReplaceMembershipRoles(ctx context.Context, membershipID string, roleIDs []string, updatedAt time.Time) error {
	if _, err := r.getDB(ctx).Exec(ctx, `DELETE FROM membership_roles WHERE membership_id=$1`, membershipID); err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if _, err := r.getDB(ctx).Exec(ctx, `INSERT INTO membership_roles (membership_id,role_id,created_at) VALUES ($1,$2,$3)`, membershipID, roleID, updatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *RoleWriteRepository) ReplaceInvitationRoles(ctx context.Context, invitationID string, roleIDs []string, updatedAt time.Time) error {
	if _, err := r.getDB(ctx).Exec(ctx, `DELETE FROM invitation_roles WHERE invitation_id=$1`, invitationID); err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if _, err := r.getDB(ctx).Exec(ctx, `INSERT INTO invitation_roles (invitation_id,role_id,created_at) VALUES ($1,$2,$3)`, invitationID, roleID, updatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *RoleReadRepository) FindByID(ctx context.Context, workspaceID, roleID string) (*domain.Role, error) {
	row := r.getDB(ctx).QueryRow(ctx, `SELECT id,workspace_id,name,permissions_mask,builtin,type,status,version,created_at,updated_at FROM roles WHERE workspace_id=$1 AND id=$2`, workspaceID, roleID)
	return scanRole(row)
}

func (r *RoleReadRepository) FindByType(ctx context.Context, workspaceID string, roleType domain.RoleType) (*domain.Role, error) {
	row := r.getDB(ctx).QueryRow(ctx, `SELECT id,workspace_id,name,permissions_mask,builtin,type,status,version,created_at,updated_at FROM roles WHERE workspace_id=$1 AND type=$2`, workspaceID, string(roleType))
	return scanRole(row)
}

func (r *RoleReadRepository) FindByIDs(ctx context.Context, workspaceID string, roleIDs []string) ([]domain.Role, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	rows, err := r.getDB(ctx).Query(ctx, `SELECT id,workspace_id,name,permissions_mask,builtin,type,status,version,created_at,updated_at FROM roles WHERE workspace_id=$1 AND id = ANY($2) ORDER BY created_at ASC`, workspaceID, roleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoles(rows)
}

func (r *RoleReadRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Role, error) {
	rows, err := r.getDB(ctx).Query(ctx, `SELECT id,workspace_id,name,permissions_mask,builtin,type,status,version,created_at,updated_at FROM roles WHERE workspace_id=$1 ORDER BY created_at ASC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoles(rows)
}

func (r *RoleReadRepository) ListByMembership(ctx context.Context, membershipID string) ([]domain.Role, error) {
	rows, err := r.getDB(ctx).Query(ctx, `
		SELECT r.id,r.workspace_id,r.name,r.permissions_mask,r.builtin,r.type,r.status,r.version,r.created_at,r.updated_at
		FROM roles r
		JOIN membership_roles mr ON mr.role_id = r.id
		WHERE mr.membership_id=$1
		ORDER BY r.created_at ASC`, membershipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoles(rows)
}

func (r *RoleReadRepository) ListByInvitation(ctx context.Context, invitationID string) ([]domain.Role, error) {
	rows, err := r.getDB(ctx).Query(ctx, `
		SELECT r.id,r.workspace_id,r.name,r.permissions_mask,r.builtin,r.type,r.status,r.version,r.created_at,r.updated_at
		FROM roles r
		JOIN invitation_roles ir ON ir.role_id = r.id
		WHERE ir.invitation_id=$1
		ORDER BY r.created_at ASC`, invitationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoles(rows)
}

func (r *RoleReadRepository) CountMembershipsByRole(ctx context.Context, workspaceID, roleID string) (int, error) {
	var count int
	err := r.getDB(ctx).QueryRow(ctx, `
		SELECT COUNT(*)
		FROM membership_roles mr
		JOIN roles r ON r.id = mr.role_id
		JOIN workspace_memberships m ON m.id = mr.membership_id
		WHERE r.workspace_id=$1 AND r.id=$2 AND m.status='active'`, workspaceID, roleID).Scan(&count)
	return count, err
}

func scanRoles(rows pgx.Rows) ([]domain.Role, error) {
	var out []domain.Role
	for rows.Next() {
		role, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *role)
	}
	return out, rows.Err()
}

func scanRole(row pgx.Row) (*domain.Role, error) {
	var role domain.Role
	var status string
	var roleType string
	if err := row.Scan(&role.ID, &role.WorkspaceID, &role.Name, &role.PermissionsMask, &role.Builtin, &roleType, &status, &role.Version, &role.CreatedAt, &role.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrRoleNotFound
		}
		return nil, err
	}
	role.Type = domain.RoleType(roleType)
	role.Status = domain.RoleStatus(status)
	return &role, nil
}

// ---- Membership Repositories ----

type MembershipWriteRepository struct{ db DBTX }
type MembershipReadRepository struct{ db DBTX }

func NewMembershipWriteRepository(db DBTX) *MembershipWriteRepository {
	return &MembershipWriteRepository{db: db}
}
func NewMembershipReadRepository(db DBTX) *MembershipReadRepository {
	return &MembershipReadRepository{db: db}
}

func (r *MembershipWriteRepository) Create(ctx context.Context, m domain.Membership) error {
	_, err := r.getDB(ctx).Exec(ctx, `INSERT INTO workspace_memberships (id,workspace_id,user_id,role,status,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		m.ID, m.WorkspaceID, m.UserID, string(m.Role), string(m.Status), m.CreatedAt, m.UpdatedAt)
	return err
}

func (r *MembershipWriteRepository) DeleteByID(ctx context.Context, membershipID string) error {
	_, err := r.getDB(ctx).Exec(ctx, `DELETE FROM workspace_memberships WHERE id=$1`, membershipID)
	return err
}

func (r *MembershipWriteRepository) UpdateRole(ctx context.Context, membershipID string, role domain.MembershipRole, updatedAt time.Time) error {
	tag, err := r.getDB(ctx).Exec(ctx, `UPDATE workspace_memberships SET role=$2, updated_at=$3 WHERE id=$1`, membershipID, string(role), updatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMembershipNotFound
	}
	return nil
}

func (r *MembershipWriteRepository) UpdateStatus(ctx context.Context, membershipID string, status domain.MembershipStatus, updatedAt time.Time) error {
	tag, err := r.getDB(ctx).Exec(ctx, `UPDATE workspace_memberships SET status=$2, updated_at=$3 WHERE id=$1`, membershipID, string(status), updatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMembershipNotFound
	}
	return nil
}

func (r *MembershipReadRepository) FindByID(ctx context.Context, membershipID string) (*domain.Membership, error) {
	row := r.getDB(ctx).QueryRow(ctx, `SELECT id,workspace_id,user_id,role,status,created_at,updated_at FROM workspace_memberships WHERE id=$1`, membershipID)
	return scanMembership(row)
}

func (r *MembershipReadRepository) FindByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	row := r.getDB(ctx).QueryRow(ctx, `SELECT id,workspace_id,user_id,role,status,created_at,updated_at FROM workspace_memberships WHERE workspace_id=$1 AND user_id=$2`, workspaceID, userID)
	return scanMembership(row)
}

func (r *MembershipReadRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Membership, error) {
	rows, err := r.getDB(ctx).Query(ctx, `
		SELECT m.id,m.workspace_id,m.user_id,COALESCE(u.email, NULL),m.role,m.status,m.created_at,m.updated_at
		FROM workspace_memberships m
		LEFT JOIN users u ON u.id = m.user_id
		WHERE m.workspace_id=$1
		ORDER BY m.created_at ASC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Membership
	for rows.Next() {
		var m domain.Membership
		var roleStr, statusStr string
		if err := rows.Scan(&m.ID, &m.WorkspaceID, &m.UserID, &m.UserEmail, &roleStr, &statusStr, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Role = domain.MembershipRole(roleStr)
		m.Status = domain.MembershipStatus(statusStr)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MembershipReadRepository) CountByWorkspaceAndRole(ctx context.Context, workspaceID string, role domain.MembershipRole) (int, error) {
	var count int
	err := r.getDB(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM workspace_memberships WHERE workspace_id=$1 AND role=$2`, workspaceID, string(role)).Scan(&count)
	return count, err
}

func scanMembership(row pgx.Row) (*domain.Membership, error) {
	var m domain.Membership
	var roleStr, statusStr string
	if err := row.Scan(&m.ID, &m.WorkspaceID, &m.UserID, &roleStr, &statusStr, &m.CreatedAt, &m.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMembershipNotFound
		}
		return nil, err
	}
	m.Role = domain.MembershipRole(roleStr)
	m.Status = domain.MembershipStatus(statusStr)
	return &m, nil
}

// ---- Invitation Repositories ----

type InvitationWriteRepository struct{ db DBTX }
type InvitationReadRepository struct{ db DBTX }

func NewInvitationWriteRepository(db DBTX) *InvitationWriteRepository {
	return &InvitationWriteRepository{db: db}
}
func NewInvitationReadRepository(db DBTX) *InvitationReadRepository {
	return &InvitationReadRepository{db: db}
}

func (r *InvitationWriteRepository) Create(ctx context.Context, inv domain.Invitation) error {
	_, err := r.getDB(ctx).Exec(ctx, `INSERT INTO workspace_invitations (id,workspace_id,email,token,role,status,expires_at,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		inv.ID, inv.WorkspaceID, inv.Email, inv.Token, string(inv.Role), string(inv.Status), inv.ExpiresAt, inv.CreatedAt, inv.UpdatedAt)
	return err
}

func (r *InvitationWriteRepository) UpdateStatus(ctx context.Context, invitationID string, status domain.InvitationStatus, updatedAt time.Time) error {
	tag, err := r.getDB(ctx).Exec(ctx, `UPDATE workspace_invitations SET status=$2, updated_at=$3 WHERE id=$1`, invitationID, string(status), updatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvitationNotFound
	}
	return nil
}

func (r *InvitationReadRepository) FindByToken(ctx context.Context, token string) (*domain.Invitation, error) {
	row := r.getDB(ctx).QueryRow(ctx, `SELECT id,workspace_id,email,token,role,status,expires_at,created_at,updated_at FROM workspace_invitations WHERE token=$1`, token)
	return scanInvitation(row)
}

func (r *InvitationReadRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Invitation, error) {
	rows, err := r.getDB(ctx).Query(ctx, `SELECT id,workspace_id,email,token,role,status,expires_at,created_at,updated_at FROM workspace_invitations WHERE workspace_id=$1 ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Invitation
	for rows.Next() {
		var inv domain.Invitation
		var roleStr, statusStr string
		if err := rows.Scan(&inv.ID, &inv.WorkspaceID, &inv.Email, &inv.Token, &roleStr, &statusStr, &inv.ExpiresAt, &inv.CreatedAt, &inv.UpdatedAt); err != nil {
			return nil, err
		}
		inv.Role = domain.MembershipRole(roleStr)
		inv.Status = domain.InvitationStatus(statusStr)
		out = append(out, inv)
	}
	return out, rows.Err()
}

func scanInvitation(row pgx.Row) (*domain.Invitation, error) {
	var inv domain.Invitation
	var roleStr, statusStr string
	if err := row.Scan(&inv.ID, &inv.WorkspaceID, &inv.Email, &inv.Token, &roleStr, &statusStr, &inv.ExpiresAt, &inv.CreatedAt, &inv.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrInvitationNotFound
		}
		return nil, err
	}
	inv.Role = domain.MembershipRole(roleStr)
	inv.Status = domain.InvitationStatus(statusStr)
	return &inv, nil
}

func placeholders(start, count int) string {
	if count == 0 {
		return ""
	}
	out := ""
	for i := 0; i < count; i++ {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("$%d", start+i)
	}
	return out
}

type OutboxRepository struct {
	db DBTX
}

func NewIdentityOutboxRepository(db DBTX) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Save(ctx context.Context, event ports.OutboxEvent) error {
	db := r.getDB(ctx)
	headersJSON, err := json.Marshal(event.Headers)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx,
		`INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, headers, workspace_id, occurred_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		event.ID, event.AggregateType, event.AggregateID, event.EventType,
		event.Payload, headersJSON, event.WorkspaceID, event.OccurredAt,
	)
	return err
}
