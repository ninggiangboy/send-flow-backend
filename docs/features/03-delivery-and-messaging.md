# Delivery And Messaging Features

## Mục tiêu nhóm feature

Nhóm này điều phối toàn bộ hành vi gửi email của sản phẩm, từ lúc tạo intent gửi đến lúc message đi vào provider, được retry, bị throttle hoặc được phản ánh vào log trạng thái.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| API contract | `../api/03-messaging-and-delivery.md` |
| Business model | `../model/03-messaging-and-delivery.md` |
| Database schema | `../database/03-delivery-and-events.md` |
| Module owner | `../modules/03-module-catalog.md` |
| Email delivery architecture | `../architecture/07-email-delivery.md` |
| Event/outbox flow | `../architecture/05-data-events-and-flows.md` |

## Feature chính: Campaign orchestration

### Sub-feature / capability chi tiết

- Tạo campaign theo workspace.
- Chọn audience cho campaign.
- Gắn template và sender phù hợp.
- Lập kế hoạch gửi và schedule thời điểm chạy.
- Sinh message candidate từ audience selection.
- Kiểm tra sender verified và audience eligibility trước khi gửi.
- Publish sự kiện campaign để delivery pipeline tiếp nhận.

### Actor sử dụng

- Marketer
- Workspace operator

### Input / output hoặc hành vi chính

- Nhận cấu hình campaign, audience, template, schedule.
- Trả campaign state và kế hoạch gửi tương ứng.

### Phụ thuộc quan trọng

- Audience and content
- Sender/domain verification
- Delivery pipeline

### Event / side effect nổi bật

- `campaign.scheduled.v1`
- `campaign message queued`

### Ghi chú phạm vi

Đây là feature user-facing chính của nhóm Deliver.

## Feature chính: Transactional email

### Sub-feature / capability chi tiết

- Nhận request gửi transactional email qua public API hoặc internal flow.
- Hỗ trợ template-based send hoặc raw send.
- Dùng API idempotency cho request dễ bị retry.
- Tôn trọng complaint/hard bounce/global block.
- Queue message thay vì gửi trực tiếp trong HTTP path.
- Trả kết quả chấp nhận request nhanh cho caller.

### Actor sử dụng

- Public API client
- Internal product integration
- Developer của tenant

### Input / output hoặc hành vi chính

- Nhận API request, template/data, recipient và sender context.
- Trả acceptance/result ở mức request, còn outcome delivery theo flow async.

### Phụ thuộc quan trọng

- API keys
- Template/rendering
- Suppression and delivery rules
- Queue/outbox

### Event / side effect nổi bật

- `transactional message queued`
- Audit request credential usage

### Ghi chú phạm vi

Feature này là product-facing API capability, khác campaign ở chỗ ưu tiên độ trễ và idempotency request.

## Feature chính: Queueing và asynchronous delivery pipeline

### Sub-feature / capability chi tiết

- Tạo message record trước khi gửi.
- Enqueue delivery jobs.
- Tách HTTP command path khỏi send execution path.
- Có outbox để ghi state và phát event nhất quán.
- Có worker xử lý message đến hạn.
- Có queue riêng hoặc policy riêng cho transactional, marketing và retry traffic.

### Actor sử dụng

- API runtime
- Worker runtime
- Operator

### Input / output hoặc hành vi chính

- Nhận message intent đã hợp lệ.
- Chuyển message qua các bước queued, processing, accepted, delivered hoặc failed state.

### Phụ thuộc quan trọng

- Transactional outbox
- Kafka/event backbone
- Worker runtime

### Event / side effect nổi bật

- `delivery.message.queued.v1`
- Publish integration event cho delivery, analytics, webhook downstream

### Ghi chú phạm vi

Đây là capability nền bắt buộc của messaging platform, không chỉ là chi tiết hạ tầng.

## Feature chính: Provider routing và send execution

### Sub-feature / capability chi tiết

- Chọn provider cho từng message.
- Tích hợp provider đầu tiên như AWS SES.
- Thiết kế adapter-ready cho Mailgun, Postmark, SendGrid.
- Gọi send API của provider.
- Lưu `provider_message_id`.
- Tách provider adapter khỏi business routing decision.

### Actor sử dụng

- Delivery worker
- Operator / deliverability owner

### Input / output hoặc hành vi chính

- Nhận message đã render và hợp lệ.
- Chọn provider, gửi đi, trả metadata accepted/send result.

### Phụ thuộc quan trọng

- Sender/domain readiness
- Throttling policy
- Provider adapter

### Event / side effect nổi bật

- `message accepted`
- `provider disabled`

### Ghi chú phạm vi

Feature này không trực tiếp là màn hình UI, nhưng là năng lực gửi cốt lõi của sản phẩm.

## Feature chính: Throttling, quota và tenant fairness

### Sub-feature / capability chi tiết

- Throttle theo workspace, provider, sending domain, recipient domain, campaign hoặc message type.
- Bảo vệ transactional traffic trước marketing traffic.
- Token bucket hoặc rate limiter cho quota/warm-up.
- Adaptive throttling khi provider 429/5xx, deferral rate, complaint rate hoặc reputation signal xấu.
- Tenant fairness để một tenant không chiếm toàn bộ tài nguyên.
- Pause hoặc giảm tốc campaign khi tín hiệu vận hành xấu.

### Actor sử dụng

- Delivery worker
- Operator

### Input / output hoặc hành vi chính

- Nhận queue item cùng load signal và reputation signal.
- Trả quyết định gửi ngay, defer, giảm tốc hoặc pause.

### Phụ thuộc quan trọng

- Redis / rate limiter
- Provider feedback
- Queue lag metrics

### Event / side effect nổi bật

- `provider rate limited`
- `campaign paused/deferred`
- Deliverability health projection update

### Ghi chú phạm vi

Đây là capability nền nhưng là một phần trực tiếp của hành vi delivery platform.

## Feature chính: Retry, failure handling và state machine

### Sub-feature / capability chi tiết

- Retry khi gặp temporary failure.
- Exponential backoff với jitter.
- Phân loại permanent vs temporary failure.
- Tách retry queue khỏi fresh delivery.
- Chống retry storm bằng fail-fast, circuit breaker hoặc retry budget.
- Quản lý delivery state transition: queued, accepted, delivered, bounced, complained, rejected, delayed.

### Actor sử dụng

- Delivery worker
- Operator

### Input / output hoặc hành vi chính

- Nhận lỗi từ provider hoặc downstream.
- Quyết định retry, DLQ, classify result hoặc kết thúc state machine.

### Phụ thuộc quan trọng

- Retry policy
- Provider/webhook signal
- Queue and DLQ operations

### Event / side effect nổi bật

- `delivery.message.bounced.v1`
- `delivery.message.delivered.v1`
- Retry schedule update

### Ghi chú phạm vi

Capability này là nền tảng để message logs và analytics phản ánh trạng thái đúng.

## Feature chính: Message logs và delivery state visibility

### Sub-feature / capability chi tiết

- Lưu và hiển thị trạng thái message theo thời gian.
- Gắn log với campaign, transactional request, sender, provider và recipient.
- Phản ánh accepted, delivered, delayed, bounced, complained, clicked, unsubscribed nếu có.
- Hỗ trợ màn hình Email Logs và một phần queue/retry visibility.

### Actor sử dụng

- Dashboard user
- Support/operator

### Input / output hoặc hành vi chính

- Nhận state update từ delivery, webhook, tracking và suppression flow.
- Trả read model/log projection cho UI hoặc API.

### Phụ thuộc quan trọng

- Delivery state machine
- Webhook normalization
- Tracking/suppression events

### Event / side effect nổi bật

- Projection update cho email logs

### Ghi chú phạm vi

Đây là feature user-facing rõ ràng nhưng phụ thuộc hoàn toàn vào các flow async nền phía dưới.
