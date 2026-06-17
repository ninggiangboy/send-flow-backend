package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadFromEnv_RequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "localhost:6379")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is missing")
	}
}

func TestLoadFromEnv_ParsesValues(t *testing.T) {
	t.Setenv("APP_NAME", "test-app")
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5432/db?sslmode=disable")
	t.Setenv("DATABASE_READ_URL", "postgres://x:y@localhost:5433/db?sslmode=disable")
	t.Setenv("AUTO_MIGRATE", "true")
	t.Setenv("REDIS_ADDR", "localhost:6380")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("KAFKA_BROKERS", "localhost:9092")
	t.Setenv("EMAIL_PROVIDER", "smtp")
	t.Setenv("SMTP_HOST", "localhost")
	t.Setenv("SMTP_PORT", "1025")
	t.Setenv("SMTP_FROM", "noreply@sendflow.local")
	t.Setenv("OBJECT_STORAGE_ENDPOINT", "localhost:9001")
	t.Setenv("OBJECT_STORAGE_REGION", "us-east-1")
	t.Setenv("OBJECT_STORAGE_ACCESS_KEY_ID", "minio")
	t.Setenv("OBJECT_STORAGE_SECRET_ACCESS_KEY", "minio-secret")
	t.Setenv("OBJECT_STORAGE_BUCKET", "sendflow-local")
	t.Setenv("OBJECT_STORAGE_FORCE_PATH_STYLE", "true")
	t.Setenv("OBJECT_STORAGE_USE_SSL", "false")
	t.Setenv("SERVICE_DISCOVERY_PROVIDER", "consul")
	t.Setenv("CONSUL_HTTP_ADDR", "http://localhost:8500")
	t.Setenv("CONSUL_SERVICE_NAME", "sendflow-api")
	t.Setenv("CONSUL_SERVICE_ID", "sendflow-api-local")
	t.Setenv("CONSUL_SERVICE_ADDRESS", "host.docker.internal")
	t.Setenv("CONSUL_SERVICE_PORT", "8081")
	t.Setenv("CONSUL_HEALTH_CHECK_PATH", "/api/readyz")
	t.Setenv("CONSUL_WORKER_SERVICE_NAME", "sendflow-worker")
	t.Setenv("CONSUL_WORKER_SERVICE_ID", "sendflow-worker-local")
	t.Setenv("CONSUL_WORKER_SERVICE_ADDRESS", "host.docker.internal")
	t.Setenv("CONSUL_WORKER_HEALTH_CHECK_PATH", "/api/readyz")
	t.Setenv("SERVICE_NAME", "worker")
	t.Setenv("INSTANCE_ADDR", ":19090")
	t.Setenv("POD_NAME", "send-flow-worker-abc")
	t.Setenv("POD_NAMESPACE", "send-flow")
	t.Setenv("NODE_NAME", "node-a")
	t.Setenv("SHUTDOWN_TIMEOUT", "5s")
	t.Setenv("WORKER_HTTP_ADDR", ":9090")
	t.Setenv("WORKER_ENABLED_CONSUMERS", "delivery, analytics, ")
	t.Setenv("WORKER_CONCURRENCY", "3")
	t.Setenv("WORKER_SHUTDOWN_TIMEOUT", "7s")
	t.Setenv("WORKER_CONSUMER_GROUP_PREFIX", "send-flow-test")
	t.Setenv("JWT_ACCESS_SECRET", "real-access-secret-for-testing")
	t.Setenv("JWT_REFRESH_SECRET", "real-refresh-secret-for-testing")
	t.Setenv("REDIS_CACHE_IDENTITY_ACCESS_TTL", "30s")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AppName != "test-app" {
		t.Fatalf("unexpected app name: %s", cfg.AppName)
	}
	if cfg.RedisDB != 2 {
		t.Fatalf("unexpected redis db: %d", cfg.RedisDB)
	}
	if cfg.DatabaseReadURL == "" {
		t.Fatal("expected read database URL")
	}
	if !cfg.AutoMigrate {
		t.Fatal("expected auto migrate enabled")
	}
	if !cfg.KafkaEnabled() {
		t.Fatal("expected kafka enabled")
	}
	if cfg.SMTP.Port != 1025 {
		t.Fatalf("unexpected smtp port: %d", cfg.SMTP.Port)
	}
	if cfg.SMTP.From != "noreply@sendflow.local" {
		t.Fatalf("unexpected smtp from: %s", cfg.SMTP.From)
	}
	if cfg.EmailProvider != "smtp" {
		t.Fatalf("unexpected email provider: %s", cfg.EmailProvider)
	}
	if cfg.WorkerHTTPAddr != ":9090" {
		t.Fatalf("unexpected worker http addr: %s", cfg.WorkerHTTPAddr)
	}
	if len(cfg.WorkerEnabledConsumers) != 2 || cfg.WorkerEnabledConsumers[0] != "delivery" || cfg.WorkerEnabledConsumers[1] != "analytics" {
		t.Fatalf("unexpected worker consumers: %#v", cfg.WorkerEnabledConsumers)
	}
	if cfg.WorkerConcurrency != 3 {
		t.Fatalf("unexpected worker concurrency: %d", cfg.WorkerConcurrency)
	}
	if cfg.WorkerShutdownTimeout.String() != "7s" {
		t.Fatalf("unexpected worker shutdown timeout: %s", cfg.WorkerShutdownTimeout)
	}
	if cfg.WorkerConsumerGroupPrefix != "send-flow-test" {
		t.Fatalf("unexpected worker consumer group prefix: %s", cfg.WorkerConsumerGroupPrefix)
	}
	if cfg.RedisCache.IdentityAccessCacheTTL != 30*time.Second {
		t.Fatalf("unexpected identity access cache TTL: %s", cfg.RedisCache.IdentityAccessCacheTTL)
	}
	if !cfg.ObjectStorageEnabled() {
		t.Fatal("expected object storage enabled")
	}
	if cfg.ObjectStorage.Bucket != "sendflow-local" {
		t.Fatalf("unexpected object storage bucket: %s", cfg.ObjectStorage.Bucket)
	}
	if !cfg.ServiceDiscovery.Enabled() {
		t.Fatal("expected service discovery enabled")
	}
	if cfg.ServiceDiscovery.ServicePort != 8081 {
		t.Fatalf("unexpected service discovery port: %d", cfg.ServiceDiscovery.ServicePort)
	}
	if cfg.ServiceDiscovery.HealthCheckPath != "/api/readyz" {
		t.Fatalf("unexpected service discovery health check path: %s", cfg.ServiceDiscovery.HealthCheckPath)
	}
	if !cfg.WorkerDiscovery.Enabled() {
		t.Fatal("expected worker service discovery enabled")
	}
	if cfg.WorkerDiscovery.ServiceName != "sendflow-worker" {
		t.Fatalf("unexpected worker service name: %s", cfg.WorkerDiscovery.ServiceName)
	}
	if cfg.WorkerDiscovery.ServicePort != 9090 {
		t.Fatalf("unexpected worker service discovery port: %d", cfg.WorkerDiscovery.ServicePort)
	}
	if cfg.RuntimeIdentity.ServiceName != "worker" {
		t.Fatalf("unexpected runtime service name: %s", cfg.RuntimeIdentity.ServiceName)
	}
	if cfg.RuntimeIdentity.InstanceID != "sendflow-worker-local" {
		t.Fatalf("unexpected runtime instance ID: %s", cfg.RuntimeIdentity.InstanceID)
	}
	if cfg.RuntimeIdentity.InstanceAddr != ":19090" {
		t.Fatalf("unexpected runtime instance addr: %s", cfg.RuntimeIdentity.InstanceAddr)
	}
	if cfg.RuntimeIdentity.PodName != "send-flow-worker-abc" || cfg.RuntimeIdentity.Namespace != "send-flow" || cfg.RuntimeIdentity.NodeName != "node-a" {
		t.Fatalf("unexpected kubernetes identity: %#v", cfg.RuntimeIdentity)
	}
}

func TestMain(m *testing.M) {
	_ = os.Unsetenv("APP_NAME")
	_ = os.Unsetenv("APP_ENV")
	_ = os.Unsetenv("HTTP_ADDR")
	_ = os.Unsetenv("LOG_LEVEL")
	_ = os.Unsetenv("AUTO_MIGRATE")
	_ = os.Unsetenv("DATABASE_URL")
	_ = os.Unsetenv("REDIS_ADDR")
	_ = os.Unsetenv("DATABASE_READ_URL")
	_ = os.Unsetenv("REDIS_PASSWORD")
	_ = os.Unsetenv("REDIS_DB")
	_ = os.Unsetenv("KAFKA_BROKERS")
	_ = os.Unsetenv("EMAIL_PROVIDER")
	_ = os.Unsetenv("SMTP_HOST")
	_ = os.Unsetenv("SMTP_PORT")
	_ = os.Unsetenv("SMTP_USERNAME")
	_ = os.Unsetenv("SMTP_PASSWORD")
	_ = os.Unsetenv("SMTP_FROM")
	_ = os.Unsetenv("SES_REGION")
	_ = os.Unsetenv("SES_ACCESS_KEY_ID")
	_ = os.Unsetenv("SES_SECRET_ACCESS_KEY")
	_ = os.Unsetenv("SES_SESSION_TOKEN")
	_ = os.Unsetenv("SES_ENDPOINT")
	_ = os.Unsetenv("SES_FROM")
	_ = os.Unsetenv("SES_CONFIGURATION_SET")
	_ = os.Unsetenv("SHUTDOWN_TIMEOUT")
	_ = os.Unsetenv("WORKER_HTTP_ADDR")
	_ = os.Unsetenv("WORKER_CONCURRENCY")
	_ = os.Unsetenv("WORKER_SHUTDOWN_TIMEOUT")
	_ = os.Unsetenv("WORKER_CONSUMER_GROUP_PREFIX")
	_ = os.Unsetenv("OBJECT_STORAGE_ENDPOINT")
	_ = os.Unsetenv("OBJECT_STORAGE_REGION")
	_ = os.Unsetenv("OBJECT_STORAGE_ACCESS_KEY_ID")
	_ = os.Unsetenv("OBJECT_STORAGE_SECRET_ACCESS_KEY")
	_ = os.Unsetenv("OBJECT_STORAGE_BUCKET")
	_ = os.Unsetenv("OBJECT_STORAGE_FORCE_PATH_STYLE")
	_ = os.Unsetenv("OBJECT_STORAGE_USE_SSL")
	_ = os.Unsetenv("SERVICE_DISCOVERY_PROVIDER")
	_ = os.Unsetenv("CONSUL_HTTP_ADDR")
	_ = os.Unsetenv("CONSUL_SERVICE_NAME")
	_ = os.Unsetenv("CONSUL_SERVICE_ID")
	_ = os.Unsetenv("CONSUL_SERVICE_ADDRESS")
	_ = os.Unsetenv("CONSUL_SERVICE_PORT")
	_ = os.Unsetenv("CONSUL_HEALTH_CHECK_PATH")
	_ = os.Unsetenv("WORKER_SERVICE_DISCOVERY_PROVIDER")
	_ = os.Unsetenv("WORKER_CONSUL_HTTP_ADDR")
	_ = os.Unsetenv("CONSUL_WORKER_SERVICE_NAME")
	_ = os.Unsetenv("CONSUL_WORKER_SERVICE_ID")
	_ = os.Unsetenv("CONSUL_WORKER_SERVICE_ADDRESS")
	_ = os.Unsetenv("CONSUL_WORKER_SERVICE_PORT")
	_ = os.Unsetenv("CONSUL_WORKER_HEALTH_CHECK_PATH")
	_ = os.Unsetenv("SERVICE_NAME")
	_ = os.Unsetenv("INSTANCE_ID")
	_ = os.Unsetenv("INSTANCE_ADDR")
	_ = os.Unsetenv("POD_NAME")
	_ = os.Unsetenv("POD_NAMESPACE")
	_ = os.Unsetenv("NODE_NAME")
	_ = os.Unsetenv("JWT_ACCESS_SECRET")
	_ = os.Unsetenv("JWT_REFRESH_SECRET")
	_ = os.Unsetenv("UNSUBSCRIBE_TOKEN_SECRET")
	_ = os.Unsetenv("REDIS_CACHE_IDENTITY_ACCESS_TTL")
	_ = os.Unsetenv("REDIS_CACHE_IDENTITY_SETTINGS_TTL")
	_ = os.Unsetenv("REDIS_CACHE_CONTENT_PREVIEW_TTL")
	_ = os.Unsetenv("REDIS_CACHE_SENDER_READINESS_TTL")
	_ = os.Unsetenv("REDIS_CACHE_AUDIENCE_RESOLUTION_TTL")
	_ = os.Unsetenv("REDIS_CACHE_DELIVERY_IDEMPOTENCY_TTL")
	_ = os.Unsetenv("REDIS_CACHE_DELIVERY_QUOTA_TTL")
	_ = os.Unsetenv("REDIS_CACHE_ANALYTICS_QUERY_TTL")
	os.Exit(m.Run())
}
