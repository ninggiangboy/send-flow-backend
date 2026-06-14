package createworkspace

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type Options struct {
	IdGen            ports.IDGenerator
	WorkspacesWrite  ports.WorkspaceWriteRepository
	MembershipsWrite ports.MembershipWriteRepository
	RolesWrite       ports.RoleWriteRepository
	SettingsWrite    ports.WorkspaceSettingsWriteRepository
	UnitOfWork       ports.UnitOfWork
	Logger           *slog.Logger
}

type Command struct {
	Name   string
	UserID string
	Now    time.Time
}

type Handler struct {
	idGen            ports.IDGenerator
	workspacesWrite  ports.WorkspaceWriteRepository
	membershipsWrite ports.MembershipWriteRepository
	rolesWrite       ports.RoleWriteRepository
	settingsWrite    ports.WorkspaceSettingsWriteRepository
	unitOfWork       ports.UnitOfWork
	log              *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		idGen:            opts.IdGen,
		workspacesWrite:  opts.WorkspacesWrite,
		membershipsWrite: opts.MembershipsWrite,
		rolesWrite:       opts.RolesWrite,
		settingsWrite:    opts.SettingsWrite,
		unitOfWork:       opts.UnitOfWork,
		log:              opts.Logger.With("usecase", "create_workspace"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.Workspace, error) {
	name := strings.TrimSpace(cmd.Name)
	if name == "" {
		return nil, domain.ErrInvalidWorkspaceName
	}
	wsID, err := h.idGen.New()
	if err != nil {
		h.log.Error("failed to generate workspace ID", "error", err)
		return nil, err
	}
	ws, err := domain.NewWorkspace(wsID, name, cmd.Now)
	if err != nil {
		h.log.Error("failed to create workspace domain object", "error", err)
		return nil, err
	}
	membershipID, err := h.idGen.New()
	if err != nil {
		h.log.Error("failed to generate membership ID", "error", err)
		return nil, err
	}
	membership := domain.NewMembership(membershipID, ws.ID, cmd.UserID, domain.MembershipRoleOwner, cmd.Now)
	defaultRoles := domain.DefaultRoleDefinitions(cmd.Now)
	var ownerRoleID string
	for i := range defaultRoles {
		roleID, err := h.idGen.New()
		if err != nil {
			h.log.Error("failed to generate role ID", "error", err)
			return nil, err
		}
		defaultRoles[i].ID = roleID
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
		if err := h.workspacesWrite.Create(ctx, ws); err != nil {
			return err
		}
		if err := h.membershipsWrite.Create(ctx, membership); err != nil {
			return err
		}
		for _, role := range defaultRoles {
			if err := h.rolesWrite.Create(ctx, role); err != nil {
				return err
			}
		}
		if err := h.rolesWrite.ReplaceMembershipRoles(ctx, membership.ID, []string{ownerRoleID}, cmd.Now); err != nil {
			return err
		}
		defaultSettings := domain.WorkspaceSettings{
			WorkspaceID:     ws.ID,
			SettingsJSON:    domain.DefaultSettingsJSON,
			Version:         1,
			CreatedAt:       cmd.Now.UTC(),
			UpdatedAt:       cmd.Now.UTC(),
			FeatureControls: make(domain.FeatureControls),
		}
		return h.settingsWrite.CreateDefault(ctx, defaultSettings)
	}
	if err := transaction.RunInTx(ctx, h.unitOfWork, createAll); err != nil {
		if errors.Is(err, domain.ErrWorkspaceNameConflict) {
			h.log.Warn("workspace creation failed: duplicate name", "name", name)
			return nil, domain.ErrWorkspaceNameConflict
		}
		h.log.Error("failed to create workspace", "error", err)
		return nil, err
	}
	ws.MembershipID = membership.ID
	ws.RoleNames = []string{string(domain.MembershipRoleOwner)}
	h.log.Info("workspace created", "workspace_id", ws.ID, "user_id", cmd.UserID)
	return &ws, nil
}
