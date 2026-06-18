package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	accessapp "github.com/ninggiangboy/send-flow/backend/internal/modules/access/app"
	accessdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/httpjson"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/httputil"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/ratelimit"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/sse"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type humaContextKey string

const (
	ctxHumaRequest        humaContextKey = "huma_request"
	ctxHumaResponseWriter humaContextKey = "huma_response_writer"
)

func openAPIConfig() huma.Config {
	cfg := huma.DefaultConfig("Sendflow API", "1.0.0")
	cfg.Info.Description = "Sendflow API provides workspace management, authentication, email delivery (campaign and transactional), workspace mail-log inspection, API key management, and webhook ingestion."
	cfg.OpenAPIPath = ""
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	cfg.CreateHooks = nil
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"bearerAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
			Description:  "Access token issued by the authentication API.",
		},
		"apiKeyAuth": {
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "API key",
			Description:  "API key for service-to-service requests.",
		},
	}
	return cfg
}

func registerOpenAPIRoutes(api huma.API, r chi.Router, deps *RouterDeps) {
	api.UseMiddleware(captureHTTPContext)

	r.Get("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/openapi+json")
		if err := json.NewEncoder(w).Encode(api.OpenAPI()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	registerHealthOperations(api, deps.HealthSvc)

	var authMiddleware func(huma.Context, func(huma.Context))
	var auditRecorder identityapp.AuditRecorder
	if deps.AuditSvc != nil {
		auditRecorder = newAuditRecorderAdapter(deps.AuditSvc)
	}

	if deps.AuthSvc != nil {
		auth := newAuthHTTP(deps.AuthSvc, deps.AuthRateLimiter, deps.SecureCookies, auditRecorder, deps.AuthMetrics)
		workspace := newWorkspaceHTTP(deps.AuthSvc, auditRecorder)
		authMiddleware = humaAuthzMiddleware(deps.AuthSvc)

		registerAuthOperations(api, auth, authMiddleware)
		registerWorkspaceOperations(api, workspace, authMiddleware)
		if deps.SenderSvc != nil {
			sender := newSenderHTTP(deps.SenderSvc, auditRecorder)
			registerSenderOperations(api, sender, authMiddleware)
		}
	}
	if deps.AudienceSvc != nil {
		audience := newAudienceHTTP(deps.AudienceSvc)
		registerAudienceOperations(api, audience, authMiddleware)
	}
	if deps.ContentSvc != nil {
		content := newContentHTTP(deps.ContentSvc)
		registerContentOperations(api, content, authMiddleware)
	}
	if deps.SuppressionSvc != nil {
		suppression := newSuppressionHTTP(deps.SuppressionSvc)
		registerSuppressionOperations(api, suppression, authMiddleware)
	}
	if deps.CampaignSvc != nil {
		campaign := newCampaignHTTP(deps.CampaignSvc)
		registerCampaignOperations(api, campaign, authMiddleware)
	}
	if deps.DeliverySvc != nil {
		delivery := newDeliveryHTTP(deps.DeliverySvc)
		registerDeliveryOperations(api, delivery, authMiddleware)

		transactional := newTransactionalHTTP(deps.DeliverySvc, auditRecorder)
		registerTransactionalOperations(api, transactional, deps.AccessSvc, deps.APIKeyMetrics, deps.AuthRateLimiter)
		registerTransactionalMailLogsOperations(api, transactional, deps.AccessSvc, deps.APIKeyMetrics, deps.AuthRateLimiter)

		mailLogs := newMailLogsHTTP(deps.DeliverySvc)
		registerMailLogsOperations(api, mailLogs, authMiddleware, deps.AccessSvc, deps.APIKeyMetrics, deps.AuthRateLimiter)
	}
	if deps.IngestionSvc != nil {
		ingestion := newIngestionHTTP(deps.IngestionSvc)
		registerIngestionWebhookOperations(api, ingestion)
	}
	if deps.AccessSvc != nil {
		apiKeyHandler := newAPIKeyHTTP(deps.AccessSvc, auditRecorder)
		registerAPIKeyOperations(api, apiKeyHandler, authMiddleware)
	}
	if deps.TrackingSvc != nil {
		tracking := newTrackingHTTP(deps.TrackingSvc)
		registerTrackingOperations(api, tracking)
	}
	if deps.AnalyticsSvc != nil {
		analytics := newAnalyticsHTTP(deps.AnalyticsSvc)
		registerAnalyticsOperations(api, analytics, authMiddleware)
	}
	if deps.WebhooksSvc != nil {
		webhookConfig := newWebhookHTTP(deps.WebhooksSvc, auditRecorder)
		registerWebhookConfigOperations(api, webhookConfig, authMiddleware)
		webhookDelivery := newWebhookDeliveryHTTP(deps.WebhooksSvc)
		registerWebhookDeliveryOperations(api, webhookDelivery, authMiddleware)
	}
	if deps.NotificationSvc != nil {
		notification := newNotificationHTTP(deps.NotificationSvc, deps.NotificationRealtime, newWorkspaceAccessAdapter(deps.AuthSvc).RequirePermission, deps.Log)
		registerNotificationOperations(api, notification, authMiddleware)
	}
	if deps.OperationsSvc != nil {
		operations := newOperationsHTTP(deps.OperationsSvc)
		registerOperationsRoutes(api, operations, authMiddleware)
	}
	if deps.AuthSvc != nil && deps.AuditSvc != nil {
		settings := newSettingsHTTP(deps.AuthSvc)
		audit := newAuditHTTP(deps.AuditSvc)
		registerSettingsOperations(api, settings, authMiddleware)
		registerAuditOperations(api, audit, authMiddleware)
	}
	registerOperationalRoutes(api, authMiddleware)
	documentApplicationErrors(api.OpenAPI())
}

func registerOperationalRoutes(api huma.API, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "events-stream",
		Method:      http.MethodGet,
		Path:        "/api/events/stream",
		Tags:        []string{"Operations"},
		Summary:     "Stream server-sent events",
		Description: "Establishes an SSE connection for receiving real-time server events. Requires authentication.",
	}, authMiddleware), func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		return delegateHTTP[struct{}](ctx, nil, func(w http.ResponseWriter, req *http.Request) {
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

	huma.Register(api, huma.Operation{
		OperationID: "get-metrics",
		Method:      http.MethodGet,
		Path:        "/api/metrics",
		Tags:        []string{"Operations"},
		Summary:     "Get Prometheus metrics",
		Description: "Returns Prometheus-formatted metrics for monitoring and alerting.",
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		return delegateHTTP[struct{}](ctx, nil, promhttp.Handler().ServeHTTP)
	})

	fixOperationalRouteResponses(api.OpenAPI())
}

func fixOperationalRouteResponses(oapi *huma.OpenAPI) {
	if oapi == nil {
		return
	}
	type fix struct {
		statusCode  string
		description string
		contentType string
	}
	paths := map[string]fix{
		"/api/events/stream": {"200", "SSE stream established", "text/event-stream"},
		"/api/metrics":       {"200", "Prometheus metrics", "text/plain"},
	}
	for pathStr, info := range paths {
		item, ok := oapi.Paths[pathStr]
		if !ok || item.Get == nil {
			continue
		}
		delete(item.Get.Responses, "204")
		item.Get.Responses[info.statusCode] = &huma.Response{
			Description: info.description,
			Content:     map[string]*huma.MediaType{info.contentType: {}},
		}
	}
}

func captureHTTPContext(ctx huma.Context, next func(huma.Context)) {
	req, w := humachi.Unwrap(ctx)
	base := ctx.Context()
	base = context.WithValue(base, ctxHumaRequest, req)
	base = context.WithValue(base, ctxHumaResponseWriter, w)
	next(huma.WithContext(ctx, base))
}

func humaAuthzMiddleware(svc *identityapp.Service) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		req, w := humachi.Unwrap(ctx)
		h := strings.TrimSpace(req.Header.Get("Authorization"))
		if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
			writeError(w, req, http.StatusUnauthorized, "auth.invalid_token", "missing bearer token", nil)
			return
		}
		token := strings.TrimSpace(h[len("Bearer "):])
		sess, user, err := svc.AuthenticateAccessToken(ctx.Context(), token)
		if err != nil {
			writeError(w, req, http.StatusUnauthorized, "auth.invalid_token", "invalid token", nil)
			return
		}
		if reqCtx, ok := ctx.Context().Value(ctxRequestContext).(*requestLogContext); ok {
			reqCtx.UserID = user.ID
			reqCtx.SessionID = sess.ID
		}
		next(huma.WithValue(huma.WithValue(ctx, ctxUserID, user.ID), ctxSessionID, sess.ID))
	}
}

// The delegateHTTP pattern bridges Huma's typed request/response cycle with existing
// http.HandlerFunc-style handlers. This provides OpenAPI documentation via Huma while
// reusing existing handler logic. The pattern is worth keeping for now because it:
//  1. Preserves backward compatibility with existing handlers
//  2. Provides automatic OpenAPI spec generation
//  3. Allows incremental migration to Huma's typed pattern
//
// Trade-off: double JSON (de)serialization (Huma decodes, then re-encodes for the handler).
// Consider migrating to Huma's native typed handlers when refactoring individual endpoints.
func delegateHTTP[O any](ctx context.Context, rawBody []byte, handler http.HandlerFunc) (*O, error) {
	req, _ := ctx.Value(ctxHumaRequest).(*http.Request)
	w, _ := ctx.Value(ctxHumaResponseWriter).(http.ResponseWriter)
	if req == nil || w == nil {
		return nil, huma.Error500InternalServerError("request context unavailable")
	}
	req = req.WithContext(ctx)
	if rawBody != nil {
		req.Body = io.NopCloser(bytes.NewReader(rawBody))
		req.ContentLength = int64(len(rawBody))
	}
	handler(w, req)
	return nil, nil
}

func jsonBody(body any) []byte {
	data, err := json.Marshal(body)
	if err != nil {
		return nil
	}
	return data
}

func documentedErrorStatuses() []int {
	return []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusConflict,
		http.StatusGone,
		http.StatusUnprocessableEntity,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	}
}

func protectedOperation(op huma.Operation, middleware func(huma.Context, func(huma.Context))) huma.Operation {
	op.Security = []map[string][]string{{"bearerAuth": {}}}
	if middleware != nil {
		op.Middlewares = append(op.Middlewares, middleware)
	}
	return op
}

func documentApplicationErrors(oapi *huma.OpenAPI) {
	if oapi == nil || oapi.Components == nil {
		return
	}
	errorSchema := huma.SchemaFromType(oapi.Components.Schemas, reflect.TypeFor[errorEnvelopeDoc]())
	for _, item := range oapi.Paths {
		if item == nil {
			continue
		}
		for _, op := range []*huma.Operation{item.Get, item.Put, item.Post, item.Delete, item.Options, item.Head, item.Patch, item.Trace} {
			if op == nil {
				continue
			}
			for status, codes := range operationErrorCodes(op) {
				if len(codes) == 0 {
					continue
				}
				statusKey := strconv.Itoa(status)
				resp := op.Responses[statusKey]
				if resp == nil {
					resp = &huma.Response{Description: http.StatusText(status)}
					op.Responses[statusKey] = resp
				}
				resp.Description = http.StatusText(status) + ". Possible error.code values: " + strings.Join(codes, ", ") + "."
				if resp.Content == nil {
					resp.Content = map[string]*huma.MediaType{}
				}
				resp.Content["application/json"] = &huma.MediaType{
					Schema:   errorSchema,
					Examples: errorExamples(status, codes),
				}
			}
		}
	}
}

func operationErrorCodes(op *huma.Operation) map[int][]string {
	switch {
	case hasTag(op, "Auth"):
		return authErrorCodes(op)
	case hasTag(op, "Settings"):
		return settingsErrorCodes()
	case hasTag(op, "Audit"):
		return auditErrorCodes()
	case hasTag(op, "Workspaces"):
		return workspaceErrorCodes()
	case hasTag(op, "Sender Domains"):
		return senderErrorCodes(op)
	case hasTag(op, "Audience"):
		return audienceErrorCodes()
	case hasTag(op, "Templates") || hasTag(op, "Template Render"):
		return templateErrorCodes()
	case hasTag(op, "Suppression"):
		return suppressionErrorCodes()
	case hasTag(op, "Campaigns"):
		return campaignErrorCodes()
	case hasTag(op, "Delivery"):
		return deliveryErrorCodes()
	case hasTag(op, "Access"):
		return apiKeyErrorCodes()
	case hasTag(op, "Webhooks"):
		return webhookErrorCodes()
	case hasTag(op, "Tracking"):
		return trackingErrorCodes()
	case hasTag(op, "Analytics"):
		return analyticsErrorCodes()
	case hasTag(op, "Notifications"):
		return notificationErrorCodes()
	case hasTag(op, "Operations"):
		return operationsErrorCodes()
	default:
		return map[int][]string{
			http.StatusInternalServerError: {"internal.error"},
		}
	}
}

func senderErrorCodes(_ *huma.Operation) map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
		},
		http.StatusUnsupportedMediaType: {
			"delivery.request_body_invalid",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"sender.manage_denied",
		},
		http.StatusNotFound: {
			"sender.domain_not_found",
		},
		http.StatusConflict: {
			"sender.domain_conflict",
			"sender.invalid_state_transition",
		},
		http.StatusUnprocessableEntity: {
			"sender.domain_invalid",
			"sender.provider_config_invalid",
		},
		http.StatusServiceUnavailable: {
			"notification.realtime_unavailable",
		},
		http.StatusInternalServerError: {
			"internal.error",
			"delivery.attachment_storage_failed",
		},
	}
}

func authErrorCodes(op *huma.Operation) map[int][]string {
	codes := map[int][]string{
		http.StatusUnauthorized: {
			"auth.invalid_token",
			"auth.invalid_credentials",
			"auth.oauth_state_invalid",
			"auth.verification_token_invalid",
			"auth.reset_token_invalid",
			"auth.mfa_required",
			"auth.mfa_invalid_code",
		},
		http.StatusNotFound: {
			"auth.provider_not_supported",
		},
		http.StatusConflict: {
			"identity.email_already_registered",
		},
		http.StatusUnprocessableEntity: {
			"auth.password_policy_violation",
		},
		http.StatusTooManyRequests: {
			"auth.rate_limited",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
	if op.Method == http.MethodPost || op.Method == http.MethodPut || op.Method == http.MethodPatch {
		codes[http.StatusBadRequest] = []string{"auth.invalid_request_body"}
	}
	if isProtectedOperation(op) {
		codes[http.StatusForbidden] = []string{"auth.session_not_owned"}
	}
	return codes
}

func workspaceErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"identity.workspace_access_denied",
			"identity.membership_manage_denied",
			"identity.role_manage_denied",
		},
		http.StatusNotFound: {
			"identity.workspace_not_found",
			"identity.membership_not_found",
			"identity.invitation_not_found",
			"identity.role_not_found",
		},
		http.StatusConflict: {
			"identity.invitation_already_accepted",
			"identity.last_owner_cannot_be_removed",
			"identity.role_name_conflict",
			"identity.role_assignment_conflict",
			"identity.workspace_name_conflict",
		},
		http.StatusGone: {
			"identity.invitation_token_expired",
		},
		http.StatusUnprocessableEntity: {
			"identity.invitation_payload_invalid",
			"identity.permission_set_invalid",
			"identity.permission_registry_unknown",
			"identity.invalid_permission_mask",
			"identity.invalid_workspace_name",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

func hasTag(op *huma.Operation, tag string) bool {
	for _, candidate := range op.Tags {
		if candidate == tag {
			return true
		}
	}
	return false
}

func isProtectedOperation(op *huma.Operation) bool {
	return len(op.Security) > 0
}

func errorExamples(status int, codes []string) map[string]*huma.Example {
	examples := make(map[string]*huma.Example, len(codes))
	for _, code := range codes {
		examples[exampleKey(code)] = &huma.Example{
			Summary: code,
			Value: errorEnvelopeDoc{
				Error: errorBodyDoc{
					Code:    code,
					Message: http.StatusText(status),
				},
				Meta: requestMetaDoc{RequestID: "018ff2d5-f49c-77f1-a3c5-5137560c97c8"},
			},
		}
	}
	return examples
}

func exampleKey(code string) string {
	return strings.NewReplacer(".", "_", "-", "_").Replace(code)
}

type emptyOutput struct{}

type requestMetaDoc struct {
	RequestID string `json:"request_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Request correlation ID."`
}

type successEnvelopeDoc[T any] struct {
	Data T              `json:"data" doc:"Response payload."`
	Meta requestMetaDoc `json:"meta" doc:"Response metadata."`
}

type errorEnvelopeDoc struct {
	Error errorBodyDoc   `json:"error" doc:"Error payload."`
	Meta  requestMetaDoc `json:"meta" doc:"Response metadata."`
}

type errorBodyDoc struct {
	Code    string         `json:"code,omitempty" example:"auth.invalid_token" doc:"Stable application error code."`
	Message string         `json:"message" example:"missing bearer token" doc:"Human-readable error message."`
	Details map[string]any `json:"details,omitempty" doc:"Optional structured error details."`
}

type healthLiveDoc struct {
	Status    string    `json:"status" example:"ok" doc:"Liveness status."`
	App       string    `json:"app" example:"sendflow" doc:"Application name."`
	Timestamp time.Time `json:"timestamp" doc:"Server timestamp."`
}

type healthReadyDoc struct {
	Status       string            `json:"status" example:"ok" doc:"Readiness status."`
	App          string            `json:"app" example:"sendflow" doc:"Application name."`
	Timestamp    time.Time         `json:"timestamp" doc:"Server timestamp."`
	Dependencies map[string]string `json:"dependencies" doc:"Dependency readiness by name."`
}

type healthLiveOutput struct {
	Body healthLiveDoc
}

type healthReadyOutput struct {
	Body healthReadyDoc
}

func registerHealthOperations(api huma.API, healthSvc *platformhealth.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-healthz",
		Method:      http.MethodGet,
		Path:        "/api/healthz",
		Tags:        []string{"Health"},
		Summary:     "Get liveness status",
		Errors:      documentedErrorStatuses(),
	}, func(ctx context.Context, _ *struct{}) (*healthLiveOutput, error) {
		_ = healthSvc
		return delegateHTTP[healthLiveOutput](ctx, nil, func(w http.ResponseWriter, _ *http.Request) {
			// The existing health handler is small enough to keep here while preserving the same response writer path.
			req, _ := ctx.Value(ctxHumaRequest).(*http.Request)
			writeJSONDirect(w, http.StatusOK, healthSvc.Live(), req)
		})
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-readyz",
		Method:      http.MethodGet,
		Path:        "/api/readyz",
		Tags:        []string{"Health"},
		Summary:     "Get readiness status",
		Errors:      documentedErrorStatuses(),
	}, func(ctx context.Context, _ *struct{}) (*healthReadyOutput, error) {
		return delegateHTTP[healthReadyOutput](ctx, nil, func(w http.ResponseWriter, req *http.Request) {
			ready := healthSvc.Ready(req.Context())
			status := http.StatusOK
			if readinessStatus(ready) != "ok" {
				status = http.StatusServiceUnavailable
			}
			writeJSONDirect(w, status, ready, req)
		})
	})
}

func writeJSONDirect(w http.ResponseWriter, status int, body any, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func readinessStatus(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	var out struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(data, &out)
	return out.Status
}

type authCredentialsBody struct {
	Email    string `json:"email,omitempty" format:"email" example:"owner@example.com" doc:"User email address."`
	Password string `json:"password,omitempty" minLength:"8" example:"StrongPassword123!" doc:"User password."`
}

type signupInput struct {
	Body authCredentialsBody `required:"true" nameHint:"SignupRequest"`
}

type loginInput struct {
	Body authCredentialsBody `required:"true" nameHint:"LoginRequest"`
}

type loginMFAInput struct {
	Body struct {
		ChallengeToken string `json:"mfa_challenge_token,omitempty" example:"mfa-challenge-token" doc:"MFA challenge token returned by login."`
		Code           string `json:"code,omitempty" example:"123456" doc:"TOTP code."`
		RecoveryCode   string `json:"recovery_code,omitempty" example:"abcd-efgh" doc:"Recovery code alternative to TOTP."`
	} `required:"true" nameHint:"MFALoginRequest"`
}

type oauthProviderInput struct {
	Provider string `path:"provider" example:"google" doc:"OAuth provider name."`
}

type oauthStartInput struct {
	Provider string `path:"provider" example:"google" doc:"OAuth provider name."`
	Body     struct {
		RedirectURI   string `json:"redirect_uri,omitempty" format:"uri" example:"http://localhost:3000/auth/callback" doc:"Frontend callback URI."`
		Intent        string `json:"intent,omitempty" enum:"login,signup" example:"login" doc:"OAuth intent."`
		CodeChallenge string `json:"code_challenge,omitempty" example:"pkce-code-challenge" doc:"PKCE code challenge."`
		CodeVerifier  string `json:"code_verifier,omitempty" example:"pkce-code-verifier" doc:"PKCE verifier for clients that send it at start."`
	} `required:"true" nameHint:"OAuthStartRequest"`
}

type oauthExchangeInput struct {
	Provider string `path:"provider" example:"google" doc:"OAuth provider name."`
	Body     struct {
		Code         string `json:"code,omitempty" example:"oauth-code" doc:"OAuth authorization code."`
		State        string `json:"state,omitempty" example:"oauth-state" doc:"OAuth state token."`
		RedirectURI  string `json:"redirect_uri,omitempty" format:"uri" example:"http://localhost:3000/auth/callback" doc:"Frontend callback URI."`
		CodeVerifier string `json:"code_verifier,omitempty" example:"pkce-code-verifier" doc:"PKCE verifier."`
	} `required:"true" nameHint:"OAuthExchangeRequest"`
}

type tokenInput struct {
	Body struct {
		Token string `json:"token,omitempty" example:"verification-token" doc:"One-time token."`
	} `required:"true" nameHint:"TokenRequest"`
}

type forgotPasswordInput struct {
	Body struct {
		Email string `json:"email,omitempty" format:"email" example:"owner@example.com" doc:"User email address."`
	} `required:"true" nameHint:"ForgotPasswordRequest"`
}

type resetPasswordInput struct {
	Body struct {
		Token       string `json:"token,omitempty" example:"reset-token" doc:"Password reset token."`
		NewPassword string `json:"new_password,omitempty" minLength:"8" example:"NewStrongPassword123!" doc:"New password."`
	} `required:"true" nameHint:"ResetPasswordRequest"`
}

type codeInput struct {
	Body struct {
		Code string `json:"code,omitempty" example:"123456" doc:"TOTP code."`
	} `required:"true" nameHint:"CodeRequest"`
}

type mfaDisableInput struct {
	Body struct {
		Password string `json:"password,omitempty" example:"StrongPassword123!" doc:"Current password."`
		Code     string `json:"code,omitempty" example:"123456" doc:"TOTP code."`
	} `required:"true" nameHint:"MFADisableRequest"`
}

type sessionPathInput struct {
	SessionID string `path:"session_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Session ID."`
}

type boolResponseDoc struct {
	Revoked  *bool `json:"revoked,omitempty" example:"true" doc:"Session revoke result."`
	Sent     *bool `json:"sent,omitempty" example:"true" doc:"Email send result."`
	Verified *bool `json:"verified,omitempty" example:"true" doc:"Email verification result."`
	Reset    *bool `json:"reset,omitempty" example:"true" doc:"Password reset result."`
	Disabled *bool `json:"disabled,omitempty" example:"true" doc:"MFA disable result."`
}

type userDoc struct {
	ID            string `json:"id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"User ID."`
	Email         string `json:"email,omitempty" format:"email" example:"owner@example.com" doc:"User email address."`
	EmailVerified bool   `json:"email_verified" example:"true" doc:"Whether email is verified."`
	MFAEnabled    bool   `json:"mfa_enabled,omitempty" example:"false" doc:"Whether TOTP MFA is enabled."`
}

type sessionDoc struct {
	ID         string    `json:"id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Session ID."`
	CreatedAt  time.Time `json:"created_at" doc:"Session creation timestamp."`
	ExpiresAt  time.Time `json:"expires_at" doc:"Session expiration timestamp."`
	IPAddress  string    `json:"ip_address" example:"127.0.0.1" doc:"Client IP address."`
	AuthMethod string    `json:"auth_method" example:"password" doc:"Authentication method."`
}

type authSessionDoc struct {
	User        userDoc    `json:"user" doc:"Authenticated user."`
	Session     sessionDoc `json:"session" doc:"Authenticated session."`
	AccessToken string     `json:"access_token" example:"eyJhbGciOi..." doc:"Bearer access token."`
	TokenType   string     `json:"token_type" example:"Bearer" doc:"Access token type."`
	ExpiresAt   time.Time  `json:"expires_at" doc:"Access token expiration timestamp."`
}

type authSessionOutput struct {
	Body successEnvelopeDoc[authSessionDoc]
}

type authProvidersOutput struct {
	Body successEnvelopeDoc[[]providerDoc]
}

type providerDoc struct {
	Provider    string `json:"provider" example:"google" doc:"Provider identifier."`
	Type        string `json:"type" example:"oauth2" doc:"Provider type."`
	DisplayName string `json:"display_name" example:"Google" doc:"Display name."`
	Enabled     bool   `json:"enabled" example:"true" doc:"Whether provider is enabled."`
}

type boolOutput struct {
	Body successEnvelopeDoc[boolResponseDoc]
}

type meOutput struct {
	Body successEnvelopeDoc[userDoc]
}

type sessionsOutput struct {
	Body successEnvelopeDoc[[]sessionDoc]
}

type oauthStartOutput struct {
	Body successEnvelopeDoc[oauthStartDoc]
}

type oauthStartDoc struct {
	Provider            string `json:"provider" example:"google" doc:"Provider identifier."`
	AuthorizationURL    string `json:"authorization_url" format:"uri" example:"https://accounts.google.com/o/oauth2/v2/auth" doc:"Provider authorization URL."`
	State               string `json:"state,omitempty" example:"oauth-state" doc:"OAuth state token."`
	CodeChallengeMethod string `json:"code_challenge_method,omitempty" example:"S256" doc:"PKCE challenge method."`
}

type mfaSetupOutput struct {
	Body successEnvelopeDoc[mfaSetupDoc]
}

type mfaSetupDoc struct {
	Secret     string `json:"secret" example:"JBSWY3DPEHPK3PXP" doc:"TOTP secret."`
	OTPAuthURL string `json:"otpauth_url" example:"otpauth://totp/sendflow:owner@example.com" doc:"Authenticator app URL."`
}

type mfaRecoveryOutput struct {
	Body successEnvelopeDoc[mfaRecoveryDoc]
}

type mfaRecoveryDoc struct {
	RecoveryCodes []string `json:"recovery_codes" example:"recovery-code-1" doc:"One-time recovery codes."`
}

func registerAuthOperations(api huma.API, auth *authHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, huma.Operation{OperationID: "signup", Method: http.MethodPost, Path: "/api/v1/auth/signup", Tags: []string{"Auth"}, Summary: "Create user account", DefaultStatus: http.StatusCreated, Errors: documentedErrorStatuses()}, func(ctx context.Context, input *signupInput) (*authSessionOutput, error) {
		return delegateHTTP[authSessionOutput](ctx, jsonBody(input.Body), auth.signup)
	})
	huma.Register(api, huma.Operation{OperationID: "login", Method: http.MethodPost, Path: "/api/v1/auth/login", Tags: []string{"Auth"}, Summary: "Login with email and password", Errors: documentedErrorStatuses()}, func(ctx context.Context, input *loginInput) (*authSessionOutput, error) {
		return delegateHTTP[authSessionOutput](ctx, jsonBody(input.Body), auth.login)
	})
	huma.Register(api, huma.Operation{OperationID: "login-mfa", Method: http.MethodPost, Path: "/api/v1/auth/login/mfa", Tags: []string{"Auth"}, Summary: "Complete MFA login", Errors: documentedErrorStatuses()}, func(ctx context.Context, input *loginMFAInput) (*authSessionOutput, error) {
		return delegateHTTP[authSessionOutput](ctx, jsonBody(input.Body), auth.loginMFA)
	})
	huma.Register(api, huma.Operation{OperationID: "refresh-auth-session", Method: http.MethodPost, Path: "/api/v1/auth/refresh", Tags: []string{"Auth"}, Summary: "Refresh access token", Errors: documentedErrorStatuses()}, func(ctx context.Context, _ *struct{}) (*authSessionOutput, error) {
		return delegateHTTP[authSessionOutput](ctx, nil, auth.refresh)
	})
	huma.Register(api, huma.Operation{OperationID: "list-auth-providers", Method: http.MethodGet, Path: "/api/v1/auth/providers", Tags: []string{"Auth"}, Summary: "List OAuth providers", Errors: documentedErrorStatuses()}, func(ctx context.Context, _ *struct{}) (*authProvidersOutput, error) {
		return delegateHTTP[authProvidersOutput](ctx, nil, auth.providers)
	})
	huma.Register(api, huma.Operation{OperationID: "start-oauth", Method: http.MethodPost, Path: "/api/v1/auth/oauth/{provider}/start", Tags: []string{"Auth"}, Summary: "Start OAuth login", Errors: documentedErrorStatuses()}, func(ctx context.Context, input *oauthStartInput) (*oauthStartOutput, error) {
		return delegateHTTP[oauthStartOutput](ctx, jsonBody(input.Body), auth.oauthStart)
	})
	huma.Register(api, huma.Operation{OperationID: "exchange-oauth", Method: http.MethodPost, Path: "/api/v1/auth/oauth/{provider}/exchange", Tags: []string{"Auth"}, Summary: "Exchange OAuth code", Errors: documentedErrorStatuses()}, func(ctx context.Context, input *oauthExchangeInput) (*authSessionOutput, error) {
		return delegateHTTP[authSessionOutput](ctx, jsonBody(input.Body), auth.oauthExchange)
	})
	huma.Register(api, huma.Operation{OperationID: "verify-email", Method: http.MethodPost, Path: "/api/v1/auth/email/verify", Tags: []string{"Auth"}, Summary: "Verify email", Errors: documentedErrorStatuses()}, func(ctx context.Context, input *tokenInput) (*boolOutput, error) {
		return delegateHTTP[boolOutput](ctx, jsonBody(input.Body), auth.verifyEmail)
	})
	huma.Register(api, huma.Operation{OperationID: "forgot-password", Method: http.MethodPost, Path: "/api/v1/auth/password/forgot", Tags: []string{"Auth"}, Summary: "Request password reset", Errors: documentedErrorStatuses()}, func(ctx context.Context, input *forgotPasswordInput) (*boolOutput, error) {
		return delegateHTTP[boolOutput](ctx, jsonBody(input.Body), auth.forgotPassword)
	})
	huma.Register(api, huma.Operation{OperationID: "reset-password", Method: http.MethodPost, Path: "/api/v1/auth/password/reset", Tags: []string{"Auth"}, Summary: "Reset password", Errors: documentedErrorStatuses()}, func(ctx context.Context, input *resetPasswordInput) (*boolOutput, error) {
		return delegateHTTP[boolOutput](ctx, jsonBody(input.Body), auth.resetPassword)
	})

	huma.Register(api, protectedOperation(huma.Operation{OperationID: "logout", Method: http.MethodPost, Path: "/api/v1/auth/logout", Tags: []string{"Auth"}, Summary: "Logout current session", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, _ *struct{}) (*boolOutput, error) {
		return delegateHTTP[boolOutput](ctx, nil, auth.logout)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "get-me", Method: http.MethodGet, Path: "/api/v1/auth/me", Tags: []string{"Auth"}, Summary: "Get current user", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, _ *struct{}) (*meOutput, error) {
		return delegateHTTP[meOutput](ctx, nil, auth.me)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "request-email-verification", Method: http.MethodPost, Path: "/api/v1/auth/email/verify/request", Tags: []string{"Auth"}, Summary: "Request verification email", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, _ *struct{}) (*boolOutput, error) {
		return delegateHTTP[boolOutput](ctx, nil, auth.requestVerifyEmail)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "setup-mfa-totp", Method: http.MethodPost, Path: "/api/v1/auth/mfa/totp/setup", Tags: []string{"Auth"}, Summary: "Setup TOTP MFA", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, _ *struct{}) (*mfaSetupOutput, error) {
		return delegateHTTP[mfaSetupOutput](ctx, nil, auth.mfaTOTPSetup)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "enable-mfa-totp", Method: http.MethodPost, Path: "/api/v1/auth/mfa/totp/enable", Tags: []string{"Auth"}, Summary: "Enable TOTP MFA", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *codeInput) (*mfaRecoveryOutput, error) {
		return delegateHTTP[mfaRecoveryOutput](ctx, jsonBody(input.Body), auth.mfaTOTPEnable)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "disable-mfa-totp", Method: http.MethodPost, Path: "/api/v1/auth/mfa/totp/disable", Tags: []string{"Auth"}, Summary: "Disable TOTP MFA", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *mfaDisableInput) (*boolOutput, error) {
		return delegateHTTP[boolOutput](ctx, jsonBody(input.Body), auth.mfaTOTPDisable)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "regenerate-mfa-recovery", Method: http.MethodPost, Path: "/api/v1/auth/mfa/recovery/regenerate", Tags: []string{"Auth"}, Summary: "Regenerate MFA recovery codes", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *codeInput) (*mfaRecoveryOutput, error) {
		return delegateHTTP[mfaRecoveryOutput](ctx, jsonBody(input.Body), auth.mfaRecoveryRegenerate)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "list-sessions", Method: http.MethodGet, Path: "/api/v1/sessions", Tags: []string{"Auth"}, Summary: "List active sessions", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, _ *struct{}) (*sessionsOutput, error) {
		return delegateHTTP[sessionsOutput](ctx, nil, auth.sessions)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "revoke-session", Method: http.MethodDelete, Path: "/api/v1/sessions/{session_id}", Tags: []string{"Auth"}, Summary: "Revoke session", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *sessionPathInput) (*boolOutput, error) {
		_ = input
		return delegateHTTP[boolOutput](ctx, nil, auth.revokeSession)
	})
}

type workspacePathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
}

type membershipPathInput struct {
	WorkspaceID  string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	MembershipID string `path:"membership_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Membership ID."`
}

type rolePathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	RoleID      string `path:"role_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Role ID."`
}

type invitationTokenInput struct {
	Token string `path:"token" example:"workspace-invitation-token" doc:"Invitation token."`
}

type createWorkspaceInput struct {
	Body struct {
		Name string `json:"name,omitempty" minLength:"1" example:"Acme" doc:"Workspace name."`
	} `required:"true" nameHint:"CreateWorkspaceRequest"`
}

type inviteWorkspaceMemberInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		Email   string   `json:"email,omitempty" format:"email" example:"member@example.com" doc:"Invitee email address."`
		RoleIDs []string `json:"role_ids,omitempty" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Role IDs to assign."`
	} `required:"true" nameHint:"InviteWorkspaceMemberRequest"`
}

type roleIDsInput struct {
	WorkspaceID  string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	MembershipID string `path:"membership_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Membership ID."`
	Body         struct {
		RoleIDs []string `json:"role_ids,omitempty" minItems:"1" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Role IDs to assign."`
	} `required:"true" nameHint:"UpdateWorkspaceMemberRolesRequest"`
}

type createRoleInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		Name            string   `json:"name,omitempty" minLength:"1" example:"Editor" doc:"Role name."`
		PermissionNames []string `json:"permission_names,omitempty" example:"workspace.members.read" doc:"Permission names."`
		PermissionsMask *int64   `json:"permissions_mask,omitempty" example:"7" doc:"Permission bit mask."`
	} `required:"true" nameHint:"CreateWorkspaceRoleRequest"`
}

type updateRoleInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	RoleID      string `path:"role_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Role ID."`
	Body        struct {
		Name            *string  `json:"name,omitempty" example:"Editor" doc:"Role name."`
		PermissionNames []string `json:"permission_names,omitempty" example:"workspace.members.read" doc:"Permission names."`
		PermissionsMask *int64   `json:"permissions_mask,omitempty" example:"7" doc:"Permission bit mask."`
		Status          *string  `json:"status,omitempty" enum:"active,archived" example:"active" doc:"Role status."`
	} `required:"true" nameHint:"UpdateWorkspaceRoleRequest"`
}

type workspaceDoc struct {
	ID           string    `json:"id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Name         string    `json:"name,omitempty" example:"Acme" doc:"Workspace name."`
	MembershipID string    `json:"membership_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Current user's membership ID."`
	RoleNames    []string  `json:"role_names" example:"Owner" doc:"Current user's role names."`
	Plan         string    `json:"plan,omitempty" example:"free" doc:"Workspace plan."`
	LogoIcon     string    `json:"logo_icon,omitempty" example:"acme" doc:"Workspace logo icon."`
	CreatedAt    time.Time `json:"created_at" doc:"Creation timestamp."`
	UpdatedAt    time.Time `json:"updated_at" doc:"Update timestamp."`
}

type workspaceAccessDoc struct {
	WorkspaceID          string   `json:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	MembershipID         string   `json:"membership_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Membership ID."`
	Status               string   `json:"status" example:"active" doc:"Membership status."`
	RoleIDs              []string `json:"role_ids,omitempty" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Assigned role IDs."`
	RoleNames            []string `json:"role_names" example:"Owner" doc:"Assigned role names."`
	EffectivePermissions []string `json:"effective_permissions" example:"workspace.members.read" doc:"Effective permission names."`
}

type memberDoc struct {
	MembershipID string   `json:"membership_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Membership ID."`
	WorkspaceID  string   `json:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	UserEmail    string   `json:"user_email" format:"email" example:"member@example.com" doc:"Member email."`
	Status       string   `json:"status" example:"active" doc:"Membership status."`
	RoleNames    []string `json:"role_names" example:"Owner" doc:"Assigned role names."`
}

type invitationDoc struct {
	Token       string    `json:"token,omitempty" example:"workspace-invitation-token" doc:"Invitation token."`
	WorkspaceID string    `json:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Email       string    `json:"email,omitempty" format:"email" example:"member@example.com" doc:"Invitee email."`
	RoleIDs     []string  `json:"role_ids,omitempty" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Assigned role IDs."`
	Status      string    `json:"status" example:"pending" doc:"Invitation status."`
	ExpiresAt   time.Time `json:"expires_at,omitempty" doc:"Expiration timestamp."`
	CreatedAt   time.Time `json:"created_at" doc:"Creation timestamp."`
}

type roleDoc struct {
	ID              string   `json:"id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Role ID."`
	Name            string   `json:"name,omitempty" example:"Owner" doc:"Role name."`
	Type            string   `json:"type" example:"owner" doc:"Role type."`
	PermissionsMask int64    `json:"permissions_mask,omitempty" example:"7" doc:"Permission bit mask."`
	PermissionNames []string `json:"permission_names,omitempty" example:"workspace.members.read" doc:"Permission names."`
	Builtin         bool     `json:"builtin" example:"true" doc:"Whether role is built in."`
	Status          string   `json:"status" example:"active" doc:"Role status."`
	Version         int      `json:"version" example:"1" doc:"Role version."`
}

type permissionDoc struct {
	Bit  int    `json:"bit" example:"1" doc:"Permission bit."`
	Name string `json:"name,omitempty" example:"workspace.members.read" doc:"Permission name."`
}

type memberRoleAssignmentDoc struct {
	WorkspaceID          string   `json:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	MembershipID         string   `json:"membership_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Membership ID."`
	RoleIDs              []string `json:"role_ids,omitempty" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Assigned role IDs."`
	EffectivePermissions []string `json:"effective_permissions" example:"workspace.members.read" doc:"Effective permission names."`
}

type workspaceOutput struct {
	Body successEnvelopeDoc[workspaceDoc]
}

type workspacesOutput struct {
	Body successEnvelopeDoc[[]workspaceDoc]
}

type workspaceAccessOutput struct {
	Body successEnvelopeDoc[workspaceAccessDoc]
}

type membersOutput struct {
	Body successEnvelopeDoc[[]memberDoc]
}

type invitationsOutput struct {
	Body successEnvelopeDoc[[]invitationDoc]
}

type invitationOutput struct {
	Body successEnvelopeDoc[invitationDoc]
}

type memberOutput struct {
	Body successEnvelopeDoc[memberDoc]
}

type rolesOutput struct {
	Body successEnvelopeDoc[[]roleDoc]
}

type roleOutput struct {
	Body successEnvelopeDoc[roleDoc]
}

type permissionsOutput struct {
	Body successEnvelopeDoc[[]permissionDoc]
}

type memberRoleAssignmentOutput struct {
	Body successEnvelopeDoc[memberRoleAssignmentDoc]
}

type senderDomainPathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	DomainID    string `path:"domain_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Sender domain ID."`
}

type createSenderDomainInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		Domain   string `json:"domain,omitempty" example:"example.com" doc:"Sender domain."`
		Provider string `json:"provider,omitempty" example:"ses" doc:"Email provider."`
	} `required:"true" nameHint:"CreateSenderDomainRequest"`
}

func registerSenderOperations(api huma.API, sender *senderHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-sender-domains",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/sender-domains",
		Tags:        []string{"Sender Domains"},
		Summary:     "List sender domains",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *senderDomainPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, sender.listSenderDomains)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "create-sender-domain",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/sender-domains",
		Tags:          []string{"Sender Domains"},
		Summary:       "Create sender domain",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *createSenderDomainInput) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), sender.createSenderDomain)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-sender-domain",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}",
		Tags:        []string{"Sender Domains"},
		Summary:     "Get sender domain",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *senderDomainPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, sender.getSenderDomain)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "verify-sender-domain",
		Method:      http.MethodPost,
		Path:        "/api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}/verify",
		Tags:        []string{"Sender Domains"},
		Summary:     "Verify sender domain DNS records",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *senderDomainPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, sender.verifySenderDomain)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "disable-sender-domain",
		Method:      http.MethodPost,
		Path:        "/api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}/disable",
		Tags:        []string{"Sender Domains"},
		Summary:     "Disable sender domain",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *senderDomainPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, sender.disableSenderDomain)
	})
}

func registerWorkspaceOperations(api huma.API, workspace *workspaceHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	register := func(op huma.Operation, fn func(context.Context) (*emptyOutput, error)) {
		huma.Register(api, protectedOperation(op, authMiddleware), func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
			return fn(ctx)
		})
	}
	_ = register

	huma.Register(api, protectedOperation(huma.Operation{OperationID: "create-workspace", Method: http.MethodPost, Path: "/api/v1/workspaces", Tags: []string{"Workspaces"}, Summary: "Create workspace", DefaultStatus: http.StatusCreated, Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *createWorkspaceInput) (*workspaceOutput, error) {
		return delegateHTTP[workspaceOutput](ctx, jsonBody(input.Body), workspace.createWorkspace)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "list-workspaces", Method: http.MethodGet, Path: "/api/v1/workspaces", Tags: []string{"Workspaces"}, Summary: "List workspaces", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, _ *struct{}) (*workspacesOutput, error) {
		return delegateHTTP[workspacesOutput](ctx, nil, workspace.listWorkspaces)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "list-permissions", Method: http.MethodGet, Path: "/api/v1/permissions", Tags: []string{"Workspaces"}, Summary: "List permissions", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, _ *struct{}) (*permissionsOutput, error) {
		return delegateHTTP[permissionsOutput](ctx, nil, workspace.listPermissions)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "get-workspace", Method: http.MethodGet, Path: "/api/v1/workspaces/{workspace_id}", Tags: []string{"Workspaces"}, Summary: "Get workspace", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*workspaceOutput, error) {
		_ = input
		return delegateHTTP[workspaceOutput](ctx, nil, workspace.getWorkspace)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "get-workspace-access", Method: http.MethodGet, Path: "/api/v1/workspaces/{workspace_id}/access", Tags: []string{"Workspaces"}, Summary: "Get workspace access", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*workspaceAccessOutput, error) {
		_ = input
		return delegateHTTP[workspaceAccessOutput](ctx, nil, workspace.getWorkspaceAccess)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "list-workspace-members", Method: http.MethodGet, Path: "/api/v1/workspaces/{workspace_id}/members", Tags: []string{"Workspaces"}, Summary: "List workspace members", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*membersOutput, error) {
		_ = input
		return delegateHTTP[membersOutput](ctx, nil, workspace.listWorkspaceMembers)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "list-workspace-invitations", Method: http.MethodGet, Path: "/api/v1/workspaces/{workspace_id}/invitations", Tags: []string{"Workspaces"}, Summary: "List workspace invitations", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*invitationsOutput, error) {
		_ = input
		return delegateHTTP[invitationsOutput](ctx, nil, workspace.listWorkspaceInvitations)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "list-workspace-roles", Method: http.MethodGet, Path: "/api/v1/workspaces/{workspace_id}/roles", Tags: []string{"Workspaces"}, Summary: "List workspace roles", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*rolesOutput, error) {
		_ = input
		return delegateHTTP[rolesOutput](ctx, nil, workspace.listWorkspaceRoles)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "create-workspace-role", Method: http.MethodPost, Path: "/api/v1/workspaces/{workspace_id}/roles", Tags: []string{"Workspaces"}, Summary: "Create workspace role", DefaultStatus: http.StatusCreated, Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *createRoleInput) (*roleOutput, error) {
		return delegateHTTP[roleOutput](ctx, jsonBody(input.Body), workspace.createWorkspaceRole)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "invite-workspace-member", Method: http.MethodPost, Path: "/api/v1/workspaces/{workspace_id}/invitations", Tags: []string{"Workspaces"}, Summary: "Invite workspace member", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *inviteWorkspaceMemberInput) (*invitationOutput, error) {
		return delegateHTTP[invitationOutput](ctx, jsonBody(input.Body), workspace.inviteWorkspaceMember)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "remove-workspace-member", Method: http.MethodDelete, Path: "/api/v1/workspaces/{workspace_id}/members/{membership_id}", Tags: []string{"Workspaces"}, Summary: "Remove workspace member", DefaultStatus: http.StatusNoContent, Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *membershipPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, workspace.removeWorkspaceMember)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "update-workspace-member-role", Method: http.MethodPut, Path: "/api/v1/workspaces/{workspace_id}/members/{membership_id}/role", Tags: []string{"Workspaces"}, Summary: "Update workspace member role", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *roleIDsInput) (*boolOutput, error) {
		return delegateHTTP[boolOutput](ctx, jsonBody(input.Body), workspace.updateWorkspaceMemberRole)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "assign-workspace-member-roles", Method: http.MethodPut, Path: "/api/v1/workspaces/{workspace_id}/members/{membership_id}/roles", Tags: []string{"Workspaces"}, Summary: "Assign workspace member roles", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *roleIDsInput) (*memberRoleAssignmentOutput, error) {
		return delegateHTTP[memberRoleAssignmentOutput](ctx, jsonBody(input.Body), workspace.assignWorkspaceMemberRoles)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "update-workspace-role", Method: http.MethodPatch, Path: "/api/v1/workspaces/{workspace_id}/roles/{role_id}", Tags: []string{"Workspaces"}, Summary: "Update workspace role", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *updateRoleInput) (*roleOutput, error) {
		return delegateHTTP[roleOutput](ctx, jsonBody(input.Body), workspace.updateWorkspaceRole)
	})
	huma.Register(api, protectedOperation(huma.Operation{OperationID: "accept-workspace-invitation", Method: http.MethodPost, Path: "/api/v1/invitations/{token}/accept", Tags: []string{"Workspaces"}, Summary: "Accept workspace invitation", Errors: documentedErrorStatuses()}, authMiddleware), func(ctx context.Context, input *invitationTokenInput) (*memberOutput, error) {
		_ = input
		return delegateHTTP[memberOutput](ctx, nil, workspace.acceptWorkspaceInvitation)
	})
}

// --- Audience Path Inputs ---

type audienceContactPathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	ContactID   string `path:"contact_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Contact ID."`
}

type audienceListPathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	ListID      string `path:"list_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"List ID."`
}

type audienceSegmentPathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	SegmentID   string `path:"segment_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Segment ID."`
}

type audienceImportPathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	ImportID    string `path:"import_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Import job ID."`
}

type audienceExportPathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	ExportID    string `path:"export_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Export job ID."`
}

type contentTemplatePathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	TemplateID  string `path:"template_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Template ID."`
}

type suppressionPathInput struct {
	WorkspaceID   string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	SuppressionID string `path:"suppression_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Suppression entry ID."`
}

type createContactInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		Email      string         `json:"email,omitempty" format:"email" example:"user@example.com" doc:"Contact email."`
		FirstName  string         `json:"first_name,omitempty" example:"John" doc:"Contact first name."`
		LastName   string         `json:"last_name,omitempty" example:"Doe" doc:"Contact last name."`
		Tags       []string       `json:"tags,omitempty" example:"vip" doc:"Contact tags."`
		Attributes map[string]any `json:"attributes,omitempty" doc:"Contact attributes."`
	} `required:"true" nameHint:"CreateContactRequest"`
}

type createAudienceListInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		Name        string         `json:"name,omitempty" minLength:"1" example:"Newsletter" doc:"List name."`
		Description string         `json:"description,omitempty" example:"Monthly newsletter subscribers" doc:"List description."`
		Metadata    map[string]any `json:"metadata,omitempty" doc:"List metadata."`
	} `required:"true" nameHint:"CreateAudienceListRequest"`
}

type createSegmentInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		Name       string         `json:"name,omitempty" minLength:"1" example:"VIP Customers" doc:"Segment name."`
		Definition map[string]any `json:"definition,omitempty" doc:"Segment definition."`
	} `required:"true" nameHint:"CreateSegmentRequest"`
}

type startAudienceImportInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		SourceURI  string         `json:"source_uri,omitempty" example:"s3://bucket/contacts.csv" doc:"Source URI for import."`
		DedupeMode string         `json:"dedupe_mode,omitempty" example:"email" doc:"Deduplication mode."`
		Metadata   map[string]any `json:"metadata,omitempty" doc:"Import metadata."`
	} `required:"true" nameHint:"StartAudienceImportRequest"`
}

type startAudienceExportInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		Format         string         `json:"format,omitempty" example:"csv" doc:"Export format."`
		ZipOutput      bool           `json:"zip_output,omitempty" example:"false" doc:"Compress the exported artifact into a ZIP file."`
		Filters        map[string]any `json:"filters,omitempty" doc:"Export filters."`
		SelectedFields []string       `json:"selected_fields,omitempty" example:"email,first_name" doc:"Fields to export."`
	} `required:"true" nameHint:"StartAudienceExportRequest"`
}

type listAudienceExportsInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Status      string `query:"status" example:"completed" doc:"Filter by job status."`
	Limit       int    `query:"limit" example:"50" doc:"Maximum number of entries to return (1-100)."`
	Cursor      string `query:"cursor" doc:"Pagination cursor from previous response."`
}

type createTemplateInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		Name     string         `json:"name,omitempty" minLength:"1" example:"Welcome Email" doc:"Template name."`
		Subject  string         `json:"subject,omitempty" example:"Welcome {{.name}}" doc:"Template subject."`
		HTML     string         `json:"html,omitempty" example:"<h1>Welcome</h1>" doc:"Template HTML."`
		Text     string         `json:"text,omitempty" example:"Welcome" doc:"Template text."`
		Metadata map[string]any `json:"metadata,omitempty" doc:"Template metadata."`
	} `required:"true" nameHint:"CreateTemplateRequest"`
}

type renderTemplateInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		TemplateID   string         `json:"template_id,omitempty" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Template ID."`
		Subject      string         `json:"subject,omitempty" example:"Welcome {{.name}}" doc:"Template subject."`
		HTML         string         `json:"html,omitempty" example:"<h1>Welcome</h1>" doc:"Template HTML."`
		Text         string         `json:"text,omitempty" example:"Welcome" doc:"Template text."`
		TemplateData map[string]any `json:"template_data,omitempty" doc:"Template data."`
	} `required:"true" nameHint:"RenderTemplateRequest"`
}

type createSuppressionEntryInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		Email  string `json:"email,omitempty" format:"email" example:"bounce@example.com" doc:"Suppressed email."`
		Scope  string `json:"scope,omitempty" example:"hard_bounce" doc:"Suppression scope."`
		Reason string `json:"reason,omitempty" example:"hard_bounce" doc:"Suppression reason."`
		Note   string `json:"note,omitempty" example:"Permanent bounce detected" doc:"Suppression note."`
	} `required:"true" nameHint:"CreateSuppressionEntryRequest"`
}

type updateContactInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	ContactID   string `path:"contact_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Contact ID."`
	Body        struct {
		Email      *string        `json:"email,omitempty" format:"email" example:"user@example.com" doc:"Contact email."`
		FirstName  *string        `json:"first_name,omitempty" example:"John" doc:"Contact first name."`
		LastName   *string        `json:"last_name,omitempty" example:"Doe" doc:"Contact last name."`
		Status     *string        `json:"status,omitempty" enum:"active,archived,bounced,unsubscribed" example:"active" doc:"Contact status."`
		Tags       []string       `json:"tags,omitempty" example:"vip" doc:"Contact tags."`
		Attributes map[string]any `json:"attributes,omitempty" doc:"Contact attributes."`
	} `required:"true" nameHint:"UpdateContactRequest"`
}

type updateAudienceListContactsInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	ListID      string `path:"list_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"List ID."`
	Body        struct {
		Mode       string   `json:"mode,omitempty" example:"replace" doc:"Update mode (add/remove/replace)."`
		ContactIDs []string `json:"contact_ids,omitempty" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Contact IDs."`
	} `required:"true" nameHint:"UpdateAudienceListContactsRequest"`
}

type updateSegmentInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	SegmentID   string `path:"segment_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Segment ID."`
	Body        struct {
		Name       string         `json:"name,omitempty" minLength:"1" example:"VIP Customers" doc:"Segment name."`
		Definition map[string]any `json:"definition,omitempty" doc:"Segment definition."`
		Status     string         `json:"status,omitempty" enum:"active,archived" example:"active" doc:"Segment status."`
	} `required:"true" nameHint:"UpdateSegmentRequest"`
}

type updateTemplateInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	TemplateID  string `path:"template_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Template ID."`
	Body        struct {
		Name     *string        `json:"name,omitempty" minLength:"1" example:"Welcome Email" doc:"Template name."`
		Subject  *string        `json:"subject,omitempty" example:"Welcome {{.name}}" doc:"Template subject."`
		HTML     *string        `json:"html,omitempty" example:"<h1>Welcome</h1>" doc:"Template HTML."`
		Text     *string        `json:"text,omitempty" example:"Welcome" doc:"Template text."`
		Metadata map[string]any `json:"metadata,omitempty" doc:"Template metadata."`
	} `required:"true" nameHint:"UpdateTemplateRequest"`
}

type previewTemplateInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	TemplateID  string `path:"template_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Template ID."`
	Body        struct {
		TemplateData map[string]any `json:"template_data,omitempty" doc:"Template data."`
	} `required:"true" nameHint:"PreviewTemplateRequest"`
}

// --- Audience Operations ---

func registerAudienceOperations(api huma.API, audience *audienceHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-contacts",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/contacts",
		Tags:        []string{"Audience"},
		Summary:     "List contacts",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, audience.listContacts)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "create-contact",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/contacts",
		Tags:          []string{"Audience"},
		Summary:       "Create contact",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *createContactInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), audience.createContact)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-contact",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/contacts/{contact_id}",
		Tags:        []string{"Audience"},
		Summary:     "Get contact",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *audienceContactPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, audience.getContact)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "update-contact",
		Method:      http.MethodPatch,
		Path:        "/api/v1/workspaces/{workspace_id}/contacts/{contact_id}",
		Tags:        []string{"Audience"},
		Summary:     "Update contact",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *updateContactInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), audience.updateContact)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "delete-contact",
		Method:      http.MethodDelete,
		Path:        "/api/v1/workspaces/{workspace_id}/contacts/{contact_id}",
		Tags:        []string{"Audience"},
		Summary:     "Delete contact",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *audienceContactPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, audience.deleteContact)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-audience-lists",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/lists",
		Tags:        []string{"Audience"},
		Summary:     "List audience lists",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, audience.listAudienceLists)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "create-audience-list",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/lists",
		Tags:          []string{"Audience"},
		Summary:       "Create audience list",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *createAudienceListInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), audience.createAudienceList)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "update-audience-list-contacts",
		Method:      http.MethodPut,
		Path:        "/api/v1/workspaces/{workspace_id}/lists/{list_id}/contacts",
		Tags:        []string{"Audience"},
		Summary:     "Update audience list contacts",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *updateAudienceListContactsInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), audience.updateAudienceListContacts)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-segments",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/segments",
		Tags:        []string{"Audience"},
		Summary:     "List segments",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, audience.listSegments)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "create-segment",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/segments",
		Tags:          []string{"Audience"},
		Summary:       "Create segment",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *createSegmentInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), audience.createSegment)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "update-segment",
		Method:      http.MethodPatch,
		Path:        "/api/v1/workspaces/{workspace_id}/segments/{segment_id}",
		Tags:        []string{"Audience"},
		Summary:     "Update segment",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *updateSegmentInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), audience.updateSegment)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "start-audience-import",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/audience/imports",
		Tags:          []string{"Audience"},
		Summary:       "Start audience import",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *startAudienceImportInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), audience.startAudienceImport)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-audience-imports",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/audience/imports",
		Tags:        []string{"Audience"},
		Summary:     "List audience imports",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, audience.listAudienceImports)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-audience-import",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/audience/imports/{import_id}",
		Tags:        []string{"Audience"},
		Summary:     "Get audience import",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *audienceImportPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, audience.getAudienceImport)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "start-audience-export",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/audience/exports",
		Tags:          []string{"Audience"},
		Summary:       "Start audience export",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *startAudienceExportInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), audience.startAudienceExport)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-audience-exports",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/audience/exports",
		Tags:        []string{"Audience"},
		Summary:     "List audience exports",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *listAudienceExportsInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, audience.listAudienceExports)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-audience-export",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/audience/exports/{export_id}",
		Tags:        []string{"Audience"},
		Summary:     "Get audience export",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *audienceExportPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, audience.getAudienceExport)
	})
}

// --- Content Operations ---

func registerContentOperations(api huma.API, content *contentHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-templates",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/templates",
		Tags:        []string{"Templates"},
		Summary:     "List templates",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, content.listTemplates)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "create-template",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/templates",
		Tags:          []string{"Templates"},
		Summary:       "Create template",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *createTemplateInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), content.createTemplate)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-template",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/templates/{template_id}",
		Tags:        []string{"Templates"},
		Summary:     "Get template",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *contentTemplatePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, content.getTemplate)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "update-template",
		Method:      http.MethodPatch,
		Path:        "/api/v1/workspaces/{workspace_id}/templates/{template_id}",
		Tags:        []string{"Templates"},
		Summary:     "Update template",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *updateTemplateInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), content.updateTemplate)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "publish-template",
		Method:      http.MethodPost,
		Path:        "/api/v1/workspaces/{workspace_id}/templates/{template_id}/publish",
		Tags:        []string{"Templates"},
		Summary:     "Publish template version",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *contentTemplatePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, content.publishTemplate)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-template-versions",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/templates/{template_id}/versions",
		Tags:        []string{"Templates"},
		Summary:     "List template versions",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *contentTemplatePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, content.listTemplateVersions)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "preview-template",
		Method:      http.MethodPost,
		Path:        "/api/v1/workspaces/{workspace_id}/templates/{template_id}/preview",
		Tags:        []string{"Template Render"},
		Summary:     "Preview template",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *previewTemplateInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), content.previewTemplate)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "render-template",
		Method:      http.MethodPost,
		Path:        "/api/v1/workspaces/{workspace_id}/render",
		Tags:        []string{"Template Render"},
		Summary:     "Render template",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *renderTemplateInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), content.render)
	})
}

// --- Suppression Operations ---

func registerSuppressionOperations(api huma.API, suppression *suppressionHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-suppression-entries",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/suppression",
		Tags:        []string{"Suppression"},
		Summary:     "List suppression entries",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, suppression.listSuppressionEntries)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "create-suppression-entry",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/suppression",
		Tags:          []string{"Suppression"},
		Summary:       "Create suppression entry",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *createSuppressionEntryInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), suppression.createSuppressionEntry)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "delete-suppression-entry",
		Method:      http.MethodDelete,
		Path:        "/api/v1/workspaces/{workspace_id}/suppression/{suppression_id}",
		Tags:        []string{"Suppression"},
		Summary:     "Delete suppression entry",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *suppressionPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, suppression.deleteSuppressionEntry)
	})
}

func audienceErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"audience.read_denied",
			"audience.write_denied",
			"audience.import_denied",
			"audience.export_denied",
		},
		http.StatusNotFound: {
			"audience.contact_not_found",
			"audience.list_not_found",
			"audience.segment_not_found",
			"audience.import_job_not_found",
			"audience.export_job_not_found",
		},
		http.StatusConflict: {
			"audience.contact_email_conflict",
			"audience.list_name_conflict",
			"audience.segment_name_conflict",
			"audience.import_duplicate_submission",
		},
		http.StatusUnprocessableEntity: {
			"audience.contact_payload_invalid",
			"audience.contact_status_invalid",
			"audience.segment_definition_invalid",
			"audience.list_membership_payload_invalid",
			"audience.import_source_invalid",
			"audience.export_filter_invalid",
			"audience.export_format_invalid",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

func templateErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"template.read_denied",
			"template.write_denied",
			"template.render_denied",
		},
		http.StatusNotFound: {
			"template.not_found",
			"template.version_not_found",
		},
		http.StatusConflict: {
			"template.publish_conflict",
			"template.version_conflict",
		},
		http.StatusUnprocessableEntity: {
			"template.source_invalid",
			"template.publish_payload_invalid",
			"template.render_payload_invalid",
			"template.render_context_invalid",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

func suppressionErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"suppression.read_denied",
			"suppression.manage_denied",
		},
		http.StatusNotFound: {
			"suppression.entry_not_found",
		},
		http.StatusConflict: {
			"suppression.unsuppress_conflict",
		},
		http.StatusUnprocessableEntity: {
			"suppression.scope_invalid",
			"suppression.reason_invalid",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

// --- Campaign Operations ---

type campaignPathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	CampaignID  string `path:"campaign_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Campaign ID."`
}

type campaignAudienceRefDoc struct {
	Type       string   `json:"type" enum:"contacts,list,segment" example:"list" doc:"Audience type."`
	ID         string   `json:"id,omitempty" example:"list_1" doc:"Audience list or segment ID."`
	ContactIDs []string `json:"contact_ids,omitempty" example:"ct_1" doc:"Direct contact IDs."`
}

type createCampaignBodyDoc struct {
	Name           string                  `json:"name,omitempty" minLength:"1" example:"Spring Sale" doc:"Campaign name."`
	AudienceRef    *campaignAudienceRefDoc `json:"audience_ref,omitempty" doc:"Audience reference."`
	TemplateID     string                  `json:"template_id,omitempty" example:"tpl_1" doc:"Template ID."`
	SenderDomainID string                  `json:"sender_domain_id,omitempty" example:"sd_1" doc:"Sender domain ID."`
	MessageType    string                  `json:"message_type,omitempty" enum:"marketing,transactional" example:"marketing" doc:"Message type."`
}

type createCampaignInput struct {
	WorkspaceID string                `path:"workspace_id"`
	Body        createCampaignBodyDoc `required:"true" nameHint:"CreateCampaignRequest"`
}

type updateCampaignBodyDoc struct {
	Name           *string                 `json:"name,omitempty" minLength:"1" example:"Spring Sale" doc:"Campaign name."`
	AudienceRef    *campaignAudienceRefDoc `json:"audience_ref,omitempty" doc:"Audience reference."`
	TemplateID     *string                 `json:"template_id,omitempty" example:"tpl_1" doc:"Template ID."`
	SenderDomainID *string                 `json:"sender_domain_id,omitempty" example:"sd_1" doc:"Sender domain ID."`
	MessageType    *string                 `json:"message_type,omitempty" enum:"marketing,transactional" example:"marketing" doc:"Message type."`
}

type updateCampaignInput struct {
	WorkspaceID string                `path:"workspace_id"`
	CampaignID  string                `path:"campaign_id"`
	Body        updateCampaignBodyDoc `required:"true" nameHint:"UpdateCampaignRequest"`
}

type scheduleCampaignBodyDoc struct {
	ScheduledAt *time.Time `json:"scheduled_at,omitempty" doc:"Scheduled send time (omit for immediate)."`
}

type scheduleCampaignInput struct {
	WorkspaceID string                  `path:"workspace_id"`
	CampaignID  string                  `path:"campaign_id"`
	Body        scheduleCampaignBodyDoc `required:"true" nameHint:"ScheduleCampaignRequest"`
}

type campaignSummaryDoc struct {
	PlannedRecipients int64     `json:"planned_recipients" example:"0" doc:"Number of planned recipients."`
	Queued            int       `json:"queued" example:"0" doc:"Number of queued messages."`
	Delivered         int       `json:"delivered" example:"0" doc:"Number of delivered messages."`
	LastUpdatedAt     time.Time `json:"last_updated_at" doc:"Last update timestamp."`
}

type campaignResultDoc struct {
	ID                string                 `json:"id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Campaign ID."`
	WorkspaceID       string                 `json:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Name              string                 `json:"name" example:"Spring Sale" doc:"Campaign name."`
	Status            string                 `json:"status" example:"draft" doc:"Campaign status."`
	AudienceRef       campaignAudienceRefDoc `json:"audience_ref" doc:"Audience reference."`
	TemplateID        string                 `json:"template_id" example:"tpl_1" doc:"Template ID."`
	TemplateVersionID string                 `json:"template_version_id" example:"tpl_v1" doc:"Pinned template version ID."`
	SenderDomainID    string                 `json:"sender_domain_id" example:"sd_1" doc:"Sender domain ID."`
	MessageType       string                 `json:"message_type" example:"marketing" doc:"Message type."`
	ScheduledAt       *time.Time             `json:"scheduled_at" doc:"Scheduled send time."`
	PlannedRecipients int64                  `json:"planned_recipients" example:"100" doc:"Number of planned recipients."`
	CreatedAt         time.Time              `json:"created_at" doc:"Creation timestamp."`
	UpdatedAt         time.Time              `json:"updated_at" doc:"Update timestamp."`
	CancelledAt       *time.Time             `json:"cancelled_at" doc:"Cancellation timestamp."`
	PausedAt          *time.Time             `json:"paused_at" doc:"Pause timestamp."`
	CompletedAt       *time.Time             `json:"completed_at" doc:"Completion timestamp."`
	Summary           campaignSummaryDoc     `json:"summary" doc:"Campaign summary."`
}

type campaignListResultDoc struct {
	Campaigns  []campaignResultDoc `json:"campaigns" doc:"List of campaigns."`
	NextCursor string              `json:"next_cursor" doc:"Pagination cursor."`
}

type candidateResultDoc struct {
	ID              string    `json:"id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Candidate ID."`
	ContactID       string    `json:"contact_id" example:"ct_1" doc:"Contact ID."`
	EmailNormalized string    `json:"email_normalized" example:"user@example.com" doc:"Normalized email."`
	Status          string    `json:"status" example:"planned" doc:"Candidate status."`
	CreatedAt       time.Time `json:"created_at" doc:"Creation timestamp."`
	UpdatedAt       time.Time `json:"updated_at" doc:"Update timestamp."`
}

type campaignListOutput struct {
	Body successEnvelopeDoc[campaignListResultDoc]
}

type campaignOutput struct {
	Body successEnvelopeDoc[campaignResultDoc]
}

type candidateListResultDoc struct {
	Candidates []candidateResultDoc `json:"candidates" doc:"List of candidates."`
	NextCursor string               `json:"next_cursor" doc:"Pagination cursor."`
}

type candidateListOutput struct {
	Body successEnvelopeDoc[candidateListResultDoc]
}

func registerCampaignOperations(api huma.API, campaign *campaignHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-campaigns",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/campaigns",
		Tags:        []string{"Campaigns"},
		Summary:     "List campaigns",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, campaign.listCampaigns)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "create-campaign",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/campaigns",
		Tags:          []string{"Campaigns"},
		Summary:       "Create campaign",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *createCampaignInput) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), campaign.createCampaign)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-campaign",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}",
		Tags:        []string{"Campaigns"},
		Summary:     "Get campaign",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *campaignPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, campaign.getCampaign)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "update-campaign",
		Method:      http.MethodPatch,
		Path:        "/api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}",
		Tags:        []string{"Campaigns"},
		Summary:     "Update campaign draft",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *updateCampaignInput) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), campaign.updateCampaign)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "schedule-campaign",
		Method:      http.MethodPost,
		Path:        "/api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/schedule",
		Tags:        []string{"Campaigns"},
		Summary:     "Schedule campaign",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *scheduleCampaignInput) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, jsonBody(input.Body), campaign.scheduleCampaign)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "cancel-campaign",
		Method:      http.MethodPost,
		Path:        "/api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/cancel",
		Tags:        []string{"Campaigns"},
		Summary:     "Cancel campaign",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *campaignPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, campaign.cancelCampaign)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "pause-campaign",
		Method:      http.MethodPost,
		Path:        "/api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/pause",
		Tags:        []string{"Campaigns"},
		Summary:     "Pause campaign",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *campaignPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, campaign.pauseCampaign)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "resume-campaign",
		Method:      http.MethodPost,
		Path:        "/api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/resume",
		Tags:        []string{"Campaigns"},
		Summary:     "Resume campaign",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *campaignPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, campaign.resumeCampaign)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-campaign-candidates",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/candidates",
		Tags:        []string{"Campaigns"},
		Summary:     "List campaign candidates",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *campaignPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, campaign.listCampaignCandidates)
	})
}

func campaignErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"campaign.read_denied",
			"campaign.write_denied",
			"campaign.send_denied",
		},
		http.StatusNotFound: {
			"campaign.not_found",
		},
		http.StatusConflict: {
			"campaign.invalid_state_transition",
		},
		http.StatusUnprocessableEntity: {
			"campaign.payload_invalid",
			"campaign.audience_not_ready",
			"sender.domain_not_verified",
			"template.publish_required",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

// --- Delivery Operations ---

type messagePathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	MessageID   string `path:"message_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Message ID."`
}

func registerDeliveryOperations(api huma.API, delivery *deliveryHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-messages",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/messages",
		Tags:        []string{"Delivery"},
		Summary:     "List messages",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, delivery.listMessages)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-message",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/messages/{message_id}",
		Tags:        []string{"Delivery"},
		Summary:     "Get message",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *messagePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, delivery.getMessage)
	})
}

type apiKeyAuthDeps struct {
	svc     *accessapp.Service
	metrics *observability.APIKeyMetrics
	limiter ratelimit.Service
}

func humaAPIKeyAuthMiddleware(deps apiKeyAuthDeps) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		req, w := humachi.Unwrap(ctx)

		authHeader := strings.TrimSpace(req.Header.Get("Authorization"))
		token, ok := httputil.ExtractBearerToken(authHeader)
		if !ok {
			if deps.metrics != nil {
				deps.metrics.RecordAuthAttempt("missing_token")
			}
			writeError(w, req, http.StatusUnauthorized, "api_key.invalid", "missing or malformed bearer token", nil)
			return
		}

		if deps.limiter != nil {
			rateKey := "api_key:auth:ip:" + clientIP(req)
			allowed, err := deps.limiter.Allow(req.Context(), rateKey, 20, time.Minute)
			if err != nil {
				writeError(w, req, http.StatusInternalServerError, "internal.error", "internal error", nil)
				return
			}
			if !allowed {
				if deps.metrics != nil {
					deps.metrics.RecordAuthAttempt("rate_limited")
				}
				writeError(w, req, http.StatusTooManyRequests, "api_key.rate_limited", "too many api key auth attempts", nil)
				return
			}
		}

		key, err := deps.svc.AuthenticateAPIKey(req.Context(), accessapp.AuthenticateAPIKeyInput{
			BearerToken: token,
		})
		if err != nil {
			if deps.metrics != nil {
				deps.metrics.RecordAuthAttempt("invalid")
			}
			if errors.Is(err, accessdomain.ErrAPIKeyInvalid) {
				writeError(w, req, http.StatusUnauthorized, "api_key.invalid", "invalid, revoked, or expired api key", nil)
				return
			}
			writeError(w, req, http.StatusInternalServerError, "internal.error", "internal error", nil)
			return
		}

		if deps.metrics != nil {
			deps.metrics.RecordAuthAttempt("success")
		}

		baseCtx := context.WithValue(req.Context(), ctxAPIKeyWorkspaceID, key.WorkspaceID)
		baseCtx = context.WithValue(baseCtx, ctxAPIKeyID, key.APIKeyID)
		baseCtx = context.WithValue(baseCtx, ctxAPIKeyScopes, key.Scopes)
		baseCtx = context.WithValue(baseCtx, ctxAPIKeyPrefix, key.KeyPrefix)

		next(huma.WithContext(ctx, baseCtx))
	}
}

func humaAPIKeyScopeMiddleware(scope string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		scopes, _ := ctx.Context().Value(ctxAPIKeyScopes).([]string)
		if !accessdomain.HasScope(scopes, scope) {
			req, w := humachi.Unwrap(ctx)
			writeError(w, req, http.StatusForbidden, "api_key.scope_denied", "api key does not have required scope: "+scope, nil)
			return
		}
		next(ctx)
	}
}

func apiKeyProtectedOperation(op huma.Operation, svc *accessapp.Service, scope string, metrics *observability.APIKeyMetrics, limiter ratelimit.Service) huma.Operation {
	op.Security = []map[string][]string{{"apiKeyAuth": {}}}
	op.Middlewares = append(op.Middlewares, humaAPIKeyAuthMiddleware(apiKeyAuthDeps{svc: svc, metrics: metrics, limiter: limiter}))
	if scope != "" {
		op.Middlewares = append(op.Middlewares, humaAPIKeyScopeMiddleware(scope))
	}
	return op
}

type transactionalMessagePathInput struct {
	MessageID string `path:"message_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Message ID."`
}

func registerTransactionalOperations(api huma.API, transactional *transactionalHTTP, accessSvc *accessapp.Service, apiKeyMetrics *observability.APIKeyMetrics, apiKeyRateLimiter ratelimit.Service) {
	huma.Register(api, apiKeyProtectedOperation(huma.Operation{
		OperationID:   "sendTransactionalEmail",
		Method:        http.MethodPost,
		Path:          "/api/v1/transactional/send",
		Tags:          []string{"Delivery"},
		Summary:       "Send a transactional email",
		Description:   "Accepts a transactional email send request. Supports two modes: template (JSON with template_id and template_data) and raw (multipart/form-data with subject, text_body/html_body, and optional attachments). Recipients are exploded into one message per recipient email. Returns 202 Accepted with request_id and message_ids. Requires API key with transactional.send scope. Limits: max 50 recipients, 25 MB per attachment, 32 MB total body.",
		DefaultStatus: http.StatusAccepted,
		Errors:        documentedErrorStatuses(),
	}, accessSvc, "transactional.send", apiKeyMetrics, apiKeyRateLimiter), func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, nil, transactional.send)
	})

	huma.Register(api, apiKeyProtectedOperation(huma.Operation{
		OperationID: "getTransactionalMessage",
		Method:      http.MethodGet,
		Path:        "/api/v1/transactional/messages/{message_id}",
		Tags:        []string{"Delivery"},
		Summary:     "Get transactional message status",
		Description: "Returns the current status and provider metadata for a transactional message by message ID. Requires API key with transactional.read scope.",
		Errors:      documentedErrorStatuses(),
	}, accessSvc, "transactional.read", apiKeyMetrics, apiKeyRateLimiter), func(ctx context.Context, input *transactionalMessagePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, transactional.getMessage)
	})
}

func registerTransactionalMailLogsOperations(api huma.API, transactional *transactionalHTTP, accessSvc *accessapp.Service, apiKeyMetrics *observability.APIKeyMetrics, apiKeyRateLimiter ratelimit.Service) {
	huma.Register(api, apiKeyProtectedOperation(huma.Operation{
		OperationID: "listMessageEvents",
		Method:      http.MethodGet,
		Path:        "/api/v1/transactional/messages/{message_id}/events",
		Tags:        []string{"Delivery"},
		Summary:     "List events for a transactional message (mail log timeline)",
		Description: "Returns the immutable timeline of events for a transactional message, including queued, processing_started, provider_accepted, delivered, bounced, complained, failed, retry_scheduled, and suppressed events. Requires API key with mail_logs.read scope.",
		Errors:      documentedErrorStatuses(),
	}, accessSvc, "mail_logs.read", apiKeyMetrics, apiKeyRateLimiter), func(ctx context.Context, input *transactionalMessagePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, transactional.listMessageEvents)
	})

	huma.Register(api, apiKeyProtectedOperation(huma.Operation{
		OperationID: "listTransactionalRequestMessages",
		Method:      http.MethodGet,
		Path:        "/api/v1/transactional/requests/{request_id}/messages",
		Tags:        []string{"Delivery"},
		Summary:     "List messages for a transactional request",
		Description: "Returns all per-recipient messages created for a transactional send request. Requires API key with mail_logs.read scope.",
		Errors:      documentedErrorStatuses(),
	}, accessSvc, "mail_logs.read", apiKeyMetrics, apiKeyRateLimiter), func(ctx context.Context, input *transactionalRequestPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, transactional.listRequestMessages)
	})
}

type mailLogPathInput struct {
	MessageID string `path:"message_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Message ID."`
}

func registerMailLogsOperations(api huma.API, mailLogs *mailLogsHTTP, authMiddleware func(huma.Context, func(huma.Context)), accessSvc *accessapp.Service, apiKeyMetrics *observability.APIKeyMetrics, apiKeyRateLimiter ratelimit.Service) {
	// Session-auth routes (workspace operators)
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "listMailLogs",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/mail-logs",
		Tags:        []string{"Delivery"},
		Summary:     "List workspace mail logs",
		Description: "Returns a paginated, filterable list of workspace mail-log entries (all tracked email messages including campaign and transactional). Supports filters: status, message_type, mode, recipient_email, provider, provider_message_id, campaign_id, transactional_request_id, date range (from/to), and cursor-based pagination. Supports session-auth (workspace operators) and API-key auth with mail_logs.read scope.",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, mailLogs.listMailLogs)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "getMailLog",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/mail-logs/{message_id}",
		Tags:        []string{"Delivery"},
		Summary:     "Get mail log detail",
		Description: "Returns full detail for a single mail-log entry, including recipient snapshot, sender domain, template info, timestamps, and last error details. Does not return raw attachment bytes.",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *mailLogPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, mailLogs.getMailLog)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "listMailLogAttempts",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/mail-logs/{message_id}/attempts",
		Tags:        []string{"Delivery"},
		Summary:     "List delivery attempts for a mail log entry",
		Description: "Returns all delivery attempts for a single message, ordered by attempt number descending. Each attempt includes provider, status, error details, and request/response snapshots.",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *mailLogPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, mailLogs.listMailLogAttempts)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "listMailLogEvents",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/mail-logs/{message_id}/events",
		Tags:        []string{"Delivery"},
		Summary:     "List timeline events for a mail log entry",
		Description: "Returns the immutable timeline of events for a message, including queued, processing_started, provider_accepted, delivered, bounced, complained, failed, retry_scheduled, and suppressed events. Events are ordered by occurred_at descending with cursor-based pagination.",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *mailLogPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, mailLogs.listMailLogEvents)
	})

	// API-key auth routes (same paths, scope: mail_logs.read)
	huma.Register(api, apiKeyProtectedOperation(huma.Operation{
		OperationID: "listMailLogsApiKey",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/mail-logs",
		Tags:        []string{"Delivery"},
		Summary:     "List workspace mail logs (API key)",
		Description: "Returns a paginated, filterable list of workspace mail-log entries using API-key auth. Supports the same filters as the session-auth variant. Requires API key with mail_logs.read scope.",
		Errors:      documentedErrorStatuses(),
	}, accessSvc, "mail_logs.read", apiKeyMetrics, apiKeyRateLimiter), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, mailLogs.listMailLogs)
	})

	huma.Register(api, apiKeyProtectedOperation(huma.Operation{
		OperationID: "getMailLogApiKey",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/mail-logs/{message_id}",
		Tags:        []string{"Delivery"},
		Summary:     "Get mail log detail (API key)",
		Description: "Returns full detail for a single mail-log entry using API-key auth. Same response as the session-auth variant. Requires API key with mail_logs.read scope.",
		Errors:      documentedErrorStatuses(),
	}, accessSvc, "mail_logs.read", apiKeyMetrics, apiKeyRateLimiter), func(ctx context.Context, input *mailLogPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, mailLogs.getMailLog)
	})

	huma.Register(api, apiKeyProtectedOperation(huma.Operation{
		OperationID: "listMailLogAttemptsApiKey",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/mail-logs/{message_id}/attempts",
		Tags:        []string{"Delivery"},
		Summary:     "List delivery attempts for a mail log entry (API key)",
		Description: "Returns all delivery attempts for a single message using API-key auth. Same response as the session-auth variant. Requires API key with mail_logs.read scope.",
		Errors:      documentedErrorStatuses(),
	}, accessSvc, "mail_logs.read", apiKeyMetrics, apiKeyRateLimiter), func(ctx context.Context, input *mailLogPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, mailLogs.listMailLogAttempts)
	})

	huma.Register(api, apiKeyProtectedOperation(huma.Operation{
		OperationID: "listMailLogEventsApiKey",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/mail-logs/{message_id}/events",
		Tags:        []string{"Delivery"},
		Summary:     "List timeline events for a mail log entry (API key)",
		Description: "Returns the immutable timeline of events for a message using API-key auth. Same response as the session-auth variant. Requires API key with mail_logs.read scope.",
		Errors:      documentedErrorStatuses(),
	}, accessSvc, "mail_logs.read", apiKeyMetrics, apiKeyRateLimiter), func(ctx context.Context, input *mailLogPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, mailLogs.listMailLogEvents)
	})
}

type transactionalRequestPathInput struct {
	RequestID string `path:"request_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Transactional request ID."`
}

func deliveryErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
			"delivery.request_body_invalid",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
			"api_key.invalid",
		},
		http.StatusForbidden: {
			"delivery.read_denied",
			"api_key.scope_denied",
			"api_key.manage_denied",
		},
		http.StatusNotFound: {
			"delivery.message_not_found",
			"delivery.transactional_request_not_found",
			"sender.domain_not_found",
			"template.not_found",
			"api_key.not_found",
		},
		http.StatusConflict: {
			"delivery.idempotency_key_conflict",
			"api_key.rotate_conflict",
		},
		http.StatusUnprocessableEntity: {
			"delivery.query_invalid",
			"delivery.recipient_invalid",
			"delivery.mode_invalid",
			"delivery.raw_body_required",
			"delivery.subject_required",
			"delivery.attachment_too_large",
			"delivery.attachment_not_supported",
			"delivery.object_storage_disabled",
			"delivery.duplicate_recipient",
			"template.render_payload_invalid",
			"sender.domain_not_verified",
			"delivery.recipient_suppressed",
			"api_key.scope_invalid",
			"api_key.config_invalid",
		},
		http.StatusTooManyRequests: {
			"delivery.request_rate_limited",
			"delivery.quota_exceeded",
		},
		http.StatusServiceUnavailable: {
			"delivery.temporarily_unavailable",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

func registerAPIKeyOperations(api huma.API, apiKey *apiKeyHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "listApiKeys",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/api-keys",
		Tags:        []string{"Access"},
		Summary:     "List API keys",
		Description: "Lists API keys for the workspace. Session-auth only. Supports cursor-based pagination and status filtering.",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, apiKey.listAPIKeys)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "createApiKey",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/api-keys",
		Tags:          []string{"Access"},
		Summary:       "Create API key",
		Description:   "Creates a new API key for the workspace. Supported scopes: transactional.send, transactional.read, mail_logs.read. The secret is returned only at creation time. Recording an audit entry.",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, apiKey.createAPIKey)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "updateApiKey",
		Method:      http.MethodPatch,
		Path:        "/api/v1/workspaces/{workspace_id}/api-keys/{api_key_id}",
		Tags:        []string{"Access"},
		Summary:     "Update or rotate API key",
		Description: "Updates API key name, scopes, or expiration. Set rotate=true to generate a new secret. Recording an audit entry.",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *apiKeyPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, apiKey.updateAPIKey)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "revokeApiKey",
		Method:        http.MethodDelete,
		Path:          "/api/v1/workspaces/{workspace_id}/api-keys/{api_key_id}",
		Tags:          []string{"Access"},
		Summary:       "Revoke API key",
		Description:   "Revokes an API key immediately. The key can no longer be used for authentication. Recording an audit entry.",
		DefaultStatus: http.StatusNoContent,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *apiKeyPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, apiKey.revokeAPIKey)
	})
}

type apiKeyPathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	APIKeyID    string `path:"api_key_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"API key ID."`
}

func registerIngestionWebhookOperations(api huma.API, ingestion *ingestionHTTP) {
	huma.Register(api, huma.Operation{
		OperationID: "ingest-provider-webhook",
		Method:      http.MethodPost,
		Path:        "/api/v1/webhooks/providers/{provider}",
		Tags:        []string{"Webhooks"},
		Summary:     "Ingest a provider webhook",
		Errors:      documentedErrorStatuses(),
	}, func(ctx context.Context, input *providerWebhookInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, ingestion.ingestProviderWebhook)
	})
}

type providerWebhookInput struct {
	Provider string `path:"provider" example:"fake" doc:"Provider identifier."`
}

type webhookConfigPathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	WebhookID   string `path:"webhook_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Webhook config ID."`
}

type webhookConfigListOutput struct {
	Body successEnvelopeDoc[[]webhookConfigListItem]
}

type webhookConfigOutput struct {
	Body successEnvelopeDoc[webhookConfigResponse]
}

type webhookDeliveryPathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	DeliveryID  string `path:"delivery_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Webhook delivery ID."`
}

type webhookDeliveryListOutput struct {
	Body successEnvelopeDoc[[]deliveryListItem]
}

type webhookDeliveryDetailOutput struct {
	Body successEnvelopeDoc[deliveryDetailDoc]
}

func registerWebhookConfigOperations(api huma.API, handler *webhookConfigHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-webhook-configs",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/webhooks",
		Tags:        []string{"Webhooks"},
		Summary:     "List webhook configs",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*webhookConfigListOutput, error) {
		_ = input
		return delegateHTTP[webhookConfigListOutput](ctx, nil, handler.listWebhookConfigs)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "create-webhook-config",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/webhooks",
		Tags:          []string{"Webhooks"},
		Summary:       "Create webhook config",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*webhookConfigOutput, error) {
		_ = input
		return delegateHTTP[webhookConfigOutput](ctx, nil, handler.createWebhookConfig)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "update-webhook-config",
		Method:      http.MethodPatch,
		Path:        "/api/v1/workspaces/{workspace_id}/webhooks/{webhook_id}",
		Tags:        []string{"Webhooks"},
		Summary:     "Update webhook config",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *webhookConfigPathInput) (*webhookConfigOutput, error) {
		_ = input
		return delegateHTTP[webhookConfigOutput](ctx, nil, handler.updateWebhookConfig)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "disable-webhook-config",
		Method:      http.MethodDelete,
		Path:        "/api/v1/workspaces/{workspace_id}/webhooks/{webhook_id}",
		Tags:        []string{"Webhooks"},
		Summary:     "Disable webhook config",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *webhookConfigPathInput) (*boolOutput, error) {
		_ = input
		return delegateHTTP[boolOutput](ctx, nil, handler.disableWebhookConfig)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "rotate-webhook-secret",
		Method:      http.MethodPost,
		Path:        "/api/v1/workspaces/{workspace_id}/webhooks/{webhook_id}/rotate-secret",
		Tags:        []string{"Webhooks"},
		Summary:     "Rotate webhook signing secret",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *webhookConfigPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, handler.rotateWebhookSecret)
	})
}

func registerWebhookDeliveryOperations(api huma.API, handler *webhookDeliveryHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-webhook-deliveries",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/webhook-deliveries",
		Tags:        []string{"Webhooks"},
		Summary:     "List webhook deliveries",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*webhookDeliveryListOutput, error) {
		_ = input
		return delegateHTTP[webhookDeliveryListOutput](ctx, nil, handler.listWebhookDeliveries)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-webhook-delivery",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/webhook-deliveries/{delivery_id}",
		Tags:        []string{"Webhooks"},
		Summary:     "Get webhook delivery detail",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *webhookDeliveryPathInput) (*webhookDeliveryDetailOutput, error) {
		_ = input
		return delegateHTTP[webhookDeliveryDetailOutput](ctx, nil, handler.getWebhookDelivery)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "retry-webhook-delivery",
		Method:      http.MethodPost,
		Path:        "/api/v1/workspaces/{workspace_id}/webhook-deliveries/{delivery_id}/retry",
		Tags:        []string{"Webhooks"},
		Summary:     "Retry a failed webhook delivery",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *webhookDeliveryPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, handler.retryWebhookDelivery)
	})
}

func webhookErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
			"webhook.payload_invalid",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
			"webhook.invalid_signature",
		},
		http.StatusForbidden: {
			"webhook.manage_denied",
			"webhook.delivery_read_denied",
			"webhook.delivery_retry_denied",
		},
		http.StatusNotFound: {
			"webhook.provider_not_supported",
			"webhook.config_not_found",
			"webhook.delivery_not_found",
		},
		http.StatusConflict: {
			"webhook.duplicate_event_conflict",
			"webhook.config_name_conflict",
			"webhook.rotate_conflict",
			"webhook.delivery_retry_conflict",
			"webhook.config_disabled",
		},
		http.StatusUnprocessableEntity: {
			"webhook.target_url_invalid",
			"webhook.subscription_invalid",
			"webhook.config_invalid",
		},
		http.StatusServiceUnavailable: {
			"webhook.ingest_temporarily_unavailable",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

func apiKeyErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"api_key.manage_denied",
		},
		http.StatusNotFound: {
			"api_key.not_found",
		},
		http.StatusConflict: {
			"api_key.rotate_conflict",
		},
		http.StatusUnprocessableEntity: {
			"api_key.scope_invalid",
			"api_key.config_invalid",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

func registerTrackingOperations(api huma.API, tracking *trackingHTTP) {
	huma.Register(api, huma.Operation{
		OperationID: "tracking-serve-open-pixel",
		Method:      http.MethodGet,
		Path:        "/o/{tracking_id}",
		Tags:        []string{"Tracking"},
		Summary:     "Serve open tracking pixel",
		Errors:      documentedErrorStatuses(),
	}, func(ctx context.Context, input *trackingOpenInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, tracking.serveOpenPixel)
	})

	huma.Register(api, huma.Operation{
		OperationID: "tracking-redirect-click",
		Method:      http.MethodGet,
		Path:        "/t/{tracking_id}",
		Tags:        []string{"Tracking"},
		Summary:     "Redirect click tracking link",
		Errors:      documentedErrorStatuses(),
	}, func(ctx context.Context, input *trackingClickInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, tracking.serveClickRedirect)
	})

	huma.Register(api, huma.Operation{
		OperationID: "tracking-unsubscribe",
		Method:      http.MethodGet,
		Path:        "/u/{token}",
		Tags:        []string{"Tracking"},
		Summary:     "Handle unsubscribe request",
		Errors:      documentedErrorStatuses(),
	}, func(ctx context.Context, input *trackingUnsubscribeInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, tracking.serveUnsubscribe)
	})
}

type trackingOpenInput struct {
	TrackingID string `path:"tracking_id" doc:"Tracking ID for the open pixel."`
}

type trackingClickInput struct {
	TrackingID string `path:"tracking_id" doc:"Tracking ID for the click redirect."`
}

type trackingUnsubscribeInput struct {
	Token string `path:"token" doc:"Unsubscribe token."`
}

func trackingErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"suppression.unsubscribe_token_invalid",
		},
		http.StatusNotFound: {
			"tracking.invalid_tracking_id",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

type analyticsWorkspacePathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
}

type analyticsCampaignPathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	CampaignID  string `path:"campaign_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Campaign ID."`
}

type analyticsMessagePathInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	MessageID   string `path:"message_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Message ID."`
}

type analyticsProviderEventPathInput struct {
	WorkspaceID     string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	ProviderEventID string `path:"provider_event_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Provider event ID."`
}

func registerAnalyticsOperations(api huma.API, analytics *analyticsHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-analytics-overview",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/overview",
		Tags:        []string{"Analytics"},
		Summary:     "Get workspace analytics overview (Postgres projection-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getDashboardOverview)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-campaign-analytics",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/campaigns/{campaign_id}",
		Tags:        []string{"Analytics"},
		Summary:     "Get campaign analytics summary (Postgres projection-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsCampaignPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getCampaignAnalytics)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-campaign-analytics-funnel",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/campaigns/{campaign_id}/funnel",
		Tags:        []string{"Analytics"},
		Summary:     "Get campaign analytics funnel (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsCampaignPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getCampaignFunnel)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-campaign-analytics-timeseries",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/campaigns/{campaign_id}/timeseries",
		Tags:        []string{"Analytics"},
		Summary:     "Get campaign analytics time series (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsCampaignPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getCampaignTimeSeries)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-campaign-analytics-breakdown",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/campaigns/{campaign_id}/breakdown",
		Tags:        []string{"Analytics"},
		Summary:     "Get campaign analytics breakdown (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsCampaignPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getCampaignBreakdown)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-campaign-analytics-events",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/campaigns/{campaign_id}/events",
		Tags:        []string{"Analytics"},
		Summary:     "List campaign analytics events (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsCampaignPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getCampaignEvents)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-deliverability-analytics",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/deliverability",
		Tags:        []string{"Analytics"},
		Summary:     "Get deliverability analytics (Postgres projection-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getDeliverability)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-deliverability-timeseries",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/deliverability/timeseries",
		Tags:        []string{"Analytics"},
		Summary:     "Get deliverability time series (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getDeliverabilityTimeSeries)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-deliverability-breakdown",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/deliverability/breakdown",
		Tags:        []string{"Analytics"},
		Summary:     "Get deliverability breakdown (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getDeliverabilityBreakdown)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-deliverability-latency",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/deliverability/latency",
		Tags:        []string{"Analytics"},
		Summary:     "Get deliverability latency percentiles (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getDeliverabilityLatency)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-deliverability-incidents",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/deliverability/incidents",
		Tags:        []string{"Analytics"},
		Summary:     "Get deliverability incident windows (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getDeliverabilityIncidents)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "search-events",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/events",
		Tags:        []string{"Analytics"},
		Summary:     "Search analytics events across the workspace (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.searchEvents)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-message-timeline",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/messages/{message_id}/timeline",
		Tags:        []string{"Analytics"},
		Summary:     "Get message event timeline (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsMessagePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getMessageTimeline)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-provider-event-trace",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/provider-events/{provider_event_id}",
		Tags:        []string{"Analytics"},
		Summary:     "Get provider event trace (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsProviderEventPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getProviderEventTrace)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-campaign-incident-timeline",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/campaigns/{campaign_id}/incident-timeline",
		Tags:        []string{"Analytics"},
		Summary:     "Get campaign incident timeline (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsCampaignPathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getCampaignIncidentTimeline)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-operations-outbox-lag",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/operations/outbox-lag",
		Tags:        []string{"Analytics"},
		Summary:     "Get outbox lag analytics (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getOutboxLag)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-operations-consumer-failures",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/operations/consumer-failures",
		Tags:        []string{"Analytics"},
		Summary:     "Get consumer failure analytics (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getConsumerFailures)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-operations-dlq-volume",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/operations/dlq",
		Tags:        []string{"Analytics"},
		Summary:     "Get dead letter queue volume analytics (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getDLQVolume)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-webhook-delivery-timeseries",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/webhooks/delivery-timeseries",
		Tags:        []string{"Analytics"},
		Summary:     "Get webhook delivery time series (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getWebhookDeliveryTimeSeries)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-webhook-reliability",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/webhooks/reliability",
		Tags:        []string{"Analytics"},
		Summary:     "Get webhook delivery reliability (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getWebhookReliability)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-usage-timeseries",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/usage/timeseries",
		Tags:        []string{"Analytics"},
		Summary:     "Get workspace usage time series (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getUsageTimeSeries)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-usage-features",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/usage/features",
		Tags:        []string{"Analytics"},
		Summary:     "Get workspace feature adoption metrics (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getUsageFeatures)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-risk-signals",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/risk/signals",
		Tags:        []string{"Analytics"},
		Summary:     "Get workspace risk signals (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getRiskSignals)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-send-volume-forecast",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/forecast/send-volume",
		Tags:        []string{"Analytics"},
		Summary:     "Get send volume forecast (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getSendVolumeForecast)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-anomalies",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/analytics/anomalies",
		Tags:        []string{"Analytics"},
		Summary:     "Get workspace anomaly detections (ClickHouse-backed).",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *analyticsWorkspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, analytics.getAnomalies)
	})

}

func analyticsErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
			"analytics.query_invalid",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"analytics.read_denied",
		},
		http.StatusNotFound: {
			"analytics.projection_not_found",
		},
		http.StatusServiceUnavailable: {
			"analytics.store_unavailable",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

func operationsErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
			"operations.filter_invalid",
			"operations.replay_target_invalid",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"operations.queue_read_denied",
			"operations.dlq_read_denied",
			"operations.replay_manage_denied",
		},
		http.StatusNotFound: {
			"operations.outbox_record_not_found",
			"operations.dead_letter_record_not_found",
			"operations.replay_job_not_found",
		},
		http.StatusConflict: {
			"operations.replay_conflict",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

type settingsGetOutput struct {
	Body successEnvelopeDoc[settingsDoc]
}

type settingsDoc struct {
	Version         int64            `json:"version" example:"1" doc:"Settings version for optimistic concurrency."`
	EmailDefaults   emailDefaultsDoc `json:"email_defaults" doc:"Email default configuration."`
	FeatureControls map[string]any   `json:"feature_controls" doc:"Feature control toggles."`
	UpdatedByUserID string           `json:"updated_by_user_id,omitempty" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"User ID that last updated settings."`
	CreatedAt       time.Time        `json:"created_at" doc:"Creation timestamp."`
	UpdatedAt       time.Time        `json:"updated_at" doc:"Update timestamp."`
}

type emailDefaultsDoc struct {
	DefaultSenderDomainID string `json:"default_sender_domain_id,omitempty" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Default sender domain ID."`
}

type settingsUpdateInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	Body        struct {
		Version         *int64            `json:"version,omitempty" example:"1" doc:"Expected current version for optimistic concurrency."`
		EmailDefaults   *emailDefaultsDoc `json:"email_defaults,omitempty" doc:"Email default configuration to update."`
		FeatureControls *map[string]any   `json:"feature_controls,omitempty" doc:"Feature control toggles to update."`
	} `required:"true" nameHint:"UpdateSettingsRequest"`
}

type auditListInput struct {
	WorkspaceID string `path:"workspace_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Workspace ID."`
	ActorUserID string `query:"actor_user_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Filter by actor user ID."`
	ActionType  string `query:"action_type" example:"workspace.settings.updated" doc:"Filter by action type."`
	TargetType  string `query:"target_type" example:"workspace_settings" doc:"Filter by target type."`
	TargetID    string `query:"target_id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Filter by target ID."`
	From        string `query:"from" example:"2024-01-01T00:00:00Z" doc:"Filter entries after this timestamp (RFC3339)."`
	To          string `query:"to" example:"2024-12-31T23:59:59Z" doc:"Filter entries before this timestamp (RFC3339)."`
	Limit       int    `query:"limit" example:"50" doc:"Maximum number of entries to return (1-100)."`
	Cursor      string `query:"cursor" doc:"Pagination cursor from previous response."`
}

type auditListOutput struct {
	Body successEnvelopeDoc[auditListResultDoc]
}

type auditListResultDoc struct {
	Entries    []auditEntryDoc `json:"entries" doc:"Audit entries."`
	NextCursor string          `json:"next_cursor" doc:"Pagination cursor for next page."`
}

type auditEntryDoc struct {
	ID             string         `json:"id" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Audit entry ID."`
	ActorUserID    string         `json:"actor_user_id,omitempty" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"User ID that performed the action."`
	ActionType     string         `json:"action_type" example:"workspace.settings.updated" doc:"Type of action performed."`
	TargetType     string         `json:"target_type,omitempty" example:"workspace_settings" doc:"Type of target resource."`
	TargetID       string         `json:"target_id,omitempty" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"ID of target resource."`
	PayloadSummary map[string]any `json:"payload_summary,omitempty" doc:"Sanitized action payload summary."`
	RequestID      string         `json:"request_id,omitempty" example:"018ff2d5-f49c-77f1-a3c5-5137560c97c8" doc:"Request correlation ID."`
	OccurredAt     time.Time      `json:"occurred_at" doc:"When the action occurred."`
}

func registerSettingsOperations(api huma.API, settings *settingsHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "getSettings",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/settings",
		Tags:        []string{"Settings"},
		Summary:     "Get workspace settings",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*settingsGetOutput, error) {
		_ = input
		return delegateHTTP[settingsGetOutput](ctx, nil, settings.getSettings)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "updateSettings",
		Method:      http.MethodPatch,
		Path:        "/api/v1/workspaces/{workspace_id}/settings",
		Tags:        []string{"Settings"},
		Summary:     "Update workspace settings",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *settingsUpdateInput) (*settingsGetOutput, error) {
		return delegateHTTP[settingsGetOutput](ctx, jsonBody(input.Body), settings.updateSettings)
	})
}

func registerAuditOperations(api huma.API, audit *auditHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "listAuditLogs",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/audit-logs",
		Tags:        []string{"Audit"},
		Summary:     "List workspace audit logs",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *auditListInput) (*auditListOutput, error) {
		_ = input
		return delegateHTTP[auditListOutput](ctx, nil, audit.listAuditLogs)
	})
}

func settingsErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"settings.manage_denied",
		},
		http.StatusNotFound: {
			"identity.workspace_not_found",
		},
		http.StatusConflict: {
			"settings.version_conflict",
		},
		http.StatusUnprocessableEntity: {
			"settings.payload_invalid",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

func auditErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
			"audit.filter_invalid",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"audit.read_denied",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}

func registerOperationsRoutes(api huma.API, ops *operationsHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	tag := []string{"Operations"}

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-outbox-summary",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/operations/outbox/summary",
		Tags:        tag,
		Summary:     "Get outbox summary for a workspace",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, nil, ops.getOutboxSummary)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-outbox-events",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/operations/outbox",
		Tags:        tag,
		Summary:     "List outbox events for a workspace",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, nil, ops.listOutboxRecords)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-outbox-event",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/operations/outbox/{outbox_id}",
		Tags:        tag,
		Summary:     "Get outbox event details",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, nil, ops.getOutboxRecord)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-dead-letter-records",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/operations/dead-letter-records",
		Tags:        tag,
		Summary:     "List dead letter records for a workspace",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, nil, ops.listDeadLetterRecords)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-dead-letter-record",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/operations/dead-letter-records/{dead_letter_id}",
		Tags:        tag,
		Summary:     "Get dead letter record details",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, nil, ops.getDeadLetterRecord)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "create-replay-job",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/operations/replay-jobs",
		Tags:          tag,
		Summary:       "Create a replay job",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, nil, ops.createReplayJob)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "list-replay-jobs",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/operations/replay-jobs",
		Tags:        tag,
		Summary:     "List replay jobs for a workspace",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, nil, ops.listReplayJobs)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "get-replay-job",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/operations/replay-jobs/{replay_job_id}",
		Tags:        tag,
		Summary:     "Get replay job details",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, nil, ops.getReplayJob)
	})
}

type notificationListInput struct {
	WorkspaceID string `path:"workspace_id" doc:"Workspace ID."`
	Type        string `query:"type" doc:"Filter by notification type."`
	Status      string `query:"status" doc:"Filter by notification status."`
	From        string `query:"from" doc:"Filter entries after this timestamp (RFC3339)."`
	To          string `query:"to" doc:"Filter entries before this timestamp (RFC3339)."`
	Limit       int    `query:"limit" doc:"Maximum number of entries to return (1-100)."`
	Cursor      string `query:"cursor" doc:"Pagination cursor from previous response."`
}

type notificationListOutput struct {
	Body successEnvelopeDoc[notificationListResultDoc]
}

type notificationListResultDoc struct {
	Notifications []notificationDoc `json:"notifications" doc:"List of notification messages."`
	NextCursor    string            `json:"next_cursor" doc:"Pagination cursor for next page."`
}

type notificationDoc struct {
	ID              string `json:"id" doc:"Notification message ID."`
	WorkspaceID     string `json:"workspace_id,omitempty" doc:"Workspace ID."`
	Type            string `json:"type" doc:"Notification type."`
	Status          string `json:"status" doc:"Current status."`
	RecipientEmail  string `json:"recipient_email" doc:"Recipient email address."`
	RecipientUserID string `json:"recipient_user_id,omitempty" doc:"Recipient user ID."`
	Subject         string `json:"subject" doc:"Email subject."`
	MaxAttempts     int    `json:"max_attempts" doc:"Maximum send attempts."`
	AttemptCount    int    `json:"attempt_count" doc:"Current attempt count."`
	LastAttemptAt   string `json:"last_attempt_at,omitempty" doc:"Last attempt timestamp."`
	CreatedAt       string `json:"created_at" doc:"Creation timestamp."`
	UpdatedAt       string `json:"updated_at" doc:"Update timestamp."`
}

type notificationDetailDoc struct {
	Notification notificationDetailResultDoc `json:"notification" doc:"Notification message with attempts."`
}

type notificationDetailResultDoc struct {
	ID              string       `json:"id" doc:"Notification message ID."`
	WorkspaceID     string       `json:"workspace_id,omitempty" doc:"Workspace ID."`
	Type            string       `json:"type" doc:"Notification type."`
	Status          string       `json:"status" doc:"Current status."`
	RecipientEmail  string       `json:"recipient_email" doc:"Recipient email address."`
	RecipientUserID string       `json:"recipient_user_id,omitempty" doc:"Recipient user ID."`
	Subject         string       `json:"subject" doc:"Email subject."`
	MaxAttempts     int          `json:"max_attempts" doc:"Maximum send attempts."`
	AttemptCount    int          `json:"attempt_count" doc:"Current attempt count."`
	LastAttemptAt   string       `json:"last_attempt_at,omitempty" doc:"Last attempt timestamp."`
	CreatedAt       string       `json:"created_at" doc:"Creation timestamp."`
	UpdatedAt       string       `json:"updated_at" doc:"Update timestamp."`
	Attempts        []attemptDoc `json:"attempts" doc:"Send attempts."`
}

type attemptDoc struct {
	ID                    string `json:"id" doc:"Attempt ID."`
	NotificationMessageID string `json:"notification_message_id" doc:"Parent notification message ID."`
	AttemptNumber         int    `json:"attempt_number" doc:"Sequential attempt number."`
	Status                string `json:"status" doc:"Attempt status."`
	Provider              string `json:"provider" doc:"Email provider used."`
	ProviderMessageID     string `json:"provider_message_id,omitempty" doc:"Provider message ID."`
	ErrorMessage          string `json:"error_message,omitempty" doc:"Error message on failure."`
	AttemptedAt           string `json:"attempted_at" doc:"Attempt timestamp."`
}

type notificationPathInput struct {
	WorkspaceID    string `path:"workspace_id" doc:"Workspace ID."`
	NotificationID string `path:"notification_id" doc:"Notification message ID."`
}

type notificationAlertInput struct {
	WorkspaceID string `path:"workspace_id" doc:"Workspace ID."`
	Body        struct {
		RecipientEmail string `json:"recipient_email" required:"true" doc:"Recipient email address."`
		Subject        string `json:"subject" required:"true" doc:"Email subject."`
		Body           string `json:"body" required:"true" doc:"Alert body content."`
	}
}

type notificationAlertOutput struct {
	Body successEnvelopeDoc[notificationDetailDoc]
}

func registerNotificationOperations(api huma.API, notification *notificationHTTP, authMiddleware func(huma.Context, func(huma.Context))) {
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "listNotifications",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/notifications",
		Tags:        []string{"Notifications"},
		Summary:     "List workspace notification messages",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *notificationListInput) (*notificationListOutput, error) {
		_ = input
		return delegateHTTP[notificationListOutput](ctx, nil, notification.listNotifications)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "getNotificationStatus",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/notifications/{notification_id}",
		Tags:        []string{"Notifications"},
		Summary:     "Get notification message with attempts",
		Errors:      documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *notificationPathInput) (*notificationDetailDoc, error) {
		_ = input
		return delegateHTTP[notificationDetailDoc](ctx, nil, notification.getNotificationStatus)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID:   "sendSystemAlert",
		Method:        http.MethodPost,
		Path:          "/api/v1/workspaces/{workspace_id}/notifications/alert",
		Tags:          []string{"Notifications"},
		Summary:       "Send a system alert notification",
		DefaultStatus: http.StatusCreated,
		Errors:        documentedErrorStatuses(),
	}, authMiddleware), func(ctx context.Context, input *notificationAlertInput) (*notificationAlertOutput, error) {
		return delegateHTTP[notificationAlertOutput](ctx, jsonBody(input.Body), notification.sendSystemAlert)
	})
	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "streamWorkspaceNotifications",
		Method:      http.MethodGet,
		Path:        "/api/v1/workspaces/{workspace_id}/notifications/stream",
		Tags:        []string{"Notifications"},
		Summary:     "Stream workspace notification events via SSE",
		Description: "Opens an SSE connection for receiving realtime notification events for the workspace. Requires notification.read permission.",
	}, authMiddleware), func(ctx context.Context, input *workspacePathInput) (*emptyOutput, error) {
		_ = input
		return delegateHTTP[emptyOutput](ctx, nil, notification.streamWorkspaceNotifications)
	})

	huma.Register(api, protectedOperation(huma.Operation{
		OperationID: "streamMyNotifications",
		Method:      http.MethodGet,
		Path:        "/api/v1/users/me/notifications/stream",
		Tags:        []string{"Notifications"},
		Summary:     "Stream my notification events via SSE",
		Description: "Opens an SSE connection for receiving realtime notification events for the authenticated user. No workspace permission required.",
	}, authMiddleware), func(ctx context.Context, _ *struct{}) (*emptyOutput, error) {
		return delegateHTTP[emptyOutput](ctx, nil, notification.streamMyNotifications)
	})

}

func notificationErrorCodes() map[int][]string {
	return map[int][]string{
		http.StatusBadRequest: {
			"auth.invalid_request_body",
			"notification.filter_invalid",
		},
		http.StatusUnauthorized: {
			"auth.invalid_token",
		},
		http.StatusForbidden: {
			"notification.read_denied",
		},
		http.StatusNotFound: {
			"notification.not_found",
		},
		http.StatusInternalServerError: {
			"internal.error",
		},
	}
}
