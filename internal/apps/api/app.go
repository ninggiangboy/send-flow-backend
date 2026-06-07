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
	audienceapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app"
	audiencepostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/infrastructure/postgres"
	campaignapp "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app"
	campaignpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/infrastructure/postgres"
	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
	contentpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/content/infrastructure/postgres"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	deliverypostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/postgres"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	identityoauth "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/oauth"
	identitypostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/postgres"
	identityredis "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/redis"
	identitytoken "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/infrastructure/token"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	senderapp "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app"
	senderdns "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/infrastructure/dns"
	senderpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/infrastructure/postgres"
	suppressionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
	suppressionpostgres "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/infrastructure/postgres"
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
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Run(ctx context.Context) error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return err
	}

	log := logger.New(cfg)

	if cfg.AutoMigrate {
		log.Info("running database migrations")
		if err := migration.Up(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
		log.Info("database migrations completed")
	}

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

	emailSender, err := platformemail.NewSender(ctx, cfg)
	if err != nil {
		return err
	}

	var objectStorageClient *objectstorage.Client
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

	authSvc := identityapp.NewService(identityapp.Options{
		UsersRead:        identitypostgres.NewUserReadRepository(pgClient.ReadPool()),
		UsersWrite:       identitypostgres.NewUserWriteRepository(pgClient.WritePool()),
		ExternalsRead:    identitypostgres.NewExternalAccountReadRepository(pgClient.ReadPool()),
		ExternalsWrite:   identitypostgres.NewExternalAccountWriteRepository(pgClient.WritePool()),
		SessionsRead:     identitypostgres.NewSessionReadRepository(pgClient.ReadPool()),
		SessionsWrite:    identitypostgres.NewSessionWriteRepository(pgClient.WritePool()),
		Hasher:           security.NewPasswordHasher(0),
		Tokens:           identitytoken.NewJWTManager(cfg.JWTIssuer, cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL),
		OAuthState:       identityredis.NewOAuthStateStore(redisClient),
		RefreshStore:     identityredis.NewRefreshStore(redisClient),
		AuthTokens:       identitypostgres.NewAuthTokenRepository(pgClient.WritePool()),
		TOTP:             identitypostgres.NewTOTPRepository(pgClient.WritePool(), pgClient.WritePool()),
		Providers:        []ports.OAuthProvider{identityoauth.NewGoogleProvider(cfg.OAuthGoogleClientID, cfg.OAuthGoogleSecret), identityoauth.NewGithubProvider(cfg.OAuthGithubClientID, cfg.OAuthGithubSecret)},
		OAuthStateTTL:    cfg.OAuthStateTTL,
		MailSender:       emailSender,
		RateLimiter:      ratelimit.NewRedisService(redisClient),
		FrontendBaseURL:  cfg.FrontendBaseURL,
		VerificationTTL:  cfg.EmailVerificationTTL,
		PasswordResetTTL: cfg.PasswordResetTTL,
		MFAChallengeTTL:  cfg.MFAChallengeTTL,
		WorkspacesRead:   identitypostgres.NewWorkspaceReadRepository(pgClient.ReadPool()),
		WorkspacesWrite:  identitypostgres.NewWorkspaceWriteRepository(pgClient.WritePool()),
		RolesRead:        identitypostgres.NewRoleReadRepository(pgClient.ReadPool()),
		RolesWrite:       identitypostgres.NewRoleWriteRepository(pgClient.WritePool()),
		MembershipsRead:  identitypostgres.NewMembershipReadRepository(pgClient.ReadPool()),
		MembershipsWrite: identitypostgres.NewMembershipWriteRepository(pgClient.WritePool()),
		InvitationsRead:  identitypostgres.NewInvitationReadRepository(pgClient.ReadPool()),
		InvitationsWrite: identitypostgres.NewInvitationWriteRepository(pgClient.WritePool()),
		Logger:           log,
		UnitOfWork:       identitypostgres.NewUnitOfWork(pgClient.WritePool()),
	})

	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:           cfg.AppName,
		PostgresCheck:     pgClient.Ping,
		PostgresReadCheck: pgClient.PingRead,
		RedisCheck:        redisClient.Ping,
		KafkaEnabled:      cfg.KafkaEnabled(),
		ClickEnabled:      cfg.ClickHouseEnabled(),
		ObjectStorageCheck: func(ctx context.Context) error {
			if objectStorageClient == nil {
				return nil
			}
			return objectStorageClient.Ping(ctx)
		},
		ObjectStorageEnabled: cfg.ObjectStorageEnabled(),
	})

	senderDomainsRead := senderpostgres.NewReadRepository(pgClient.ReadPool())
	senderDomainsWrite := senderpostgres.NewWriteRepository(pgClient.WritePool())
	senderResolver := senderdns.NewResolver()
	senderSvc := senderapp.NewService(senderapp.Options{
		DomainsRead:   senderDomainsRead,
		DomainsWrite:  senderDomainsWrite,
		DNSResolver:   senderResolver,
		AccessChecker: newWorkspaceAccessAdapter(authSvc),
		IDGen:         id.NewUUIDGenerator().New,
		Logger:        log,
	})

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

	contentTemplatesRead := contentpostgres.NewTemplateReadRepository(pgClient.ReadPool())
	contentTemplatesWrite := contentpostgres.NewTemplateWriteRepository(pgClient.WritePool())
	contentSvc := contentapp.NewService(contentapp.Options{
		TemplatesRead:  contentTemplatesRead,
		TemplatesWrite: contentTemplatesWrite,
		AccessChecker:  newWorkspaceAccessAdapter(authSvc),
		IDGen:          id.NewUUIDGenerator().New,
		Logger:         log,
	})

	suppressionEntriesRead := suppressionpostgres.NewReadRepository(pgClient.ReadPool())
	suppressionEntriesWrite := suppressionpostgres.NewWriteRepository(pgClient.WritePool())
	suppressionSvc := suppressionapp.NewService(suppressionapp.Options{
		EntriesRead:   suppressionEntriesRead,
		EntriesWrite:  suppressionEntriesWrite,
		AccessChecker: newWorkspaceAccessAdapter(authSvc),
		IDGen:         id.NewUUIDGenerator().New,
		Logger:        log,
	})

	campaignReadRepo := campaignpostgres.NewCampaignReadRepository(pgClient.ReadPool())
	campaignWriteRepo := campaignpostgres.NewCampaignWriteRepository(pgClient.WritePool())
	campaignOutboxRepo := campaignpostgres.NewOutboxRepository(pgClient.WritePool())
	campaignTxManager := campaignpostgres.NewTransactionManager(pgClient.WritePool())
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
	deliverySvc := deliveryapp.NewService(deliveryapp.Options{
		MessagesRead:  deliveryMsgReadRepo,
		MessagesWrite: deliverypostgres.NewMessageWriteRepository(pgClient.WritePool()),
		AccessChecker: newWorkspaceAccessAdapter(authSvc),
		Logger:        log,
		IDGen:         id.NewUUIDGenerator().New,
	})

	r := newRouter(healthSvc, authSvc, senderSvc, audienceSvc, contentSvc, suppressionSvc, campaignSvc, deliverySvc, ratelimit.NewRedisService(redisClient), cfg.SecureCookies(), cfg.FrontendBaseURL, httpMetrics, log)

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

func newRouter(healthSvc *platformhealth.Service, authSvc *identityapp.Service, senderSvc *senderapp.Service, audienceSvc *audienceapp.Service, contentSvc *contentapp.Service, suppressionSvc *suppressionapp.Service, campaignSvc *campaignapp.Service, deliverySvc *deliveryapp.Service, authRateLimiter ratelimit.Service, secureCookies bool, frontendBaseURL string, httpMetrics *observability.HTTPMetrics, log *slog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(corsMiddleware(corsOptions{
		AllowedOrigins: []string{frontendBaseURL},
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
			httpMetrics.Record(r.Method, route, strconv.Itoa(rec.Status), duration.Seconds())
			if log != nil && shouldLogHTTPRequest(route) {
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
				log.Info("http request", attrs...)
			}
		})
	})
	humaAPI := humachi.New(r, openAPIConfig())
	registerOpenAPIRoutes(humaAPI, r, healthSvc, authSvc, authRateLimiter, secureCookies, senderSvc, audienceSvc, contentSvc, suppressionSvc, campaignSvc, deliverySvc)

	r.Route("/api", func(r chi.Router) {
		r.Get("/events/stream", func(w http.ResponseWriter, req *http.Request) {
			stream, err := sse.New(w, req, sse.Options{HeartbeatInterval: sse.DefaultHeartbeatInterval})
			if err != nil {
				httpjson.Write(w, http.StatusInternalServerError, map[string]string{"error": "streaming is not supported"})
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
		r.Handle("/metrics", promhttp.Handler())
	})
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
