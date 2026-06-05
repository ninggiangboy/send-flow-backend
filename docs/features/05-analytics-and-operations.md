# Analytics And Operations Features

## Mục tiêu nhóm feature

Nhóm này biến các signal runtime và email event thành khả năng quan sát, vận hành và điều khiển hệ thống ở mức sản phẩm. Đây là phần giúp `send-flow` không chỉ gửi được email mà còn vận hành được khi tải cao và sự cố xảy ra.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| API contract | `../api/04-events-webhooks-and-operations.md` |
| Business model | `../model/04-events-analytics-and-operations.md` |
| Database schema | `../database/04-analytics-and-operations.md` |
| Module owner | `../modules/03-module-catalog.md` |
| Infrastructure | `../architecture/06-infrastructure.md` |
| Scale/resilience | `../architecture/08-scale-and-resilience.md` |

## Feature chính: Dashboard và reporting read models

### Sub-feature / capability chi tiết

- Dashboard tổng quan cho delivery health và campaign performance.
- Read model/projection cho email logs, queue/retry, bounces/complaints, deliverability.
- Aggregate metrics theo thời gian, tenant, campaign, provider hoặc recipient domain.
- Hiển thị trạng thái `pending`, `processing`, `last_updated_at` khi projection còn eventual consistency.

### Actor sử dụng

- Dashboard user
- Operator

### Input / output hoặc hành vi chính

- Nhận event từ delivery/tracking/suppression.
- Trả dashboard data và operational screen data phù hợp cho UI/API.

### Phụ thuộc quan trọng

- Kafka events
- PostgreSQL projections
- ClickHouse analytics store

### Event / side effect nổi bật

- Projection update

### Ghi chú phạm vi

Đây là phần user-facing chính của analytics plane.

## Feature chính: Email analytics

### Sub-feature / capability chi tiết

- Lưu email events volume lớn.
- Aggregate delivery, open, click, bounce, complaint, unsubscribe.
- Funnel queued -> accepted -> delivered -> opened -> clicked -> unsubscribed.
- Báo cáo theo campaign, provider, sending domain, recipient domain, tenant.
- Materialized view hoặc aggregate read model cho dashboard thường xuyên xem.

### Actor sử dụng

- Marketer
- Workspace admin
- Deliverability operator

### Input / output hoặc hành vi chính

- Nhận normalized event hoặc delivery event.
- Trả thống kê và xu hướng theo chiều phân tích phù hợp.

### Phụ thuộc quan trọng

- Webhook normalization
- Tracking events
- ClickHouse hoặc analytics store

### Event / side effect nổi bật

- Analytics consumer insert/batch update

### Ghi chú phạm vi

Đây là feature user-facing ở màn hình Analytics, nhưng phụ thuộc mạnh vào event pipeline nền.

## Feature chính: Queue, retry, DLQ và replay operations

### Sub-feature / capability chi tiết

- Quan sát queue depth, queue age, consumer lag.
- Tách retry queue khỏi fresh queue.
- Dead letter queue cho failure lặp lại hoặc poison message.
- Replay event hoặc backfill khi cần phục hồi dữ liệu.
- Xử lý outbox stuck, webhook replay, analytics backfill.
- Cung cấp khả năng support/operator xử lý sự cố mà không sửa tay business state.

### Actor sử dụng

- Operator / developer
- On-call engineer

### Input / output hoặc hành vi chính

- Nhận failure signal, lag signal hoặc yêu cầu replay.
- Trả trạng thái queue/DLQ và kích hoạt luồng replay/backfill phù hợp.

### Phụ thuộc quan trọng

- Kafka / outbox
- Worker runtime
- Observability stack

### Event / side effect nổi bật

- DLQ message created
- Replay/backfill job executed

### Ghi chú phạm vi

Đây là capability operations cốt lõi, được phản ánh ra màn hình Queue / Retry nhưng chủ yếu phục vụ vận hành.

## Feature chính: Health, readiness và graceful degradation

### Sub-feature / capability chi tiết

- Health endpoint.
- Readiness endpoint.
- Graceful shutdown cho API và worker.
- Backpressure khi downstream chậm.
- Pause/resume consumer khi cần.
- Defer marketing trước để giữ transactional traffic.
- Timeout budget và deadline propagation cho downstream call.

### Actor sử dụng

- Operator
- Platform/deployment system

### Input / output hoặc hành vi chính

- Nhận tín hiệu trạng thái của dependency và workload.
- Trả readiness thực tế của runtime hoặc chủ động giảm tải có kiểm soát.

### Phụ thuộc quan trọng

- Runtime lifecycle management
- DB/Redis/Kafka/provider health checks
- Queue metrics

### Event / side effect nổi bật

- Health status reported
- Consumer pause/resume

### Ghi chú phạm vi

Đây là capability nền để sản phẩm chịu tải và deploy an toàn.

## Feature chính: Observability

### Sub-feature / capability chi tiết

- Structured logs với request id, trace id, workspace id.
- Metrics cho HTTP, DB, Redis, Kafka, outbox, queue age, provider error, webhook ingestion failure.
- Tracing cho API, worker và downstream call.
- Dashboard và alert rule cơ bản cho incident phổ biến.
- Theo dõi bounce rate, complaint rate, lag, provider outage và delivery health.

### Actor sử dụng

- Operator / developer
- On-call engineer

### Input / output hoặc hành vi chính

- Thu thập signal từ runtime và business flow.
- Trả khả năng quan sát để debug, alert và điều tra.

### Phụ thuộc quan trọng

- Observability stack
- Event and runtime instrumentation

### Event / side effect nổi bật

- Logs, metrics, traces emitted
- Alerts fired

### Ghi chú phạm vi

Không phải một feature end-user truyền thống, nhưng là một phần chính thức của sản phẩm vận hành.

## Feature chính: Feature flags và operational controls

### Sub-feature / capability chi tiết

- Bật/tắt capability theo environment hoặc tenant.
- Rollout an toàn cho flow mới.
- Cô lập feature rủi ro khi cần mitigation nhanh.
- Điều chỉnh behavior runtime mà không đổi business ownership.

### Actor sử dụng

- Operator / developer
- Workspace admin trong một số setting được expose

### Input / output hoặc hành vi chính

- Nhận config/flag update.
- Ảnh hưởng đến quyết định runtime hoặc availability của một số capability.

### Phụ thuộc quan trọng

- Config and secrets
- Settings model
- Audit trail

### Event / side effect nổi bật

- Feature/config snapshot refreshed

### Ghi chú phạm vi

Đây là capability operations và release control, không đồng nghĩa với roadmap product.

## Feature chính: Scale và resilience controls

### Sub-feature / capability chi tiết

- Circuit breaker cho provider hoặc dependency lỗi liên tục.
- Bulkhead isolation giữa transactional, marketing, analytics, webhook ingestion.
- Priority queue theo mức độ quan trọng của email.
- Competing consumers và queue-based load leveling.
- Leader election hoặc distributed singleton cho coordinator job.
- Partitioning/retention cho event/log volume lớn.
- Tenant abuse control và fairness.

### Actor sử dụng

- Operator / platform owner
- Delivery worker

### Input / output hoặc hành vi chính

- Nhận symptom từ runtime như lag, timeout, quota depletion, retry storm.
- Áp dụng pattern chịu tải phù hợp để hệ thống chậm lại có kiểm soát thay vì sập dây chuyền.

### Phụ thuộc quan trọng

- Queue metrics
- Provider health
- Redis/Kafka/PostgreSQL/ClickHouse operational signals

### Event / side effect nổi bật

- Circuit opened/closed
- Priority or concurrency policy changed
- Abuse/throttle decision recorded

### Ghi chú phạm vi

Đây là nhóm capability nền cho production readiness, không phải UI feature độc lập nhưng là một phần không thể thiếu của sản phẩm theo architecture hiện tại.
