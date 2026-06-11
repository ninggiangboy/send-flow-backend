package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/ninggiangboy/send-flow/backend"
	"github.com/ninggiangboy/send-flow/backend/internal/apps/shared"
	accessapp "github.com/ninggiangboy/send-flow/backend/internal/modules/access/app"
	accessinfrastructure "github.com/ninggiangboy/send-flow/backend/internal/modules/access/infrastructure"
	accesspostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/access/infrastructure/postgres"
	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	audienceapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app"
	audiencepostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/infrastructure/postgres"
	auditapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/app"
	auditpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/infrastructure/postgres"
	campaignapp "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app"
	campaignpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/infrastructure/postgres"
	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	deliveryinfrastructure "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure"
	deliverypostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/postgres"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	identityoauth "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/oauth"
	identitypostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/postgres"
	identityredis "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/redis"
	identitytoken "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/token"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	ingestionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/app"
	ingestionpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/infrastructure/postgres"
	ingestionprovider "github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/infrastructure/provider"
	notificationapp "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app"
	operationsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/operations/app"
	operationspostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/operations/infrastructure/postgres"
	senderapp "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app"
	senderdns "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/infrastructure/dns"
	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
	trackingapp "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app"
	webhooksapp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
	platformemail "github.com/ninggiangboy/send-flow/backend/internal/platform/email"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/httpjson"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/logger"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/migration"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/objectstorage"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/postgres"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/ratelimit"
	platformredis "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/security"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/sse"
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

	if cfg.AutoMigrate {
		log.Info("running database migrations")
		if err := migration.Up(ctx, cfg.DatabaseURL, backend.MigrationFS()); err != nil {
			return err
		}
		log.Info("database migrations completed")
	}

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

	emailSender, err := platformemail.NewSender(ctx, cfg)
	if err != nil {
		return err
	}

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

	settingsRead := identitypostgres.NewSettingsReadRepository(pgClient.ReadPool())
	settingsWrite := identitypostgres.NewSettingsWriteRepository(pgClient.WritePool())

	auditReadRepo := auditpostgres.NewReadRepository(pgClient.ReadPool())
	auditWriteRepo := auditpostgres.NewWriteRepository(pgClient.WritePool())

	authSvc := identityapp.NewService(identityapp.Options{
		UsersRead:         identitypostgres.NewUserReadRepository(pgClient.ReadPool()),
		UsersWrite:        identitypostgres.NewUserWriteRepository(pgClient.WritePool()),
		ExternalsRead:     identitypostgres.NewExternalAccountReadRepository(pgClient.ReadPool()),
		ExternalsWrite:    identitypostgres.NewExternalAccountWriteRepository(pgClient.WritePool()),
		SessionsRead:      identitypostgres.NewSessionReadRepository(pgClient.ReadPool()),
		SessionsWrite:     identitypostgres.NewSessionWriteRepository(pgClient.WritePool()),
		Hasher:            security.NewPasswordHasher(0),
		Tokens:            identitytoken.NewJWTManager(cfg.JWTIssuer, cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL),
		OAuthState:        identityredis.NewOAuthStateStore(redisClient),
		RefreshStore:      identityredis.NewRefreshStore(redisClient),
		AuthTokens:        identitypostgres.NewAuthTokenRepository(pgClient.WritePool()),
		TOTP:              identitypostgres.NewTOTPRepository(pgClient.WritePool()),
		Providers:         []ports.OAuthProvider{identityoauth.NewGoogleProvider(cfg.OAuthGoogleClientID, cfg.OAuthGoogleSecret, nil), identityoauth.NewGithubProvider(cfg.OAuthGithubClientID, cfg.OAuthGithubSecret, nil)},
		OAuthStateTTL:     cfg.OAuthStateTTL,
		MailSender:        &mailerAdapter{sender: emailSender},
		RateLimiter:       &rateLimiterAdapter{svc: ratelimit.NewRedisService(redisClient)},
		IDGen:             &uuidIDGeneratorAdapter{gen: id.NewUUIDGenerator()},
		TokenGen:          &tokenGeneratorAdapter{},
		TokenHasher:       &tokenHasherAdapter{},
		PasswordValidator: &passwordValidatorAdapter{},
		TOTPVerifier:      &totpVerifierAdapter{},
		TOTPSecretGen:     &totpSecretGeneratorAdapter{},
		RecoveryCodeGen:   &recoveryCodeGeneratorAdapter{},
		FrontendBaseURL:   cfg.FrontendBaseURL,
		VerificationTTL:   cfg.EmailVerificationTTL,
		PasswordResetTTL:  cfg.PasswordResetTTL,
		MFAChallengeTTL:   cfg.MFAChallengeTTL,
		WorkspacesRead:    identitypostgres.NewWorkspaceReadRepository(pgClient.ReadPool()),
		WorkspacesWrite:   identitypostgres.NewWorkspaceWriteRepository(pgClient.WritePool()),
		RolesRead:         identitypostgres.NewRoleReadRepository(pgClient.ReadPool()),
		RolesWrite:        identitypostgres.NewRoleWriteRepository(pgClient.WritePool()),
		MembershipsRead:   identitypostgres.NewMembershipReadRepository(pgClient.ReadPool()),
		MembershipsWrite:  identitypostgres.NewMembershipWriteRepository(pgClient.WritePool()),
		InvitationsRead:   identitypostgres.NewInvitationReadRepository(pgClient.ReadPool()),
		InvitationsWrite:  identitypostgres.NewInvitationWriteRepository(pgClient.WritePool()),
		SettingsRead:      settingsRead,
		SettingsWrite:     settingsWrite,
		Logger:            log,
		UnitOfWork:        transaction.NewManager(pgClient.WritePool()),
		OutboxWriter:      identitypostgres.NewIdentityOutboxRepository(pgClient.WritePool()),
	})

	auditSvc := auditapp.NewService(auditapp.Options{
		EntriesRead:  auditReadRepo,
		EntriesWrite: auditWriteRepo,
		PermChecker:  newPermissionCheckerAdapter(authSvc),
		IDGen:        id.NewUUIDGenerator().New,
		Logger:       log,
	})

	authSvc.SetAuditRecorder(newAuditRecorderAdapter(auditSvc))

	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:           cfg.AppName,
		PostgresCheck:     pgClient.Ping,
		PostgresReadCheck: pgClient.PingRead,
		RedisCheck:        redisClient.Ping,
		KafkaEnabled:      cfg.KafkaEnabled(),
		ObjectStorageCheck: func(ctx context.Context) error {
			if objectStorageClient == nil {
				return nil
			}
			return objectStorageClient.Ping(ctx)
		},
		ObjectStorageEnabled: cfg.ObjectStorageEnabled(),
	})

	senderReadRepo, senderWriteRepo := shared.NewSenderRepos(pgClient.ReadPool(), pgClient.WritePool())
	senderResolver := senderdns.NewResolver()
	senderSvc := shared.NewSenderService(senderReadRepo, senderWriteRepo, senderResolver, newWorkspaceAccessAdapter(authSvc), log)

	audienceContactsRead := audiencepostgres.NewContactReadRepository(pgClient.ReadPool())
	audienceContactsWrite := audiencepostgres.NewContactWriteRepository(pgClient.WritePool())
	audienceListsRead := audiencepostgres.NewListReadRepository(pgClient.ReadPool())
	audienceListsWrite := audiencepostgres.NewListWriteRepository(pgClient.WritePool())
	audienceSegmentsRead := audiencepostgres.NewSegmentReadRepository(pgClient.ReadPool())
	audienceSegmentsWrite := audiencepostgres.NewSegmentWriteRepository(pgClient.WritePool())
	audienceImportJobsRead := audiencepostgres.NewImportJobReadRepository(pgClient.ReadPool())
	audienceImportJobsWrite := audiencepostgres.NewImportJobWriteRepository(pgClient.WritePool())
	audienceExportJobsRead := audiencepostgres.NewExportJobReadRepository(pgClient.ReadPool())
	audienceExportJobsWrite := audiencepostgres.NewExportJobWriteRepository(pgClient.WritePool())
	audienceSvc := audienceapp.NewService(audienceapp.Options{
		ContactsRead:    audienceContactsRead,
		ContactsWrite:   audienceContactsWrite,
		ListsRead:       audienceListsRead,
		ListsWrite:      audienceListsWrite,
		SegmentsRead:    audienceSegmentsRead,
		SegmentsWrite:   audienceSegmentsWrite,
		ImportJobsRead:  audienceImportJobsRead,
		ImportJobsWrite: audienceImportJobsWrite,
		ExportJobsRead:  audienceExportJobsRead,
		ExportJobsWrite: audienceExportJobsWrite,
		AccessChecker:   newWorkspaceAccessAdapter(authSvc),
		IDGen:           id.NewUUIDGenerator().New,
		Logger:          log,
	})

	contentReadRepo, contentWriteRepo := shared.NewContentRepos(pgClient.ReadPool(), pgClient.WritePool())
	contentSvc := shared.NewContentService(contentReadRepo, contentWriteRepo, newWorkspaceAccessAdapter(authSvc), log)

	suppressionReadRepo, suppressionWriteRepo := shared.NewSuppressionRepos(pgClient.ReadPool(), pgClient.WritePool())
	suppressionSvc := shared.NewSuppressionService(suppressionReadRepo, suppressionWriteRepo, newWorkspaceAccessAdapter(authSvc), log)

	campaignReadRepo := campaignpostgres.NewCampaignReadRepository(pgClient.ReadPool())
	campaignWriteRepo := campaignpostgres.NewCampaignWriteRepository(pgClient.WritePool())
	campaignOutboxRepo := campaignpostgres.NewOutboxRepository(pgClient.WritePool())
	campaignTxManager := transaction.NewManager(pgClient.WritePool())
	campaignSvc := campaignapp.NewService(campaignapp.Options{
		CampaignsRead:    campaignReadRepo,
		CampaignsWrite:   campaignWriteRepo,
		AudienceResolver: newAudienceResolverAdapter(audienceSvc),
		ContentService:   newContentServiceAdapter(contentSvc),
		SenderService:    newSenderServiceAdapter(senderSvc),
		OutboxWriter:     campaignOutboxRepo,
		TxManager:        campaignTxManager,
		AccessChecker:    newWorkspaceAccessAdapter(authSvc),
		IDGen:            id.NewUUIDGenerator().New,
		Logger:           log,
	})

	deliveryMsgReadRepo := deliverypostgres.NewMessageReadRepository(pgClient.ReadPool())
	deliveryMsgWriteRepo := deliverypostgres.NewMessageWriteRepository(pgClient.WritePool())
	deliveryTxReqReadRepo := deliverypostgres.NewTransactionalRequestReadRepository(pgClient.ReadPool())
	deliveryTxReqWriteRepo := deliverypostgres.NewTransactionalRequestWriteRepository(pgClient.WritePool())
	deliveryAttemptReadRepo := deliverypostgres.NewAttemptReadRepository(pgClient.ReadPool())
	deliveryAttemptWriteRepo := deliverypostgres.NewAttemptWriteRepository(pgClient.WritePool())
	deliveryRetryStateReadRepo := deliverypostgres.NewRetryStateReadRepository(pgClient.ReadPool())
	deliveryRetryStateWriteRepo := deliverypostgres.NewRetryStateWriteRepository(pgClient.WritePool())
	deliveryOutboxRepo := deliverypostgres.NewOutboxRepository(pgClient.WritePool())
	deliveryTxManager := transaction.NewManager(pgClient.WritePool())
	deliveryContentRenderer := newTransactionalContentRenderer(contentSvc)
	deliverySenderChecker := newTransactionalSenderChecker(senderSvc)
	deliverySuppressionChecker := newTransactionalSuppressionChecker(suppressionSvc)
	deliverySvc := deliveryapp.NewService(deliveryapp.Options{
		MessagesRead:       deliveryMsgReadRepo,
		MessagesWrite:      deliveryMsgWriteRepo,
		AttemptsRead:       deliveryAttemptReadRepo,
		AttemptsWrite:      deliveryAttemptWriteRepo,
		RetryStatesRead:    deliveryRetryStateReadRepo,
		RetryStatesWrite:   deliveryRetryStateWriteRepo,
		TxRequestsRead:     deliveryTxReqReadRepo,
		TxRequestsWrite:    deliveryTxReqWriteRepo,
		ContentRenderer:    deliveryContentRenderer,
		SenderChecker:      deliverySenderChecker,
		SuppressionChecker: deliverySuppressionChecker,
		OutboxWriter:       deliveryOutboxRepo,
		TxManager:          deliveryTxManager,
		AccessChecker:      newWorkspaceAccessAdapter(authSvc),
		Logger:             log,
		IDGen:              id.NewUUIDGenerator().New,
	})

	accessAPIKeyRepo := accesspostgres.NewAPIKeyRepository(pgClient.ReadPool(), pgClient.WritePool())
	accessSvc := accessapp.NewService(accessapp.Options{
		APIKeyRepo:    accessAPIKeyRepo,
		AccessChecker: newWorkspaceAccessAdapter(authSvc),
		IDGen:         id.NewUUIDGenerator().New,
		SecretGen:     accessinfrastructure.NewCryptoSecretGenerator(),
		SecretHasher:  accessinfrastructure.NewBcryptSecretHasher(),
		Logger:        log,
	})

	ingestionRawReadRepo := ingestionpostgres.NewRawEventRepository(pgClient.ReadPool())
	ingestionRawWriteRepo := ingestionpostgres.NewRawEventRepository(pgClient.WritePool())
	ingestionNormReadRepo := ingestionpostgres.NewNormalizedEventRepository(pgClient.ReadPool())
	ingestionNormWriteRepo := ingestionpostgres.NewNormalizedEventRepository(pgClient.WritePool())
	ingestionOutboxRepo := ingestionpostgres.NewOutboxRepository(pgClient.WritePool())
	ingestionTxManager := transaction.NewManager(pgClient.WritePool())
	ingestionMsgResolver := deliveryinfrastructure.NewIngestionMessageResolver(deliveryMsgReadRepo)

	ingestionReg := ingestionprovider.NewRegistry()
	sesVerifier := ingestionprovider.NewSESVerifier()
	sesNormalizer := &ingestionprovider.SESNormalizer{}
	ingestionReg.Register("ses", sesVerifier, sesNormalizer)
	if cfg.AppEnv == "local" || cfg.AppEnv == "test" {
		ingestionFakeVerifier := &ingestionprovider.FakeVerifier{Secret: cfg.FakeWebhookSecret}
		ingestionFakeNormalizer := &ingestionprovider.FakeNormalizer{}
		ingestionReg.Register("fake", ingestionFakeVerifier, ingestionFakeNormalizer)
	}

	trackingDeps := shared.NewTrackingRepos(pgClient.ReadPool(), pgClient.WritePool())
	trackingMessageResolver := shared.NewTrackingMessageResolverAdapter(deliveryinfrastructure.NewMessageResolver(deliveryMsgReadRepo))
	trackingSuppressor := shared.NewTrackingSuppressorAdapter(suppressionSvc)
	trackingSvc := shared.NewTrackingService(trackingDeps, trackingMessageResolver, trackingSuppressor, cfg.UnsubscribeTokenSecret, log)

	ingestionSvc := ingestionapp.NewService(ingestionapp.Options{
		RawEventsRead:         ingestionRawReadRepo,
		RawEventsWrite:        ingestionRawWriteRepo,
		NormalizedEventsRead:  ingestionNormReadRepo,
		NormalizedEventsWrite: ingestionNormWriteRepo,
		ProviderRegistry:      ingestionReg,
		MessageResolver:       ingestionMsgResolver,
		OutboxWriter:          ingestionOutboxRepo,
		TxManager:             ingestionTxManager,
		IDGen:                 id.NewUUIDGenerator().New,
		Logger:                log,
	})

	analyticsOpts, _ := shared.NewAnalyticsRepos(pgClient.WritePool())
	analyticsOpts.AccessChecker = newWorkspaceAccessAdapter(authSvc)
	analyticsOpts.Logger = log
	analyticsSvc := analyticsapp.NewService(analyticsOpts)

	webhooksSvc := shared.NewWebhooksService(pgClient.ReadPool(), pgClient.WritePool(), newWorkspaceAccessAdapter(authSvc), log)

	operationsOutboxRepo := operationspostgres.NewOutboxRepository(pgClient.WritePool())
	operationsDeadLetterRepo := operationspostgres.NewDeadLetterRepository(pgClient.ReadPool())
	operationsReplayJobRepo := operationspostgres.NewReplayJobRepository(pgClient.WritePool())
	operationsOutboxWriter := operationspostgres.NewOutboxWriterRepo(pgClient.WritePool())
	operationsTxManager := transaction.NewManager(pgClient.WritePool())

	operationsSvc := operationsapp.NewService(operationsapp.Options{
		OutboxRepo:     operationsOutboxRepo,
		DeadLetterRepo: operationsDeadLetterRepo,
		ReplayJobRepo:  operationsReplayJobRepo,
		OutboxWriter:   operationsOutboxWriter,
		TxManager:      operationsTxManager,
		AccessChecker:  newWorkspaceAccessAdapter(authSvc),
		IDGen:          id.NewUUIDGenerator().New,
		Logger:         log,
	})

	notificationOpts := shared.NewNotificationRepos(pgClient.ReadPool(), pgClient.WritePool(), emailSender)
	notificationOpts.AccessChecker = newWorkspaceAccessAdapter(authSvc)
	notificationOpts.Logger = log
	notificationSvc := notificationapp.NewService(notificationOpts)

	r := newRouter(&RouterDeps{
		HealthSvc:       healthSvc,
		AuthSvc:         authSvc,
		SenderSvc:       senderSvc,
		AudienceSvc:     audienceSvc,
		ContentSvc:      contentSvc,
		SuppressionSvc:  suppressionSvc,
		CampaignSvc:     campaignSvc,
		DeliverySvc:     deliverySvc,
		AccessSvc:       accessSvc,
		IngestionSvc:    ingestionSvc,
		TrackingSvc:     trackingSvc,
		AnalyticsSvc:    analyticsSvc,
		WebhooksSvc:     webhooksSvc,
		OperationsSvc:   operationsSvc,
		NotificationSvc: notificationSvc,
		SettingsSvc:     authSvc,
		AuditSvc:        auditSvc,
		AuthRateLimiter: ratelimit.NewRedisService(redisClient),
		SecureCookies:   cfg.SecureCookies(),
		FrontendBaseURL: cfg.FrontendBaseURL,
		HTTPMetrics:     httpMetrics,
		Log:             log,
	})

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: r,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("api server starting", "addr", cfg.HTTPAddr)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
	case serveErr := <-errCh:
		if serveErr != nil {
			return serveErr
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	log.Info("api server stopping")
	return server.Shutdown(shutdownCtx)
}

type RouterDeps struct {
	HealthSvc       *platformhealth.Service
	AuthSvc         *identityapp.Service
	SenderSvc       *senderapp.Service
	AudienceSvc     *audienceapp.Service
	ContentSvc      *contentapp.Service
	SuppressionSvc  *suppressionapp.Service
	CampaignSvc     *campaignapp.Service
	DeliverySvc     *deliveryapp.Service
	AccessSvc       *accessapp.Service
	IngestionSvc    *ingestionapp.Service
	TrackingSvc     *trackingapp.Service
	AnalyticsSvc    *analyticsapp.Service
	WebhooksSvc     *webhooksapp.Service
	OperationsSvc   *operationsapp.Service
	NotificationSvc *notificationapp.Service
	SettingsSvc     *identityapp.Service
	AuditSvc        *auditapp.Service
	AuthRateLimiter ratelimit.Service
	SecureCookies   bool
	FrontendBaseURL string
	HTTPMetrics     *observability.HTTPMetrics
	Log             *slog.Logger
}

func newRouter(deps *RouterDeps) http.Handler {
	r := chi.NewRouter()
	r.Use(corsMiddleware(corsOptions{
		AllowedOrigins: []string{deps.FrontendBaseURL},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqCtx := &requestLogContext{RequestID: requestID(r)}
			r = r.WithContext(context.WithValue(r.Context(), ctxRequestContext, reqCtx))
			start := time.Now()
			rec := &observability.ResponseRecorder{ResponseWriter: w, Status: http.StatusOK}
			next.ServeHTTP(rec, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}
			duration := time.Since(start)
			deps.HTTPMetrics.Record(r.Method, route, strconv.Itoa(rec.Status), duration.Seconds())
			if deps.Log != nil && shouldLogHTTPRequest(route) {
				attrs := []any{
					"request_id", reqCtx.RequestID,
					"method", r.Method,
					"route", route,
					"status", rec.Status,
					"duration_ms", duration.Milliseconds(),
				}
				if reqCtx.UserID != "" {
					attrs = append(attrs, "user_id", reqCtx.UserID)
				}
				if reqCtx.SessionID != "" {
					attrs = append(attrs, "session_id", reqCtx.SessionID)
				}
				deps.Log.Info("http request", attrs...)
			}
		})
	})
	humaAPI := humachi.New(r, openAPIConfig())
	registerOpenAPIRoutes(humaAPI, r, deps)

	r.Route("/api", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(authzMiddleware(deps.AuthSvc))
			r.Get("/events/stream", func(w http.ResponseWriter, req *http.Request) {
				stream, err := sse.New(w, req, sse.Options{HeartbeatInterval: sse.DefaultHeartbeatInterval})
				if err != nil {
					_ = httpjson.Write(w, http.StatusInternalServerError, map[string]string{"error": "streaming is not supported"})
					return
				}
				defer stream.Close()
				if err := stream.WriteEvent(sse.Event{Event: "connected", Data: "send-flow stream connected"}); err != nil {
					return
				}
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-req.Context().Done():
						return
					case ts := <-ticker.C:
						if err := stream.WriteEvent(sse.Event{Event: "tick", ID: fmt.Sprintf("%d", ts.Unix()), Data: ts.UTC().Format(time.RFC3339)}); err != nil {
							return
						}
					}
				}
			})
		})
		r.Handle("/metrics", promhttp.Handler())
	})
	if deps.TrackingSvc != nil {
		tracking := newTrackingHTTP(deps.TrackingSvc)
		r.Get("/o/{tracking_id}", tracking.serveOpenPixel)
		r.Get("/t/{tracking_id}", tracking.serveClickRedirect)
		r.Get("/u/{token}", tracking.serveUnsubscribe)
	}
	return r
}

func shouldLogHTTPRequest(route string) bool {
	switch route {
	case "/api/metrics", "/api/healthz", "/api/readyz":
		return false
	default:
		return true
	}
}

type corsOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
}

func corsMiddleware(opts corsOptions) func(http.Handler) http.Handler {
	origins := map[string]struct{}{
		"http://localhost:3000": {},
	}
	for _, origin := range opts.AllowedOrigins {
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}

	allowedMethods := strings.Join(opts.AllowedMethods, ", ")
	allowedHeaders := strings.Join(opts.AllowedHeaders, ", ")
	exposeHeaders := strings.Join(opts.ExposeHeaders, ", ")
	maxAge := strconv.Itoa(opts.MaxAge)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := origins[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				if opts.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if exposeHeaders != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposeHeaders)
				}
			}

			if r.Method == http.MethodOptions {
				if origin != "" {
					if allowedMethods != "" {
						w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
					}
					if allowedHeaders != "" {
						w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
					}
					if opts.MaxAge > 0 {
						w.Header().Set("Access-Control-Max-Age", maxAge)
					}
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
