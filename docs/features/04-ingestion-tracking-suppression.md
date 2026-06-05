# Ingestion Tracking And Suppression Features

## Mục tiêu nhóm feature

Nhóm này xử lý toàn bộ tín hiệu đến sau khi message rời hệ thống: provider webhook, hành vi recipient và các quyết định chặn gửi lại để bảo vệ deliverability và compliance.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| API contract | `../api/04-events-webhooks-and-operations.md` |
| Business model | `../model/03-messaging-and-delivery.md` |
| Database schema | `../database/03-delivery-and-events.md` |
| Module owner | `../modules/03-module-catalog.md` |
| Email delivery architecture | `../architecture/07-email-delivery.md` |
| Event/outbox flow | `../architecture/05-data-events-and-flows.md` |

## Feature chính: Provider webhook ingestion

### Sub-feature / capability chi tiết

- Nhận webhook từ provider.
- Verify signature của provider.
- Persist raw provider payload.
- Normalize event external thành internal event có schema ổn định.
- Xử lý request nhanh và retry-safe.
- Không làm business side effect nặng trực tiếp trong HTTP request.

### Actor sử dụng

- Email provider
- API runtime
- Worker/consumer phía sau

### Input / output hoặc hành vi chính

- Nhận event từ provider như accepted, delivered, delayed, bounced, complained, opened, clicked, unsubscribed, rendering_failed.
- Trả acknowledgement nhanh và chuyển event sang pipeline nội bộ.

### Phụ thuộc quan trọng

- Provider adapter
- Event normalization
- Raw event storage
- Outbox/Kafka

### Event / side effect nổi bật

- `provider webhook normalized`
- Publish normalized event sang consumer liên quan

### Ghi chú phạm vi

Đây là capability nền rất quan trọng của email platform, không chỉ là một endpoint kỹ thuật.

## Feature chính: Normalization và deduplication

### Sub-feature / capability chi tiết

- Ánh xạ event provider-specific sang taxonomy nội bộ.
- Dedupe theo provider event id hoặc tổ hợp message id/event type/thời điểm.
- Gắn `provider_message_id`, `message_id`, `workspace_id`, `event_type`, `occurred_at`, `received_at`.
- Đảm bảo webhook retry không nhân đôi side effect.
- Làm nền cho analytics, logs, suppression và tracking.

### Actor sử dụng

- Webhook processor
- Delivery/suppression/tracking/analytics consumer

### Input / output hoặc hành vi chính

- Nhận raw provider event.
- Trả normalized event idempotent cho hệ thống nội bộ.

### Phụ thuộc quan trọng

- Raw event persistence
- Idempotency store
- Event contracts

### Event / side effect nổi bật

- Dedupe marker persisted
- Normalized event fan-out

### Ghi chú phạm vi

Capability này không user-facing, nhưng là điều kiện để mọi màn hình hậu gửi hiển thị đúng.

## Feature chính: Bounce và complaint handling

### Sub-feature / capability chi tiết

- Nhận tín hiệu bounce, complaint, reject, delivery delayed.
- Phân loại outcome theo delivery domain.
- Chặn retry sai đối tượng khi là hard bounce hoặc complaint.
- Cập nhật message state và deliverability projection.
- Làm đầu vào cho suppression decision.

### Actor sử dụng

- Provider
- Delivery worker
- Operator

### Input / output hoặc hành vi chính

- Nhận normalized failure signal.
- Trả delivery classification và tác động lên message/suppression state.

### Phụ thuộc quan trọng

- Webhook normalization
- Delivery state machine
- Suppression service

### Event / side effect nổi bật

- `delivery.message.bounced.v1`
- `delivery.message.complained.v1`
- `suppression.recipient.suppressed.v1`

### Ghi chú phạm vi

Đây là feature vận hành lõi, đồng thời ảnh hưởng trực tiếp đến compliance và reputation.

## Feature chính: Suppression service

### Sub-feature / capability chi tiết

- Global suppression.
- Tenant-level suppression.
- List/audience-level unsubscribe.
- Campaign/category-level unsubscribe.
- Mirror suppression từ provider nếu có.
- Suppression reasons: unsubscribe, hard_bounce, complaint, manual_block, invalid_recipient, policy_violation.
- Viết suppression idempotent.
- Audit suppression decision.
- Lookup suppression theo email normalized và/hoặc email hash.

### Actor sử dụng

- Recipient
- Delivery worker
- Workspace operator
- Operator

### Input / output hoặc hành vi chính

- Nhận tín hiệu unsubscribe hoặc failure signal.
- Trả quyết định suppressed/not suppressed theo scope và reason.

### Phụ thuộc quan trọng

- Webhook signal
- Audience eligibility
- Delivery pipeline
- Audit logs

### Event / side effect nổi bật

- `recipient suppressed`
- `recipient unsuppressed`

### Ghi chú phạm vi

Suppression vừa là feature user-facing ở Audience, vừa là guardrail nền cho toàn bộ sending path.

## Feature chính: Unsubscribe handling

### Sub-feature / capability chi tiết

- One-click unsubscribe cho marketing/subscription email.
- Idempotent unsubscribe endpoint.
- Ghi suppression theo đúng scope.
- Cập nhật analytics/logs sau unsubscribe.

### Actor sử dụng

- Recipient
- Workspace operator khi review kết quả

### Input / output hoặc hành vi chính

- Nhận hành động unsubscribe từ email link hoặc provider signal.
- Trả kết quả thành công ngay cả khi request lặp lại.

### Phụ thuộc quan trọng

- Tracking links
- Suppression service
- Compliance policy

### Event / side effect nổi bật

- `unsubscribed`
- `suppression.recipient.suppressed.v1`

### Ghi chú phạm vi

Đây là feature vừa user-facing đối với recipient, vừa là compliance control bắt buộc của platform.

## Feature chính: Open và click tracking

### Sub-feature / capability chi tiết

- Theo dõi email opened.
- Theo dõi link clicked.
- Tracking redirect state cho click flow.
- Gắn tracking event với message delivered/sent snapshot.
- Có privacy và abuse control cho tracking endpoint.
- Dedupe hoặc tolerate duplicate signal tùy loại sự kiện.

### Actor sử dụng

- Recipient
- Dashboard user xem analytics
- Operator

### Input / output hoặc hành vi chính

- Nhận request open pixel hoặc click redirect.
- Ghi tracking event và redirect người dùng tới đích phù hợp khi click.

### Phụ thuộc quan trọng

- Message/log context
- Analytics consumer
- Abuse protection

### Event / side effect nổi bật

- `email opened`
- `link clicked`

### Ghi chú phạm vi

Feature này liên kết trực tiếp giữa recipient behavior và analytics/reporting phía workspace.
