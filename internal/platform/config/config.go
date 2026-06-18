package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/sliceutil"
)

type Config struct {
	AppName          string
	AppEnv           string
	RuntimeIdentity  RuntimeIdentityConfig
	HTTPAddr         string
	LogLevel         string
	AutoMigrate      bool
	DatabaseURL      string
	DatabaseReadURL  string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	KafkaBrokers     string
	ClickHouseDSN    string
	EmailProvider    string
	FrontendBaseURL  string
	SMTP             SMTPConfig
	SES              SESConfig
	ObjectStorage    ObjectStorageConfig
	ServiceDiscovery ServiceDiscoveryConfig
	WorkerDiscovery  ServiceDiscoveryConfig
	ShutdownTimeout  time.Duration

	WorkerHTTPAddr            string
	WorkerEnabledConsumers    []string
	WorkerConcurrency         int
	WorkerShutdownTimeout     time.Duration
	WorkerConsumerGroupPrefix string

	JWTIssuer              string
	JWTAccessSecret        string
	JWTRefreshSecret       string
	JWTAccessTTL           time.Duration
	JWTRefreshTTL          time.Duration
	OAuthStateTTL          time.Duration
	OAuthGoogleClientID    string
	OAuthGoogleSecret      string
	OAuthGithubClientID    string
	OAuthGithubSecret      string
	EmailVerificationTTL   time.Duration
	PasswordResetTTL       time.Duration
	MFAChallengeTTL        time.Duration
	FakeWebhookSecret      string
	UnsubscribeTokenSecret string

	RedisCache RedisFeatures
}

type RedisFeatures struct {
	IdentityAccessCacheTTL       time.Duration
	IdentitySettingsCacheTTL     time.Duration
	ContentPreviewCacheTTL       time.Duration
	SenderReadinessCacheTTL      time.Duration
	AudienceResolutionCacheTTL   time.Duration
	DeliveryIdempotencyTTL       time.Duration
	DeliveryQuotaCacheTTL        time.Duration
	DeliveryAPIKeyQuotaLimitsTTL time.Duration
	AnalyticsQueryCacheTTL       time.Duration
}

type RuntimeIdentityConfig struct {
	ServiceName  string
	InstanceID   string
	InstanceAddr string
	PodName      string
	Namespace    string
	NodeName     string
}

type DatabaseConfig struct {
	URL     string
	ReadURL string
}

type JWTConfig struct {
	Issuer        string
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

type OAuthConfig struct {
	StateTTL       time.Duration
	GoogleClientID string
	GoogleSecret   string
	GithubClientID string
	GithubSecret   string
}

type ObjectStorageConfig struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	ForcePathStyle  bool
	UseSSL          bool
}

type ServiceDiscoveryConfig struct {
	Provider        string
	ConsulHTTPAddr  string
	ServiceName     string
	ServiceID       string
	ServiceAddress  string
	ServicePort     int
	HealthCheckPath string
}

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type SESConfig struct {
	Region        string
	AccessKeyID   string
	SecretKey     string
	SessionToken  string
	Endpoint      string
	From          string
	Configuration string
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		AppName:         getenv("APP_NAME", "send-flow-backend"),
		ClickHouseDSN:   os.Getenv("CLICKHOUSE_DSN"),
		AppEnv:          getenv("APP_ENV", "local"),
		HTTPAddr:        getenv("HTTP_ADDR", ":8081"),
		LogLevel:        getenv("LOG_LEVEL", "info"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		DatabaseReadURL: os.Getenv("DATABASE_READ_URL"),
		RedisAddr:       getenv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		KafkaBrokers:    os.Getenv("KAFKA_BROKERS"),
		EmailProvider:   getenv("EMAIL_PROVIDER", "smtp"),
		FrontendBaseURL: os.Getenv("FRONTEND_BASE_URL"),
		SMTP: SMTPConfig{
			Host:     getenv("SMTP_HOST", "localhost"),
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     getenv("SMTP_FROM", "noreply@sendflow.local"),
		},
		SES: SESConfig{
			Region:        getenv("SES_REGION", "us-east-1"),
			AccessKeyID:   os.Getenv("SES_ACCESS_KEY_ID"),
			SecretKey:     os.Getenv("SES_SECRET_ACCESS_KEY"),
			SessionToken:  os.Getenv("SES_SESSION_TOKEN"),
			Endpoint:      os.Getenv("SES_ENDPOINT"),
			From:          os.Getenv("SES_FROM"),
			Configuration: os.Getenv("SES_CONFIGURATION_SET"),
		},
		ObjectStorage: ObjectStorageConfig{
			Endpoint:        os.Getenv("OBJECT_STORAGE_ENDPOINT"),
			Region:          getenv("OBJECT_STORAGE_REGION", "us-east-1"),
			AccessKeyID:     os.Getenv("OBJECT_STORAGE_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("OBJECT_STORAGE_SECRET_ACCESS_KEY"),
			Bucket:          getenv("OBJECT_STORAGE_BUCKET", "sendflow-local"),
		},
		ServiceDiscovery: ServiceDiscoveryConfig{
			Provider:        strings.ToLower(strings.TrimSpace(os.Getenv("SERVICE_DISCOVERY_PROVIDER"))),
			ConsulHTTPAddr:  getenv("CONSUL_HTTP_ADDR", "http://localhost:8500"),
			ServiceName:     getenv("CONSUL_SERVICE_NAME", "sendflow-api"),
			ServiceID:       getenv("CONSUL_SERVICE_ID", "sendflow-api-local"),
			ServiceAddress:  getenv("CONSUL_SERVICE_ADDRESS", "host.docker.internal"),
			HealthCheckPath: getenv("CONSUL_HEALTH_CHECK_PATH", "/api/readyz"),
		},
		WorkerHTTPAddr:            getenv("WORKER_HTTP_ADDR", ":8082"),
		WorkerEnabledConsumers:    parseCSV("WORKER_ENABLED_CONSUMERS"),
		WorkerConsumerGroupPrefix: getenv("WORKER_CONSUMER_GROUP_PREFIX", "send-flow"),

		JWTIssuer:              getenv("JWT_ISSUER", "send-flow"),
		JWTAccessSecret:        os.Getenv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret:       os.Getenv("JWT_REFRESH_SECRET"),
		OAuthGoogleClientID:    os.Getenv("OAUTH_GOOGLE_CLIENT_ID"),
		OAuthGoogleSecret:      os.Getenv("OAUTH_GOOGLE_CLIENT_SECRET"),
		OAuthGithubClientID:    os.Getenv("OAUTH_GITHUB_CLIENT_ID"),
		OAuthGithubSecret:      os.Getenv("OAUTH_GITHUB_CLIENT_SECRET"),
		FakeWebhookSecret:      os.Getenv("FAKE_WEBHOOK_SECRET"),
		UnsubscribeTokenSecret: os.Getenv("UNSUBSCRIBE_TOKEN_SECRET"),
	}

	var errs []error

	if v, err := parseBool("AUTO_MIGRATE", false); err != nil {
		errs = append(errs, fmt.Errorf("AUTO_MIGRATE: %w", err))
	} else {
		cfg.AutoMigrate = v
	}
	if v, err := parseInt("SMTP_PORT", 1025); err != nil {
		errs = append(errs, fmt.Errorf("SMTP_PORT: %w", err))
	} else {
		cfg.SMTP.Port = v
	}
	if v, err := parseBool("OBJECT_STORAGE_FORCE_PATH_STYLE", true); err != nil {
		errs = append(errs, fmt.Errorf("OBJECT_STORAGE_FORCE_PATH_STYLE: %w", err))
	} else {
		cfg.ObjectStorage.ForcePathStyle = v
	}
	if v, err := parseBool("OBJECT_STORAGE_USE_SSL", false); err != nil {
		errs = append(errs, fmt.Errorf("OBJECT_STORAGE_USE_SSL: %w", err))
	} else {
		cfg.ObjectStorage.UseSSL = v
	}
	if v, err := parseInt("CONSUL_SERVICE_PORT", 8081); err != nil {
		errs = append(errs, fmt.Errorf("CONSUL_SERVICE_PORT: %w", err))
	} else {
		cfg.ServiceDiscovery.ServicePort = v
	}
	cfg.WorkerDiscovery = ServiceDiscoveryConfig{
		Provider:        strings.ToLower(strings.TrimSpace(getenv("WORKER_SERVICE_DISCOVERY_PROVIDER", cfg.ServiceDiscovery.Provider))),
		ConsulHTTPAddr:  getenv("WORKER_CONSUL_HTTP_ADDR", cfg.ServiceDiscovery.ConsulHTTPAddr),
		ServiceName:     getenv("CONSUL_WORKER_SERVICE_NAME", "sendflow-worker"),
		ServiceID:       getenv("CONSUL_WORKER_SERVICE_ID", "sendflow-worker-local"),
		ServiceAddress:  getenv("CONSUL_WORKER_SERVICE_ADDRESS", cfg.ServiceDiscovery.ServiceAddress),
		HealthCheckPath: getenv("CONSUL_WORKER_HEALTH_CHECK_PATH", "/api/readyz"),
		ServicePort:     portFromAddr(cfg.WorkerHTTPAddr, 8082),
	}
	if v, err := parseInt("CONSUL_WORKER_SERVICE_PORT", cfg.WorkerDiscovery.ServicePort); err != nil {
		errs = append(errs, fmt.Errorf("CONSUL_WORKER_SERVICE_PORT: %w", err))
	} else {
		cfg.WorkerDiscovery.ServicePort = v
	}
	cfg.RuntimeIdentity = runtimeIdentity(cfg)
	if v, err := parseDuration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err))
	} else {
		cfg.ShutdownTimeout = v
	}
	if v, err := parseInt("WORKER_CONCURRENCY", 1); err != nil {
		errs = append(errs, fmt.Errorf("WORKER_CONCURRENCY: %w", err))
	} else {
		cfg.WorkerConcurrency = v
	}
	if v, err := parseDuration("WORKER_SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		errs = append(errs, fmt.Errorf("WORKER_SHUTDOWN_TIMEOUT: %w", err))
	} else {
		cfg.WorkerShutdownTimeout = v
	}
	if v, err := parseDuration("JWT_ACCESS_TTL", 15*time.Minute); err != nil {
		errs = append(errs, fmt.Errorf("JWT_ACCESS_TTL: %w", err))
	} else {
		cfg.JWTAccessTTL = v
	}
	if v, err := parseDuration("JWT_REFRESH_TTL", 7*24*time.Hour); err != nil {
		errs = append(errs, fmt.Errorf("JWT_REFRESH_TTL: %w", err))
	} else {
		cfg.JWTRefreshTTL = v
	}
	if v, err := parseDuration("OAUTH_STATE_TTL", 10*time.Minute); err != nil {
		errs = append(errs, fmt.Errorf("OAUTH_STATE_TTL: %w", err))
	} else {
		cfg.OAuthStateTTL = v
	}
	if v, err := parseDuration("AUTH_EMAIL_VERIFICATION_TTL", 24*time.Hour); err != nil {
		errs = append(errs, fmt.Errorf("AUTH_EMAIL_VERIFICATION_TTL: %w", err))
	} else {
		cfg.EmailVerificationTTL = v
	}
	if v, err := parseDuration("AUTH_PASSWORD_RESET_TTL", time.Hour); err != nil {
		errs = append(errs, fmt.Errorf("AUTH_PASSWORD_RESET_TTL: %w", err))
	} else {
		cfg.PasswordResetTTL = v
	}
	if v, err := parseDuration("AUTH_MFA_CHALLENGE_TTL", 10*time.Minute); err != nil {
		errs = append(errs, fmt.Errorf("AUTH_MFA_CHALLENGE_TTL: %w", err))
	} else {
		cfg.MFAChallengeTTL = v
	}

	redisDB, err := strconv.Atoi(getenv("REDIS_DB", "0"))
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid REDIS_DB: %w", err))
	} else {
		cfg.RedisDB = redisDB
	}

	cfg.RedisCache = RedisFeatures{
		IdentityAccessCacheTTL:       parseDurationOrDefault("REDIS_CACHE_IDENTITY_ACCESS_TTL", 5*time.Minute),
		IdentitySettingsCacheTTL:     parseDurationOrDefault("REDIS_CACHE_IDENTITY_SETTINGS_TTL", 5*time.Minute),
		ContentPreviewCacheTTL:       parseDurationOrDefault("REDIS_CACHE_CONTENT_PREVIEW_TTL", 10*time.Minute),
		SenderReadinessCacheTTL:      parseDurationOrDefault("REDIS_CACHE_SENDER_READINESS_TTL", 5*time.Minute),
		AudienceResolutionCacheTTL:   parseDurationOrDefault("REDIS_CACHE_AUDIENCE_RESOLUTION_TTL", 2*time.Minute),
		DeliveryIdempotencyTTL:       parseDurationOrDefault("REDIS_CACHE_DELIVERY_IDEMPOTENCY_TTL", 24*time.Hour),
		DeliveryQuotaCacheTTL:        parseDurationOrDefault("REDIS_CACHE_DELIVERY_QUOTA_TTL", time.Minute),
		DeliveryAPIKeyQuotaLimitsTTL: parseDurationOrDefault("REDIS_CACHE_API_KEY_QUOTA_LIMITS_TTL", 5*time.Minute),
		AnalyticsQueryCacheTTL:       parseDurationOrDefault("REDIS_CACHE_ANALYTICS_QUERY_TTL", time.Minute),
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func runtimeIdentity(cfg Config) RuntimeIdentityConfig {
	serviceName := strings.TrimSpace(os.Getenv("SERVICE_NAME"))
	podName := strings.TrimSpace(os.Getenv("POD_NAME"))
	identity := RuntimeIdentityConfig{
		ServiceName:  serviceName,
		InstanceID:   strings.TrimSpace(os.Getenv("INSTANCE_ID")),
		InstanceAddr: strings.TrimSpace(os.Getenv("INSTANCE_ADDR")),
		PodName:      podName,
		Namespace:    strings.TrimSpace(os.Getenv("POD_NAMESPACE")),
		NodeName:     strings.TrimSpace(os.Getenv("NODE_NAME")),
	}
	if identity.InstanceAddr == "" {
		switch serviceName {
		case "worker":
			identity.InstanceAddr = cfg.WorkerHTTPAddr
		case "api":
			identity.InstanceAddr = cfg.HTTPAddr
		}
	}
	if identity.InstanceID == "" {
		switch serviceName {
		case "worker":
			identity.InstanceID = strings.TrimSpace(os.Getenv("CONSUL_WORKER_SERVICE_ID"))
		case "api":
			identity.InstanceID = strings.TrimSpace(os.Getenv("CONSUL_SERVICE_ID"))
		}
	}
	if identity.InstanceID == "" {
		identity.InstanceID = podName
	}
	if identity.InstanceID == "" {
		switch serviceName {
		case "worker":
			identity.InstanceID = cfg.WorkerDiscovery.ServiceID
		case "api":
			identity.InstanceID = cfg.ServiceDiscovery.ServiceID
		}
	}
	return identity
}

var devSecrets = []string{
	"dev-access-secret-change-me",
	"dev-refresh-secret-change-me",
	"dev-unsubscribe-secret-change-me",
}

func isDevSecret(s string) bool {
	for _, d := range devSecrets {
		if s == d {
			return true
		}
	}
	return false
}

// WarnDevSecrets logs a warning if any known dev secrets are detected.
func WarnDevSecrets(log *slog.Logger, cfg Config) {
	if isDevSecret(cfg.JWTAccessSecret) {
		log.Warn("JWT_ACCESS_SECRET is set to a known dev default, change it for non-local environments")
	}
	if isDevSecret(cfg.JWTRefreshSecret) {
		log.Warn("JWT_REFRESH_SECRET is set to a known dev default, change it for non-local environments")
	}
	if isDevSecret(cfg.UnsubscribeTokenSecret) {
		log.Warn("UNSUBSCRIBE_TOKEN_SECRET is set to a known dev default, change it for non-local environments")
	}
}

func (c Config) Validate() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.RedisAddr == "" {
		return errors.New("REDIS_ADDR is required")
	}
	if c.JWTAccessSecret == "" {
		return errors.New("JWT_ACCESS_SECRET is required")
	}
	if c.JWTRefreshSecret == "" {
		return errors.New("JWT_REFRESH_SECRET is required")
	}
	if c.AppEnv != "local" {
		if isDevSecret(c.JWTAccessSecret) {
			return errors.New("JWT_ACCESS_SECRET must not be a dev default in non-local environment")
		}
		if isDevSecret(c.JWTRefreshSecret) {
			return errors.New("JWT_REFRESH_SECRET must not be a dev default in non-local environment")
		}
		if isDevSecret(c.UnsubscribeTokenSecret) {
			return errors.New("UNSUBSCRIBE_TOKEN_SECRET must not be a dev default in non-local environment")
		}
	}
	if c.EmailProvider != "smtp" && c.EmailProvider != "ses" && c.EmailProvider != "fake" {
		return errors.New("EMAIL_PROVIDER must be one of: smtp, ses, fake")
	}
	if c.EmailProvider == "fake" && c.AppEnv != "local" && c.AppEnv != "test" {
		return errors.New("EMAIL_PROVIDER=fake is not allowed outside local/test environment")
	}
	if c.EmailProvider == "smtp" {
		if c.SMTP.Host == "" {
			return errors.New("SMTP_HOST is required when EMAIL_PROVIDER=smtp")
		}
		if c.SMTP.Port <= 0 {
			return errors.New("SMTP_PORT must be > 0 when EMAIL_PROVIDER=smtp")
		}
		if c.SMTP.From == "" {
			return errors.New("SMTP_FROM is required when EMAIL_PROVIDER=smtp")
		}
	}
	if c.EmailProvider == "ses" {
		if c.SES.Region == "" {
			return errors.New("SES_REGION is required when EMAIL_PROVIDER=ses")
		}
		if c.SES.From == "" {
			return errors.New("SES_FROM is required when EMAIL_PROVIDER=ses")
		}
	}
	if c.ObjectStorageEnabled() {
		if c.ObjectStorage.AccessKeyID == "" {
			return errors.New("OBJECT_STORAGE_ACCESS_KEY_ID is required when OBJECT_STORAGE_ENDPOINT is set")
		}
		if c.ObjectStorage.SecretAccessKey == "" {
			return errors.New("OBJECT_STORAGE_SECRET_ACCESS_KEY is required when OBJECT_STORAGE_ENDPOINT is set")
		}
		if c.ObjectStorage.Bucket == "" {
			return errors.New("OBJECT_STORAGE_BUCKET is required when OBJECT_STORAGE_ENDPOINT is set")
		}
	}
	if c.ServiceDiscovery.Enabled() {
		if c.ServiceDiscovery.Provider != "consul" {
			return errors.New("SERVICE_DISCOVERY_PROVIDER must be empty or consul")
		}
		if c.ServiceDiscovery.ConsulHTTPAddr == "" {
			return errors.New("CONSUL_HTTP_ADDR is required when service discovery is enabled")
		}
		if c.ServiceDiscovery.ServiceName == "" {
			return errors.New("CONSUL_SERVICE_NAME is required when service discovery is enabled")
		}
		if c.ServiceDiscovery.ServiceID == "" {
			return errors.New("CONSUL_SERVICE_ID is required when service discovery is enabled")
		}
		if c.ServiceDiscovery.ServiceAddress == "" {
			return errors.New("CONSUL_SERVICE_ADDRESS is required when service discovery is enabled")
		}
		if c.ServiceDiscovery.ServicePort <= 0 {
			return errors.New("CONSUL_SERVICE_PORT must be > 0 when service discovery is enabled")
		}
		if c.ServiceDiscovery.HealthCheckPath == "" || !strings.HasPrefix(c.ServiceDiscovery.HealthCheckPath, "/") {
			return errors.New("CONSUL_HEALTH_CHECK_PATH must start with / when service discovery is enabled")
		}
	}
	if c.WorkerDiscovery.Enabled() {
		if c.WorkerDiscovery.Provider != "consul" {
			return errors.New("WORKER_SERVICE_DISCOVERY_PROVIDER must be empty or consul")
		}
		if c.WorkerDiscovery.ConsulHTTPAddr == "" {
			return errors.New("WORKER_CONSUL_HTTP_ADDR is required when worker service discovery is enabled")
		}
		if c.WorkerDiscovery.ServiceName == "" {
			return errors.New("CONSUL_WORKER_SERVICE_NAME is required when worker service discovery is enabled")
		}
		if c.WorkerDiscovery.ServiceID == "" {
			return errors.New("CONSUL_WORKER_SERVICE_ID is required when worker service discovery is enabled")
		}
		if c.WorkerDiscovery.ServiceAddress == "" {
			return errors.New("CONSUL_WORKER_SERVICE_ADDRESS is required when worker service discovery is enabled")
		}
		if c.WorkerDiscovery.ServicePort <= 0 {
			return errors.New("CONSUL_WORKER_SERVICE_PORT must be > 0 when worker service discovery is enabled")
		}
		if c.WorkerDiscovery.HealthCheckPath == "" || !strings.HasPrefix(c.WorkerDiscovery.HealthCheckPath, "/") {
			return errors.New("CONSUL_WORKER_HEALTH_CHECK_PATH must start with / when worker service discovery is enabled")
		}
	}
	if c.WorkerHTTPAddr == "" {
		return errors.New("WORKER_HTTP_ADDR is required")
	}
	if c.WorkerConcurrency <= 0 {
		return errors.New("WORKER_CONCURRENCY must be > 0")
	}
	if c.WorkerConsumerGroupPrefix == "" {
		return errors.New("WORKER_CONSUMER_GROUP_PREFIX is required")
	}
	if c.needsObjectStorage() && !c.ObjectStorageEnabled() {
		return errors.New("OBJECT_STORAGE_ENDPOINT is required when audience import/export processors are enabled")
	}
	if c.ClickHouseDSN == "" && c.AppEnv != "local" && c.AppEnv != "test" {
		return errors.New("CLICKHOUSE_DSN is required for analytics queries and event processing")
	}
	return nil
}

func (c Config) ClickHouseEnabled() bool {
	return c.ClickHouseDSN != ""
}

func (c Config) KafkaEnabled() bool {
	return c.KafkaBrokers != ""
}

func (c Config) ObjectStorageEnabled() bool {
	return c.ObjectStorage.Endpoint != ""
}

func (c ServiceDiscoveryConfig) Enabled() bool {
	return c.Provider != ""
}

func portFromAddr(addr string, fallback int) int {
	addr = strings.TrimSpace(addr)
	idx := strings.LastIndex(addr, ":")
	if idx < 0 || idx == len(addr)-1 {
		return fallback
	}
	port, err := strconv.Atoi(addr[idx+1:])
	if err != nil || port <= 0 {
		return fallback
	}
	return port
}

func (c Config) needsObjectStorage() bool {
	for _, consumer := range c.WorkerEnabledConsumers {
		if consumer == "audience.import_processor" || consumer == "audience.export_processor" {
			return true
		}
	}
	return false
}

func (c Config) SecureCookies() bool {
	return strings.ToLower(strings.TrimSpace(c.AppEnv)) != "local"
}

func (c Config) DatabaseConfig() DatabaseConfig {
	return DatabaseConfig{URL: c.DatabaseURL, ReadURL: c.DatabaseReadURL}
}

func (c Config) JWTConfig() JWTConfig {
	return JWTConfig{
		Issuer:        c.JWTIssuer,
		AccessSecret:  c.JWTAccessSecret,
		RefreshSecret: c.JWTRefreshSecret,
		AccessTTL:     c.JWTAccessTTL,
		RefreshTTL:    c.JWTRefreshTTL,
	}
}

func (c Config) OAuthConfig() OAuthConfig {
	return OAuthConfig{
		StateTTL:       c.OAuthStateTTL,
		GoogleClientID: c.OAuthGoogleClientID,
		GoogleSecret:   c.OAuthGoogleSecret,
		GithubClientID: c.OAuthGithubClientID,
		GithubSecret:   c.OAuthGithubSecret,
	}
}

func getenv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func parseDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s=%q: %w", key, raw, err)
	}
	return d, nil
}

func parseBool(key string, fallback bool) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("failed to parse %s=%q: %w", key, raw, err)
	}
	return v, nil
}

func parseInt(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s=%q: %w", key, raw, err)
	}
	return v, nil
}

func parseBoolOrDefault(key string, fallback bool) bool {
	v, err := parseBool(key, fallback)
	if err != nil {
		return fallback
	}
	return v
}

func parseDurationOrDefault(key string, fallback time.Duration) time.Duration {
	v, err := parseDuration(key, fallback)
	if err != nil {
		return fallback
	}
	return v
}

func parseCSV(key string) []string {
	return sliceutil.ParseCSV(os.Getenv(key))
}
