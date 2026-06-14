package app

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/createsenderdomain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/disablesenderdomain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/getsenderdomain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/getsenderreadiness"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/listsenderdomains"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app/refreshsenderdomaindnsstatus"
	senderdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
)

type CommandBus interface {
	CreateSenderDomain(ctx context.Context, cmd createsenderdomain.Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error)
	RefreshSenderDomainDNSStatus(ctx context.Context, cmd refreshsenderdomaindnsstatus.Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error)
	DisableSenderDomain(ctx context.Context, cmd disablesenderdomain.Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error)
}

type QueryBus interface {
	GetSenderDomain(ctx context.Context, cmd getsenderdomain.Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error)
	ListSenderDomains(ctx context.Context, cmd listsenderdomains.Command) ([]senderdomain.SenderDomain, error)
	GetSenderReadiness(ctx context.Context, cmd getsenderreadiness.Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error)
}

type commandBus struct {
	create  *createsenderdomain.Handler
	refresh *refreshsenderdomaindnsstatus.Handler
	disable *disablesenderdomain.Handler
}

func newCommandBus(
	createH *createsenderdomain.Handler,
	refreshH *refreshsenderdomaindnsstatus.Handler,
	disableH *disablesenderdomain.Handler,
) CommandBus {
	return &commandBus{
		create:  createH,
		refresh: refreshH,
		disable: disableH,
	}
}

func (b *commandBus) CreateSenderDomain(ctx context.Context, cmd createsenderdomain.Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	return b.create.Execute(ctx, cmd)
}

func (b *commandBus) RefreshSenderDomainDNSStatus(ctx context.Context, cmd refreshsenderdomaindnsstatus.Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	return b.refresh.Execute(ctx, cmd)
}

func (b *commandBus) DisableSenderDomain(ctx context.Context, cmd disablesenderdomain.Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	return b.disable.Execute(ctx, cmd)
}

type queryBus struct {
	get       *getsenderdomain.Handler
	list      *listsenderdomains.Handler
	readiness *getsenderreadiness.Handler
}

func newQueryBus(
	getH *getsenderdomain.Handler,
	listH *listsenderdomains.Handler,
	readinessH *getsenderreadiness.Handler,
) QueryBus {
	return &queryBus{
		get:       getH,
		list:      listH,
		readiness: readinessH,
	}
}

func (b *queryBus) GetSenderDomain(ctx context.Context, cmd getsenderdomain.Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	return b.get.Execute(ctx, cmd)
}

func (b *queryBus) ListSenderDomains(ctx context.Context, cmd listsenderdomains.Command) ([]senderdomain.SenderDomain, error) {
	return b.list.Execute(ctx, cmd)
}

func (b *queryBus) GetSenderReadiness(ctx context.Context, cmd getsenderreadiness.Command) (*senderdomain.SenderDomain, []senderdomain.DNSRecord, error) {
	return b.readiness.Execute(ctx, cmd)
}
