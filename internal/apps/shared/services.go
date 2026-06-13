package shared

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	analyticsclickhouse "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/infrastructure/clickhouse"
	analyticspostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/infrastructure/postgres"
	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
	contentpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/content/infrastructure/postgres"
	notificationapp "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app"
	notificationemail "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/infrastructure/email"
	notificationpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/infrastructure/postgres"
	senderapp "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app"
	senderdns "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/infrastructure/dns"
	senderpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/infrastructure/postgres"
	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
	suppressionpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/infrastructure/postgres"
	trackingapp "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/unsubscribetoken"
	trackingpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/infrastructure/postgres"
	trackingports "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/ports"
	webhooksapp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app"
	webhookshttp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/infrastructure/http"
	webhookspostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/infrastructure/postgres"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
	platformclickhouse "github.com/ninggiangboy/send-flow/backend/internal/platform/clickhouse"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/email"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

func NewSuppressionRepos(pgReadPool, pgWritePool *pgxpool.Pool) (*suppressionpostgres.ReadRepository, *suppressionpostgres.WriteRepository) {
	return suppressionpostgres.NewReadRepository(pgReadPool),
		suppressionpostgres.NewWriteRepository(pgWritePool)
}

func NewSuppressionService(readRepo *suppressionpostgres.ReadRepository, writeRepo *suppressionpostgres.WriteRepository, accessChecker auth.WorkspaceAccessChecker, logger *slog.Logger) *suppressionapp.Service {
	return suppressionapp.NewService(suppressionapp.Options{
		EntriesRead:   readRepo,
		EntriesWrite:  writeRepo,
		AccessChecker: accessChecker,
		IDGen:         id.NewUUIDGenerator().New,
		Logger:        logger,
	})
}

func NewContentRepos(pgReadPool, pgWritePool *pgxpool.Pool) (*contentpostgres.TemplateReadRepository, *contentpostgres.TemplateWriteRepository) {
	return contentpostgres.NewTemplateReadRepository(pgReadPool),
		contentpostgres.NewTemplateWriteRepository(pgWritePool)
}

func NewContentService(readRepo *contentpostgres.TemplateReadRepository, writeRepo *contentpostgres.TemplateWriteRepository, accessChecker auth.WorkspaceAccessChecker, logger *slog.Logger) *contentapp.Service {
	return contentapp.NewService(contentapp.Options{
		TemplatesRead:  readRepo,
		TemplatesWrite: writeRepo,
		AccessChecker:  accessChecker,
		IDGen:          id.NewUUIDGenerator().New,
		Logger:         logger,
	})
}

func NewSenderRepos(pgReadPool, pgWritePool *pgxpool.Pool) (*senderpostgres.ReadRepository, *senderpostgres.WriteRepository) {
	return senderpostgres.NewReadRepository(pgReadPool),
		senderpostgres.NewWriteRepository(pgWritePool)
}

func NewSenderService(readRepo *senderpostgres.ReadRepository, writeRepo *senderpostgres.WriteRepository, dnsResolver *senderdns.Resolver, accessChecker auth.WorkspaceAccessChecker, logger *slog.Logger) *senderapp.Service {
	return senderapp.NewService(senderapp.Options{
		DomainsRead:   readRepo,
		DomainsWrite:  writeRepo,
		DNSResolver:   dnsResolver,
		AccessChecker: accessChecker,
		IDGen:         id.NewUUIDGenerator().New,
		Logger:        logger,
	})
}

func NewAnalyticsRepos(writePool *pgxpool.Pool, chClient *platformclickhouse.Client) analyticsapp.Options {
	opts := analyticsapp.Options{
		FactRepo:        analyticspostgres.NewEventFactRepository(writePool),
		ProjectionRead:  analyticspostgres.NewProjectionRepository(writePool),
		ProjectionWrite: analyticspostgres.NewProjectionRepository(writePool),
		TxManager:       transaction.NewManager(writePool),
		OutboxWriter:    analyticspostgres.NewOutboxRepository(writePool),
		IDGen:           id.NewUUIDGenerator().New,
	}
	if chClient != nil {
		opts.ClickHouseFactRepo = analyticsclickhouse.NewFactRepository(chClient.Conn())
		opts.CampaignQueryRepo = analyticsclickhouse.NewCampaignRepository(chClient.Conn())
		opts.DeliverabilityQueryRepo = analyticsclickhouse.NewDeliverabilityRepository(chClient.Conn())
		opts.ForensicQueryRepo = analyticsclickhouse.NewForensicsRepository(chClient.Conn())
		opts.OperationsQueryRepo = analyticsclickhouse.NewOperationsRepository(chClient.Conn())
		opts.UsageQueryRepo = analyticsclickhouse.NewUsageRepository(chClient.Conn())
		opts.AnomalySignalWriteRepo = analyticsclickhouse.NewAnomalySignalRepository(chClient.Conn())
		opts.OperationsEventWriter = analyticsclickhouse.NewOperationsWriter(chClient.Conn())
	}
	return opts
}

func NewNotificationRepos(pgReadPool, pgWritePool *pgxpool.Pool, emailSender email.Sender) notificationapp.Options {
	return notificationapp.Options{
		MessagesRead:  notificationpostgres.NewMessageReadRepository(pgReadPool),
		MessagesWrite: notificationpostgres.NewMessageWriteRepository(pgWritePool),
		AttemptsRead:  notificationpostgres.NewAttemptReadRepository(pgReadPool),
		AttemptsWrite: notificationpostgres.NewAttemptWriteRepository(pgWritePool),
		OutboxWriter:  notificationpostgres.NewOutboxRepository(pgWritePool),
		TxManager:     transaction.NewManager(pgWritePool),
		EmailSender:   notificationemail.NewEmailAdapter(emailSender),
		IDGen:         id.NewUUIDGenerator().New,
	}
}

func NewWebhooksService(pgReadPool, pgWritePool *pgxpool.Pool, accessChecker auth.WorkspaceAccessChecker, logger *slog.Logger) *webhooksapp.Service {
	return webhooksapp.NewService(webhooksapp.Options{
		ConfigRead:    webhookspostgres.NewConfigReadRepository(pgReadPool),
		ConfigWrite:   webhookspostgres.NewConfigWriteRepository(pgWritePool),
		DeliveryRead:  webhookspostgres.NewDeliveryReadRepository(pgReadPool),
		DeliveryWrite: webhookspostgres.NewDeliveryWriteRepository(pgWritePool),
		AttemptRead:   webhookspostgres.NewAttemptReadRepository(pgReadPool),
		AttemptWrite:  webhookspostgres.NewAttemptWriteRepository(pgWritePool),
		TxManager:     transaction.NewManager(pgWritePool),
		OutboxWriter:  webhookspostgres.NewOutboxRepository(pgWritePool),
		Deliverer:     webhookshttp.NewDeliverer(),
		AccessChecker: accessChecker,
		IDGen:         id.NewUUIDGenerator().New,
		Logger:        logger,
	})
}

type TrackingRepoDeps struct {
	LinkReadRepo   *trackingpostgres.TrackingLinkRepository
	LinkWriteRepo  *trackingpostgres.TrackingLinkRepository
	EventReadRepo  *trackingpostgres.TrackingEventRepository
	EventWriteRepo *trackingpostgres.TrackingEventRepository
	OutboxRepo     *trackingpostgres.OutboxRepository
	TxManager      *transaction.Manager
}

func NewTrackingRepos(pgReadPool, pgWritePool *pgxpool.Pool) TrackingRepoDeps {
	return TrackingRepoDeps{
		LinkReadRepo:   trackingpostgres.NewTrackingLinkRepository(pgReadPool),
		LinkWriteRepo:  trackingpostgres.NewTrackingLinkRepository(pgWritePool),
		EventReadRepo:  trackingpostgres.NewTrackingEventRepository(pgReadPool),
		EventWriteRepo: trackingpostgres.NewTrackingEventRepository(pgWritePool),
		OutboxRepo:     trackingpostgres.NewOutboxRepository(pgWritePool),
		TxManager:      transaction.NewManager(pgWritePool),
	}
}

func NewTrackingService(deps TrackingRepoDeps, messageResolver trackingports.DeliveryMessageResolver, suppressor trackingapp.RecipientSuppressor, unsubscribeSecret string, logger *slog.Logger) *trackingapp.Service {
	return trackingapp.NewService(trackingapp.Options{
		LinkReadRepo:        deps.LinkReadRepo,
		LinkWriteRepo:       deps.LinkWriteRepo,
		EventReadRepo:       deps.EventReadRepo,
		EventWriteRepo:      deps.EventWriteRepo,
		MessageResolver:     messageResolver,
		RecipientSuppressor: suppressor,
		OutboxWriter:        deps.OutboxRepo,
		TxManager:           deps.TxManager,
		IDGen:               id.NewUUIDGenerator().New,
		Logger:              logger,
		TokenSigner:         unsubscribetoken.NewSigner(unsubscribeSecret),
	})
}
