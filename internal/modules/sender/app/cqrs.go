package app

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/read"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/registration"
	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
)

type CommandBus interface {
	CreateSenderDomain(ctx context.Context, cmd registration.CreateInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error)
	RefreshSenderDomainDNSStatus(ctx context.Context, cmd registration.RefreshInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error)
	DisableSenderDomain(ctx context.Context, cmd registration.DisableInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error)
}

type QueryBus interface {
	GetSenderDomain(ctx context.Context, cmd read.GetInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error)
	ListSenderDomains(ctx context.Context, cmd read.ListInput) ([]senderdomain.SenderDomain, error)
	GetSenderReadiness(ctx context.Context, cmd read.ReadinessInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error)
}

type commandBus struct {
	create  *registration.CreateHandler
	refresh *registration.RefreshHandler
	disable *registration.DisableHandler
}

func newCommandBus(
	createH *registration.CreateHandler,
	refreshH *registration.RefreshHandler,
	disableH *registration.DisableHandler,
) CommandBus {
	return &commandBus{
		create:  createH,
		refresh: refreshH,
		disable: disableH,
	}
}

func (b *commandBus) CreateSenderDomain(ctx context.Context, cmd registration.CreateInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	return b.create.Execute(ctx, cmd)
}

func (b *commandBus) RefreshSenderDomainDNSStatus(ctx context.Context, cmd registration.RefreshInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	return b.refresh.Execute(ctx, cmd)
}

func (b *commandBus) DisableSenderDomain(ctx context.Context, cmd registration.DisableInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	return b.disable.Execute(ctx, cmd)
}

type queryBus struct {
	get *read.QueryService
}

func newQueryBus(querySvc *read.QueryService) QueryBus {
	return &queryBus{get: querySvc}
}

func (b *queryBus) GetSenderDomain(ctx context.Context, cmd read.GetInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	return b.get.GetSenderDomain(ctx, cmd)
}

func (b *queryBus) ListSenderDomains(ctx context.Context, cmd read.ListInput) ([]senderdomain.SenderDomain, error) {
	return b.get.ListSenderDomains(ctx, cmd)
}

func (b *queryBus) GetSenderReadiness(ctx context.Context, cmd read.ReadinessInput) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	return b.get.GetSenderReadiness(ctx, cmd)
}
