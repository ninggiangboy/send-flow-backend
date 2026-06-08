package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppName         string
	AppEnv          string
	HTTPAddr        string
	LogLevel        string
	AutoMigrate     bool
	DatabaseURL     string
	DatabaseReadURL string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	KafkaBrokers    string
	ClickHouseDSN   string
	EmailProvider   string
	FrontendBaseURL string
	SMTP            SMTPConfig
	SES             SESConfig
	ObjectStorage   ObjectStorageConfig
	ShutdownTimeout time.Duration

	WorkerHTTPAddr            string
	WorkerEnabledConsumers    []string
	WorkerConcurrency         int
	WorkerShutdownTimeout     time.Duration
	WorkerConsumerGroupPrefix string

	JWTIssuer            string
	JWTAccessSecret      string
	JWTRefreshSecret     string
	JWTAccessTTL         time.Duration
	JWTRefreshTTL        time.Duration
	OAuthStateTTL        time.Duration
	OAuthGoogleClientID  string
	OAuthGoogleSecret    string
	OAuthGithubClientID  string
	OAuthGithubSecret    string
	EmailVerificationTTL time.Duration
	PasswordResetTTL     time.Duration
	MFAChallengeTTL      time.Duration
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
		AppEnv:          getenv("APP_ENV", "local"),
		HTTPAddr:        getenv("HTTP_ADDR", ":8081"),
		LogLevel:        getenv("LOG_LEVEL", "info"),
		AutoMigrate:     parseBool("AUTO_MIGRATE", false),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		DatabaseReadURL: os.Getenv("DATABASE_READ_URL"),
		RedisAddr:       getenv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		KafkaBrokers:    os.Getenv("KAFKA_BROKERS"),
		ClickHouseDSN:   os.Getenv("CLICKHOUSE_DSN"),
		EmailProvider:   getenv("EMAIL_PROVIDER", "smtp"),
		FrontendBaseURL: os.Getenv("FRONTEND_BASE_URL"),
		SMTP: SMTPConfig{
			Host:     getenv("SMTP_HOST", "localhost"),
			Port:     parseInt("SMTP_PORT", 1025),
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
			ForcePathStyle:  parseBool("OBJECT_STORAGE_FORCE_PATH_STYLE", true),
			UseSSL:          parseBool("OBJECT_STORAGE_USE_SSL", false),
		},
		ShutdownTimeout: parseDuration("SHUTDOWN_TIMEOUT", 10*time.Second),

		WorkerHTTPAddr:            getenv("WORKER_HTTP_ADDR", ":8082"),
		WorkerEnabledConsumers:    parseCSV("WORKER_ENABLED_CONSUMERS"),
		WorkerConcurrency:         parseInt("WORKER_CONCURRENCY", 1),
		WorkerShutdownTimeout:     parseDuration("WORKER_SHUTDOWN_TIMEOUT", 10*time.Second),
		WorkerConsumerGroupPrefix: getenv("WORKER_CONSUMER_GROUP_PREFIX", "send-flow"),

		JWTIssuer:            getenv("JWT_ISSUER", "send-flow"),
		JWTAccessSecret:      getenv("JWT_ACCESS_SECRET", "dev-access-secret-change-me"),
		JWTRefreshSecret:     getenv("JWT_REFRESH_SECRET", "dev-refresh-secret-change-me"),
		JWTAccessTTL:         parseDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:        parseDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		OAuthStateTTL:        parseDuration("OAUTH_STATE_TTL", 10*time.Minute),
		OAuthGoogleClientID:  os.Getenv("OAUTH_GOOGLE_CLIENT_ID"),
		OAuthGoogleSecret:    os.Getenv("OAUTH_GOOGLE_CLIENT_SECRET"),
		OAuthGithubClientID:  os.Getenv("OAUTH_GITHUB_CLIENT_ID"),
		OAuthGithubSecret:    os.Getenv("OAUTH_GITHUB_CLIENT_SECRET"),
		EmailVerificationTTL: parseDuration("AUTH_EMAIL_VERIFICATION_TTL", 24*time.Hour),
		PasswordResetTTL:     parseDuration("AUTH_PASSWORD_RESET_TTL", time.Hour),
		MFAChallengeTTL:      parseDuration("AUTH_MFA_CHALLENGE_TTL", 10*time.Minute),
	}

	redisDB, err := strconv.Atoi(getenv("REDIS_DB", "0"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid REDIS_DB: %w", err)
	}
	cfg.RedisDB = redisDB

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
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
	if c.EmailProvider != "smtp" && c.EmailProvider != "ses" && c.EmailProvider != "fake" {
		return errors.New("EMAIL_PROVIDER must be one of: smtp, ses, fake")
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
	if c.WorkerHTTPAddr == "" {
		return errors.New("WORKER_HTTP_ADDR is required")
	}
	if c.WorkerConcurrency <= 0 {
		return errors.New("WORKER_CONCURRENCY must be > 0")
	}
	if c.WorkerConsumerGroupPrefix == "" {
		return errors.New("WORKER_CONSUMER_GROUP_PREFIX is required")
	}
	return nil
}

func (c Config) KafkaEnabled() bool {
	return c.KafkaBrokers != ""
}

func (c Config) ClickHouseEnabled() bool {
	return c.ClickHouseDSN != ""
}

func (c Config) ObjectStorageEnabled() bool {
	return c.ObjectStorage.Endpoint != ""
}

func (c Config) SecureCookies() bool {
	return strings.ToLower(strings.TrimSpace(c.AppEnv)) != "local"
}

func getenv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func parseDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}

func parseBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func parseInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func parseCSV(key string) []string {
	raw := os.Getenv(key)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}
