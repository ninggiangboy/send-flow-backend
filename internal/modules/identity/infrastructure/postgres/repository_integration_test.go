//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("sendflow_test"),
		tcpostgres.WithUsername("sendflow"),
		tcpostgres.WithPassword("sendflow"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("build dsn: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("new pg pool: %v", err)
	}
	t.Cleanup(pool.Close)

	schema := []string{
		`CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			hashed_password TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			primary_auth_method TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE external_auth_accounts (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			provider TEXT NOT NULL,
			provider_user_id TEXT NOT NULL,
			provider_email TEXT,
			provider_email_verified BOOLEAN NOT NULL DEFAULT FALSE,
			linked_at TIMESTAMPTZ NOT NULL,
			last_login_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			UNIQUE(provider, provider_user_id)
		)`,
		`CREATE TABLE sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id),
			auth_method TEXT NOT NULL,
			access_jti TEXT UNIQUE NOT NULL,
			refresh_jti TEXT UNIQUE NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			revoked_at TIMESTAMPTZ,
			ip_address TEXT,
			user_agent TEXT,
			created_at TIMESTAMPTZ NOT NULL
		)`,
	}
	for _, stmt := range schema {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return pool
}

func TestUserReadWriteRepositoryIntegration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()
	w := NewUserWriteRepository(pool)
	r := NewUserReadRepository(pool)

	now := time.Now().UTC()
	user := domain.User{
		ID:                "u1",
		Email:             "a@example.com",
		HashedPassword:    "hash",
		PrimaryAuthMethod: "password",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := w.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	got, err := r.FindByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("find user by email: %v", err)
	}
	if got.ID != "u1" {
		t.Fatalf("unexpected user id: %s", got.ID)
	}
}

func TestSessionReadWriteRepositoryIntegration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()
	userW := NewUserWriteRepository(pool)
	sessW := NewSessionWriteRepository(pool)
	sessR := NewSessionReadRepository(pool)

	now := time.Now().UTC()
	if err := userW.Create(ctx, domain.User{
		ID: "u1", Email: "s@example.com", PrimaryAuthMethod: "password", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	sess := domain.Session{
		ID:         "s1",
		UserID:     "u1",
		AuthMethod: "password",
		AccessJTI:  "a1",
		RefreshJTI: "r1",
		ExpiresAt:  now.Add(1 * time.Hour),
		CreatedAt:  now,
	}
	if err := sessW.Create(ctx, sess); err != nil {
		t.Fatalf("create session: %v", err)
	}
	got, err := sessR.FindByAccessJTI(ctx, "a1")
	if err != nil {
		t.Fatalf("find session: %v", err)
	}
	if got.ID != "s1" {
		t.Fatalf("unexpected session id: %s", got.ID)
	}

	if err := sessW.RevokeByID(ctx, "s1", now.Add(5*time.Minute)); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	got2, err := sessR.FindByID(ctx, "s1")
	if err != nil {
		t.Fatalf("find revoked session: %v", err)
	}
	if got2.RevokedAt == nil {
		t.Fatal("expected revoked_at to be set")
	}
}

func TestExternalAccountReadWriteRepositoryIntegration(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()
	userW := NewUserWriteRepository(pool)
	exW := NewExternalAccountWriteRepository(pool)
	exR := NewExternalAccountReadRepository(pool)

	now := time.Now().UTC()
	if err := userW.Create(ctx, domain.User{
		ID: "u1", Email: "oauth@example.com", PrimaryAuthMethod: "oauth_google", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	acc := domain.ExternalAuthAccount{
		ID:                    "acc1",
		UserID:                "u1",
		Provider:              "google",
		ProviderUserID:        "gid-1",
		ProviderEmail:         "oauth@example.com",
		ProviderEmailVerified: true,
		LinkedAt:              now,
	}
	if err := exW.Create(ctx, acc); err != nil {
		t.Fatalf("create account: %v", err)
	}
	got, err := exR.FindByProviderIdentity(ctx, "google", "gid-1")
	if err != nil {
		t.Fatalf("find account: %v", err)
	}
	if got.ID != "acc1" {
		t.Fatalf("unexpected account id: %s", got.ID)
	}

	if err := exW.TouchLogin(ctx, "acc1", now.Add(2*time.Minute)); err != nil {
		t.Fatalf("touch login: %v", err)
	}
	got2, err := exR.FindByProviderIdentity(ctx, "google", "gid-1")
	if err != nil {
		t.Fatalf("find account after touch: %v", err)
	}
	if got2.LastLoginAt == nil {
		t.Fatal("expected last_login_at to be set")
	}
}

func TestSessionListByUserOnlyReturnsActive(t *testing.T) {
	pool := setupIntegrationDB(t)
	ctx := context.Background()
	userW := NewUserWriteRepository(pool)
	sessW := NewSessionWriteRepository(pool)
	sessR := NewSessionReadRepository(pool)

	now := time.Now().UTC()
	if err := userW.Create(ctx, domain.User{
		ID: "u1", Email: "list@example.com", PrimaryAuthMethod: "password", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	sessions := []domain.Session{
		{ID: "s1", UserID: "u1", AuthMethod: "password", AccessJTI: "a1", RefreshJTI: "r1", ExpiresAt: now.Add(1 * time.Hour), CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "s2", UserID: "u1", AuthMethod: "password", AccessJTI: "a2", RefreshJTI: "r2", ExpiresAt: now.Add(-1 * time.Minute), CreatedAt: now.Add(-1 * time.Minute)},
		{ID: "s3", UserID: "u1", AuthMethod: "password", AccessJTI: "a3", RefreshJTI: "r3", ExpiresAt: now.Add(2 * time.Hour), CreatedAt: now},
	}
	for _, s := range sessions {
		if err := sessW.Create(ctx, s); err != nil {
			t.Fatalf("create session %s: %v", s.ID, err)
		}
	}
	if err := sessW.RevokeByID(ctx, "s3", now.Add(1*time.Second)); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	got, err := sessR.ListByUser(ctx, "u1", now)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(got) != 1 || got[0].ID != "s1" {
		t.Fatalf("expected only s1 active, got %v", summarizeSessions(got))
	}
}

func summarizeSessions(s []domain.Session) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ","
		}
		out += v.ID
	}
	return fmt.Sprintf("[%s]", out)
}
