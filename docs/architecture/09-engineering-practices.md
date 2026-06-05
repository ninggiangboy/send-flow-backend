# Engineering Practices

File này là working agreement cho team: test thế nào, ship theo phase nào, đặt tên ra sao, config ở đâu, và checklist trước khi coi feature là xong.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Code architecture | `04-code-architecture.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| API map | `../api/00-api-map.md` |
| Event/outbox flow | `05-data-events-and-flows.md` |
| Infrastructure | `06-infrastructure.md` |
| Testing guide | `../testing/00-testing-guide.md` |

## Testing strategy

Chi tiết đầy đủ về cách viết test, chọn layer test, naming, fixture/fake và local/CI workflow nằm ở `../testing/00-testing-guide.md`.

Baseline của repo:

```text
unit          ~80%
integration  ~15%
e2e           ~5%
```

Tóm tắt rule chính:

- `Unit`: bắt invariant, state transition, validation, error mapping, orchestration decision.
- `Integration`: bắt query/mapping/transaction/outbox/adapter boundary với dependency thật khi cần.
- `E2E`: chỉ giữ cho vài flow critical như API -> DB -> outbox -> worker.
- Domain thuần không mock nếu có thể giữ pure.
- App/use case mock repository và ports để test orchestration.
- Async flow phải test idempotency, rollback và publish timing.

Ví dụ vị trí:

```text
modules/identity/domain/user_test.go
modules/identity/app/signup/handler_test.go
modules/identity/infrastructure/postgres/user_repository_test.go
tests/api/identity_test.go
tests/worker/outbox_test.go
internal/platform/postgres/tx_test.go
```

---

## Implementation roadmap

Không cần build toàn bộ stack ngay từ ngày đầu. Roadmap nên đi theo mức rủi ro và dependency thật sự của sản phẩm.

### Phase 1. Backend skeleton

Mục tiêu: API chạy được, module đầu tiên có use case thật, DB migration ổn.

- Scaffold `cmd/api`, `internal/apps/api`, `internal/modules/identity`, `internal/platform/config`, `logger`, `postgres`.
- Chọn router `chi`, DI `wire`, DB `pgxpool`, migration `goose`.
- Implement health/readiness endpoint.
- Implement config loader và env convention.
- Implement use case đầu tiên: `identity/signup` hoặc `identity/login`.
- Có unit test cho domain/app và integration test cho repository.

Chưa cần Kafka/LGTM đầy đủ ở phase này nếu chưa có async flow thật.

### Phase 2. Redis và security baseline

Mục tiêu: auth/session/rate limit/idempotency cơ bản.

- Thêm `platform/redis`.
- Thêm session/refresh token store nếu auth cần revoke.
- Thêm rate limit cho auth endpoint.
- Thêm idempotency key cho command dễ bị retry.
- Thêm password hashing, JWT/key loading, CORS allowlist, request size limit.

### Phase 3. Kafka, Debezium và transactional outbox

Mục tiêu: async side effect chạy đúng, không mất event.

- Thêm `platform/kafka`, `platform/events`.
- Thêm outbox table.
- Ưu tiên Debezium/Kafka Connect cho local/dev event propagation từ `outbox_events`.
- Outbox publisher/poller giữ như fallback, replay worker hoặc lựa chọn cho môi trường chưa có Kafka Connect.
- Event contract có `event_type` versioned.
- Consumer idempotent.
- Retry/DLQ convention rõ.
- E2E test: API command -> DB row -> outbox row -> Kafka publish -> consumer xử lý.

### Phase 4. Email delivery core

Mục tiêu: gửi email theo pipeline đúng và bảo vệ deliverability baseline.

- Thêm `sender`, `template`, `delivery`, `suppression`, `webhook` module tối thiểu.
- Tích hợp provider đầu tiên, ưu tiên AWS SES.
- Domain verification workflow cho SPF/DKIM/DMARC.
- Suppression check trước khi enqueue/send.
- Provider webhook ingestion cho bounce/complaint/delivery.
- One-click unsubscribe cho marketing/subscription email.
- Tracking click/unsubscribe trước; open tracking có thể thêm sau.
- Fake provider/Mailpit cho local test.

### Phase 5. Observability

Mục tiêu: nhìn được hệ thống khi có lỗi.

- Thêm OpenTelemetry tracing.
- Thêm structured logs có request id/trace id.
- Thêm metrics HTTP/use case/DB/Kafka/Redis/outbox.
- Thêm metrics email: bounce rate, complaint rate, queue age, provider error, webhook ingestion failure.
- Thêm Docker Compose local cho Loki, Grafana, Tempo, Prometheus/Mimir.
- Thêm dashboard tối thiểu và alert rule cơ bản.

### Phase 6. Hardening

Mục tiêu: production-ready.

- Graceful shutdown đầy đủ cho API/Worker.
- Timeout/retry/backoff cho external call.
- Backpressure, circuit breaker, bulkhead isolation cho provider/dependency quan trọng.
- Priority queue và adaptive throttling cho transactional/marketing delivery.
- Queue-based load leveling và competing consumers cho workload spike.
- Exponential backoff + jitter, retry budget, poison message DLQ.
- Timeout budget/deadline propagation cho API, worker và downstream call.
- Token bucket/leaky bucket cho quota, warm-up và provider throttling.
- Leader election/distributed singleton cho scheduled coordinator job.
- Replay/backfill tooling cho webhook, DLQ, outbox, ClickHouse analytics.
- Tenant fairness và abuse control.
- Retention/TTL policy cho PostgreSQL, ClickHouse, object storage.
- Migration rollback/forward strategy.
- Backup/restore drill cho PostgreSQL.
- Secret rotation plan.
- Load/capacity test critical endpoint và delivery pipeline.
- Runbook cho incident phổ biến: DB down, Kafka lag, outbox stuck, Redis unavailable, provider outage, bounce/complaint spike.

---

## Architecture decision records

Các quyết định dưới đây là baseline hiện tại. Nếu sau này đổi, thêm ADR mới thay vì âm thầm sửa code.

| Decision | Chọn | Không chọn mặc định | Lý do |
|---|---|---|---|
| HTTP | `chi + net/http` | Gin/Echo/Fiber | Nhẹ, ít magic, hợp Clean Architecture |
| DI | `wire` | fx/dig/service locator | Compile-time, tránh runtime magic |
| DB access | `pgxpool + sqlc` hoặc SQL viết tay | ORM nặng | SQL rõ, type-safe nếu dùng `sqlc`, ít abstraction |
| Migration | `goose` | nhiều migration tool cùng lúc | Đơn giản, dễ chạy local/CI |
| Kafka client | `franz-go` | tự viết wrapper thấp tầng quá sớm | Client mạnh, control tốt |
| Redis client | `go-redis` | custom protocol/client | Chuẩn ecosystem |
| Logging | `slog` | nhiều logger song song | Native, đủ tốt để bắt đầu |
| Observability | OpenTelemetry + LGTM | vendor-specific instrumentation | Dễ đổi backend observability |
| Module style | Modular monolith | microservices sớm | Giảm distributed complexity khi domain chưa ổn định |

Nguyên tắc ADR:

- Quyết định quan trọng phải ghi lý do và trade-off.
- Không cần văn vẻ dài; 5-10 dòng là đủ.
- Nếu đổi quyết định, ghi quyết định mới và trạng thái của quyết định cũ.

Thư mục đề xuất:

```text
docs/adr
├── 0001-use-chi-for-http.md
├── 0002-use-wire-for-di.md
└── 0003-use-transactional-outbox.md
```

---

## Naming convention

Naming nhất quán giúp repo dễ đoán.

### Package và folder

```text
package/folder     lowercase, không underscore nếu không cần
use case package   động từ hoặc verb+noun: signup, login, refreshsession, getuser
module             domain noun: identity, notification, billing
platform package   technical noun: postgres, redis, kafka, observability
```

Không dùng:

```text
user_service
UserService
identityModule
common
utils
helpers
```

Nếu phải tạo `utils`, thường là chưa tìm đúng owner package.

### Common utilities policy

Không tạo package root kiểu:

```text
internal/common
internal/utils
internal/helpers
pkg/utils
```

Thay vào đó, đặt theo owner/capability:

```text
Technical helper dùng chung     -> internal/platform/<capability>
Domain concept dùng chung       -> internal/sharedkernel/<concept>
Chỉ dùng trong một module       -> internal/modules/<module>/<layer>
Chỉ dùng trong API/Worker       -> internal/apps/<runtime>/...
```

Ví dụ:

| Nhu cầu | Đặt ở đâu |
|---|---|
| Chunk slice generic | `internal/platform/batch` |
| Flush theo size/interval generic | `internal/platform/batch` |
| Goroutine group/concurrency limit generic | `internal/platform/async` |
| Retry/backoff external call | `internal/platform/retry` |
| Scheduled job primitive | `internal/platform/scheduler` hoặc `internal/apps/worker` |
| Batch insert ClickHouse | `internal/platform/clickhouse` |
| Kafka consumer worker group | `internal/platform/kafka` |
| Outbox polling loop | `modules/<module>/infrastructure/outbox` hoặc `apps/worker` |
| Email delivery throttling | `modules/delivery/app` hoặc `modules/delivery/infrastructure` |
| HTTP JSON response helper | `internal/apps/api/response` hoặc `internal/apps/api/httpjson` |
| Email normalization | `internal/sharedkernel/email` nếu thật sự dùng chung |
| DNS lookup helper | `internal/platform/dns` hoặc `modules/sender/infrastructure/dns` |

Chỉ kéo lên `platform` khi có ít nhất 2-3 nơi dùng chung thật. Nếu mới có một caller, đặt gần caller trước.

Ví dụ structure nếu đã chứng minh dùng chung:

```text
internal/platform
├── async
│   ├── group.go
│   ├── semaphore.go
│   └── worker_pool.go
├── batch
│   ├── chunk.go
│   ├── flush.go
│   └── pipeline.go   # chỉ khi đã có nhiều pipeline giống nhau
└── retry
    └── backoff.go
```

Không tạo abstraction chỉ vì “có thể sẽ dùng chung”. Platform là nơi đã chứng minh dùng chung, không phải nơi chứa mọi ý tưởng có vẻ reusable.

Batch/async primitive nên nhỏ:

```text
batch.Chunk
batch.FlushOnSizeOrInterval
async.Group
async.Semaphore
async.WorkerPool
```

Pipeline cụ thể nên nằm gần domain/worker cho đến khi pattern lặp lại rõ ràng.

### Event/topic

```text
topic       <module>.<aggregate>.<event>
event_type  <module>.<aggregate>.<event>.v<version>
dlq topic   <topic>.dlq
```

Ví dụ:

```text
identity.user.registered
identity.user.registered.v1
identity.user.registered.dlq
```

### Error code

```text
<MODULE>_<REASON>
```

Ví dụ:

```text
IDENTITY_EMAIL_ALREADY_REGISTERED
IDENTITY_INVALID_PASSWORD
BILLING_SUBSCRIPTION_EXPIRED
```

### Env var

```text
APP_ENV
HTTP_PORT
POSTGRES_DSN
REDIS_ADDR
KAFKA_BROKERS
OTEL_EXPORTER_OTLP_ENDPOINT
```

---

## Coding rules

Các rule này quan trọng hơn việc chọn library.

- Không truyền `gin.Context`, `echo.Context`, `chi` route context hoặc `http.ResponseWriter` vào use case.
- Use case chỉ nhận `context.Context` chuẩn và command/query struct.
- Không import `infrastructure` từ `app`.
- Không import `apps` từ `modules`.
- Không ignore error bằng `_` trong production code.
- Không publish Kafka trực tiếp trong use case nếu use case cũng ghi DB.
- Không đặt HTTP DTO trong `contracts`.
- Không log password/token/API key/raw authorization header.
- Không dùng global mutable state cho dependency runtime.
- Không tạo abstraction chỉ để “cho clean”; chỉ tạo khi có boundary thật.
- Không để business rule trong handler, repository hoặc mapper.
- Không gửi email marketing nếu chưa check suppression/consent.
- Không coi provider `accepted` là `delivered`.
- Không xử lý provider webhook mà không verify signature.
- Không dùng queue in-memory vô hạn.
- Không để retry queue chiếm hết worker fresh delivery.
- Không để analytics/dashboard path block delivery path.
- Không dùng Redis `KEYS` để xóa theo prefix trong production; dùng `SCAN` + batch delete.
- Không dùng Redis lock nếu thiếu TTL và owner token.
- Không để cache miss stampede gọi source of truth đồng loạt; dùng singleflight/in-flight dedupe và lock khi cần.
- Không để cache set/delete lỗi làm fail business flow nếu source of truth đã thành công.
- Không retry không giới hạn; retry phải có backoff, jitter, max attempts hoặc retry budget.
- Không gọi downstream nếu không có timeout/deadline.
- Không dùng hedged request cho operation có side effect như gửi email hoặc ghi DB.
- Không để poison message quay lại main queue vô hạn; phân loại rồi đưa DLQ.
- Không chạy singleton scheduled job bằng niềm tin; dùng lease/lock có TTL và owner.

Rule cho transaction:

- Transaction bắt đầu ở application use case.
- Repository nhận `ctx` đã gắn transaction qua `TxManager` hoặc abstraction tương đương.
- Không expose `*sql.Tx`/`pgx.Tx` lên API handler.
- Ghi business state và outbox event trong cùng transaction.

Rule cho context:

- `context.Context` dùng cho cancellation, deadline, trace, request scope.
- Không nhét business input vào context nếu có thể đưa vào command/query.
- Không lưu context trong struct lâu dài.

---

## Config và env convention

Config chia làm hai loại:

```text
non-secret config  # port, timeout, feature default, log level
secret             # password, token, signing key, provider credential
```

Non-secret có thể nằm trong `ops/configs/*.yaml`. Secret phải đi qua env/secret manager.

Env tối thiểu:

```text
APP_ENV=local
APP_NAME=send-flow-api
HTTP_PORT=8081
LOG_LEVEL=debug

POSTGRES_DSN=postgres://sendflow:sendflow@localhost:5432/sendflow?sslmode=disable
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=

KAFKA_BROKERS=localhost:9092
KAFKA_CLIENT_ID=send-flow

JWT_ISSUER=send-flow
JWT_ACCESS_TOKEN_TTL=15m
JWT_REFRESH_TOKEN_TTL=720h

OTEL_SERVICE_NAME=send-flow-api
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317

EMAIL_PROVIDER=ses
EMAIL_DEFAULT_FROM_DOMAIN=mail.example.com
WEBHOOK_PUBLIC_BASE_URL=https://api.example.com
```

Config loading order:

```text
default values
  -> ops/configs/<env>.yaml
  -> env vars
  -> secret provider
```

Không để code đọc env trực tiếp ở mọi nơi. Gom vào `platform/config` và `platform/secrets`.

---

## Security baseline

Security nên có ngay từ skeleton, vì sửa muộn thường mệt.

Nên có:

- Password hashing bằng Argon2id hoặc bcrypt.
- JWT access token ngắn hạn, refresh token có revoke.
- CORS explicit allowlist, không wildcard ở production.
- Request size limit.
- Rate limit cho auth endpoint và public endpoint.
- Audit log cho action quan trọng.
- PII masking trong log.
- Input validation ở API layer và invariant validation ở domain.
- Encryption at rest cho secret/credential do app lưu hộ user nếu có.

Không log:

- Password, token, API key.
- Full authorization header.
- Raw webhook secret.
- PII nhạy cảm nếu không cần debug.

## Background jobs và scheduling

Worker có hai loại workload khác nhau:

```text
Consumer job   # đọc Kafka event
Scheduled job  # chạy theo lịch, ví dụ cleanup, replay, retry, fallback outbox poller
```

Khuyến nghị:

- Fallback outbox publisher là scheduled/loop job riêng, có lock nếu chạy nhiều replica.
- Cleanup expired session/idempotency/audit retention nên là scheduled job.
- Scheduled job cần metrics: last success time, duration, failure count.
- Job phải support graceful shutdown.
- Job dài nên checkpoint được, tránh restart là làm lại từ đầu.

## Async batch pipeline pattern

Các workload lớn nên dùng pipeline có backpressure thay vì loop xử lý tuần tự hoặc spawn goroutine vô hạn.

Pattern:

```text
Reader
  -> bounded input queue
  -> Processor pool
  -> bounded output queue
  -> Batch writer
```

Dùng cho:

```text
outbox publisher hoặc Debezium-fed batch source
ClickHouse analytics consumer
provider webhook replay
campaign recipient expansion
suppression import
delivery retry processor
```

Thành phần:

```text
Reader       đọc item theo shard/checkpoint
Processor    transform/validate/enrich song song
Writer       ghi batch, flush theo size hoặc interval
Queue        bounded để tạo backpressure
Inflight     semaphore/limit để giới hạn work-in-progress
Context      cancellation/graceful shutdown
Metrics      read/processed/written/failed/inflight/queue age
```

Rule:

- Queue phải bounded.
- Có `max_inflight`, không chỉ giới hạn queue size.
- Writer phải flush khi đủ batch size, hết interval, hoặc shutdown.
- Writer phải idempotent hoặc có dedupe strategy.
- Lỗi ở stage nào phải cancel toàn pipeline.
- Không dùng goroutine unbounded cho từng item.
- Không release semaphore kiểu làm sai counter để unblock; dùng `context` cancellation.
- Long-running reader phải checkpoint được.

Pseudo-flow:

```text
start reader
start N processors
start writer

reader:
  read item
  acquire inflight
  send to input queue
  on EOF send end signal

processor:
  receive item
  process item
  send output to output queue
  if filtered/no output release inflight

writer:
  collect outputs
  flush when len(batch) >= batch_size
  flush when interval tick
  after successful write release inflight by batch size
```

Config gợi ý:

```text
buffered_items_size
processor_concurrency
write_batch_size
flush_interval
max_inflight
shard_id
total_shards
```

Package placement:

```text
Generic primitives        -> internal/platform/batch, internal/platform/async
ClickHouse batch writer   -> internal/platform/clickhouse
Outbox publisher loop     -> apps/worker hoặc modules/<module>/infrastructure/outbox
Campaign expansion        -> modules/campaign/app
Suppression import        -> modules/suppression/app
```

Chỉ tạo generic pipeline framework nếu có nhiều pipeline thật sự giống nhau. Nếu mới có một use case, implement gần use case trước rồi extract sau.

## API contract và documentation

Nên có OpenAPI cho HTTP API khi public surface bắt đầu ổn định.

Khuyến nghị:

- API DTO nằm ở `internal/apps/api/modules/<module>/dto.go`.
- OpenAPI spec generate từ code hoặc maintain riêng, chọn một cách và giữ nhất quán.
- Error response có shape thống nhất.
- Public API versioning đặt ở route hoặc header, ví dụ `/api/v1`.

Error response gợi ý:

```json
{
  "error": {
    "code": "IDENTITY_EMAIL_ALREADY_REGISTERED",
    "message": "email already registered",
    "request_id": "req_123"
  }
}
```

---

## Shared Kernel

`sharedkernel` chỉ dành cho concept dùng chung, ổn định, không thuộc riêng module nào.

Nên có:

```text
EmailAddress
Money
Currency
PhoneNumber
TenantID
Pagination
```

Không nên có:

```text
UserStatus
OrderStatus
CampaignType
PlanName
```

Nếu một concept có owner rõ ràng, đặt nó trong module owner. Đừng đưa vào shared kernel chỉ vì có hai nơi đang muốn dùng.

---

## Definition of Done cho backend feature

Một backend feature được xem là xong khi:

- Có domain/use case rõ owner module.
- Có migration nếu thay đổi schema.
- Có API DTO hoặc event contract nếu feature lộ ra ngoài.
- Error được map đúng từ domain -> app -> runtime.
- Có unit test cho domain/app path quan trọng.
- Có integration test cho repository/adapter rủi ro.
- Có metrics/logs/traces cho flow quan trọng.
- Có idempotency nếu command/consumer có khả năng retry.
- Có outbox nếu vừa ghi DB vừa phát event.
- Có suppression/consent check nếu feature gửi email.
- Có provider webhook handling nếu feature phụ thuộc delivery outcome.
- Có backpressure/retry/idempotency policy nếu feature chạy async hoặc tải cao.
- Có bounded async batch pipeline nếu feature xử lý nhiều item hoặc batch write.
- Có SLO/metric/alert nếu feature nằm trên critical path.
- Có config/env/secret documented nếu thêm dependency mới.
- Có rollback hoặc mitigation note nếu rollout rủi ro.

---

## Checklist khi thêm module mới

Khi thêm một module, kiểm tra:

- Module có owner/domain language rõ ràng.
- Domain không import app/infrastructure/platform driver.
- Use case không import infrastructure.
- API/Worker DTO không đặt trong `contracts`.
- Public event contract có version.
- Use case ghi DB + phát event dùng outbox.
- Consumer xử lý idempotent.
- Tech stack dùng đúng vai trò: PostgreSQL là source of truth, Redis là ephemeral state, Kafka là event backbone.
- Nếu module thuộc email delivery, đã xét provider adapter, suppression, webhook, throttle và tracking impact.
- Nếu module có tải cao, đã xét backpressure, circuit breaker, bulkhead, priority, queue leveling, retry jitter, timeout budget, replay và retention.
- Metrics/logs/traces tối thiểu đã được gắn cho use case quan trọng.
- Secret không nằm trong config commit vào repo.
- Feature flag có owner và plan dọn nếu dùng cho rollout tạm thời.
- API public có DTO/error shape/OpenAPI rõ ràng.
- Error được map từ domain -> app -> runtime.
- Có unit test cho domain/app quan trọng.
- Có integration test cho repository hoặc adapter có rủi ro.

---

## Tóm tắt quyết định

Kiến trúc chọn:

```text
Domain không duplicate.
Core use case không duplicate.
API/Worker orchestration tách riêng.
Module giao tiếp qua event/contract.
Outbox bảo vệ consistency giữa DB và Kafka.
PostgreSQL giữ business state, Redis giữ ephemeral state, Kafka giữ event stream.
LGTM + OpenTelemetry là nền observability.
Idempotency, feature flags, secrets management và local compose stack là phần baseline.
Email delivery core gồm provider adapter, sender authentication, suppression, webhook ingestion, throttling, tracking và analytics.
High-load baseline gồm backpressure, circuit breaker, bulkhead, priority queue, adaptive throttling, replay, tenant fairness và retention.
Named resilience patterns gồm queue-based load leveling, competing consumers, retry with jitter, timeout budget, fail fast, token bucket, poison message handling, leader election, hedged requests và saga/process manager.
Worker/batch processing dùng bounded async pipeline: reader, processor pool, batch writer, max inflight, flush size/interval.
Roadmap, ADR, naming convention, coding rules và Definition of Done giữ team đi cùng hướng.
Dependency direction ưu tiên tránh import cycle trong Go.
```
