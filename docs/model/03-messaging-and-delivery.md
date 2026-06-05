# Messaging And Delivery Model

## Mục tiêu nhóm model

Nhóm model này quản lý ý định gửi, thực thi gửi, trạng thái delivery và các guardrail ngăn gửi sai.

## Module ownership

| Model | Module owner chính |
|---|---|
| `Campaign`, `CampaignMessageCandidate` | `campaign` |
| `TransactionalSendRequest`, `Message`, `DeliveryAttempt`, `RetryState` | `delivery` |
| `SuppressionEntry` | `suppression` |
| `ProviderWebhookEvent`, `NormalizedProviderEvent` | `ingestion` |
| `TrackingLink`, `TrackingEvent` | `tracking` |
| `CustomerWebhookDelivery` | `webhooks` |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/03-delivery-and-messaging.md`, `../features/04-ingestion-tracking-suppression.md` |
| API contract | `../api/03-messaging-and-delivery.md`, `../api/04-events-webhooks-and-operations.md` |
| Database schema | `../database/03-delivery-and-events.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| Event/outbox flow | `../architecture/05-data-events-and-flows.md` |

## Aggregate / entity chính

### `Campaign`

- Vai trò: kế hoạch gửi email cho một audience.
- Thuộc tính chính: `id`, `workspace_id`, `name`, audience reference, template reference, sender reference, `status`, `scheduled_at`, timestamps.
- Invariant chính:
  - Campaign không được schedule/send khi sender chưa verified hoặc audience/content chưa sẵn sàng theo rule.

### `CampaignMessageCandidate`

- Vai trò: bản ghi trung gian đại diện recipient đã được chọn cho campaign.
- Thuộc tính chính: `campaign_id`, `contact_id` hoặc recipient identity, planning metadata.

### `TransactionalSendRequest`

- Vai trò: ý định gửi transactional email từ API hoặc internal integration.
- Thuộc tính chính: `id`, `workspace_id`, `idempotency_key`, request payload reference, `status`, timestamps.

### `Message`

- Vai trò: đơn vị delivery lõi.
- Thuộc tính chính: `id`, `workspace_id`, `campaign_id` hoặc `transactional_request_id`, recipient, sender, provider, `status`, timestamps.
- Invariant chính:
  - Message state transition phải đi theo lifecycle cho phép.

### `DeliveryAttempt`

- Vai trò: một lần gửi message qua provider.
- Thuộc tính chính: `id`, `message_id`, `provider`, attempt number, request metadata, response metadata, timestamps.

### `RetryState`

- Vai trò: kế hoạch retry cho message hoặc downstream delivery.
- Thuộc tính chính: retry count, next attempt at, last error class, policy metadata.

### `SuppressionEntry`

- Vai trò: quyết định chặn gửi cho recipient.
- Thuộc tính chính: `id`, `workspace_id` hoặc global scope, `email_normalized`, `email_hash`, `scope`, `reason`, `source`, timestamps.
- Invariant chính:
  - Complaint/hard bounce phải chặn các send path liên quan theo scope phù hợp.

### `ProviderWebhookEvent`

- Vai trò: raw event nhận từ provider.
- Thuộc tính chính: `id`, `provider`, payload, `provider_event_id`, `provider_message_id`, `received_at`, verification metadata.

### `NormalizedProviderEvent`

- Vai trò: event nội bộ đã chuẩn hóa.
- Thuộc tính chính: `event_id`, `workspace_id`, `message_id`, `provider_message_id`, `event_type`, `occurred_at`, payload.

### `TrackingLink`

- Vai trò: mapping click tracking sang URL đích và message context.
- Thuộc tính chính: `id`, `message_id`, destination URL, metadata.

### `TrackingEvent`

- Vai trò: ghi nhận open/click.
- Thuộc tính chính: `id`, `workspace_id`, `message_id`, `event_type`, `occurred_at`, metadata.

### `CustomerWebhookDelivery`

- Vai trò: một lần đẩy event ra webhook downstream.
- Thuộc tính chính: `id`, `workspace_id`, `webhook_config_id`, `event_type`, `status`, retry metadata, timestamps.

## Value object chính

- `CampaignID`
- `MessageID`
- `ProviderMessageID`
- `SuppressionScope`
- `SuppressionReason`
- `MessageType`
- `DeliveryStatus`
- `EventType`

## Event model chính

- `campaign.scheduled.v1`
- `transactional message queued`
- `delivery.message.queued.v1`
- `delivery.message.delivered.v1`
- `delivery.message.bounced.v1`
- `delivery.message.complained.v1`
- `suppression.recipient.suppressed.v1`
- `suppression.recipient.unsuppressed.v1`
- `email opened`
- `link clicked`
- `customer webhook delivery failed/succeeded`

## Read model / projection quan trọng

- Campaign detail/status view
- Message logs view
- Queue/retry view
- Bounces/complaints view
- Deliverability health view
- Webhook delivery history view
