package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/ratelimit"
)

type humaContextKey string

const (
	ctxHumaRequest        humaContextKey = "huma_request"
	ctxHumaResponseWriter humaContextKey = "huma_response_writer"
)

func openAPIConfig() huma.Config {
	cfg := huma.DefaultConfig("Sendflow API", "1.0.0")
	cfg.Info.Description = "HTTP API for send-flow authentication, workspace management, health, and operations."
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
	}
	return cfg
}

func registerOpenAPIRoutes(api huma.API, r chi.Router, healthSvc *platformhealth.Service, authSvc *identityapp.Service, authRateLimiter ratelimit.Service, secureCookies bool) {
	api.UseMiddleware(captureHTTPContext)

	r.Get("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/openapi+json")
		if err := json.NewEncoder(w).Encode(api.OpenAPI()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	registerHealthOperations(api, healthSvc)

	if authSvc == nil {
		return
	}
	auth := newAuthHTTP(authSvc, authRateLimiter, secureCookies)
	workspace := newWorkspaceHTTP(authSvc)
	authMiddleware := humaAuthzMiddleware(authSvc)

	registerAuthOperations(api, auth, authMiddleware)
	registerWorkspaceOperations(api, workspace, authMiddleware)
	documentApplicationErrors(api.OpenAPI())
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
	op.Middlewares = append(op.Middlewares, middleware)
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
	case hasTag(op, "Workspaces"):
		return workspaceErrorCodes()
	default:
		return map[int][]string{
			http.StatusInternalServerError: {"health.runtime_not_ready"},
		}
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
			"health.runtime_not_ready",
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
			"health.runtime_not_ready",
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
