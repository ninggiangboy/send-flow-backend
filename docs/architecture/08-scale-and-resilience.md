# Scale And Resilience

Các pattern chịu tải cao không nên được áp dụng như checklist. Chọn pattern theo symptom: nghẽn queue, external dependency chậm, retry storm, tenant abuse, dữ liệu quá lớn, hoặc projection bị trễ.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Analytics/operations feature | `../features/05-analytics-and-operations.md` |
| Infrastructure | `06-infrastructure.md` |
| Event/outbox flow | `05-data-events-and-flows.md` |
| Operations model | `../model/04-events-analytics-and-operations.md` |
| Operations schema | `../database/04-analytics-and-operations.md` |
| Engineering practices | `09-engineering-practices.md` |

## Pattern taxonomy

| Symptom | Pattern ưu tiên | Dấu hiệu cần đo |
|---|---|---|
| Producer nhanh hơn consumer | Backpressure, queue-based load leveling, competing consumers | queue depth, lag age, processing throughput |
| External provider chậm/lỗi | Timeout budget, circuit breaker, retry with jitter, bulkhead | timeout rate, open circuit, provider error rate |
| Retry làm hệ thống tệ hơn | Multi-level retry, DLQ, poison message handling, fail fast | retry count, DLQ count, repeated failure key |
| Một tenant chiếm tài nguyên | Tenant fairness, token bucket, priority queue | per-tenant throughput/error/lag |
| Query/report quá nặng | CQRS projection, ClickHouse, compaction/retention | query latency, DB CPU, scan volume |
| Job chỉ được chạy một bản | Leader election, distributed singleton job | duplicate execution, lock contention |
| Workflow dài nhiều bước | Saga/process manager | stuck saga, compensation rate |

## Nguyên tắc áp dụng

- Đo symptom trước, rồi mới thêm pattern.
- Ưu tiên limit đơn giản, timeout rõ, queue bounded và idempotency trước khi thêm cơ chế phức tạp.
- Pattern nào tạo state mới thì cần owner, retention, metric và runbook.
- Với email delivery, tenant fairness và provider throttling thường quan trọng sớm hơn sharding.

Khi email delivery platform bắt đầu có nhiều tenant, campaign lớn, webhook spike và analytics volume cao, hệ thống phải được thiết kế để **chậm lại có kiểm soát** thay vì sập dây chuyền.

## Backpressure

Backpressure là cơ chế hãm tải khi downstream chậm.

Áp dụng khi:

```text
Kafka lag cao
Email provider trả 429/5xx
PostgreSQL latency tăng
Redis latency tăng
ClickHouse insert chậm
Webhook traffic spike
```

Kỹ thuật:

- Bounded queue, không dùng queue in-memory vô hạn.
- Worker concurrency limit theo topic/provider/tenant.
- Pause/resume Kafka consumer khi downstream không theo kịp.
- Dynamic rate limiter dựa trên queue age/provider error.
- Defer marketing email trước, giữ transactional email chạy.
- Drop hoặc delay non-critical analytics nếu cần, nhưng không làm mất business state.

Metric cần có:

```text
queue_age_p95
worker_inflight
worker_rejected_total
kafka_consumer_lag
provider_rate_limited_total
```

## Circuit breaker

Circuit breaker ngăn retry storm khi dependency lỗi liên tục.

Dùng cho:

```text
email provider send API
DNS verification
webhook forwarding
external enrichment API
ClickHouse insert nếu analytics không critical
```

State:

```text
closed     # gọi bình thường
open       # tạm ngắt, fail/defer nhanh
half-open  # thử một lượng nhỏ request để xem đã hồi phục chưa
```

Rule:

- Provider circuit open thì delivery worker chuyển sang defer hoặc route provider khác.
- Không retry ngay lập tức khi circuit open.
- Circuit breaker phải có metric và log rõ provider/tenant/domain.
- Transactional traffic có policy riêng, không bị marketing traffic làm open circuit chung nếu có thể tách.

## Bulkhead isolation

Bulkhead là tách tài nguyên để một phần hỏng không kéo toàn hệ thống xuống.

Tách theo:

```text
transactional vs marketing
tenant lớn vs tenant thường
provider
recipient domain
webhook ingestion vs delivery sending
analytics vs core delivery
```

Ví dụ:

- Worker pool riêng cho transactional email.
- Topic riêng cho `delivery.transactional`, `delivery.marketing`, `delivery.retry`.
- DB connection pool không để analytics job chiếm hết connection.
- ClickHouse consumer lỗi không được làm chậm suppression/delivery.
- Tenant quota riêng để một tenant không đốt hết provider capacity.

## Priority queue

Không phải email nào cũng quan trọng như nhau.

Priority gợi ý:

```text
P0 password reset, OTP, security alert
P1 transactional notification
P2 scheduled campaign
P3 marketing retry, analytics recompute
```

Kafka topic gợi ý:

```text
delivery.transactional
delivery.marketing
delivery.retry
analytics.email.events
```

Rule:

- P0/P1 có concurrency/quota bảo vệ riêng.
- Marketing campaign bị delay khi hệ thống quá tải.
- Retry queue không được chiếm hết worker của fresh delivery.
- Dashboard/reporting không được cạnh tranh tài nguyên với sending path.

## Adaptive throttling

Rate limit tĩnh là chưa đủ. Email platform cần giảm tốc theo tín hiệu reputation và provider.

Tín hiệu:

```text
provider 429/5xx rate
gmail/yahoo/outlook deferral rate
bounce rate
complaint rate
queue age
new domain warm-up day
provider quota remaining
```

Hành động:

- Giảm tốc theo recipient domain nếu deferral tăng.
- Pause sending domain nếu DNS authentication fail.
- Pause tenant/campaign nếu complaint rate vượt ngưỡng.
- Giảm marketing trước, giữ transactional.
- Tăng lại từ từ khi tín hiệu ổn định.

Throttle key:

```text
tenant_id
sending_domain
provider
recipient_domain
message_type
campaign_id
```

## Idempotency và dedup ở mọi boundary

High-load system sẽ luôn có retry, redelivery và duplicate webhook.

Boundary cần idempotency:

```text
API command
Kafka consumer
outbox publisher
provider webhook
tracking open/click endpoint
one-click unsubscribe endpoint
ClickHouse analytics consumer
```

Rule:

- API command dùng `Idempotency-Key` cho operation quan trọng.
- Provider webhook dedupe theo provider event id hoặc `(provider_message_id, event_type, occurred_at)`.
- Kafka consumer lưu processed event id nếu side effect không tự idempotent.
- ClickHouse consumer batch insert cần dedupe strategy hoặc tolerate duplicate bằng aggregate query/materialized view phù hợp.
- Unsubscribe luôn idempotent: gọi nhiều lần vẫn cùng kết quả.

## Partitioning và sharding

Chọn partition key sai sẽ làm nghẽn một điểm.

Kafka partition key:

```text
message_id          # ordering theo message
campaign_id         # campaign processing locality
tenant_id           # tenant isolation, nhưng tenant lớn có thể hot partition
recipient_domain    # throttle theo mailbox provider
```

PostgreSQL partition candidates:

```text
provider_webhook_events by received_at
message_events by occurred_at
messages by tenant/time nếu volume rất lớn
outbox by created_at/status nếu outbox lớn
```

ClickHouse partition/order:

```text
PARTITION BY toYYYYMM(event_date)
ORDER BY (tenant_id, campaign_id, event_type, occurred_at)
```

Rule:

- Đừng shard PostgreSQL sớm.
- Partition bảng event/log trước bảng core transactional.
- Tránh tenant lớn tạo hot partition trong Kafka.
- Dùng synthetic bucket nếu một key quá nóng.

## CQRS và projections

Không query dashboard trực tiếp từ write model lớn.

Vai trò:

```text
PostgreSQL   write model/source of truth
Redis        hot counters/cache/rate limit
ClickHouse   analytics/read model
Kafka        projection update stream
```

Projection ví dụ:

```text
campaign_delivery_summary
recipient_domain_hourly_stats
provider_error_rate
tenant_usage_daily
```

Rule:

- Projection có thể eventually consistent.
- Write path không phụ thuộc ClickHouse.
- Dashboard đọc aggregate/materialized view trước, raw event sau cùng.
- Có backfill job để rebuild projection khi logic đổi.

## Graceful degradation

Hệ thống nên giữ phần quan trọng chạy khi một phần phụ lỗi.

Ví dụ:

```text
ClickHouse down      -> vẫn gửi email, analytics replay sau
tracking down        -> vẫn delivery email
provider A down      -> route provider B hoặc defer
Redis cache down     -> degrade cache, giữ durable idempotency nếu critical
Grafana/Loki down    -> app vẫn chạy, local logs/metrics buffer nếu có
```

Rule:

- Phân loại dependency critical/non-critical.
- Non-critical dependency lỗi không được block core delivery.
- Critical dependency lỗi thì fail fast/defer, không treo worker.
- Có replay/backfill cho phần bị degrade.

## Replay và reprocessing

Replay là bắt buộc cho event-heavy platform.

Cần có:

```text
raw provider event storage
Kafka retention đủ dài
DLQ replay command
outbox republish command
webhook replay tool
ClickHouse backfill worker
projection rebuild job
```

Rule:

- Replay phải idempotent.
- Replay có rate limit riêng, không cạnh tranh với traffic live.
- Replay command cần dry-run mode.
- DLQ message phải có reason và first/last failure metadata.
- Không sửa data trực tiếp trong DB nếu có thể replay event đúng cách.

## Load shedding

Khi quá tải, từ chối hoặc delay traffic ít quan trọng.

Ví dụ:

- Reject campaign import lớn khi queue age quá cao.
- Delay marketing send khi provider quota thấp.
- Tắt expensive dashboard query.
- Limit webhook replay concurrency.
- Disable non-critical tracking enrichment.

Response nên rõ:

```text
HTTP 429 Too Many Requests
HTTP 503 Service Unavailable
Retry-After header nếu client có thể retry
```

## Multi-level retry

Không retry mù.

Policy:

```text
provider 429              -> retry after/backoff
provider 5xx              -> retry exponential backoff
network timeout           -> retry bounded
invalid recipient         -> no retry + suppression nếu hard bounce
complaint                 -> no retry + suppression
template render error     -> no retry until template/data fixed
DNS/auth domain fail      -> pause sender domain
ClickHouse insert fail    -> retry/replay analytics, không block delivery
```

Retry queue nên có:

```text
attempt_count
next_attempt_at
last_error_code
first_failed_at
max_attempts
```

## SLO và observability cho tải cao

Ngoài metric cơ bản, cần SLO.

SLO gợi ý:

```text
P95 API latency < 300ms cho non-heavy endpoints
P95 transactional enqueue latency < 1s
P95 time-to-provider-accepted < 60s
P99 webhook ingestion latency < 10s
Outbox oldest unpublished age < 2m
Analytics freshness < 5m
```

Metric high-load:

```text
queue_age_p95/p99 by priority
kafka_lag by topic/group
outbox_oldest_unpublished_age
provider_429_5xx_rate
bounce_rate by tenant/domain/provider
complaint_rate by tenant/domain/provider
clickhouse_insert_lag
webhook_ingestion_lag
tenant_quota_usage
```

Alert nên có runbook đi kèm. Alert không có runbook thường chỉ làm team mất ngủ sáng tạo.

## Tenant fairness và abuse control

Một tenant không được làm nghẽn toàn hệ thống.

Kỹ thuật:

- Per-tenant quota.
- Weighted fair queue.
- Tenant-level circuit breaker.
- Tenant-level suppression/complaint guardrail.
- Abuse detection: bounce/complaint spike, import list bẩn, spammy domain.
- Manual pause tenant/campaign/sending domain.

Rule:

- Tenant mới có quota thấp hơn và warm-up riêng.
- Tenant vượt complaint/bounce threshold bị pause marketing.
- Transactional critical email vẫn có policy riêng, nhưng không bỏ qua complaint/global suppression.

## Data retention và compaction

High load mà giữ mọi thứ mãi sẽ nổ storage.

Policy gợi ý:

```text
raw provider payload       7-30 ngày
provider webhook events    30-90 ngày trong PostgreSQL
message state              90-180 ngày hot
ClickHouse raw events      12-18 tháng
ClickHouse aggregates      24+ tháng
object storage archive     theo compliance/customer plan
```

Rule:

- Retention phải theo tenant plan/compliance nếu sản phẩm cần.
- TTL ClickHouse đặt theo `event_date`.
- PostgreSQL event table nên partition để drop partition nhanh.
- PII retention phải được review riêng.

## Capacity testing

Trước production, cần test theo scenario thật.

Scenario:

- Campaign lớn enqueue 1M recipients.
- Provider trả 429 liên tục 10 phút.
- Webhook duplicate/spike.
- ClickHouse down 30 phút rồi recover/backfill.
- Kafka consumer lag cao.
- Redis unavailable khi rate limit/idempotency đang dùng.
- PostgreSQL slow query hoặc connection pool gần cạn.

Kết quả test phải trả lời:

```text
hệ thống degrade thế nào
traffic nào bị delay/reject
có mất event không
replay/backfill mất bao lâu
alert nào bắn
runbook có đủ không
```

## Queue-based load leveling

Queue-based load leveling dùng queue/log làm buffer để làm phẳng spike tải.

Áp dụng cho:

```text
campaign import lớn
recipient expansion
delivery scheduling
provider webhook spike
analytics ingestion
ClickHouse batch insert
```

Flow:

```text
API/request producer
  -> durable queue/Kafka topic
  -> worker consumes at controlled rate
  -> downstream protected by throttle/backpressure
```

Rule:

- Producer không gọi downstream nặng trực tiếp nếu request có thể async.
- Queue depth/age là tín hiệu backpressure chính.
- Queue phải có DLQ/retry policy.
- Dùng priority queue/topic cho traffic quan trọng.
- Queue leveling không thay thế rate limit; hai pattern bổ sung nhau.

Email example:

```text
campaign created
  -> enqueue campaign expansion
  -> enqueue delivery jobs
  -> delivery workers send under provider/domain quota
```

## Competing consumers

Competing consumers là nhiều worker instance cùng consume một queue/topic để scale ngang.

Dùng cho:

```text
delivery workers
webhook normalization workers
analytics consumers
suppression import workers
outbox publishers theo shard
```

Rule:

- Consumer phải idempotent.
- Partition key phải tránh hot partition.
- Scale dựa trên lag/queue age, không chỉ CPU.
- Worker phải graceful shutdown và commit offset đúng lúc.
- Không scale consumer vô hạn nếu downstream provider/DB/ClickHouse không chịu nổi.

Metric:

```text
consumer_lag
consumer_processing_latency
consumer_error_rate
consumer_rebalance_count
downstream_latency
```

## Retry with exponential backoff and jitter

Retry phải có backoff và jitter để tránh retry storm.

Pattern:

```text
delay = min(max_delay, base_delay * 2^attempt)
delay = delay + random_jitter
```

Rule:

- Chỉ retry transient error.
- Non-transient error fail fast hoặc DLQ.
- Retry phải có max attempts hoặc retry budget.
- Retry operation phải idempotent.
- Tôn trọng `Retry-After` nếu provider trả về.
- Không retry ở nhiều layer cùng lúc nếu có thể tránh; chọn một owner retry chính.

Ví dụ:

```text
provider 429      -> retry after/backoff + jitter
provider 5xx      -> exponential backoff + jitter
invalid recipient -> no retry
complaint         -> no retry + suppression
```

## Timeout budget và deadline propagation

Không call downstream vô hạn. Mỗi request/job phải có timeout budget.

Ví dụ budget:

```text
API request total timeout          2s
DB query timeout                   300-800ms
Redis operation timeout            50-200ms
provider send API timeout          3-10s
webhook ingestion request timeout  1-3s
worker job timeout                 theo workload
```

Rule:

- Dùng `context.Context` để propagate deadline.
- Downstream timeout phải nhỏ hơn caller timeout.
- Timeout phải đo bằng latency thực tế, không chọn ngẫu nhiên.
- Khi timeout, classify error rõ: retryable hay non-retryable.
- Không giữ DB transaction trong lúc gọi external provider.

## Fail fast

Fail fast giúp giải phóng tài nguyên khi biết chắc request/job không thể thành công.

Fail fast khi:

```text
provider circuit open
sender domain chưa verified
tenant quota exhausted
recipient suppressed
template invalid
payload không parse được
queue full với low-priority work
```

Action:

- Return validation/conflict error cho API.
- Mark job suppressed/failed non-retryable.
- Defer job nếu dependency tạm lỗi.
- DLQ poison message nếu message không thể xử lý.

Fail fast không có nghĩa là mất dữ liệu; nó có nghĩa là dừng sớm với trạng thái rõ.

## Token bucket và leaky bucket

Rate limiting/throttling nên có thuật toán rõ.

Token bucket:

```text
bucket refill N tokens per interval
each send consumes token
burst allowed up to bucket capacity
```

Hợp cho:

```text
tenant quota
provider quota
recipient domain throttle
transactional burst nhỏ
```

Leaky bucket:

```text
requests drain at fixed rate
burst được làm mượt thành dòng ổn định
```

Hợp cho:

```text
provider/domain warm-up
marketing campaign pacing
webhook replay pacing
```

Rule:

- Redis giữ counter/token fast path.
- PostgreSQL giữ quota config bền.
- Throttle key phải include tenant/domain/provider/message type khi cần.
- Khi token thiếu, delay/defer thay vì busy-wait.

## Poison message handling

Poison message là message luôn fail vì data sai hoặc không tương thích.

Ví dụ:

```text
invalid JSON/schema
unknown event_type
template variable missing
recipient email invalid
provider permanent rejection
message references deleted campaign
```

Rule:

- Không retry vô hạn poison message.
- Sau max attempts hoặc non-retryable classification, đưa vào DLQ.
- DLQ record phải có reason, error class, first/last failure time, attempts, payload reference.
- Có tooling inspect/replay/skip DLQ.
- Replay DLQ phải có dry-run và rate limit.

DLQ metadata:

```text
original_topic
consumer_group
event_id
error_class
error_message
attempt_count
first_failed_at
last_failed_at
payload_ref
```

## Leader election và distributed singleton jobs

Một số job chỉ nên có một owner tại một thời điểm.

Dùng cho:

```text
scheduled cleanup
domain verification scheduler
outbox shard coordinator
retention/TTL coordinator
provider quota refresh
campaign scheduler tick
```

Implementation options:

```text
PostgreSQL advisory lock
Redis lock with TTL + owner token
Kubernetes Lease nếu chạy trên Kubernetes
```

Rule:

- Lock phải có TTL/lease.
- Lock phải có owner token để tránh unlock nhầm.
- Job phải idempotent vì leader có thể chết giữa chừng.
- Có heartbeat/renewal metric.
- Nếu có thể shard job thay vì singleton toàn cục, ưu tiên shard.

## Hedged requests

Hedged request gửi request phụ khi request đầu chậm bất thường để giảm tail latency.

Chỉ dùng cho:

```text
read-only idempotent query
DNS lookup qua nhiều resolver
metadata read từ replicated store
```

Không dùng cho:

```text
send email
charge payment
write DB
provider mutation
```

Rule:

- Chỉ hedge sau p95/p99 threshold, không gửi song song ngay từ đầu.
- Có concurrency budget riêng.
- Cancel request chậm khi request nhanh trả về.
- Theo dõi amplification factor để tránh nhân tải khi hệ thống đã quá tải.

## Saga / Process manager

Saga/process manager phù hợp cho workflow nhiều bước, nhiều module, có trạng thái trung gian.

Dùng cho:

```text
sender domain verification -> warm-up -> enable sending
campaign create -> audience expansion -> delivery scheduling -> analytics aggregation
tenant onboarding -> domain setup -> provider config -> first send
provider migration -> dual send disabled -> route switch -> cleanup
```

Rule:

- Mỗi step idempotent.
- State machine rõ trạng thái và transition.
- Timeout/compensation cho step treo.
- Không rollback side effect kiểu gửi email; dùng compensating action như pause, suppress, notify.
- Process manager nằm ở app layer/module owner, không nằm trong platform.

Ví dụ state:

```text
pending_domain_dns
verifying_dns
authenticated
warming_up
enabled
paused
failed
```

## High-load pattern references

Nguồn tham khảo chính:

- Google SRE - Addressing Cascading Failures: https://sre.google/sre-book/addressing-cascading-failures/
- Google SRE - Handling Overload: https://sre.google/sre-book/handling-overload/
- AWS Retry with Backoff: https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/retry-backoff.html
- AWS Circuit Breaker: https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/circuit-breaker.html
- Azure Queue-Based Load Leveling: https://learn.microsoft.com/en-us/azure/architecture/patterns/queue-based-load-leveling
- Azure Competing Consumers: https://learn.microsoft.com/en-us/azure/architecture/patterns/competing-consumers
- Martin Fowler Circuit Breaker: https://martinfowler.com/bliki/CircuitBreaker.html

---
