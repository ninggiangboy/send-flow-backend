# Events Webhooks And Operations API

## Mục tiêu nhóm API

Nhóm API này bao phủ ingress từ provider/recipient và các endpoint phục vụ observability, analytics, health, cũng như egress webhook cho downstream system.

## Module ownership

| API/resource | Module owner chính | Ghi chú boundary |
|---|---|---|
| Provider webhook ingress | `ingestion` | Sở hữu raw ingest, signature verification, normalize/dedupe |
| Click/open tracking endpoint | `tracking` | Sở hữu tracking link/event |
| Unsubscribe endpoint | `suppression` | Sở hữu suppress/unsuppress decision |
| Customer webhook delivery view/retry | `webhooks` | Sở hữu outbound delivery và retry history |
| Analytics/dashboard endpoints | `analytics` | Sở hữu fact/projection/reporting, không là workflow source of truth |
| Queue, DLQ, replay, health/readiness, runtime stream (SSE) | `operations` | Sở hữu operational workflow/view |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/04-ingestion-tracking-suppression.md`, `../features/05-analytics-and-operations.md` |
| Business model | `../model/03-messaging-and-delivery.md`, `../model/04-events-analytics-and-operations.md` |
| Database schema | `../database/03-delivery-and-events.md`, `../database/04-analytics-and-operations.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| Event/outbox flow | `../architecture/05-data-events-and-flows.md` |
| API conventions | `00-api-map.md` |

## Resource: Provider webhook ingress

### Actor chính

- Email provider

### Auth model

- Không dùng session/API key của tenant.
- Xác thực bằng provider signature, secret hoặc verification metadata provider-specific.

### API capability

- `POST /api/v1/webhooks/providers/{provider}`

### Hành vi chính

- Verify webhook signature.
- Persist raw payload.
- Normalize event và đẩy sang pipeline nội bộ.

### Request và response chính

`POST /api/v1/webhooks/providers/{provider}`

- Request chính: provider payload nguyên bản và header xác thực đi kèm.
- Response chính: acknowledgement tối giản, ví dụ `{ "accepted": true }`.
- Status code:
  - `2xx` khi đã nhận và persist/ack an toàn
  - `4xx` nếu signature sai hoặc payload không parse được
  - `5xx` chỉ khi hệ thống thật sự chưa thể xử lý retry an toàn

Headers:

- Provider-specific signature headers như `X-Signature`, `X-Timestamp` hoặc equivalent
- `Content-Type: application/json`

Request body:

- Giữ nguyên payload provider gửi vào; không ép một schema chung ở boundary HTTP này.

Response body gợi ý:

```json
{
  "accepted": true
}
```

Error codes:

- `400 webhook.payload_invalid` payload không parse được
- `401 webhook.invalid_signature` signature hoặc secret không hợp lệ
- `404 webhook.provider_not_supported` provider không được hỗ trợ
- `409 webhook.duplicate_event_conflict` event duplicate nhưng policy coi là conflict ở ingress
- `503 webhook.ingest_temporarily_unavailable` tạm thời không persist/ack an toàn

### Dữ liệu tối thiểu cần rút ra

- `provider_event_id`
- `provider_message_id`
- `event_type`
- `occurred_at`
- tenant/message mapping nếu suy ra được

### Ghi chú contract

- Endpoint phải idempotent, nhanh và retry-safe.

## Resource: Tracking và unsubscribe endpoint

### Actor chính

- Recipient

### Auth model

- Không yêu cầu login.
- Tin cậy dựa trên opaque token/tracking id hoặc signed unsubscribe token.

### API capability

- `GET /t/{tracking_id}`           # click redirect
- `GET /o/{tracking_id}`           # open tracking pixel
- `POST /u/{token}`                # one-click unsubscribe

### Hành vi chính

- Ghi open/click event.
- Redirect recipient đến URL đích.
- Ghi unsubscribe idempotent.

### Request và response chính

`GET /t/{tracking_id}`

- Hành vi: resolve tracking link, ghi click event, redirect `302/307` tới URL đích.
- Failure mode: nếu token hỏng/hết hạn có thể redirect đến fallback page hoặc trả landing page lỗi có kiểm soát.

Headers:

- Không yêu cầu auth header
- Request có thể mang `User-Agent`, `Referer` phục vụ analytics

Response headers gợi ý:

- `Location: https://destination.example.com`
- `Cache-Control: no-store`

Response body:

- Thường không có body đáng kể khi redirect.

Error codes:

- `302 tracking.redirect_success` redirect thành công
- `307 tracking.redirect_preserve_method` redirect thành công và giữ method khi cần
- `404 tracking.invalid_tracking_id` tracking id không hợp lệ
- `410 tracking.tracking_id_expired` tracking id đã hết hạn

`GET /o/{tracking_id}`

- Hành vi: ghi open event nếu hợp lệ và trả pixel response nhỏ.
- Response chính: `200` với image payload hoặc equivalent.

Headers:

- Không yêu cầu auth header

Response headers gợi ý:

- `Content-Type: image/gif`
- `Cache-Control: no-store, no-cache, must-revalidate`

Response body:

- 1x1 pixel payload.

Error codes:

- `200 tracking.pixel_served` trả pixel kể cả khi duplicate open
- `404 tracking.invalid_tracking_id` tracking id không hợp lệ nếu không muốn trả pixel mù

`POST /u/{token}`

- Hành vi: resolve recipient + scope, ghi suppression idempotent.
- Response chính: unsubscribe result summary.
- Status code: `200` kể cả khi token đã được xử lý trước đó nếu muốn semantics idempotent.

Headers:

- `Content-Type: application/json` nếu endpoint nhận body

Request body gợi ý:

```json
{
  "reason": "user_request"
}
```

Response body gợi ý:

```json
{
  "data": {
    "status": "unsubscribed",
    "scope": "workspace"
  }
}
```

Error codes:

- `200 suppression.unsubscribe_applied` unsubscribe thành công hoặc đã unsubscribe trước đó
- `400 suppression.unsubscribe_token_invalid` token malformed
- `404 suppression.unsubscribe_token_not_found` token không tìm thấy
- `410 suppression.unsubscribe_token_expired` token hết hạn

### Ghi chú contract

- Endpoint này ưu tiên độ tin cậy và abuse control hơn hình thức API dashboard.

## Resource: Customer webhook delivery view

### Actor chính

- Workspace admin
- Developer của tenant
- Operator

### Auth model

- Yêu cầu dashboard session với permission developer/operations read.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/webhook-deliveries`
- `GET /api/v1/workspaces/{workspace_id}/webhook-deliveries/{delivery_id}`
- `POST /api/v1/workspaces/{workspace_id}/webhook-deliveries/{delivery_id}/retry`

### Hành vi chính

- Xem kết quả đẩy event ra downstream webhook.
- Retry khi delivery thất bại nếu policy cho phép.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/webhook-deliveries`

- Query gợi ý: `status`, `event_type`, `webhook_id`, `from`, `to`, `cursor`.
- Response chính: delivery attempt list với status, retry count, last error summary.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "delivery_id": "whd_123",
      "event_type": "delivery.message.delivered.v1",
      "status": "failed",
      "retry_count": 2,
      "last_error": "timeout"
    }
  ]
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 webhook.delivery_read_denied` không có quyền developer/operations read

`GET /api/v1/workspaces/{workspace_id}/webhook-deliveries/{delivery_id}`

- Response chính: delivery detail, request/response metadata an toàn để expose, retry timeline.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "delivery_id": "whd_123",
    "status": "failed",
    "request": {
      "target_url": "https://example.com/webhook"
    },
    "response": {
      "status_code": 504
    }
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 webhook.delivery_read_denied` không có quyền developer/operations read
- `404 webhook.delivery_not_found` không tìm thấy delivery

`POST /api/v1/workspaces/{workspace_id}/webhook-deliveries/{delivery_id}/retry`

- Response chính: retry accepted state hoặc updated delivery status.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "delivery_id": "whd_123",
    "status": "retry_scheduled"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 webhook.delivery_retry_denied` không có quyền retry webhook delivery
- `404 webhook.delivery_not_found` không tìm thấy delivery
- `409 webhook.delivery_retry_conflict` delivery không ở trạng thái có thể retry

### Ghi chú contract

- Khác với resource cấu hình webhook ở control plane; đây là view thực thi delivery.

## Resource: Analytics và dashboard

### Actor chính

- Dashboard user
- Operator

### Auth model

- Yêu cầu permission analytics read hoặc operations read.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/analytics/overview`
- `GET /api/v1/workspaces/{workspace_id}/analytics/campaigns/{campaign_id}`
- `GET /api/v1/workspaces/{workspace_id}/analytics/deliverability`

### Hành vi chính

- Trả aggregate metrics cho dashboard.
- Trả breakdown theo campaign, provider, domain hoặc event type.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/analytics/overview`

- Query gợi ý: `window`, `compare_to`, `provider`, `message_type`.
- Response chính: KPI tổng quan, trend summary, freshness metadata.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "window": "7d",
    "kpis": {
      "accepted": 10234,
      "delivered": 9988,
      "opened": 4120,
      "clicked": 621
    },
    "last_updated_at": "2026-05-31T10:05:00Z"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 analytics.read_denied` không có quyền analytics read

`GET /api/v1/workspaces/{workspace_id}/analytics/campaigns/{campaign_id}`

- Query gợi ý: `window`.
- Response chính: campaign funnel, time series, top errors/segments phù hợp.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "campaign_id": "camp_123",
    "funnel": {
      "queued": 12000,
      "delivered": 11810,
      "opened": 4120,
      "clicked": 621
    }
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 analytics.read_denied` không có quyền analytics read
- `404 campaign.not_found` không tìm thấy campaign

`GET /api/v1/workspaces/{workspace_id}/analytics/deliverability`

- Response chính: health metrics theo sender domain/provider/recipient domain.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "providers": [
      {
        "provider": "ses",
        "deferred_rate": 0.03
      }
    ]
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 analytics.read_denied` không có quyền analytics read

### Ghi chú contract

- Response nên làm rõ `window`, `last_updated_at` và độ trễ projection nếu có.

## Resource: Health và readiness

### Actor chính

- Load balancer
- Platform/operator

### Auth model

- Có thể không yêu cầu auth sau private network/gateway.
- Nếu expose công khai, cần policy bảo vệ riêng.

### API capability

- `GET /api/healthz`
- `GET /api/readyz`
- `GET /api/events/stream`

### Hành vi chính

- Báo tình trạng sống của process.
- Báo khả năng nhận traffic hoặc xử lý job an toàn.

### Request và response chính

`GET /api/healthz`

- Response chính: process alive, version/build metadata tối thiểu nếu muốn.

Response headers gợi ý:

- `Content-Type: application/json`

Response body gợi ý:

```json
{
  "status": "ok"
}
```

Error codes:

- `200 health.process_alive` process còn sống

`GET /api/readyz`

- Response chính: readiness tổng hợp từ dependency tối thiểu như DB, Redis, Kafka hoặc provider-critical path theo runtime.
- Status code: `200` khi sẵn sàng, `503` khi chưa nên nhận traffic hoặc job mới.

Response headers gợi ý:

- `Content-Type: application/json`

Response body gợi ý:

```json
{
  "status": "ready",
  "dependencies": {
    "postgres": "ok",
    "redis": "ok",
    "kafka": "ok"
  }
}
```

Error codes:

- `200 health.runtime_ready` runtime sẵn sàng
- `503 health.runtime_not_ready` chưa sẵn sàng nhận traffic/job

`GET /api/events/stream`

- Response chính: mở SSE stream để client theo dõi runtime events dạng realtime.
- Status code: `200` khi mở stream thành công.

Response headers gợi ý:

- `Content-Type: text/event-stream`
- `Cache-Control: no-cache`
- `Connection: keep-alive`

Response event gợi ý:

```text
event: connected
data: send-flow stream connected

: keepalive
```

Error codes:

- `200 operations.stream_opened` stream mở thành công
- `500 operations.stream_not_supported` runtime không hỗ trợ streaming transport

### Ghi chú contract

- Readiness phải phản ánh dependency tối thiểu quan trọng, không chỉ trạng thái process.
- SSE stream giữ kết nối mở cho tới khi client ngắt kết nối hoặc request context bị hủy.
