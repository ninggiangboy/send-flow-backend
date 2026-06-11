package api

import (
	"context"
	"errors"

	audienceapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app"
	audiencedomain "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	campaigndomain "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	campaignports "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/ports"
	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
	contentdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	senderapp "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app"
	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
)

type audienceResolverAdapter struct {
	svc *audienceapp.Service
}

func newAudienceResolverAdapter(svc *audienceapp.Service) *audienceResolverAdapter {
	return &audienceResolverAdapter{svc: svc}
}

func (a *audienceResolverAdapter) ResolveAudienceRecipients(ctx context.Context, workspaceID, userID string, ref campaignports.AudienceSelectionRef) ([]campaignports.Recipient, error) {
	recipients, err := a.svc.ResolveAudienceRecipients(ctx, workspaceID, userID, audienceapp.AudienceSelectionRef{
		ListID:     ref.ListID,
		SegmentID:  ref.SegmentID,
		ContactIDs: ref.ContactIDs,
	})
	if err != nil {
		if errors.Is(err, audiencedomain.ErrListNotFound) ||
			errors.Is(err, audiencedomain.ErrSegmentNotFound) ||
			errors.Is(err, audiencedomain.ErrContactNotFound) {
			return nil, err
		}
		return nil, err
	}

	result := make([]campaignports.Recipient, len(recipients))
	for i, r := range recipients {
		result[i] = campaignports.Recipient{
			ContactID:       r.ContactID,
			Email:           r.Email,
			EmailNormalized: r.EmailNormalized,
			FirstName:       r.FirstName,
			LastName:        r.LastName,
			Tags:            r.Tags,
			Attributes:      r.Attributes,
		}
	}
	return result, nil
}

func (a *audienceResolverAdapter) EstimateAudienceSize(ctx context.Context, workspaceID, userID string, ref campaignports.AudienceSelectionRef) (int, error) {
	return a.svc.EstimateAudienceSize(ctx, workspaceID, userID, audienceapp.AudienceSelectionRef{
		ListID:     ref.ListID,
		SegmentID:  ref.SegmentID,
		ContactIDs: ref.ContactIDs,
	})
}

type contentServiceAdapter struct {
	svc *contentapp.Service
}

func newContentServiceAdapter(svc *contentapp.Service) *contentServiceAdapter {
	return &contentServiceAdapter{svc: svc}
}

func (a *contentServiceAdapter) GetPublishedTemplateVersion(ctx context.Context, workspaceID, templateID string) (*campaignports.TemplateVersion, error) {
	version, err := a.svc.GetPublishedTemplateVersion(ctx, workspaceID, templateID)
	if err != nil {
		if errors.Is(err, contentdomain.ErrTemplateNotFound) ||
			errors.Is(err, contentdomain.ErrTemplateVersionNotFound) {
			return nil, err
		}
		return nil, err
	}
	if version == nil {
		return nil, nil
	}
	return &campaignports.TemplateVersion{
		ID:            version.ID,
		TemplateID:    version.TemplateID,
		VersionNumber: version.VersionNumber,
		Subject:       version.Subject,
		SourceHTML:    version.SourceHTML,
		SourceText:    version.SourceText,
		PublishedAt:   version.PublishedAt.String(),
		CreatedAt:     version.CreatedAt.String(),
	}, nil
}

func (a *contentServiceAdapter) ValidateTemplateRenderable(ctx context.Context, workspaceID, templateID string) error {
	return a.svc.ValidateTemplateRenderable(ctx, workspaceID, templateID)
}

type senderServiceAdapter struct {
	svc *senderapp.Service
}

func newSenderServiceAdapter(svc *senderapp.Service) *senderServiceAdapter {
	return &senderServiceAdapter{svc: svc}
}

func (a *senderServiceAdapter) GetSenderReadiness(ctx context.Context, workspaceID, senderDomainID string) (*campaignports.SenderReadiness, error) {
	result, err := a.svc.GetSenderDomainReadiness(ctx, workspaceID, senderDomainID)
	if err != nil {
		if errors.Is(err, senderdomain.ErrDomainNotFound) {
			return nil, err
		}
		return nil, err
	}
	if result == nil {
		return nil, campaigndomain.ErrSenderNotVerified
	}
	return &campaignports.SenderReadiness{
		Ready:     result.Ready,
		Reason:    result.Reason,
		CheckedAt: result.CheckedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}
