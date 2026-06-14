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
	"github.com/ninggiangboy/send-flow/backend/internal/platform/buildinfo"
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
	"github.com/ninggiangboy/send-flow/backend/internal/platform/servicediscovery"
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

	chMetrics, err := observability.NewClickHouseMetrics(nil)
	if err != nil {
		return err
	}

	syncMetrics, err := observability.NewSyncMetrics(nil)
	if err != nil {
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
		clickHouseClient, err = clickhouse.New(ctx, cfg.ClickHouseDSN, clickhouse.WithMetrics(chMetrics, "analytics"))
		if err != nil {
			return err
		}
		defer clickHouseClient.Close()
	}

	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:           cfg.AppName,
		Build:             buildinfo.Current(),
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
	senderSvc := shared.NewSenderService(senderReadRepo, senderWriteRepo, nil, nil, log, nil)

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

	var schedulerProducer kafka.Producer
	if cfg.KafkaEnabled() {
		schedulerProducer = kafka.NewWriterProducer(kafka.Brokers(cfg.KafkaBrokers))
		defer schedulerProducer.Close()
	}

	if cfg.KafkaEnabled() {
		dueMsgScheduler := NewDueMessageScheduler(
			deliveryMsgReadRepo.ListDistinctWorkspacesWithDue,
			schedulerProducer,
			log,
			"marketing",
			50,
		)
		if err := registry.Register(dueMsgScheduler); err != nil {
			return err
		}

		dueMsgConsumer := NewDueMessageConsumer(
			deliverySvc,
			log,
			kafka.Brokers(cfg.KafkaBrokers),
			cfg.WorkerConsumerGroupPrefix+".delivery_due_messages",
			pgClient.WritePool(),
		)
		if err := registry.Register(dueMsgConsumer); err != nil {
			return err
		}
	} else {
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

		dueMsgSchedulerFallback := newDueMessageProcessor(
			"delivery.due_message_scheduler",
			deliverySvc,
			log,
			5*time.Second,
			50,
			"marketing",
		)
		if err := registry.Register(dueMsgSchedulerFallback); err != nil {
			return err
		}

		dueMsgConsumer := NewDueMessageConsumer(
			deliverySvc,
			log,
			kafka.Brokers(cfg.KafkaBrokers),
			cfg.WorkerConsumerGroupPrefix+".delivery_due_messages",
			pgClient.WritePool(),
		)
		if err := registry.Register(dueMsgConsumer); err != nil {
			return err
		}
	}

	mapperRegistry := analyticsapp.NewMapperRegistry()
	deliveryAppMappers.RegisterAll(mapperRegistry)
	trackingAppMappers.RegisterAll(mapperRegistry)

	analyticsOpts := shared.NewAnalyticsRepos(pgClient.WritePool(), clickHouseClient)
	analyticsOpts.Logger = log
	analyticsSvc := analyticsapp.NewService(analyticsOpts)

	providerEventConsumer.SetOperationsRecorder(newOpsRecorderAdapter(analyticsSvc))

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

		outboxLagSampler := NewOutboxLagSampler(
			analyticsSvc,
			pgClient.WritePool(),
			log,
			60*time.Second,
		)
		if err := registry.Register(outboxLagSampler); err != nil {
			return err
		}

		clickHouseSync := NewAnalyticsClickHouseSyncProcessor(analyticsSvc, log, WithSyncMetrics(syncMetrics))
		if err := registry.Register(clickHouseSync); err != nil {
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

	if cfg.KafkaEnabled() {
		dueNotifScheduler := NewDueNotificationScheduler(
			schedulerProducer,
			log,
			50,
		)
		if err := registry.Register(dueNotifScheduler); err != nil {
			return err
		}

		dueNotifConsumer := NewDueNotificationConsumer(
			notificationSvc,
			log,
			kafka.Brokers(cfg.KafkaBrokers),
			cfg.WorkerConsumerGroupPrefix+".notification_due_retries",
			pgClient.WritePool(),
		)
		if err := registry.Register(dueNotifConsumer); err != nil {
			return err
		}
	} else {
		dueNotificationProcessor := NewDueNotificationProcessor(
			notificationSvc,
			log,
			10*time.Second,
			50,
		)
		if err := registry.Register(dueNotificationProcessor); err != nil {
			return err
		}

		dueNotificationSchedulerFallback := newDueNotificationProcessor(
			"notification.due_notification_scheduler",
			notificationSvc,
			log,
			10*time.Second,
			50,
		)
		if err := registry.Register(dueNotificationSchedulerFallback); err != nil {
			return err
		}

		dueNotifConsumer := NewDueNotificationConsumer(
			notificationSvc,
			log,
			kafka.Brokers(cfg.KafkaBrokers),
			cfg.WorkerConsumerGroupPrefix+".notification_due_retries",
			pgClient.WritePool(),
		)
		if err := registry.Register(dueNotifConsumer); err != nil {
			return err
		}
	}

	webhooksSvc := shared.NewWebhooksService(pgReadPool, pgWritePool, nil, log)

	webhookConsumer := NewWebhookEventConsumer(
		webhooksSvc,
		log,
		kafka.Brokers(cfg.KafkaBrokers),
		cfg.WorkerConsumerGroupPrefix+".webhooks_deliver_events",
		pgClient.WritePool(),
	)
	if analyticsSvc != nil {
		webhookConsumer.SetOperationsRecorder(newOpsRecorderAdapter(analyticsSvc))
	}
	if err := registry.Register(webhookConsumer); err != nil {
		return err
	}

	if cfg.KafkaEnabled() {
		dueWebhookScheduler := NewDueWebhookScheduler(
			schedulerProducer,
			log,
			50,
		)
		if err := registry.Register(dueWebhookScheduler); err != nil {
			return err
		}

		dueWebhookConsumer := NewDueWebhookConsumer(
			webhooksSvc,
			log,
			kafka.Brokers(cfg.KafkaBrokers),
			cfg.WorkerConsumerGroupPrefix+".webhooks_due_deliveries",
			pgClient.WritePool(),
		)
		if analyticsSvc != nil {
			dueWebhookConsumer.SetOperationsRecorder(newOpsRecorderAdapter(analyticsSvc))
		}
		if err := registry.Register(dueWebhookConsumer); err != nil {
			return err
		}
	} else {
		webhookDeliveryProcessor := NewDueWebhookDeliveryProcessor(
			webhooksSvc,
			log,
			10*time.Second,
			50,
		)
		if analyticsSvc != nil {
			webhookDeliveryProcessor.SetOperationsRecorder(newOpsRecorderAdapter(analyticsSvc))
		}
		if err := registry.Register(webhookDeliveryProcessor); err != nil {
			return err
		}

		webhookDeliverySchedulerFallback := newDueWebhookDeliveryProcessor(
			"webhooks.due_webhook_scheduler",
			webhooksSvc,
			log,
			10*time.Second,
			50,
		)
		if analyticsSvc != nil {
			webhookDeliverySchedulerFallback.SetOperationsRecorder(newOpsRecorderAdapter(analyticsSvc))
		}
		if err := registry.Register(webhookDeliverySchedulerFallback); err != nil {
			return err
		}

		dueWebhookConsumer := NewDueWebhookConsumer(
			webhooksSvc,
			log,
			kafka.Brokers(cfg.KafkaBrokers),
			cfg.WorkerConsumerGroupPrefix+".webhooks_due_deliveries",
			pgClient.WritePool(),
		)
		if err := registry.Register(dueWebhookConsumer); err != nil {
			return err
		}
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

	var consulRegistrar *servicediscovery.ConsulRegistrar
	if cfg.WorkerDiscovery.Enabled() {
		consulRegistrar = servicediscovery.NewConsulRegistrar(cfg.WorkerDiscovery)
		registerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := consulRegistrar.Register(registerCtx); err != nil {
			cancel()
			stopWorkers()
			_ = server.Close()
			return err
		}
		cancel()
		log.Info("worker service registered", "provider", cfg.WorkerDiscovery.Provider, "service_id", cfg.WorkerDiscovery.ServiceID)
	}

	var runErr error
	select {
	case <-ctx.Done():
	case workerErr := <-workerErrCh:
		if workerErr != nil {
			stopWorkers()
			runErr = workerErr
		}
	case serveErr := <-httpErrCh:
		if serveErr != nil {
			stopWorkers()
			runErr = serveErr
		}
	}

	stopWorkers()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.WorkerShutdownTimeout)
	defer cancel()

	log.Info("worker stopping")
	if consulRegistrar != nil {
		if err := consulRegistrar.Deregister(shutdownCtx); err != nil {
			log.Warn("failed to deregister worker service", "provider", cfg.WorkerDiscovery.Provider, "service_id", cfg.WorkerDiscovery.ServiceID, "error", err)
		} else {
			log.Info("worker service deregistered", "provider", cfg.WorkerDiscovery.Provider, "service_id", cfg.WorkerDiscovery.ServiceID)
		}
	}
	if err := server.Shutdown(shutdownCtx); err != nil && runErr == nil {
		runErr = err
	}
	return runErr
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
