# Events Analytics And Operations Model

## Mục tiêu nhóm model

Nhóm model này mô tả các model dùng cho event backbone, projection, analytics và operations.

## Module ownership

| Model | Module owner chính |
|---|---|
| `IntegrationEventEnvelope` | `operations`/shared event contract, producer module sở hữu payload cụ thể |
| `OutboxRecord`, `ProcessedEventMarker`, `DeadLetterRecord`, `ReplayJob`, `RateLimitState`, `CircuitBreakerState` | `operations` |
| `EmailEventFact`, `CampaignDeliverySummary`, `RecipientDomainHourlyStats`, `ProviderDeliveryStats`, `DashboardOverview` | `analytics` |
| `AuditEntry` | `audit` |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/05-analytics-and-operations.md` |
| API contract | `../api/04-events-webhooks-and-operations.md` |
| Database schema | `../database/04-analytics-and-operations.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| Event/outbox flow | `../architecture/05-data-events-and-flows.md` |
| Scale/resilience | `../architecture/08-scale-and-resilience.md` |

## Integration event envelope

### `IntegrationEventEnvelope`

- Vai trò: contract chung giữa producer và consumer nội bộ.
- Thuộc tính chính:
  - `event_id`
  - `event_type`
  - `aggregate_id`
  - `workspace_id`
  - `occurred_at`
  - `payload`
- Invariant chính:
  - `event_type` phải versioned.
  - Consumer phải tolerate redelivery và field mở rộng.

## Operational model chính

### `OutboxRecord`

- Vai trò: nối strong consistency của DB với eventual publish.
- Thuộc tính chính: `id`, aggregate metadata, topic, payload, `published_at`, status, retry metadata.

### `ProcessedEventMarker`

- Vai trò: dấu idempotency cho consumer.
- Thuộc tính chính: consumer name, `event_id`, processed timestamp, optional TTL.

### `DeadLetterRecord`

- Vai trò: biểu diễn message hoặc event bị đẩy vào DLQ.
- Thuộc tính chính: source topic, event payload, failure reason, retry count, moved at.

### `ReplayJob`

- Vai trò: yêu cầu replay/backfill có kiểm soát.
- Thuộc tính chính: target stream/table, filter window, status, operator metadata, timestamps.

### `RateLimitState`

- Vai trò: trạng thái throttling/quota tại runtime.
- Thuộc tính chính: scope key, quota window, current usage, last updated.

### `CircuitBreakerState`

- Vai trò: trạng thái dependency protection.
- Thuộc tính chính: dependency key, state (`closed/open/half-open`), error counters, timestamps.

## Analytics model chính

### `EmailEventFact`

- Vai trò: fact event cho analytics volume lớn.
- Thuộc tính chính: event time, workspace, campaign, message, provider, event type, recipient domain, metadata.

### `CampaignDeliverySummary`

- Vai trò: projection tổng hợp theo campaign.
- Thuộc tính chính: campaign id, counts theo status/event, last updated.

### `RecipientDomainHourlyStats`

- Vai trò: theo dõi health theo mailbox/recipient domain.
- Thuộc tính chính: workspace/domain/provider/time bucket metrics.

### `ProviderDeliveryStats`

- Vai trò: đo hiệu năng và lỗi theo provider.
- Thuộc tính chính: provider, workspace or global scope, rate/error counters, time bucket.

### `DashboardOverview`

- Vai trò: read model tổng quan cho UI analytics/operations.
- Thuộc tính chính: counters, trends, top issues, freshness metadata.

### `AuditEntry`

- Vai trò: record hành động nhạy cảm để review và điều tra.
- Thuộc tính chính: actor, workspace, action type, target, payload summary, occurred at.

## Read model / projection quan trọng

- Email logs projection
- Queue lag / retry projection
- Deliverability projection
- Audit log projection
- Health/readiness status view
- Feature/config snapshot view
