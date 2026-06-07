package worker

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	campaignpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/infrastructure/postgres"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	deliverycampaign "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/campaign"
	deliverypostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/postgres"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
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

	deliverySvc := deliveryapp.NewService(deliveryapp.Options{
		MessagesRead:      deliveryMsgReadRepo,
		MessagesWrite:     deliveryMsgWriteRepo,
		AttemptsRead:      deliveryAttemptReadRepo,
		AttemptsWrite:     deliveryAttemptWriteRepo,
		RetryStatesRead:   deliveryRetryReadRepo,
		RetryStatesWrite:  deliveryRetryWriteRepo,
		TxRequestsRead:    deliveryTxReqReadRepo,
		TxRequestsWrite:   deliveryTxReqWriteRepo,
		CampaignReader:    campaignCandidateReader,
		OutboxWriter:      deliveryOutboxRepo,
		TxManager:         deliveryTxManager,
		IDGen:             id.NewUUIDGenerator().New,
		Logger:            log,
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
