package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type memoryWorkspaceRepo struct {
	mu    sync.Mutex
	items map[string]domain.Workspace
}

func newMemoryWorkspaceRepo() *memoryWorkspaceRepo {
	return &memoryWorkspaceRepo{items: map[string]domain.Workspace{}}
}

func (r *memoryWorkspaceRepo) Create(_ context.Context, workspace domain.Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[workspace.ID] = workspace
	return nil
}

func (r *memoryWorkspaceRepo) FindByID(_ context.Context, workspaceID string) (*domain.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	workspace, ok := r.items[workspaceID]
	if !ok {
		return nil, domain.ErrWorkspaceNotFound
	}
	return &workspace, nil
}

func (r *memoryWorkspaceRepo) ListByUser(_ context.Context, userID string) ([]domain.Workspace, error) {
	return nil, nil
}

type memoryMembershipRepo struct {
	mu    sync.Mutex
	items map[string]domain.Membership
}

func newMemoryMembershipRepo() *memoryMembershipRepo {
	return &memoryMembershipRepo{items: map[string]domain.Membership{}}
}

func (r *memoryMembershipRepo) Create(_ context.Context, membership domain.Membership) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[membership.ID] = membership
	return nil
}

func (r *memoryMembershipRepo) DeleteByID(_ context.Context, membershipID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, membershipID)
	return nil
}

func (r *memoryMembershipRepo) UpdateRole(_ context.Context, membershipID string, role domain.MembershipRole, updatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.items[membershipID]
	item.Role = role
	item.UpdatedAt = updatedAt
	r.items[membershipID] = item
	return nil
}

func (r *memoryMembershipRepo) UpdateStatus(_ context.Context, membershipID string, status domain.MembershipStatus, updatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.items[membershipID]
	item.Status = status
	item.UpdatedAt = updatedAt
	r.items[membershipID] = item
	return nil
}

func (r *memoryMembershipRepo) FindByID(_ context.Context, membershipID string) (*domain.Membership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[membershipID]
	if !ok {
		return nil, domain.ErrMembershipNotFound
	}
	return &item, nil
}

func (r *memoryMembershipRepo) FindByWorkspaceAndUser(_ context.Context, workspaceID, userID string) (*domain.Membership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.items {
		if item.WorkspaceID == workspaceID && item.UserID == userID {
			copy := item
			return &copy, nil
		}
	}
	return nil, domain.ErrMembershipNotFound
}

func (r *memoryMembershipRepo) ListByWorkspace(_ context.Context, workspaceID string) ([]domain.Membership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Membership{}
	for _, item := range r.items {
		if item.WorkspaceID == workspaceID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (r *memoryMembershipRepo) CountByWorkspaceAndRole(_ context.Context, workspaceID string, role domain.MembershipRole) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, item := range r.items {
		if item.WorkspaceID == workspaceID && item.Role == role && item.Status == domain.MembershipStatusActive {
			count++
		}
	}
	return count, nil
}

type memoryInvitationRepo struct {
	mu    sync.Mutex
	items map[string]domain.Invitation
}

func newMemoryInvitationRepo() *memoryInvitationRepo {
	return &memoryInvitationRepo{items: map[string]domain.Invitation{}}
}

func (r *memoryInvitationRepo) Create(_ context.Context, invitation domain.Invitation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[invitation.ID] = invitation
	return nil
}

func (r *memoryInvitationRepo) UpdateStatus(_ context.Context, invitationID string, status domain.InvitationStatus, updatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.items[invitationID]
	item.Status = status
	item.UpdatedAt = updatedAt
	r.items[invitationID] = item
	return nil
}

func (r *memoryInvitationRepo) FindByToken(_ context.Context, token string) (*domain.Invitation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.items {
		if item.Token == token {
			copy := item
			return &copy, nil
		}
	}
	return nil, domain.ErrInvitationNotFound
}

func (r *memoryInvitationRepo) ListByWorkspace(_ context.Context, workspaceID string) ([]domain.Invitation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Invitation{}
	for _, item := range r.items {
		if item.WorkspaceID == workspaceID {
			out = append(out, item)
		}
	}
	return out, nil
}

type memoryRoleRepo struct {
	mu               sync.Mutex
	roles            map[string]domain.Role
	membershipRoles  map[string][]string
	invitationRoles  map[string][]string
	membershipLookup *memoryMembershipRepo
}

func newMemoryRoleRepo(membershipLookup *memoryMembershipRepo) *memoryRoleRepo {
	return &memoryRoleRepo{
		roles:            map[string]domain.Role{},
		membershipRoles:  map[string][]string{},
		invitationRoles:  map[string][]string{},
		membershipLookup: membershipLookup,
	}
}

func (r *memoryRoleRepo) Create(_ context.Context, role domain.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roles[role.ID] = role
	return nil
}

func (r *memoryRoleRepo) Update(_ context.Context, role domain.Role) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roles[role.ID] = role
	return nil
}

func (r *memoryRoleRepo) ReplaceMembershipRoles(_ context.Context, membershipID string, roleIDs []string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.membershipRoles[membershipID] = append([]string(nil), roleIDs...)
	return nil
}

func (r *memoryRoleRepo) ReplaceInvitationRoles(_ context.Context, invitationID string, roleIDs []string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invitationRoles[invitationID] = append([]string(nil), roleIDs...)
	return nil
}

func (r *memoryRoleRepo) FindByID(_ context.Context, workspaceID, roleID string) (*domain.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	role, ok := r.roles[roleID]
	if !ok || role.WorkspaceID != workspaceID {
		return nil, domain.ErrRoleNotFound
	}
	return &role, nil
}

func (r *memoryRoleRepo) FindByType(_ context.Context, workspaceID string, roleType domain.RoleType) (*domain.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, role := range r.roles {
		if role.WorkspaceID == workspaceID && role.Type == roleType {
			copy := role
			return &copy, nil
		}
	}
	return nil, domain.ErrRoleNotFound
}

func (r *memoryRoleRepo) FindByIDs(_ context.Context, workspaceID string, roleIDs []string) ([]domain.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Role, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		role, ok := r.roles[roleID]
		if ok && role.WorkspaceID == workspaceID {
			out = append(out, role)
		}
	}
	return out, nil
}

func (r *memoryRoleRepo) ListByWorkspace(_ context.Context, workspaceID string) ([]domain.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []domain.Role{}
	for _, role := range r.roles {
		if role.WorkspaceID == workspaceID {
			out = append(out, role)
		}
	}
	return out, nil
}

func (r *memoryRoleRepo) ListByMembership(_ context.Context, membershipID string) ([]domain.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rolesByIDs(r.membershipRoles[membershipID]), nil
}

func (r *memoryRoleRepo) ListByInvitation(_ context.Context, invitationID string) ([]domain.Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.rolesByIDs(r.invitationRoles[invitationID]), nil
}

func (r *memoryRoleRepo) CountMembershipsByRole(_ context.Context, workspaceID, roleID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for membershipID, roleIDs := range r.membershipRoles {
		item, ok := r.membershipLookup.items[membershipID]
		if !ok || item.WorkspaceID != workspaceID || item.Status != domain.MembershipStatusActive {
			continue
		}
		for _, assignedRoleID := range roleIDs {
			if assignedRoleID == roleID {
				count++
				break
			}
		}
	}
	return count, nil
}

func (r *memoryRoleRepo) rolesByIDs(roleIDs []string) []domain.Role {
	out := make([]domain.Role, 0, len(roleIDs))
	for _, roleID := range roleIDs {
		if role, ok := r.roles[roleID]; ok {
			out = append(out, role)
		}
	}
	return out
}

func TestWorkspaceCreateAndAccessRBAC(t *testing.T) {
	router, token := signupWorkspaceUser(t)

	createRec := httptest.NewRecorder()
	req := jsonRequest(t, http.MethodPost, "/api/v1/workspaces", map[string]string{"name": "Acme"})
	authHeader(req, token)
	router.ServeHTTP(createRec, req)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createRec.Code, createRec.Body.String())
	}

	createData := decodeData(t, createRec)
	workspaceID := createData["id"].(string)
	if createData["membership_id"] == "" {
		t.Fatal("expected membership_id in create response")
	}

	accessRec := httptest.NewRecorder()
	accessReq := jsonRequest(t, http.MethodGet, "/api/v1/workspaces/"+workspaceID+"/access", nil)
	authHeader(accessReq, token)
	router.ServeHTTP(accessRec, accessReq)
	if accessRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", accessRec.Code, accessRec.Body.String())
	}
	accessData := decodeData(t, accessRec)
	roleNames := accessData["role_names"].([]any)
	if len(roleNames) != 1 || roleNames[0] != "Owner" {
		t.Fatalf("expected owner role_names, got %#v", roleNames)
	}
	permissions := accessData["effective_permissions"].([]any)
	if len(permissions) == 0 {
		t.Fatal("expected effective_permissions")
	}
}

func TestWorkspaceRoleLifecycleAndAssignment(t *testing.T) {
	router, token := signupWorkspaceUser(t)

	createRec := httptest.NewRecorder()
	createReq := jsonRequest(t, http.MethodPost, "/api/v1/workspaces", map[string]string{"name": "Acme"})
	authHeader(createReq, token)
	router.ServeHTTP(createRec, createReq)
	workspaceID := decodeData(t, createRec)["id"].(string)

	rolesRec := httptest.NewRecorder()
	rolesReq := jsonRequest(t, http.MethodPost, "/api/v1/workspaces/"+workspaceID+"/roles", map[string]any{
		"name":             "operator",
		"permission_names": []string{"workspace.read", "workspace.manage_members"},
	})
	authHeader(rolesReq, token)
	router.ServeHTTP(rolesRec, rolesReq)
	if rolesRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rolesRec.Code, rolesRec.Body.String())
	}
	roleID := decodeData(t, rolesRec)["id"].(string)

	listRolesRec := httptest.NewRecorder()
	listRolesReq := jsonRequest(t, http.MethodGet, "/api/v1/workspaces/"+workspaceID+"/roles", nil)
	authHeader(listRolesReq, token)
	router.ServeHTTP(listRolesRec, listRolesReq)
	if listRolesRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", listRolesRec.Code, listRolesRec.Body.String())
	}
	var listRolesBody map[string]any
	if err := json.Unmarshal(listRolesRec.Body.Bytes(), &listRolesBody); err != nil {
		t.Fatalf("decode roles: %v", err)
	}
	ownerRoleID := ""
	for _, raw := range listRolesBody["data"].([]any) {
		role := raw.(map[string]any)
		if role["name"] == "Owner" {
			ownerRoleID = role["id"].(string)
			break
		}
	}
	if ownerRoleID == "" {
		t.Fatal("expected owner role")
	}

	membersRec := httptest.NewRecorder()
	membersReq := jsonRequest(t, http.MethodGet, "/api/v1/workspaces/"+workspaceID+"/members", nil)
	authHeader(membersReq, token)
	router.ServeHTTP(membersRec, membersReq)
	if membersRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", membersRec.Code, membersRec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(membersRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode members: %v", err)
	}
	memberID := body["data"].([]any)[0].(map[string]any)["membership_id"].(string)

	assignRec := httptest.NewRecorder()
	assignReq := jsonRequest(t, http.MethodPut, "/api/v1/workspaces/"+workspaceID+"/members/"+memberID+"/roles", map[string]any{
		"role_ids": []string{ownerRoleID, roleID},
	})
	authHeader(assignReq, token)
	router.ServeHTTP(assignRec, assignReq)
	if assignRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", assignRec.Code, assignRec.Body.String())
	}
	assignData := decodeData(t, assignRec)
	roleIDs := assignData["role_ids"].([]any)
	if len(roleIDs) != 2 {
		t.Fatalf("expected 2 role_ids, got %#v", roleIDs)
	}
}

func signupWorkspaceUser(t *testing.T) (http.Handler, string) {
	t.Helper()
	router, _ := setupAuthRouter(t)

	signupRec := httptest.NewRecorder()
	router.ServeHTTP(signupRec, jsonRequest(t, http.MethodPost, "/api/v1/auth/signup", map[string]string{
		"email":    "owner@example.com",
		"password": "StrongPassword123!",
	}))
	token := decodeData(t, signupRec)["access_token"].(string)
	return router, token
}
