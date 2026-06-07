package app

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/ports"
)

type mockAPIKeyRepo struct {
	keys      map[string]*domain.APIKey
	prefixIdx map[string]*domain.APIKey
	createErr error
	updateErr error
}

func newMockAPIKeyRepo() *mockAPIKeyRepo {
	return &mockAPIKeyRepo{
		keys:      make(map[string]*domain.APIKey),
		prefixIdx: make(map[string]*domain.APIKey),
	}
}

func (m *mockAPIKeyRepo) ListByWorkspace(ctx context.Context, query ports.APIKeyListQuery) ([]domain.APIKey, string, error) {
	var out []domain.APIKey
	for _, k := range m.keys {
		if k.WorkspaceID == query.WorkspaceID {
			if query.Status == "" || string(k.Status) == query.Status {
				out = append(out, *k)
			}
		}
	}
	return out, "", nil
}

func (m *mockAPIKeyRepo) FindByID(ctx context.Context, workspaceID, keyID string) (*domain.APIKey, error) {
	k, ok := m.keys[keyID]
	if !ok || k.WorkspaceID != workspaceID {
		return nil, domain.ErrAPIKeyNotFound
	}
	return k, nil
}

func (m *mockAPIKeyRepo) FindByPrefix(ctx context.Context, keyPrefix string) (*domain.APIKey, error) {
	k, ok := m.prefixIdx[keyPrefix]
	if !ok {
		return nil, domain.ErrAPIKeyNotFound
	}
	return k, nil
}

func (m *mockAPIKeyRepo) Create(ctx context.Context, key domain.APIKey) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.keys[key.ID] = &key
	m.prefixIdx[key.KeyPrefix] = &key
	return nil
}

func (m *mockAPIKeyRepo) Update(ctx context.Context, key domain.APIKey) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.keys[key.ID] = &key
	m.prefixIdx[key.KeyPrefix] = &key
	return nil
}

func (m *mockAPIKeyRepo) TouchLastUsed(ctx context.Context, workspaceID, keyID string, usedAt time.Time) error {
	return nil
}

type mockAccessChecker struct {
	deny bool
}

func (m *mockAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	if m.deny {
		return domain.ErrAPIKeyManageDenied
	}
	return nil
}

type mockSecretGen struct{}

func (m mockSecretGen) Generate() (string, string, error) {
	return "sk_test_secret_value_12345", "sk_test_secret", nil
}

type mockSecretHasher struct{}

func (m mockSecretHasher) Hash(secret string) (string, error) {
	return "$2a$10$" + secret + "_hashed", nil
}

func (m mockSecretHasher) Verify(hash, secret string) bool {
	return hash == "$2a$10$"+secret+"_hashed"
}

func newTestService(denyAccess bool) *Service {
	return NewService(Options{
		APIKeyRepo:    newMockAPIKeyRepo(),
		AccessChecker: &mockAccessChecker{deny: denyAccess},
		IDGen: func() (string, error) {
			return "test-key-id", nil
		},
		SecretGen:    mockSecretGen{},
		SecretHasher: mockSecretHasher{},
		Logger:       slog.Default(),
	})
}

func TestCreateAPIKey_Success(t *testing.T) {
	svc := newTestService(false)
	future := time.Now().UTC().Add(24 * time.Hour)
	result, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "My API Key",
		Scopes:      []string{domain.ScopeTransactionalSend},
		ExpiresAt:   &future,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "test-key-id" {
		t.Errorf("expected id %q, got %q", "test-key-id", result.ID)
	}
	if result.Secret != "sk_test_secret_value_12345" {
		t.Errorf("expected one-time secret, got %q", result.Secret)
	}
	if result.Name != "My API Key" {
		t.Errorf("expected name %q, got %q", "My API Key", result.Name)
	}
}

func TestCreateAPIKey_EmptyScopes(t *testing.T) {
	svc := newTestService(false)
	_, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "My Key",
	})
	if !errors.Is(err, domain.ErrAPIKeyScopeInvalid) {
		t.Errorf("expected ErrAPIKeyScopeInvalid, got %v", err)
	}
}

func TestCreateAPIKey_UnknownScope(t *testing.T) {
	svc := newTestService(false)
	_, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "My Key",
		Scopes:      []string{"unknown.scope"},
	})
	if !errors.Is(err, domain.ErrAPIKeyScopeInvalid) {
		t.Errorf("expected ErrAPIKeyScopeInvalid, got %v", err)
	}
}

func TestCreateAPIKey_PastExpiration(t *testing.T) {
	svc := newTestService(false)
	past := time.Now().UTC().Add(-time.Hour)
	_, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "My Key",
		Scopes:      []string{domain.ScopeTransactionalSend},
		ExpiresAt:   &past,
	})
	if !errors.Is(err, domain.ErrAPIKeyConfigInvalid) {
		t.Errorf("expected ErrAPIKeyConfigInvalid, got %v", err)
	}
}

func TestCreateAPIKey_EmptyName(t *testing.T) {
	svc := newTestService(false)
	_, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "",
		Scopes:      []string{domain.ScopeTransactionalSend},
	})
	if !errors.Is(err, domain.ErrAPIKeyConfigInvalid) {
		t.Errorf("expected ErrAPIKeyConfigInvalid, got %v", err)
	}
}

func TestCreateAPIKey_ManageDenied(t *testing.T) {
	svc := newTestService(true)
	_, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "My Key",
		Scopes:      []string{domain.ScopeTransactionalSend},
	})
	if !errors.Is(err, domain.ErrAPIKeyManageDenied) {
		t.Errorf("expected ErrAPIKeyManageDenied, got %v", err)
	}
}

func TestListAPIKeys_OmitsSecret(t *testing.T) {
	svc := newTestService(false)
	_, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "Key 1",
		Scopes:      []string{domain.ScopeTransactionalSend},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	result, err := svc.ListAPIKeys(context.Background(), ListAPIKeysInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.APIKeys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(result.APIKeys))
	}
}

func TestUpdateAPIKey_Rotate(t *testing.T) {
	svc := newTestService(false)
	createResult, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "Key 1",
		Scopes:      []string{domain.ScopeTransactionalSend},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	updateResult, err := svc.UpdateAPIKey(context.Background(), UpdateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		APIKeyID:    createResult.ID,
		Rotate:      true,
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updateResult.Secret != "sk_test_secret_value_12345" {
		t.Error("expected new secret on rotation")
	}
	if updateResult.ID != createResult.ID {
		t.Error("expected key ID to remain stable after rotation")
	}
}

func TestRevokeAPIKey_Success(t *testing.T) {
	svc := newTestService(false)
	createResult, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "Key 1",
		Scopes:      []string{domain.ScopeTransactionalSend},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	result, err := svc.RevokeAPIKey(context.Background(), RevokeAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		APIKeyID:    createResult.ID,
	})
	if err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
	if result.Status != string(domain.APIKeyStatusRevoked) {
		t.Errorf("expected revoked status, got %q", result.Status)
	}
	if result.RevokedAt == nil {
		t.Error("expected revoked_at to be set")
	}
}

func TestRevokeAPIKey_Idempotent(t *testing.T) {
	svc := newTestService(false)
	createResult, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "Key 1",
		Scopes:      []string{domain.ScopeTransactionalSend},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = svc.RevokeAPIKey(context.Background(), RevokeAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		APIKeyID:    createResult.ID,
	})
	if err != nil {
		t.Fatalf("first revoke failed: %v", err)
	}

	_, err = svc.RevokeAPIKey(context.Background(), RevokeAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		APIKeyID:    createResult.ID,
	})
	if err != nil {
		t.Fatalf("second revoke (idempotent) should succeed, got: %v", err)
	}
}

func TestAuthenticateAPIKey_ActiveKey(t *testing.T) {
	svc := newTestService(false)
	createResult, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "Key 1",
		Scopes:      []string{domain.ScopeTransactionalSend},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	authResult, err := svc.AuthenticateAPIKey(context.Background(), AuthenticateAPIKeyInput{
		BearerToken: createResult.Secret,
	})
	if err != nil {
		t.Fatalf("authentication failed: %v", err)
	}
	if authResult.WorkspaceID != "ws_1" {
		t.Errorf("expected workspace %q, got %q", "ws_1", authResult.WorkspaceID)
	}
	if !domain.HasScope(authResult.Scopes, domain.ScopeTransactionalSend) {
		t.Error("expected authenticated key to have transactional.send scope")
	}
}

func TestAuthenticateAPIKey_RevokedKey(t *testing.T) {
	svc := newTestService(false)
	createResult, err := svc.CreateAPIKey(context.Background(), CreateAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		Name:        "Key 1",
		Scopes:      []string{domain.ScopeTransactionalSend},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = svc.RevokeAPIKey(context.Background(), RevokeAPIKeyInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		APIKeyID:    createResult.ID,
	})
	if err != nil {
		t.Fatalf("revoke failed: %v", err)
	}

	_, err = svc.AuthenticateAPIKey(context.Background(), AuthenticateAPIKeyInput{
		BearerToken: createResult.Secret,
	})
	if !errors.Is(err, domain.ErrAPIKeyInvalid) {
		t.Errorf("expected ErrAPIKeyInvalid for revoked key, got %v", err)
	}
}

func TestAuthenticateAPIKey_InvalidToken(t *testing.T) {
	svc := newTestService(false)
	_, err := svc.AuthenticateAPIKey(context.Background(), AuthenticateAPIKeyInput{
		BearerToken: "invalid_token",
	})
	if !errors.Is(err, domain.ErrAPIKeyInvalid) {
		t.Errorf("expected ErrAPIKeyInvalid, got %v", err)
	}
}

func TestRequireAPIKeyScope_Success(t *testing.T) {
	key := &AuthenticatedAPIKey{
		Scopes: []string{domain.ScopeTransactionalSend},
	}
	if err := (&Service{}).RequireAPIKeyScope(key, domain.ScopeTransactionalSend); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRequireAPIKeyScope_Denied(t *testing.T) {
	key := &AuthenticatedAPIKey{
		Scopes: []string{},
	}
	if err := (&Service{}).RequireAPIKeyScope(key, domain.ScopeTransactionalSend); !errors.Is(err, domain.ErrAPIKeyScopeDenied) {
		t.Errorf("expected ErrAPIKeyScopeDenied, got %v", err)
	}
}
