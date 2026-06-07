package createworkspace

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

type Command struct {
	Name   string
	UserID string
	Now    time.Time
}

type Handler struct {
	deps usecase.Deps
	log  *slog.Logger
}

func New(deps usecase.Deps) *Handler {
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "create_workspace")}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.Workspace, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, domain.ErrInvalidWorkspaceName
	}
	ws := domain.NewWorkspace(id.Must(id.NewUUIDGenerator()), name, cmd.Now)
	membership := domain.NewMembership(id.Must(id.NewUUIDGenerator()), ws.ID, cmd.UserID, domain.MembershipRoleOwner, cmd.Now)
	defaultRoles := domain.DefaultRoleDefinitions(cmd.Now)
	var ownerRoleID string
	for i := range defaultRoles {
		defaultRoles[i].ID = id.Must(id.NewUUIDGenerator())
		defaultRoles[i].WorkspaceID = ws.ID
		if defaultRoles[i].Type == domain.RoleTypeOwner {
			ownerRoleID = defaultRoles[i].ID
		}
	}
	if ownerRoleID == "" {
		h.log.Error("owner role not found among default roles", "workspace_id", ws.ID)
		return nil, domain.ErrRoleNotFound
	}
	createAll := func(ctx context.Context) error {
		if err := h.deps.WorkspacesWrite.Create(ctx, ws); err != nil {
			return err
		}
		if err := h.deps.MembershipsWrite.Create(ctx, membership); err != nil {
			return err
		}
		for _, role := range defaultRoles {
			if err := h.deps.RolesWrite.Create(ctx, role); err != nil {
				return err
			}
		}
		return h.deps.RolesWrite.ReplaceMembershipRoles(ctx, membership.ID, []string{ownerRoleID}, cmd.Now)
	}
	if h.deps.UnitOfWork != nil {
		err := h.deps.UnitOfWork.WithinTx(ctx, createAll)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
				h.log.Warn("workspace creation failed: duplicate name", "name", name)
				return nil, domain.ErrWorkspaceNameConflict
			}
			h.log.Error("failed to create workspace", "error", err)
			return nil, err
		}
	} else {
		if err := createAll(ctx); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
				h.log.Warn("workspace creation failed: duplicate name", "name", name)
				return nil, domain.ErrWorkspaceNameConflict
			}
			h.log.Error("failed to create workspace", "error", err)
			return nil, err
		}
	}
	ws.MembershipID = membership.ID
	ws.RoleNames = []string{string(domain.MembershipRoleOwner)}
	h.log.Info("workspace created", "workspace_id", ws.ID, "user_id", cmd.UserID)
	return &ws, nil
}
