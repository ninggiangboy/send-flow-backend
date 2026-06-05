# Infrastructure

File này mô tả vai trò của hạ tầng. Nguyên tắc chung: mỗi tool có một job rõ ràng; không dùng tool kỹ thuật để né business boundary.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Database map | `../database/00-database-map.md` |
| Code architecture | `04-code-architecture.md` |
| Event/outbox flow | `05-data-events-and-flows.md` |
| Scale/resilience | `08-scale-and-resilience.md` |
| Engineering practices | `09-engineering-practices.md` |

## Platform package

`platform` chứa technical foundation.

```text
internal/platform
├── config          # load config, env override
├── logger          # slog/zap wrapper
├── postgres        # db pool, low-level tx helpers
├── transaction     # transaction manager implementation
├── kafka           # producer, consumer, envelope
├── redis           # client, cache, lock
├── objectstorage   # S3-compatible object storage client
├── events          # envelope serializer, metadata
├── id              # UUID/ULID generator
├── clock           # mockable clock
├── errors          # technical errors
├── validator       # validation wrapper
├── security        # password hashing, JWT, crypto
├── secrets         # secret loading/provider abstraction
├── featureflags    # feature flag client/evaluator
├── observability   # metrics, tracing, structured logs
└── health          # health/readiness checks
```

`platform` không chứa:

```text
UserService
IdentityRepository
BusinessRule
SignUpHandler
```

Nếu tên package platform bắt đầu nghe giống nghiệp vụ, nó có lẽ đang nằm sai chỗ.

---

## PostgreSQL

PostgreSQL là **source of truth** cho business state.

Dùng cho:

- Transactional data: user, campaign, flow, message, subscription, invoice.
- Constraint quan trọng: unique email, foreign key, state integrity.
- Outbox table để đảm bảo ghi DB và phát event không bị lệch.
- Query nghiệp vụ cần consistency cao.

Không dùng PostgreSQL cho:

- Distributed lock ngắn hạn nếu Redis đã có.
- Message queue thay Kafka, trừ việc ghi `outbox_events` để Debezium hoặc outbox poller đẩy ra event backbone.
- Cache response nóng.

Khuyến nghị:

- Migration dùng tool nhất quán như `golang-migrate`, `goose` hoặc `atlas`; chọn một.
- Mỗi bounded context có migration folder riêng nếu schema tách theo module.
- Dùng connection pool có timeout rõ: max open, max idle, conn max lifetime.
- Luôn có index cho lookup chính: email, tenant id, external id, outbox status.
- Dùng transaction qua `ports.TxManager`, không truyền `*sql.Tx` xuyên layer lung tung.

## Redis

Redis là **ephemeral state store**, không phải source of truth.

Dùng cho:

- Cache dữ liệu đọc nhiều, có thể rebuild.
- Session/token store nếu muốn revoke nhanh.
- Rate limit.
- Distributed lock ngắn hạn.
- Idempotency key cho API hoặc consumer.
- Lightweight queue chỉ cho tác vụ nhỏ, không thay Kafka cho event backbone.

Không dùng Redis cho:

- Business state không thể mất.
- Event lịch sử cần replay.
- Workflow state dài hạn.

Khuyến nghị:

- Key có prefix rõ: `identity:session:<token>`, `rate:user:<id>`, `idem:<key>`.
- Mọi key cache/session phải có TTL.
- Lock phải có TTL và owner token để tránh unlock nhầm.
- Cache invalidation nên đi qua event hoặc write path rõ ràng.
- Không dùng `KEYS` trong production; dùng `SCAN` và delete theo batch.
- Cache failure không nên làm fail business flow nếu source of truth vẫn đọc được.

Cache-aside pattern:

```text
get cache
  -> hit: return value
  -> miss:
       dedupe local in-flight load
       acquire short Redis lock
       double-check cache
       load from source of truth
       set cache with TTL
       release lock
```

Chống cache stampede:

```text
local in-flight future/singleflight   # dedupe concurrent load trong cùng process
Redis distributed lock                # dedupe load giữa nhiều instance
double-check after lock               # tránh load thừa
short wait + retry cache              # chờ instance khác populate
fallback load                         # không treo request mãi
```

Key convention:

```text
cache:<module>:<resource>:<version>:<id>
cache:lock:<module>:<resource>:<id>
```

Ví dụ:

```text
cache:sender:domain-auth:v1:tenant_123:example.com
cache:featureflags:snapshot:v1:tenant_123
cache:provider:quota:v1:ses:tenant_123
```

Batch delete / prefix invalidation:

```text
SCAN prefix
  -> collect small batch
  -> DEL batch
  -> repeat
```

Không collect toàn bộ key của prefix lớn vào memory trước khi delete.

## Kafka

Kafka là **event backbone** giữa module/runtime.

Dùng cho:

- Domain/application events cần async processing.
- Tích hợp Worker, Notification, Analytics, Audit.
- Event fan-out: một event nhiều consumer group xử lý độc lập.
- Retry/DLQ cho side effect như gửi email, webhook, sync external system.

Không dùng Kafka cho:

- Request/response synchronous flow.
- Command bắt buộc cần kết quả ngay trong HTTP request.
- Event phát trực tiếp từ use case sau khi ghi DB. Use case phải ghi outbox trước.

Khuyến nghị:

- Topic đặt tên theo domain event: `identity.user.registered`.
- Payload có version: `identity.user.registered.v1`.
- Message có `event_id`, `event_type`, `occurred_at`, `aggregate_id`, `tenant_id` nếu multi-tenant.
- Consumer phải idempotent vì Kafka có thể deliver hơn một lần.
- Partition key nên là aggregate id để giữ ordering trong cùng aggregate.
- DLQ topic có convention rõ: `<topic>.dlq`.

## Debezium và Kafka Connect

Debezium + Kafka Connect là lớp CDC nằm giữa PostgreSQL và Kafka.

Dùng cho:

- Capture `outbox_events` từ WAL rồi route sang business topic.
- CDC có chủ đích cho một số bảng nghiệp vụ khi cần analytics/debug/bootstrap projection.
- Giảm nhu cầu viết poller outbox sớm ở local/dev hoặc ở môi trường chấp nhận Kafka Connect là dependency chuẩn.

Không dùng Debezium như public contract mặc định cho mọi bảng nghiệp vụ.

Khuyến nghị:

- `outbox_events` là luồng chính cho integration event public.
- Dùng outbox SMT/router để emit thẳng business topic, không buộc consumer đọc topic CDC thô.
- Mỗi connector có replication slot riêng, tên rõ owner/use case.
- CDC bảng nghiệp vụ chỉ bật opt-in; không stream toàn bộ schema theo thói quen.
- Poller outbox có thể giữ như fallback/replay tool, nhưng không nên là local/dev default khi đã có Debezium stack.

Khi nào dùng `outbox_events`:

- Use case vừa ghi business state vừa cần phát integration event.
- Event sẽ được module hoặc runtime khác consume lâu dài.
- Cần event naming/versioning ổn định như `identity.user.registered.v1`.
- Cần tránh consumer phụ thuộc trực tiếp vào schema bảng nghiệp vụ.

Flow khuyến nghị:

```text
use case transaction
  -> save business state
  -> insert outbox_events row
  -> commit

Debezium outbox connector
  -> capture WAL change
  -> route row theo cột topic
  -> publish business topic lên Kafka
```

Khi nào CDC bảng nghiệp vụ trực tiếp:

- Cần feed analytics/raw lake/debug stream.
- Cần bootstrap projection không phải contract public chính.
- Chấp nhận consumer biết schema của producer table.

Không nên dùng CDC bảng nghiệp vụ trực tiếp để thay integration event contract nếu:

- Event cần versioning/public compatibility rõ.
- Producer còn có thể refactor schema bảng thường xuyên.
- Consumer chỉ cần sự kiện business chứ không cần toàn bộ row change.

Nếu sau này implement local/dev stack thật trong repo, nên có tối thiểu:

```text
docker-compose*.yml cho PostgreSQL, Kafka, Kafka Connect, Kafka UI
connector config cho outbox_events
connector template cho bảng CDC opt-in khác
bootstrap schema local đủ để verify outbox flow
```

Checklist verify khi bắt đầu implement:

- Kafka Connect register được connector outbox mặc định.
- Insert vào `outbox_events` tạo ra business topic như `identity.user.registered`.
- Message key theo `aggregate_id` hoặc key tương đương giữ ordering.
- Header hoặc metadata giữ được `event_type`, `aggregate_type`, `aggregate_id`, `workspace_id` nếu cần.
- CDC bảng khác chỉ được bật opt-in, không bật mặc định cho toàn schema.

## ClickHouse

ClickHouse là **analytics/event store** cho email delivery events khi volume bắt đầu lớn.

## Object storage (MinIO/S3)

Object storage là nơi lưu artifact file (import/export audience, raw archive, debug payload) theo mô hình async.

Dùng cho:

- Import source file lớn (CSV/JSONL) trước khi worker xử lý.
- Export artifact có TTL.
- Raw event archive khi cần debug/replay ngắn hạn.

Trong local development:

- Dùng MinIO (S3-compatible API) trong `deployments/local/docker-compose.yml`.
- Backend truy cập qua S3 SDK với endpoint local và path-style.
- Bucket local mặc định: `sendflow-local` (backend tự ensure/create khi startup).

Dùng cho:

- Delivery/open/click/bounce/complaint event analytics.
- Campaign reporting theo thời gian, tenant, provider, recipient domain.
- Funnel: queued -> accepted -> delivered -> opened -> clicked -> unsubscribed.
- Dashboard truy vấn aggregate lớn, ví dụ hourly/daily metrics.
- Raw-ish event fact table có retention dài hơn hot PostgreSQL projection.

Không dùng ClickHouse cho:

- Source of truth của business state.
- Transactional workflow state.
- Constraint quan trọng như unique email, tenant membership, suppression decision.
- Request path cần strong consistency ngay lập tức.

Ingestion path khuyến nghị:

```text
provider webhook / delivery worker
  -> normalize event
  -> publish Kafka event
  -> analytics consumer
  -> batch insert ClickHouse
  -> materialized views for dashboard
```

PostgreSQL vẫn giữ state quan trọng:

```text
messages
campaigns
suppression_entries
sender_domains
provider_webhook_events dedupe/projection nếu cần
```

ClickHouse giữ event analytics:

```text
email_events
campaign_daily_stats
recipient_domain_hourly_stats
provider_delivery_stats
```

Schema fact table gợi ý:

```text
email_events
├── event_date Date
├── occurred_at DateTime64
├── received_at DateTime64
├── tenant_id String
├── campaign_id String
├── message_id String
├── recipient_domain LowCardinality(String)
├── provider LowCardinality(String)
├── sending_domain String
├── event_type LowCardinality(String)
├── provider_message_id String
├── metadata_json String
└── inserted_at DateTime64
```

Engine gợi ý:

```text
MergeTree
PARTITION BY toYYYYMM(event_date)
ORDER BY (tenant_id, campaign_id, event_type, occurred_at)
TTL event_date + INTERVAL 18 MONTH
```

Khuyến nghị:

- Insert theo batch, không insert từng event một nếu traffic cao.
- Consumer phải idempotent hoặc có dedupe strategy vì Kafka có thể redeliver.
- Không query ClickHouse để quyết định có gửi email hay không; suppression decision vẫn ở PostgreSQL/Redis.
- Materialized view dùng cho dashboard thường xuyên xem.
- Retention raw event và aggregate có thể khác nhau.

Package đề xuất:

```text
internal/platform/clickhouse
├── client.go
├── batch.go
└── health.go
```

Library Go:

```text
github.com/ClickHouse/clickhouse-go/v2
```

Chỉ thêm ClickHouse khi:

- PostgreSQL bắt đầu chậm vì event aggregate/reporting.
- Event volume đủ lớn để batch analytics có ý nghĩa.
- Product cần campaign analytics gần real-time.

## Observability

LGTM gồm:

```text
Loki     # logs
Grafana  # dashboard/visualization
Tempo    # traces
Mimir    # metrics, Prometheus-compatible
```

Stack này nên đi cùng OpenTelemetry trong app.

Dùng cho:

- Structured logs: request id, trace id, user id, tenant id, module, use case.
- Metrics: latency, throughput, error rate, DB pool, Kafka lag, Redis errors.
- Traces: HTTP request -> use case -> DB/Kafka/Redis call.
- Dashboard và alerting.

Khuyến nghị package:

```text
internal/platform/observability
├── logger.go       # slog/zap setup, trace-aware fields
├── metrics.go      # Prometheus/Mimir metrics
├── tracing.go      # OpenTelemetry tracer/provider
└── middleware.go   # HTTP/Kafka instrumentation helpers
```

Chỉ số tối thiểu nên có:

- HTTP: request count, latency p95/p99, status code, route.
- Use case: duration, success/failure count.
- PostgreSQL: query duration, pool in-use/idle, tx rollback count.
- Kafka: consumer lag, processed count, retry count, DLQ count.
- Redis: operation latency, hit/miss ratio, lock acquisition failure.
- Outbox: unpublished count, publish failure count, oldest unpublished age.

Alert tối thiểu:

- API 5xx tăng bất thường.
- Kafka consumer lag vượt ngưỡng.
- Outbox oldest unpublished age quá cao.
- PostgreSQL connection pool cạn.
- DLQ có message mới.
- Redis unavailable nếu đang dùng session/rate limit/idempotency.

## Feature flags

Feature flags hữu ích khi sản phẩm có rollout theo tenant hoặc cần kill switch.

Dùng cho:

- Rollout flow/campaign feature theo tenant.
- Bật/tắt integration provider.
- Kill switch khi một worker gây lỗi.
- A/B hoặc gradual rollout đơn giản.

Không dùng feature flag để thay config lâu dài. Flag nên có owner và ngày dọn.

Package đề xuất:

```text
internal/platform/featureflags
├── client.go
├── evaluator.go
└── noop.go
```

Giai đoạn đầu có thể implement bằng config/local file. Khi cần UI/audit/rollout động, cân nhắc Unleash, OpenFeature-compatible provider hoặc LaunchDarkly.

## Secrets management

Không commit secret vào repo và không để secret thật trong `ops/configs/*.yaml`.

Secret gồm:

- Database password.
- Redis/Kafka credentials.
- JWT signing key.
- OAuth client secret.
- SMTP/API provider key.
- Encryption key.

Khuyến nghị:

- Local dev dùng `.env.local` hoặc secret file không commit.
- Dev/staging/prod dùng secret manager của môi trường deploy.
- App đọc secret qua `platform/secrets`, không đọc rải rác trực tiếp từ env.
- Config file chỉ chứa non-secret default và tên secret reference.
- Key rotation phải được tính cho JWT/encryption key nếu dùng lâu dài.

Package đề xuất:

```text
internal/platform/secrets
├── provider.go
├── env.go
└── file.go
```

## Local development stack

Nên có Docker Compose local để dev chạy được toàn bộ backbone.

Tối thiểu:

```text
PostgreSQL
Redis
Kafka
Kafka Connect / Debezium
ClickHouse
Loki
Grafana
Tempo
Mimir hoặc Prometheus-compatible metrics backend
```

Gợi ý thực tế:

- Với CDC/local outbox stack, ưu tiên Kafka-native setup có Kafka Connect support rõ ràng hơn Redpanda tối giản.
- Dùng **Prometheus** local nếu Mimir quá nặng cho máy dev; production vẫn có thể dùng Mimir.
- Dùng ClickHouse local để test analytics consumer/materialized view khi module analytics bắt đầu có thật.
- Seed topic, migration và dashboard bằng script trong `ops/scripts`.
- Có healthcheck cho từng service để API/Worker không start quá sớm.
- Nếu cần verify event propagation trước khi có backend app hoàn chỉnh, nên chuẩn bị bootstrap `outbox_events` schema local và connector mẫu như một phần của local stack.

Thư mục đề xuất:

```text
ops/deployments/compose
├── docker-compose.yml
├── grafana
│   ├── dashboards
│   └── datasources
└── observability
    ├── loki.yml
    ├── tempo.yml
    └── prometheus.yml
```

## Go library baseline

Chọn library nên ưu tiên ít magic, dễ test, dễ thay.

Baseline tối giản nên dùng:

```text
HTTP router       github.com/go-chi/chi/v5
DI                github.com/google/wire
PostgreSQL        github.com/jackc/pgx/v5/pgxpool
Migration         github.com/pressly/goose/v3 hoặc github.com/golang-migrate/migrate/v4
Redis             github.com/redis/go-redis/v9
Kafka             github.com/twmb/franz-go
ClickHouse        github.com/ClickHouse/clickhouse-go/v2
Validation        github.com/go-playground/validator/v10
Logging           log/slog hoặc go.uber.org/zap
Tracing/Metrics   go.opentelemetry.io/otel
Testing           testing + github.com/stretchr/testify
Containers test   github.com/testcontainers/testcontainers-go
Config            github.com/knadh/koanf hoặc custom loader nhỏ
```

Nếu muốn một combo gọn cho giai đoạn đầu:

```text
chi + wire + pgxpool + goose + go-redis + franz-go + slog + OpenTelemetry + testcontainers-go
```

Gợi ý chi tiết theo nhóm:

| Nhóm | Library | Ghi chú |
|---|---|---|
| HTTP router | `github.com/go-chi/chi/v5` | Nhẹ, gần `net/http`, middleware rõ |
| HTTP render | custom JSON helper hoặc `github.com/go-chi/render` | Custom helper là đủ nếu API đơn giản |
| CORS | `github.com/go-chi/cors` | Dùng allowlist explicit ở production |
| Request ID | `github.com/go-chi/chi/v5/middleware` | Có sẵn request id, recoverer, timeout |
| DI | `github.com/google/wire` | Compile-time DI |
| PostgreSQL driver/pool | `github.com/jackc/pgx/v5/pgxpool` | Nên dùng trực tiếp thay `database/sql` nếu muốn control tốt |
| SQL builder | `github.com/Masterminds/squirrel` | Optional, hữu ích khi query dynamic |
| Type-safe SQL | `github.com/sqlc-dev/sqlc` | Rất đáng cân nhắc nếu muốn query SQL rõ và type-safe |
| Migration | `github.com/pressly/goose/v3` | Đơn giản, hợp project Go |
| Migration alternative | `github.com/golang-migrate/migrate/v4` | Phổ biến, tốt cho CI/CD |
| Redis | `github.com/redis/go-redis/v9` | Client chuẩn, hỗ trợ tracing hook |
| Kafka | `github.com/twmb/franz-go` | Mạnh, performant, control sâu |
| Kafka alternative | `github.com/segmentio/kafka-go` | API dễ đọc hơn, đủ dùng nhiều case |
| ClickHouse | `github.com/ClickHouse/clickhouse-go/v2` | Dùng cho analytics event batch insert/query |
| Validation | `github.com/go-playground/validator/v10` | Dùng ở API/app command boundary |
| Config | `github.com/knadh/koanf` | Gọn, merge file/env/flags tốt |
| Env decode | `github.com/caarlos0/env/v11` | Tốt nếu config chủ yếu từ env |
| Logging | `log/slog` | Native, đủ tốt; nên bắt đầu từ đây |
| Logging alternative | `go.uber.org/zap` | Khi cần performance/control cao |
| OpenTelemetry | `go.opentelemetry.io/otel` | Chuẩn instrumentation |
| Metrics | `go.opentelemetry.io/otel/metric` | Export sang Prometheus/Mimir |
| UUID | `github.com/google/uuid` | Đơn giản, phổ biến |
| ULID | `github.com/oklog/ulid/v2` | Hữu ích nếu muốn sortable id |
| Password hash | `golang.org/x/crypto/argon2` hoặc `golang.org/x/crypto/bcrypt` | Argon2id tốt hơn, bcrypt đơn giản hơn |
| JWT | `github.com/golang-jwt/jwt/v5` | Chỉ dùng nếu tự quản JWT |
| Rate limit | `golang.org/x/time/rate` | Local rate limit đơn giản |
| Distributed rate limit | Redis custom hoặc `github.com/redis/go-redis/extra/redisotel/v9` + script riêng | Dùng khi multi-instance |
| In-flight dedupe | `golang.org/x/sync/singleflight` | Chống cache stampede trong cùng process |
| Retry/backoff | `github.com/cenkalti/backoff/v4` | Hữu ích cho external call |
| Cron | `github.com/robfig/cron/v3` | Scheduled job đơn giản |
| Error wrapping | standard `errors` | Ưu tiên standard library |
| CLI/tooling | `github.com/spf13/cobra` | Chỉ cần nếu có CLI admin/migration tool |
| Test assert | `github.com/stretchr/testify` | Assert/require/suite |
| Mock | `go.uber.org/mock` hoặc `github.com/vektra/mockery/v2` | Chọn một, đừng trộn |
| DB test | `github.com/testcontainers/testcontainers-go/modules/postgres` | Integration test thật |
| Redis test | `github.com/testcontainers/testcontainers-go/modules/redis` | Integration test thật |
| Kafka test | Redpanda/Kafka qua testcontainers | Test consumer/outbox thật |

Thứ nên tránh đưa vào sớm:

- ORM nặng nếu team vẫn đang ổn với SQL/`sqlc`.
- Runtime DI container kiểu service locator.
- Framework quá opinionated nếu project muốn giữ Clean Architecture rõ.
- Nhiều library cùng làm một việc, ví dụ vừa `zap` vừa `zerolog` vừa `slog`.
- Custom abstraction quá sớm cho mọi dependency; chỉ bọc lại khi boundary thật sự cần.

Lựa chọn khuyến nghị hiện tại:

```text
Router      chi
DB          pgxpool + sqlc hoặc pgxpool + SQL hand-written
Migration   goose
Kafka       franz-go
Redis       go-redis
Analytics   ClickHouse khi event volume tăng
Config      koanf
Logging     slog
Tracing     OpenTelemetry
Testing     testify + testcontainers-go
Mock        go.uber.org/mock
```

## Khi nào chưa nên thêm hạ tầng mới

Nên thêm sớm:

- **OpenTelemetry** làm chuẩn instrumentation, để sau này đổi backend observability ít đau.
- **Schema/version convention cho event** trước khi có nhiều consumer.
- **Idempotency store** cho API quan trọng và Kafka consumer.
- **Feature flags** nếu flow/campaign có rollout theo tenant.
- **Secrets management** theo môi trường, không để secret trong `ops/configs/*.yaml`.
- **Docker Compose local stack** gồm PostgreSQL, Kafka, Debezium/Kafka Connect và các dependency thật sự cần cho flow đang làm.

Chưa cần vội nếu project còn nhỏ:

- Tách microservice theo module.
- Service mesh.
- CQRS read database riêng cho mọi module.
- Event sourcing toàn hệ thống.
- Kubernetes phức tạp ngay từ đầu.
