package worker

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/apps/shared"
	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	audiencepostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/infrastructure/postgres"
	campaignpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/infrastructure/postgres"
	deliveryAppMappers "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/analyticsmappers"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	deliveryinfrastructure "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure"
	deliverycampaign "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/campaign"
	deliverypostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/postgres"
	deliveryports "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	notificationapp "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app"
	trackingAppMappers "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/analyticsmappers"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/clickhouse"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
	platformemail "github.com/ninggiangboy/send-flow/backend/internal/platform/email"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/httpjson"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/logger"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/objectstorage"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Run(ctx context.Context) error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	log := logger.New(cfg)
	config.WarnDevSecrets(log, cfg)
	httpMetrics, err := observability.NewHTTPMetrics(nil)
	if err != nil {
		return err
	}
	if err := observability.RegisterFallbackProcessMetrics(nil); err != nil {
		return err
	}

	pgClient, err := postgres.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer pgClient.Close()

	redisClient, err := platformredis.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	var objectStorageClient objectstorage.ObjectStorage
	if cfg.ObjectStorageEnabled() {
		objectStorageClient, err = objectstorage.New(ctx, cfg.ObjectStorage)
		if err != nil {
			return err
		}
		ensureCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := objectStorageClient.EnsureBucket(ensureCtx); err != nil {
			return err
		}
	}

	var clickHouseClient *clickhouse.Client
	if cfg.ClickHouseEnabled() {
		clickHouseClient, err = clickhouse.New(ctx, cfg.ClickHouseDSN)
		if err != nil {
			return err
		}
		defer clickHouseClient.Close()
	}

	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:           cfg.AppName,
		PostgresCheck:     pgClient.Ping,
		PostgresReadCheck: pgClient.PingRead,
		RedisCheck:        redisClient.Ping,
		ClickHouseEnabled: cfg.ClickHouseEnabled(),
		ClickHouseCheck: func(ctx context.Context) error {
			if clickHouseClient == nil {
				return nil
			}
			return clickHouseClient.Ping(ctx)
		},
		KafkaEnabled: cfg.KafkaEnabled(),
		ObjectStorageCheck: func(ctx context.Context) error {
			if objectStorageClient == nil {
				return nil
			}
			return objectStorageClient.Ping(ctx)
		},
		ObjectStorageEnabled: cfg.ObjectStorageEnabled(),
	})

	registry := NewRegistry()

	// Wire delivery module
	pgReadPool := pgClient.ReadPool()
	pgWritePool := pgClient.WritePool()

	campaignReadRepo := campaignpostgres.NewCampaignReadRepository(pgReadPool)
	campaignCandidateReader := deliverycampaign.NewCandidateReader(campaignReadRepo)

	deliveryMsgReadRepo := deliverypostgres.NewMessageReadRepository(pgReadPool)
	deliveryMsgWriteRepo := deliverypostgres.NewMessageWriteRepository(pgWritePool)
	deliveryOutboxRepo := deliverypostgres.NewOutboxRepository(pgWritePool)
	deliveryTxManager := transaction.NewManager(pgWritePool)

	deliveryAttemptReadRepo := deliverypostgres.NewAttemptReadRepository(pgReadPool)
	deliveryAttemptWriteRepo := deliverypostgres.NewAttemptWriteRepository(pgWritePool)
	deliveryRetryReadRepo := deliverypostgres.NewRetryStateReadRepository(pgReadPool)
	deliveryRetryWriteRepo := deliverypostgres.NewRetryStateWriteRepository(pgWritePool)
	deliveryTxReqReadRepo := deliverypostgres.NewTransactionalRequestReadRepository(pgReadPool)
	deliveryTxReqWriteRepo := deliverypostgres.NewTransactionalRequestWriteRepository(pgWritePool)

	suppressionReadRepo, suppressionWriteRepo := shared.NewSuppressionRepos(pgReadPool, pgWritePool)
	suppressionSvc := shared.NewSuppressionService(suppressionReadRepo, suppressionWriteRepo, nil, log)
	suppressionRecipientSuppressorAdapter := newRecipientSuppressorAdapter(suppressionSvc)

	contentReadRepo, contentWriteRepo := shared.NewContentRepos(pgReadPool, pgWritePool)
	contentSvc := shared.NewContentService(contentReadRepo, contentWriteRepo, nil, log)

	senderReadRepo, senderWriteRepo := shared.NewSenderRepos(pgReadPool, pgWritePool)
	senderSvc := shared.NewSenderService(senderReadRepo, senderWriteRepo, nil, nil, log)

	// Create platform email sender (shared across modules)
	emailSender, err := platformemail.NewSender(ctx, cfg)
	if err != nil {
		return err
	}

	// Create delivery-facing adapters
	deliverySuppressionAdapter := newSuppressionAdapter(suppressionSvc)
	deliveryContentAdapter := newContentRendererAdapter(contentSvc)
	deliverySenderAdapter := newSenderReadinessAdapter(senderSvc)
	var deliveryProvider deliveryports.EmailProvider
	switch cfg.EmailProvider {
	case "fake":
		deliveryProvider = NewFakeEmailProvider()
	default:
		deliveryProvider = newDeliveryEmailProvider(cfg.EmailProvider, emailSender)
	}

	deliverySvc := deliveryapp.NewService(deliveryapp.Options{
		MessagesRead:        deliveryMsgReadRepo,
		MessagesWrite:       deliveryMsgWriteRepo,
		AttemptsRead:        deliveryAttemptReadRepo,
		AttemptsWrite:       deliveryAttemptWriteRepo,
		RetryStatesRead:     deliveryRetryReadRepo,
		RetryStatesWrite:    deliveryRetryWriteRepo,
		TxRequestsRead:      deliveryTxReqReadRepo,
		TxRequestsWrite:     deliveryTxReqWriteRepo,
		CampaignReader:      campaignCandidateReader,
		ContentRenderer:     deliveryContentAdapter,
		SenderChecker:       deliverySenderAdapter,
		SuppressionChecker:  deliverySuppressionAdapter,
		RecipientSuppressor: suppressionRecipientSuppressorAdapter,
		EmailProvider:       deliveryProvider,
		OutboxWriter:        deliveryOutboxRepo,
		TxManager:           deliveryTxManager,
		IDGen:               id.NewUUIDGenerator().New,
		Logger:              log,
	})

	consumer := NewCampaignScheduledConsumer(
		deliverySvc,
		log,
		kafka.Brokers(cfg.KafkaBrokers),
		cfg.WorkerConsumerGroupPrefix+".delivery_queue_campaign_messages",
		pgClient.WritePool(),
	)
	if err := registry.Register(consumer); err != nil {
		return err
	}

	providerEventConsumer := NewProviderEventConsumer(
		deliverySvc,
		log,
		kafka.Brokers(cfg.KafkaBrokers),
		cfg.WorkerConsumerGroupPrefix+".delivery_provider_events",
		pgClient.WritePool(),
	)
	if err := registry.Register(providerEventConsumer); err != nil {
		return err
	}

	// Wire tracking module
	trackingDeps := shared.NewTrackingRepos(pgReadPool, pgWritePool)
	trackingMessageResolver := shared.NewTrackingMessageResolverAdapter(deliveryinfrastructure.NewMessageResolver(deliveryMsgReadRepo))
	trackingSuppressor := shared.NewTrackingSuppressorAdapter(suppressionSvc)
	trackingSvc := shared.NewTrackingService(trackingDeps, trackingMessageResolver, trackingSuppressor, cfg.UnsubscribeTokenSecret, log)

	trackingConsumer := NewTrackingProviderEventConsumer(
		trackingSvc,
		log,
		kafka.Brokers(cfg.KafkaBrokers),
		cfg.WorkerConsumerGroupPrefix+".tracking_provider_events",
		pgClient.WritePool(),
	)
	if err := registry.Register(trackingConsumer); err != nil {
		return err
	}

	dueMsgProcessor := NewDueMessageProcessor(
		deliverySvc,
		log,
		5*time.Second,
		50,
		"marketing",
	)
	if err := registry.Register(dueMsgProcessor); err != nil {
		return err
	}

	mapperRegistry := analyticsapp.NewMapperRegistry()
	deliveryAppMappers.RegisterAll(mapperRegistry)
	trackingAppMappers.RegisterAll(mapperRegistry)

	analyticsOpts := shared.NewAnalyticsRepos(pgClient.WritePool(), clickHouseClient)
	analyticsOpts.Logger = log
	analyticsSvc := analyticsapp.NewService(analyticsOpts)

	analyticsConsumer := NewAnalyticsEventConsumer(
		analyticsSvc,
		mapperRegistry,
		log,
		kafka.Brokers(cfg.KafkaBrokers),
		cfg.WorkerConsumerGroupPrefix+".analytics_events",
		pgClient.WritePool(),
	)
	if err := registry.Register(analyticsConsumer); err != nil {
		return err
	}

	if clickHouseClient != nil {
		anomalyProcessor := NewAnalyticsAnomalyProcessor(
			analyticsSvc,
			log,
			15*time.Minute,
		)
		if err := registry.Register(anomalyProcessor); err != nil {
			return err
		}
	}

	notificationOpts := shared.NewNotificationRepos(pgReadPool, pgWritePool, emailSender)
	notificationOpts.Logger = log
	notificationSvc := notificationapp.NewService(notificationOpts)

	notificationConsumer := NewNotificationEventConsumer(
		notificationSvc,
		log,
		kafka.Brokers(cfg.KafkaBrokers),
		cfg.WorkerConsumerGroupPrefix+".notification_identity_events",
		pgClient.WritePool(),
		cfg.FrontendBaseURL,
	)
	if err := registry.Register(notificationConsumer); err != nil {
		return err
	}

	dueNotificationProcessor := NewDueNotificationProcessor(
		notificationSvc,
		log,
		10*time.Second,
		50,
	)
	if err := registry.Register(dueNotificationProcessor); err != nil {
		return err
	}

	webhooksSvc := shared.NewWebhooksService(pgReadPool, pgWritePool, nil, log)

	webhookConsumer := NewWebhookEventConsumer(
		webhooksSvc,
		log,
		kafka.Brokers(cfg.KafkaBrokers),
		cfg.WorkerConsumerGroupPrefix+".webhooks_deliver_events",
		pgClient.WritePool(),
	)
	if err := registry.Register(webhookConsumer); err != nil {
		return err
	}

	webhookDeliveryProcessor := NewDueWebhookDeliveryProcessor(
		webhooksSvc,
		log,
		10*time.Second,
		50,
	)
	if err := registry.Register(webhookDeliveryProcessor); err != nil {
		return err
	}

	// Wire audience import/export processors
	if objectStorageClient != nil {
		audienceContactsRead := audiencepostgres.NewContactReadRepository(pgReadPool)
		audienceContactsWrite := audiencepostgres.NewContactWriteRepository(pgWritePool)
		audienceImportJobsWrite := audiencepostgres.NewImportJobWriteRepository(pgWritePool)
		audienceExportJobsWrite := audiencepostgres.NewExportJobWriteRepository(pgWritePool)
		audienceOutboxRepo := audiencepostgres.NewOutboxRepository(pgWritePool)

		importProcessor := NewAudienceImportProcessor(
			audienceImportJobsWrite,
			audienceContactsWrite,
			audienceContactsRead,
			audienceOutboxRepo,
			transaction.NewManager(pgWritePool),
			objectStorageClient,
			log,
			10*time.Second,
			10,
		)
		if err := registry.Register(importProcessor); err != nil {
			return err
		}

		exportProcessor := NewAudienceExportProcessor(
			audienceExportJobsWrite,
			audienceContactsRead,
			audienceOutboxRepo,
			transaction.NewManager(pgWritePool),
			objectStorageClient,
			log,
			10*time.Second,
			10,
		)
		if err := registry.Register(exportProcessor); err != nil {
			return err
		}
	}

	workerCtx, stopWorkers := context.WithCancel(ctx)
	defer stopWorkers()

	workerErrCh := make(chan error, 1)
	go func() {
		workerErrCh <- registry.Run(workerCtx, cfg.WorkerEnabledConsumers, log)
	}()

	server := &http.Server{
		Addr:    cfg.WorkerHTTPAddr,
		Handler: newRouter(healthSvc, httpMetrics),
	}

	httpErrCh := make(chan error, 1)
	go func() {
		log.Info("worker health server starting", "addr", cfg.WorkerHTTPAddr)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			httpErrCh <- serveErr
		}
		close(httpErrCh)
	}()

	select {
	case <-ctx.Done():
	case runErr := <-workerErrCh:
		if runErr != nil {
			stopWorkers()
			return runErr
		}
	case serveErr := <-httpErrCh:
		if serveErr != nil {
			stopWorkers()
			return serveErr
		}
	}

	stopWorkers()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.WorkerShutdownTimeout)
	defer cancel()

	log.Info("worker stopping")
	return server.Shutdown(shutdownCtx)
}

func newRouter(healthSvc *platformhealth.Service, httpMetrics *observability.HTTPMetrics) http.Handler {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			rec := &observability.ResponseRecorder{ResponseWriter: w, Status: http.StatusOK}
			next.ServeHTTP(rec, req)
			route := chi.RouteContext(req.Context()).RoutePattern()
			if route == "" {
				route = req.URL.Path
			}
			httpMetrics.Record(req.Method, route, strconv.Itoa(rec.Status), time.Since(start).Seconds())
		})
	})
	r.Route("/api", func(r chi.Router) {
		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			_ = httpjson.Write(w, http.StatusOK, healthSvc.Live())
		})
		r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
			ready := healthSvc.Ready(req.Context())
			if ready.Status != "ok" {
				_ = httpjson.Write(w, http.StatusServiceUnavailable, ready)
				return
			}
			_ = httpjson.Write(w, http.StatusOK, ready)
		})
		r.Handle("/metrics", promhttp.Handler())
	})
	return r
}
