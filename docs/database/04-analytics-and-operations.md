# Analytics And Operations Database

## Mục tiêu nhóm schema

Nhóm schema này lưu projection, analytics fact và operational state phục vụ observability, retry, replay và production readiness.

## Module ownership

| Table / state | Module owner chính |
|---|---|
| `message_log_projection`, `campaign_delivery_summary`, `deliverability_projection`, `customer_webhook_delivery_projection` | `analytics` nếu phục vụ reporting; module source vẫn sở hữu workflow state gốc |
| `processed_event_markers`, `dead_letter_records`, `replay_jobs`, Redis health/breaker/quota/feature flag snapshot | `operations` |
| `email_events`, `campaign_daily_stats`, `recipient_domain_hourly_stats`, `provider_delivery_stats` | `analytics` |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/05-analytics-and-operations.md` |
| API contract | `../api/04-events-webhooks-and-operations.md` |
| Business model | `../model/04-events-analytics-and-operations.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| Infrastructure | `../architecture/06-infrastructure.md` |
| Scale/resilience | `../architecture/08-scale-and-resilience.md` |

## PostgreSQL projection tables

### `message_log_projection`

| Cột chính | Ý nghĩa |
|---|---|
| `message_id` | message owner |
| `workspace_id` | workspace owner |
| current status columns | trạng thái hiện tại |
| latest provider/tracking/suppression fields | tín hiệu mới nhất |
| `last_updated_at` | freshness metadata |

### `campaign_delivery_summary`

| Cột chính | Ý nghĩa |
|---|---|
| `campaign_id` | campaign owner |
| `workspace_id` | workspace owner |
| counters by outcome | queued/delivered/bounced/... |
| `last_updated_at` | freshness metadata |

### `deliverability_projection`

| Cột chính | Ý nghĩa |
|---|---|
| scope columns | workspace/provider/domain |
| health counters/rates | complaint, bounce, defer, throttle |
| `last_updated_at` | freshness metadata |

### `customer_webhook_delivery_projection`

| Cột chính | Ý nghĩa |
|---|---|
| `delivery_id` | delivery owner |
| `workspace_id` | workspace owner |
| delivery status columns | execution view |
| `last_updated_at` | freshness metadata |

### `processed_event_markers`

| Cột chính | Ý nghĩa |
|---|---|
| `consumer_name` | consumer xử lý event |
| `event_id` | event đã xử lý |
| `processed_at` | thời điểm xử lý thành công |
| optional metadata | topic, partition/offset, checksum nếu cần |

Ràng buộc gợi ý:

- unique: `(consumer_name, event_id)`

### `dead_letter_records`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | dlq id |
| source topic/consumer | nơi phát sinh |
| payload columns | event/message lỗi |
| failure metadata | lý do lỗi |
| timestamps | lifecycle metadata |

### `replay_jobs`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | replay job id |
| target columns | stream/table/window |
| operator metadata | ai tạo |
| `status` | queued/running/completed/failed |
| timestamps | lifecycle metadata |

## Redis operational state

- `health:dependency:<name>`
- `breaker:<dependency_key>`
- `quota:<scope_key>`
- `featureflags:snapshot:v1:<workspace_id>`

## ClickHouse fact và aggregate tables

### `email_events`

| Cột chính | Ý nghĩa |
|---|---|
| `event_date` | partition date |
| `occurred_at` | thời điểm event |
| `received_at` | thời điểm ingest |
| `workspace_id` | workspace owner |
| `campaign_id` | campaign liên quan |
| `message_id` | message liên quan |
| `recipient_domain` | domain người nhận |
| `provider` | provider gửi |
| `sending_domain` | domain gửi |
| `event_type` | delivered/opened/clicked/... |
| `provider_message_id` | id provider |
| `metadata_json` | metadata linh hoạt |
| `inserted_at` | thời điểm insert |

Gợi ý vật lý:

- `PARTITION BY toYYYYMM(event_date)`
- `ORDER BY (workspace_id, campaign_id, event_type, occurred_at)`
- `TTL event_date + INTERVAL 18 MONTH`

### `campaign_daily_stats`

| Cột chính | Ý nghĩa |
|---|---|
| `workspace_id` | workspace owner |
| `campaign_id` | campaign owner |
| `event_date` | ngày thống kê |
| aggregate counters | metrics theo ngày |

### `recipient_domain_hourly_stats`

| Cột chính | Ý nghĩa |
|---|---|
| `workspace_id` | workspace owner |
| `recipient_domain` | domain người nhận |
| `provider` | provider |
| `hour_bucket` | mốc giờ |
| aggregate counters | rate/error/volume |

### `provider_delivery_stats`

| Cột chính | Ý nghĩa |
|---|---|
| `provider` | provider |
| `workspace_id` hoặc global scope | phạm vi thống kê |
| time bucket | khoảng thời gian |
| aggregate counters | accept/deliver/bounce/error |

## Index và retention quan trọng

- PostgreSQL projection nên index theo `(workspace_id, last_updated_at)` và khóa tra cứu màn hình chính.
- `processed_event_markers(consumer_name, event_id)` để bảo vệ idempotency consumer.
- DLQ/replay tables cần index theo `status`, `created_at`.
- ClickHouse raw fact giữ lâu hơn hot projection, nhưng vẫn cần TTL rõ.
