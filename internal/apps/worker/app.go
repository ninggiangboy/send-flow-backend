package worker

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	analyticspostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/infrastructure/postgres"
	campaignpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/infrastructure/postgres"
	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
	contentpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/content/infrastructure/postgres"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	deliverycampaign "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/campaign"
	deliverypostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/postgres"
	deliveryports "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	notificationapp "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app"
	notificationemail "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/infrastructure/email"
	notificationpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/infrastructure/postgres"
	senderapp "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app"
	senderpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/infrastructure/postgres"
	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
	suppressionpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/infrastructure/postgres"
	trackingapp "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/unsubscribetoken"
	trackingpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/infrastructure/postgres"
	webhooksapp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app"
	webhookshttp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/infrastructure/http"
	webhookspostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/infrastructure/postgres"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
	platformemail "github.com/ninggiangboy/send-flow/backend/internal/platform/email"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/httpjson"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/logger"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Run(ctx context.Context) error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	log := logger.New(cfg)
	httpMetrics := observability.NewHTTPMetrics(nil)
	observability.RegisterFallbackProcessMetrics(nil)

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

	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:           cfg.AppName,
		PostgresCheck:     pgClient.Ping,
		PostgresReadCheck: pgClient.PingRead,
		RedisCheck:        redisClient.Ping,
		KafkaEnabled:      cfg.KafkaEnabled(),
		ClickEnabled:      cfg.ClickHouseEnabled(),
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
	deliveryTxManager := deliverypostgres.NewTransactionManager(pgWritePool)

	deliveryAttemptReadRepo := deliverypostgres.NewAttemptReadRepository(pgReadPool)
	deliveryAttemptWriteRepo := deliverypostgres.NewAttemptWriteRepository(pgWritePool)
	deliveryRetryReadRepo := deliverypostgres.NewRetryStateReadRepository(pgReadPool)
	deliveryRetryWriteRepo := deliverypostgres.NewRetryStateWriteRepository(pgWritePool)
	deliveryTxReqReadRepo := deliverypostgres.NewTransactionalRequestReadRepository(pgReadPool)
	deliveryTxReqWriteRepo := deliverypostgres.NewTransactionalRequestWriteRepository(pgWritePool)

	// Wire suppression module
	suppressionReadRepo := suppressionpostgres.NewReadRepository(pgReadPool)
	suppressionWriteRepo := suppressionpostgres.NewWriteRepository(pgWritePool)

	suppressionSvc := suppressionapp.NewService(suppressionapp.Options{
		EntriesRead:  suppressionReadRepo,
		EntriesWrite: suppressionWriteRepo,
		IDGen:        id.NewUUIDGenerator().New,
		Logger:       log,
	})
	suppressionRecipientSuppressorAdapter := newRecipientSuppressorAdapter(suppressionSvc)

	// Wire content module
	contentReadRepo := contentpostgres.NewTemplateReadRepository(pgReadPool)
	contentWriteRepo := contentpostgres.NewTemplateWriteRepository(pgWritePool)

	contentSvc := contentapp.NewService(contentapp.Options{
		TemplatesRead:  contentReadRepo,
		TemplatesWrite: contentWriteRepo,
		IDGen:          id.NewUUIDGenerator().New,
		Logger:         log,
	})

	// Wire sender module
	senderReadRepo := senderpostgres.NewReadRepository(pgReadPool)
	senderWriteRepo := senderpostgres.NewWriteRepository(pgWritePool)

	senderSvc := senderapp.NewService(senderapp.Options{
		DomainsRead:  senderReadRepo,
		DomainsWrite: senderWriteRepo,
		IDGen:        id.NewUUIDGenerator().New,
		Logger:       log,
	})

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
	trackingLinkReadRepo := trackingpostgres.NewTrackingLinkRepository(pgReadPool)
	trackingLinkWriteRepo := trackingpostgres.NewTrackingLinkRepository(pgWritePool)
	trackingEventReadRepo := trackingpostgres.NewTrackingEventRepository(pgReadPool)
	trackingEventWriteRepo := trackingpostgres.NewTrackingEventRepository(pgWritePool)
	trackingMessageResolver := newTrackingMessageResolverAdapter(deliveryMsgReadRepo)
	trackingSuppressor := newTrackingSuppressorAdapter(suppressionSvc)
	trackingOutboxRepo := trackingpostgres.NewOutboxRepository(pgWritePool)
	trackingTxManager := trackingpostgres.NewTransactionManager(pgWritePool)
	trackingSvc := trackingapp.NewService(trackingapp.Options{
		LinkReadRepo:        trackingLinkReadRepo,
		LinkWriteRepo:       trackingLinkWriteRepo,
		EventReadRepo:       trackingEventReadRepo,
		EventWriteRepo:      trackingEventWriteRepo,
		MessageResolver:     trackingMessageResolver,
		RecipientSuppressor: trackingSuppressor,
		OutboxWriter:        trackingOutboxRepo,
		TxManager:           trackingTxManager,
		IDGen:               id.NewUUIDGenerator().New,
		Logger:              log,
		TokenSigner:         unsubscribetoken.NewSigner(cfg.UnsubscribeTokenSecret),
	})

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

	// Wire analytics module
	analyticsWritePool := pgClient.WritePool()
	analyticsFactRepo := analyticspostgres.NewEventFactRepository(analyticsWritePool)
	analyticsProjectionRepo := analyticspostgres.NewProjectionRepository(analyticsWritePool)
	analyticsTxManager := analyticspostgres.NewTransactionManager(analyticsWritePool)
	analyticsOutboxRepo := analyticspostgres.NewOutboxRepository(analyticsWritePool)

	analyticsSvc := analyticsapp.NewService(analyticsapp.Options{
		FactRepo:       analyticsFactRepo,
		ProjectionRepo: analyticsProjectionRepo,
		TxManager:      analyticsTxManager,
		OutboxWriter:   analyticsOutboxRepo,
		IDGen:          id.NewUUIDGenerator().New,
		Logger:         log,
	})

	analyticsConsumer := NewAnalyticsEventConsumer(
		analyticsSvc,
		log,
		kafka.Brokers(cfg.KafkaBrokers),
		cfg.WorkerConsumerGroupPrefix+".analytics_events",
		pgClient.WritePool(),
	)
	if err := registry.Register(analyticsConsumer); err != nil {
		return err
	}

	notificationMsgRead := notificationpostgres.NewMessageReadRepository(pgReadPool)
	notificationMsgWrite := notificationpostgres.NewMessageWriteRepository(pgWritePool)
	notificationAttemptRead := notificationpostgres.NewAttemptReadRepository(pgReadPool)
	notificationAttemptWrite := notificationpostgres.NewAttemptWriteRepository(pgWritePool)
	notificationOutbox := notificationpostgres.NewOutboxRepository(pgWritePool)
	notificationTxManager := notificationpostgres.NewTransactionManager(pgWritePool)
	notificationEmailAdapter := notificationemail.NewEmailAdapter(emailSender)

	notificationSvc := notificationapp.NewService(notificationapp.Options{
		MessagesRead:  notificationMsgRead,
		MessagesWrite: notificationMsgWrite,
		AttemptsRead:  notificationAttemptRead,
		AttemptsWrite: notificationAttemptWrite,
		OutboxWriter:  notificationOutbox,
		TxManager:     notificationTxManager,
		EmailSender:   notificationEmailAdapter,
		IDGen:         id.NewUUIDGenerator().New,
		Logger:        log,
	})

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

	webhooksConfigRead := webhookspostgres.NewConfigReadRepository(pgReadPool)
	webhooksConfigWrite := webhookspostgres.NewConfigWriteRepository(pgWritePool)
	webhooksDeliveryRead := webhookspostgres.NewDeliveryReadRepository(pgReadPool)
	webhooksDeliveryWrite := webhookspostgres.NewDeliveryWriteRepository(pgWritePool)
	webhooksAttemptRead := webhookspostgres.NewAttemptReadRepository(pgReadPool)
	webhooksAttemptWrite := webhookspostgres.NewAttemptWriteRepository(pgWritePool)
	webhooksTxManager := webhookspostgres.NewTransactionManager(pgWritePool)
	webhooksOutbox := webhookspostgres.NewOutboxRepository(pgWritePool)
	webhooksDeliverer := webhookshttp.NewDeliverer()

	webhooksSvc := webhooksapp.NewService(webhooksapp.Options{
		ConfigRead:    webhooksConfigRead,
		ConfigWrite:   webhooksConfigWrite,
		DeliveryRead:  webhooksDeliveryRead,
		DeliveryWrite: webhooksDeliveryWrite,
		AttemptRead:   webhooksAttemptRead,
		AttemptWrite:  webhooksAttemptWrite,
		TxManager:     webhooksTxManager,
		OutboxWriter:  webhooksOutbox,
		Deliverer:     webhooksDeliverer,
		IDGen:         id.NewUUIDGenerator().New,
		Logger:        log,
	})

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
			httpjson.Write(w, http.StatusOK, healthSvc.Live())
		})
		r.Get("/readyz", func(w http.ResponseWriter, req *http.Request) {
			ready := healthSvc.Ready(req.Context())
			if ready.Status != "ok" {
				httpjson.Write(w, http.StatusServiceUnavailable, ready)
				return
			}
			httpjson.Write(w, http.StatusOK, ready)
		})
		r.Handle("/metrics", promhttp.Handler())
	})
	return r
}
