# Delivery And Events Database

## Mục tiêu nhóm schema

Nhóm schema này lưu workflow giao dịch cho send pipeline, webhook, suppression, tracking và event propagation.

## Module ownership

| Table | Module owner chính |
|---|---|
| `campaigns`, `campaign_message_candidates` | `campaign` |
| `transactional_send_requests`, `messages`, `delivery_attempts`, `retry_states` | `delivery` |
| `suppression_entries` | `suppression` |
| `provider_webhook_events`, `normalized_provider_events` | `ingestion` |
| `tracking_links`, `tracking_events` | `tracking` |
| `customer_webhook_deliveries` | `webhooks` |
| `outbox_events` | `operations`, với payload/event type thuộc producer module |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/03-delivery-and-messaging.md`, `../features/04-ingestion-tracking-suppression.md` |
| API contract | `../api/03-messaging-and-delivery.md`, `../api/04-events-webhooks-and-operations.md` |
| Business model | `../model/03-messaging-and-delivery.md`, `../model/04-events-analytics-and-operations.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| Event/outbox flow | `../architecture/05-data-events-and-flows.md` |
| Infrastructure | `../architecture/06-infrastructure.md` |

## PostgreSQL tables

### `campaigns`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | campaign id |
| `workspace_id` | workspace owner |
| audience/template/sender refs | input gửi |
| `status` | draft/scheduled/running/paused/completed |
| `scheduled_at` | thời điểm gửi |
| timestamps | lifecycle metadata |

### `campaign_message_candidates`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | candidate id |
| `workspace_id` | workspace owner |
| `campaign_id` | campaign owner |
| `contact_id` hoặc recipient fields | recipient đã chọn |
| planning metadata | thông tin lập kế hoạch |

### `transactional_send_requests`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | request id |
| `workspace_id` | workspace owner |
| `idempotency_key` | key chống duplicate |
| request payload columns | dữ liệu gửi |
| `status` | accepted/processing/completed/failed |
| timestamps | lifecycle metadata |

Ràng buộc gợi ý:

- unique: `(workspace_id, idempotency_key)`

### `messages`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | message id |
| `workspace_id` | workspace owner |
| `campaign_id` / `transactional_request_id` | nguồn tạo message |
| recipient/sender/provider columns | context gửi |
| `status` | queued/accepted/delivered/bounced/... |
| timestamps | lifecycle metadata |

### `delivery_attempts`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | attempt id |
| `message_id` | message owner |
| `provider` | provider đã dùng |
| `attempt_no` | số lần thử |
| request/response metadata | dữ liệu provider |
| timestamps | lifecycle metadata |

### `retry_states`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | retry state id |
| `message_id` hoặc target id | đối tượng retry |
| `retry_count` | số lần retry |
| `next_attempt_at` | lần thử kế |
| `last_error_class` | loại lỗi gần nhất |
| policy metadata | rule retry |

### `suppression_entries`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | suppression id |
| `workspace_id` nullable nếu global | scope owner |
| `email_normalized` | email gốc |
| `email_hash` | lookup/hash hỗ trợ PII minimization |
| `scope` | global/tenant/list/category/provider |
| `reason` | unsubscribe/hard_bounce/... |
| `source` | webhook/manual/system |
| timestamps | lifecycle metadata |

### `provider_webhook_events`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | raw event id |
| `provider` | provider nguồn |
| `provider_event_id` | id external |
| `provider_message_id` | id message external |
| `workspace_id` nếu suy ra được | tenant liên quan |
| `payload_json` | raw payload |
| `received_at` | thời điểm nhận |
| verification columns | chữ ký/check metadata |

Ràng buộc gợi ý:

- unique: `(provider, provider_event_id)` nếu provider hỗ trợ id bền

### `normalized_provider_events`

| Cột chính | Ý nghĩa |
|---|---|
| `event_id` | event id nội bộ |
| `workspace_id` | workspace owner |
| `message_id` | message liên quan |
| `provider_message_id` | id provider |
| `event_type` | delivered/bounced/... |
| `occurred_at` | thời điểm thực tế |
| `received_at` | thời điểm ingest |
| `payload_json` | normalized payload |

### `tracking_links`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | tracking token/id |
| `workspace_id` | workspace owner |
| `message_id` | message owner |
| `destination_url` | URL đích |
| metadata | context |

### `tracking_events`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | tracking event id |
| `workspace_id` | workspace owner |
| `message_id` | message owner |
| `event_type` | open/click |
| `occurred_at` | thời điểm event |
| metadata | context |

### `customer_webhook_deliveries`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | delivery id |
| `workspace_id` | workspace owner |
| `webhook_config_id` | config đích |
| `event_type` | loại event được đẩy |
| `status` | queued/sent/failed |
| `retry_count` | số lần retry |
| timestamps | lifecycle metadata |

### `outbox_events`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | outbox id |
| `aggregate_type` / `aggregate_id` | nguồn phát sinh |
| `workspace_id` | tenant liên quan |
| `topic` | đích publish |
| `event_type` | contract versioned |
| `payload_json` | nội dung event |
| `published_at` | thời điểm publish |
| `status` | pending/published/failed |
| timestamps | lifecycle metadata |

Ghi chú triển khai:

- Nếu dùng Debezium outbox SMT, có thể chọn shape cột Debezium-friendly tương đương như `aggregatetype`, `aggregateid`, `type`, `payload`, `created_at` để giảm connector ceremony.
- Dù chọn shape vật lý nào, contract business vẫn nên giữ `event_type` versioned và `topic` theo naming domain event, ví dụ `identity.user.registered`.

## Redis keys

- `idem:kafka:<consumer_name>:<event_id>`
- `rate:<scope_key>`
- `lock:delivery:<message_id>`
- `cache:provider:quota:v1:<provider>:<workspace_id>`

## Index và truy vấn quan trọng

- `campaigns(workspace_id, status, scheduled_at)`
- `transactional_send_requests(workspace_id, idempotency_key)`
- `messages(workspace_id, status, created_at desc)`
- `delivery_attempts(message_id, attempt_no desc)`
- `suppression_entries(workspace_id, email_hash, scope)`
- `provider_webhook_events(provider, provider_event_id)`
- `normalized_provider_events(workspace_id, message_id, event_type, occurred_at)`
- `outbox_events(status, created_at)`
