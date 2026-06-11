package config

import (
	"io"
	"log/slog"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		DatabaseURL:               "postgres://localhost:5432/test",
		RedisAddr:                 "localhost:6379",
		JWTAccessSecret:           "real-secret-not-dev",
		JWTRefreshSecret:          "real-refresh-not-dev",
		EmailProvider:             "smtp",
		SMTP:                      SMTPConfig{Host: "localhost", Port: 1025, From: "test@test.com"},
		WorkerHTTPAddr:            ":8082",
		WorkerConcurrency:         1,
		WorkerConsumerGroupPrefix: "test",
	}
}

func TestValidate_EmptyDatabaseURL(t *testing.T) {
	cfg := validConfig()
	cfg.DatabaseURL = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty DATABASE_URL")
	}
}

func TestValidate_EmptyRedisAddr(t *testing.T) {
	cfg := validConfig()
	cfg.RedisAddr = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty REDIS_ADDR")
	}
}

func TestValidate_EmptyJWTAccessSecret(t *testing.T) {
	cfg := validConfig()
	cfg.JWTAccessSecret = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty JWT_ACCESS_SECRET")
	}
}

func TestValidate_EmptyJWTRefreshSecret(t *testing.T) {
	cfg := validConfig()
	cfg.JWTRefreshSecret = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty JWT_REFRESH_SECRET")
	}
}

func TestValidate_InvalidEmailProvider(t *testing.T) {
	cfg := validConfig()
	cfg.EmailProvider = "invalid"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid EMAIL_PROVIDER")
	}
}

func TestValidate_SMTPRequiresHost(t *testing.T) {
	cfg := validConfig()
	cfg.SMTP.Host = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when SMTP_HOST is missing")
	}
}

func TestValidate_SMTPRequiresPort(t *testing.T) {
	cfg := validConfig()
	cfg.SMTP.Port = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when SMTP_PORT is 0")
	}
}

func TestValidate_SMTPRequiresFrom(t *testing.T) {
	cfg := validConfig()
	cfg.SMTP.From = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when SMTP_FROM is missing")
	}
}

func TestValidate_SESRequiresRegion(t *testing.T) {
	cfg := validConfig()
	cfg.EmailProvider = "ses"
	cfg.SES.From = "test@test.com"
	cfg.SES.Region = ""
	cfg.SMTP = SMTPConfig{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when SES_REGION is missing")
	}
}

func TestValidate_SESRequiresFrom(t *testing.T) {
	cfg := validConfig()
	cfg.EmailProvider = "ses"
	cfg.SES.Region = "us-east-1"
	cfg.SES.From = ""
	cfg.SMTP = SMTPConfig{}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when SES_FROM is missing")
	}
}

func TestValidate_ObjectStorageRequiresKeys(t *testing.T) {
	cfg := validConfig()
	cfg.ObjectStorage.Endpoint = "localhost:9001"
	cfg.ObjectStorage.AccessKeyID = ""
	cfg.ObjectStorage.SecretAccessKey = ""
	cfg.ObjectStorage.Bucket = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when object storage keys are missing")
	}
}

func TestValidate_DevSecretsRejectedNonLocal(t *testing.T) {
	cfg := validConfig()
	cfg.AppEnv = "production"
	cfg.JWTAccessSecret = "dev-access-secret-change-me"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for dev access secret in non-local env")
	}
}

func TestValidate_DevSecretsAcceptedLocal(t *testing.T) {
	cfg := validConfig()
	cfg.AppEnv = "local"
	cfg.JWTAccessSecret = "dev-access-secret-change-me"
	cfg.JWTRefreshSecret = "dev-refresh-secret-change-me"
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error for dev secrets in local env, got: %v", err)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := validConfig()
	err := cfg.Validate()
	if err != nil {
		t.Fatalf("expected no error for valid config, got: %v", err)
	}
}

func TestValidate_WorkerHTTPAddrRequired(t *testing.T) {
	cfg := validConfig()
	cfg.WorkerHTTPAddr = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty WORKER_HTTP_ADDR")
	}
}

func TestValidate_WorkerConcurrencyPositive(t *testing.T) {
	cfg := validConfig()
	cfg.WorkerConcurrency = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for WORKER_CONCURRENCY <= 0")
	}
}

func TestValidate_WorkerConsumerGroupPrefixRequired(t *testing.T) {
	cfg := validConfig()
	cfg.WorkerConsumerGroupPrefix = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty WORKER_CONSUMER_GROUP_PREFIX")
	}
}

func TestIsDevSecret_KnownReturnsTrue(t *testing.T) {
	if !isDevSecret("dev-access-secret-change-me") {
		t.Fatal("expected dev-access-secret-change-me to be recognized")
	}
	if !isDevSecret("dev-refresh-secret-change-me") {
		t.Fatal("expected dev-refresh-secret-change-me to be recognized")
	}
	if !isDevSecret("dev-unsubscribe-secret-change-me") {
		t.Fatal("expected dev-unsubscribe-secret-change-me to be recognized")
	}
}

func TestIsDevSecret_RandomReturnsFalse(t *testing.T) {
	if isDevSecret("some-random-secret") {
		t.Fatal("expected random secret to not be recognized")
	}
}

func TestIsDevSecret_EmptyReturnsFalse(t *testing.T) {
	if isDevSecret("") {
		t.Fatal("expected empty string to not be recognized")
	}
}

func TestKafkaEnabled_WhenBrokersSet(t *testing.T) {
	cfg := validConfig()
	cfg.KafkaBrokers = "localhost:9092"
	if !cfg.KafkaEnabled() {
		t.Fatal("expected KafkaEnabled to be true when brokers are set")
	}
}

func TestKafkaEnabled_WhenBrokersEmpty(t *testing.T) {
	cfg := validConfig()
	cfg.KafkaBrokers = ""
	if cfg.KafkaEnabled() {
		t.Fatal("expected KafkaEnabled to be false when brokers are empty")
	}
}

func TestClickHouseEnabled_WhenDSNSet(t *testing.T) {
	cfg := validConfig()
	cfg.ClickHouseDSN = "clickhouse://localhost:9000/default"
	if !cfg.ClickHouseEnabled() {
		t.Fatal("expected ClickHouseEnabled to be true when DSN is set")
	}
}

func TestClickHouseEnabled_WhenDSNEmpty(t *testing.T) {
	cfg := validConfig()
	cfg.ClickHouseDSN = ""
	if cfg.ClickHouseEnabled() {
		t.Fatal("expected ClickHouseEnabled to be false when DSN is empty")
	}
}

func TestObjectStorageEnabled_WhenEndpointSet(t *testing.T) {
	cfg := validConfig()
	cfg.ObjectStorage.Endpoint = "localhost:9001"
	if !cfg.ObjectStorageEnabled() {
		t.Fatal("expected ObjectStorageEnabled to be true when endpoint is set")
	}
}

func TestObjectStorageEnabled_WhenEndpointEmpty(t *testing.T) {
	cfg := validConfig()
	cfg.ObjectStorage.Endpoint = ""
	if cfg.ObjectStorageEnabled() {
		t.Fatal("expected ObjectStorageEnabled to be false when endpoint is empty")
	}
}

func TestSecureCookies_NonLocalEnv(t *testing.T) {
	cfg := validConfig()
	cfg.AppEnv = "production"
	if !cfg.SecureCookies() {
		t.Fatal("expected SecureCookies to be true in non-local env")
	}
}

func TestSecureCookies_LocalEnv(t *testing.T) {
	cfg := validConfig()
	cfg.AppEnv = "local"
	if cfg.SecureCookies() {
		t.Fatal("expected SecureCookies to be false in local env")
	}
}

func TestDatabaseConfig_ReturnsCorrectValues(t *testing.T) {
	cfg := validConfig()
	cfg.DatabaseURL = "postgres://prod:5432/db"
	cfg.DatabaseReadURL = "postgres://prod-read:5432/db"
	dc := cfg.DatabaseConfig()
	if dc.URL != "postgres://prod:5432/db" {
		t.Fatalf("unexpected URL: %s", dc.URL)
	}
	if dc.ReadURL != "postgres://prod-read:5432/db" {
		t.Fatalf("unexpected ReadURL: %s", dc.ReadURL)
	}
}

func TestJWTConfig_ReturnsCorrectValues(t *testing.T) {
	cfg := validConfig()
	cfg.JWTIssuer = "test-issuer"
	cfg.JWTAccessSecret = "access-secret"
	cfg.JWTRefreshSecret = "refresh-secret"
	cfg.JWTAccessTTL = 15 * time.Minute
	cfg.JWTRefreshTTL = 24 * time.Hour
	jc := cfg.JWTConfig()
	if jc.Issuer != "test-issuer" {
		t.Fatalf("unexpected Issuer: %s", jc.Issuer)
	}
	if jc.AccessSecret != "access-secret" {
		t.Fatalf("unexpected AccessSecret: %s", jc.AccessSecret)
	}
	if jc.RefreshSecret != "refresh-secret" {
		t.Fatalf("unexpected RefreshSecret: %s", jc.RefreshSecret)
	}
	if jc.AccessTTL != 15*time.Minute {
		t.Fatalf("unexpected AccessTTL: %v", jc.AccessTTL)
	}
	if jc.RefreshTTL != 24*time.Hour {
		t.Fatalf("unexpected RefreshTTL: %v", jc.RefreshTTL)
	}
}

func TestOAuthConfig_ReturnsCorrectValues(t *testing.T) {
	cfg := validConfig()
	cfg.OAuthStateTTL = 30 * time.Minute
	cfg.OAuthGoogleClientID = "google-id"
	cfg.OAuthGoogleSecret = "google-secret"
	cfg.OAuthGithubClientID = "github-id"
	cfg.OAuthGithubSecret = "github-secret"
	oc := cfg.OAuthConfig()
	if oc.StateTTL != 30*time.Minute {
		t.Fatalf("unexpected StateTTL: %v", oc.StateTTL)
	}
	if oc.GoogleClientID != "google-id" {
		t.Fatalf("unexpected GoogleClientID: %s", oc.GoogleClientID)
	}
	if oc.GoogleSecret != "google-secret" {
		t.Fatalf("unexpected GoogleSecret: %s", oc.GoogleSecret)
	}
	if oc.GithubClientID != "github-id" {
		t.Fatalf("unexpected GithubClientID: %s", oc.GithubClientID)
	}
	if oc.GithubSecret != "github-secret" {
		t.Fatalf("unexpected GithubSecret: %s", oc.GithubSecret)
	}
}

func TestParseDuration_ValidDuration(t *testing.T) {
	t.Setenv("TEST_DURATION", "5m")
	d, err := parseDuration("TEST_DURATION", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 5*time.Minute {
		t.Fatalf("unexpected duration: %v", d)
	}
}

func TestParseDuration_EmptyReturnsFallback(t *testing.T) {
	t.Setenv("TEST_DURATION", "")
	d, err := parseDuration("TEST_DURATION", 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != 30*time.Second {
		t.Fatalf("unexpected duration: %v", d)
	}
}

func TestParseDuration_InvalidReturnsError(t *testing.T) {
	t.Setenv("TEST_DURATION", "not-a-duration")
	_, err := parseDuration("TEST_DURATION", 0)
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestParseBool_True(t *testing.T) {
	t.Setenv("TEST_BOOL", "true")
	v, err := parseBool("TEST_BOOL", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != true {
		t.Fatal("expected true")
	}
}

func TestParseBool_False(t *testing.T) {
	t.Setenv("TEST_BOOL", "false")
	v, err := parseBool("TEST_BOOL", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != false {
		t.Fatal("expected false")
	}
}

func TestParseBool_One(t *testing.T) {
	t.Setenv("TEST_BOOL", "1")
	v, err := parseBool("TEST_BOOL", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != true {
		t.Fatal("expected true")
	}
}

func TestParseBool_Zero(t *testing.T) {
	t.Setenv("TEST_BOOL", "0")
	v, err := parseBool("TEST_BOOL", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != false {
		t.Fatal("expected false")
	}
}

func TestParseBool_EmptyReturnsFallback(t *testing.T) {
	t.Setenv("TEST_BOOL", "")
	v, err := parseBool("TEST_BOOL", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != true {
		t.Fatal("expected fallback true")
	}
}

func TestParseBool_InvalidReturnsError(t *testing.T) {
	t.Setenv("TEST_BOOL", "not-a-bool")
	_, err := parseBool("TEST_BOOL", false)
	if err == nil {
		t.Fatal("expected error for invalid bool")
	}
}

func TestParseInt_Valid(t *testing.T) {
	t.Setenv("TEST_INT", "42")
	v, err := parseInt("TEST_INT", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 42 {
		t.Fatalf("unexpected value: %d", v)
	}
}

func TestParseInt_EmptyReturnsFallback(t *testing.T) {
	t.Setenv("TEST_INT", "")
	v, err := parseInt("TEST_INT", 99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 99 {
		t.Fatalf("unexpected value: %d", v)
	}
}

func TestParseInt_InvalidReturnsError(t *testing.T) {
	t.Setenv("TEST_INT", "not-an-int")
	_, err := parseInt("TEST_INT", 0)
	if err == nil {
		t.Fatal("expected error for invalid int")
	}
}

func TestWarnDevSecrets_NoPanic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := validConfig()
	WarnDevSecrets(logger, cfg)
}
