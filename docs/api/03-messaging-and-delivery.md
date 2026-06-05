# Messaging And Delivery API

## Mục tiêu nhóm API

Nhóm API này phục vụ việc tạo intent gửi email, theo dõi tiến trình gửi và điều khiển các capability delivery ở mức sản phẩm.

## Module ownership

| API/resource | Module owner chính | Ghi chú boundary |
|---|---|---|
| Campaigns | `campaign` | Sở hữu draft/schedule/state của campaign |
| Campaign audience/template/sender references | `campaign` | Chỉ giữ reference; readiness được hỏi từ `audience`, `content`, `sender` |
| Transactional send request | `delivery` | Sở hữu idempotency và acceptance của send request |
| Messages và delivery attempts | `delivery` | Source of truth cho message lifecycle và retry |
| Queue/retry visibility | `delivery` + `operations` | Delivery sở hữu state nghiệp vụ, operations sở hữu view vận hành chung |
| Message logs | `delivery` hoặc projection `analytics` | Nếu log là workflow state thì owner là delivery; nếu aggregate/reporting thì analytics |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/03-delivery-and-messaging.md` |
| Business model | `../model/03-messaging-and-delivery.md` |
| Database schema | `../database/03-delivery-and-events.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| Event/outbox flow | `../architecture/05-data-events-and-flows.md` |
| API conventions | `00-api-map.md` |

## Resource: Campaigns

### Actor chính

- Dashboard user

### Auth model

- Yêu cầu permission campaign read/write/send theo hành động.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/campaigns`
- `POST /api/v1/workspaces/{workspace_id}/campaigns`
- `GET /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}`
- `PATCH /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}`
- `POST /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/schedule`
- `POST /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/pause`

### Hành vi chính

- Tạo campaign.
- Gắn audience, template, sender.
- Lập lịch hoặc dừng campaign.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/campaigns`

- Query gợi ý: `status`, `sender_domain_id`, `template_id`, `from`, `to`, `cursor`, `limit`.
- Response chính: campaign list view cùng progress/summary field sơ bộ.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "id": "camp_123",
      "name": "June promo",
      "status": "scheduled",
      "sender_domain_id": "sd_123",
      "template_id": "tpl_123",
      "scheduled_at": "2026-06-01T08:00:00Z",
      "summary": {
        "planned_recipients": 12000,
        "queued": 0,
        "delivered": 0
      }
    }
  ],
  "meta": {
    "next_cursor": "cursor_123"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 campaign.read_denied` không có `campaign.read`

`POST /api/v1/workspaces/{workspace_id}/campaigns`

- Request chính: `name`, `audience_ref`, `template_ref`, `sender_domain_id`, optional scheduling metadata.
- Response chính: campaign draft mới tạo.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "name": "June promo",
  "audience_ref": {
    "type": "segment",
    "id": "seg_123"
  },
  "template_ref": {
    "template_id": "tpl_123",
    "version_id": "tplv_003"
  },
  "sender_domain_id": "sd_123",
  "message_type": "marketing"
}
```

Response body gợi ý:

```json
{
  "data": {
    "id": "camp_123",
    "status": "draft"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 campaign.write_denied` không có `campaign.write`
- `404 audience.segment_not_found` không tìm thấy segment trong workspace
- `404 template.not_found` không tìm thấy template trong workspace
- `404 sender.domain_not_found` không tìm thấy sender domain trong workspace
- `422 campaign.payload_invalid` payload campaign không hợp lệ
- `422 campaign.audience_not_ready` audience chưa sẵn sàng để dùng
- `422 template.source_invalid` template chưa hợp lệ để gắn vào campaign

`GET /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}`

- Response chính: campaign detail, audience selection summary, template/sender summary, orchestration state.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "id": "camp_123",
    "name": "June promo",
    "status": "scheduled",
    "audience_ref": {
      "type": "segment",
      "id": "seg_123",
      "estimated_recipients": 12000
    },
    "template_ref": {
      "template_id": "tpl_123",
      "version_id": "tplv_003"
    },
    "sender_domain_id": "sd_123",
    "scheduled_at": "2026-06-01T08:00:00Z",
    "orchestration": {
      "last_updated_at": "2026-05-31T10:00:00Z"
    }
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 campaign.read_denied` không có `campaign.read`
- `404 campaign.not_found` không tìm thấy campaign

`PATCH /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}`

- Request chính: sửa draft fields khi campaign còn ở trạng thái cho phép.
- Response chính: campaign sau cập nhật.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "name": "June promo updated",
  "sender_domain_id": "sd_456"
}
```

Response body gợi ý:

```json
{
  "data": {
    "id": "camp_123",
    "status": "draft",
    "name": "June promo updated"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 campaign.write_denied` không có `campaign.write`
- `404 campaign.not_found` không tìm thấy campaign
- `409 campaign.invalid_state_transition` campaign đã ở trạng thái không cho sửa
- `422 campaign.update_payload_invalid` dữ liệu cập nhật không hợp lệ

`POST /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/schedule`

- Request chính: `scheduled_at`, optional execution options.
- Response chính: campaign state đã schedule và timing summary.
- Status code: `202` hoặc `200` nếu orchestration được chấp nhận async.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "scheduled_at": "2026-06-01T08:00:00Z",
  "send_options": {
    "respect_quiet_hours": true
  }
}
```

Response body gợi ý:

```json
{
  "data": {
    "id": "camp_123",
    "status": "scheduled",
    "scheduled_at": "2026-06-01T08:00:00Z"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 campaign.send_denied` không có `campaign.send`
- `404 campaign.not_found` không tìm thấy campaign
- `409 campaign.invalid_state_transition` campaign đã schedule/running/completed
- `422 sender.domain_not_verified` sender chưa đủ điều kiện gửi
- `422 campaign.audience_not_ready` audience chưa đủ điều kiện
- `422 template.publish_required` template chưa có published version hợp lệ

`POST /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/pause`

- Response chính: campaign state sau pause/resume nếu API chọn chung command shape.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "id": "camp_123",
    "status": "paused"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 campaign.send_denied` không có `campaign.send`
- `404 campaign.not_found` không tìm thấy campaign
- `409 campaign.pause_conflict` campaign không ở trạng thái có thể pause

### Ghi chú contract

- Schedule là command làm phát sinh async flow; API đọc campaign phải cho thấy trạng thái orchestration hiện tại.

## Resource: Transactional email

### Actor chính

- Public API client
- Internal integration

### Auth model

- Yêu cầu API key hợp lệ với scope transactional send.

### API capability

- `POST /api/v1/transactional/send`
- `GET /api/v1/transactional/messages/{message_id}`

### Hành vi chính

- Nhận request gửi transactional email.
- Trả kết quả accept nhanh.
- Cho phép query message outcome theo `message_id` hoặc request reference.

### Request và response chính

`POST /api/v1/transactional/send`

- Header quan trọng: `Authorization`, `Idempotency-Key`.
- Request chính:
  - `to`
  - `sender_domain_id` hoặc sender ref
  - `template_id` + `template_data` hoặc raw subject/body
  - optional `metadata`, `tags`, `message_type`
- Response chính:
  - `message_id`
  - `request_id`
  - `status: accepted`
  - optional acceptance timestamp
- Status code: `202`, `401`, `403`, `409` khi idempotency conflict, `422` nếu sender/template/recipient không hợp lệ.

Headers:

- `Authorization: Bearer <api_key>`
- `Idempotency-Key: idem_123`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "to": [
    {
      "email": "alice@example.com",
      "name": "Alice"
    }
  ],
  "sender_domain_id": "sd_123",
  "template_id": "tpl_123",
  "template_data": {
    "first_name": "Alice",
    "otp_code": "492113"
  },
  "metadata": {
    "order_id": "ord_123"
  },
  "tags": [
    "otp"
  ],
  "message_type": "transactional"
}
```

Response body gợi ý:

```json
{
  "data": {
    "message_id": "msg_123",
    "request_id": "req_123",
    "status": "accepted",
    "accepted_at": "2026-05-31T10:00:00Z"
  }
}
```

Error codes:

- `400 delivery.request_body_invalid` body sai shape hoặc thiếu field bắt buộc
- `401 api_key.invalid` API key không hợp lệ
- `403 api_key.scope_denied` API key không có scope gửi transactional
- `404 sender.domain_not_found` không tìm thấy sender trong workspace
- `404 template.not_found` không tìm thấy template trong workspace
- `409 delivery.idempotency_key_conflict` `Idempotency-Key` xung đột với payload khác
- `422 delivery.recipient_invalid` recipient không hợp lệ
- `422 template.render_payload_invalid` template data không hợp lệ
- `422 sender.domain_not_verified` sender chưa hợp lệ để gửi
- `422 delivery.recipient_suppressed` recipient đang bị suppress
- `429 delivery.request_rate_limited` vượt rate limit ở request boundary
- `503 delivery.temporarily_unavailable` runtime/dependency tạm thời không sẵn sàng

`GET /api/v1/transactional/messages/{message_id}`

- Response chính: accepted state, delivery status hiện tại, provider metadata nếu đã có, `last_updated_at`.

Headers:

- `Authorization: Bearer <api_key>`

Response body gợi ý:

```json
{
  "data": {
    "message_id": "msg_123",
    "status": "delivered",
    "provider": "ses",
    "provider_message_id": "ses_abc_123",
    "last_updated_at": "2026-05-31T10:02:00Z"
  }
}
```

Error codes:

- `401 api_key.invalid` API key không hợp lệ
- `403 api_key.scope_denied` API key không có scope đọc message tương ứng
- `404 delivery.message_not_found` không tìm thấy message trong workspace

### Ghi chú contract

- Endpoint này nên hỗ trợ `Idempotency-Key`.
- Auth dùng API key, không dùng dashboard session.

## Resource: Message logs

### Actor chính

- Dashboard user
- Operator

### Auth model

- Dashboard session với permission campaign/delivery read hoặc operations read.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/messages`
- `GET /api/v1/workspaces/{workspace_id}/messages/{message_id}`

### Hành vi chính

- Trả trạng thái message và lịch sử event liên quan.
- Hỗ trợ lọc theo campaign, provider, trạng thái hoặc recipient.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/messages`

- Query gợi ý: `campaign_id`, `status`, `provider`, `recipient`, `from`, `to`, `cursor`, `limit`.
- Response chính: log rows với message status, timestamps quan trọng, recipient/provider summary.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "message_id": "msg_123",
      "recipient": "alice@example.com",
      "status": "delivered",
      "provider": "ses",
      "sent_at": "2026-05-31T10:00:30Z",
      "last_updated_at": "2026-05-31T10:02:00Z"
    }
  ],
  "meta": {
    "next_cursor": "cursor_123"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 delivery.logs_read_denied` không có quyền đọc message logs
- `404 identity.workspace_not_found` workspace không hợp lệ

`GET /api/v1/workspaces/{workspace_id}/messages/{message_id}`

- Response chính: timeline event của message, delivery attempts, suppression/tracking summary nếu có.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "message_id": "msg_123",
    "status": "delivered",
    "timeline": [
      {
        "event_type": "delivery.message.queued.v1",
        "occurred_at": "2026-05-31T10:00:00Z"
      },
      {
        "event_type": "delivery.message.delivered.v1",
        "occurred_at": "2026-05-31T10:02:00Z"
      }
    ],
    "tracking": {
      "opens": 1,
      "clicks": 0
    }
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 delivery.logs_read_denied` không có quyền đọc message logs
- `404 delivery.message_not_found` không tìm thấy message

### Ghi chú contract

- Đây là read model tổng hợp từ delivery, webhook, tracking, suppression.

## Resource: Queue và retry operations

### Actor chính

- Operator

### Auth model

- Yêu cầu role operations/support hoặc permission delivery manage tương đương.

### API capability

- `GET /api/v1/operations/queue`
- `GET /api/v1/operations/retries`
- `POST /api/v1/operations/messages/{message_id}/retry`
- `POST /api/v1/operations/messages/{message_id}/cancel`

### Hành vi chính

- Xem tình trạng queue/retry.
- Trigger retry hoặc cancel ở mức operations khi policy cho phép.

### Request và response chính

`GET /api/v1/operations/queue`

- Query gợi ý: `topic`, `priority`, `workspace_id`, `status`.
- Response chính: queue depth, lag age, inflight summary.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "queues": [
      {
        "topic": "delivery.transactional",
        "lag": 12,
        "queue_age_p95_ms": 3200,
        "inflight": 24
      }
    ]
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 operations.queue_read_denied` không có quyền operations

`GET /api/v1/operations/retries`

- Query gợi ý: `workspace_id`, `provider`, `status`, `cursor`.
- Response chính: retry items hoặc retry projection view.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "message_id": "msg_123",
      "retry_count": 2,
      "next_attempt_at": "2026-05-31T10:05:00Z",
      "last_error_class": "provider_timeout"
    }
  ]
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 operations.retry_read_denied` không có quyền operations

`POST /api/v1/operations/messages/{message_id}/retry`

- Request chính: optional operator reason.
- Response chính: retry accepted/result snapshot.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "reason": "provider recovered"
}
```

Response body gợi ý:

```json
{
  "data": {
    "message_id": "msg_123",
    "status": "retry_scheduled"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 operations.retry_manage_denied` không có quyền operations
- `404 delivery.message_not_found` không tìm thấy message
- `409 delivery.retry_conflict` message không ở trạng thái có thể retry

`POST /api/v1/operations/messages/{message_id}/cancel`

- Response chính: cancel result nếu trạng thái cho phép dừng tiếp.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "message_id": "msg_123",
    "status": "cancelled"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 operations.cancel_manage_denied` không có quyền operations
- `404 delivery.message_not_found` không tìm thấy message
- `409 delivery.cancel_conflict` message không ở trạng thái có thể cancel

### Ghi chú contract

- Đây là operational API, không phải workflow chính cho end user thông thường.

## Resource: Deliverability và rate limits

### Actor chính

- Operator
- Workspace admin trong các màn hình điều hành

### Auth model

- Yêu cầu read permission cho operations/deliverability.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/deliverability`
- `GET /api/v1/workspaces/{workspace_id}/rate-limits`

### Hành vi chính

- Hiển thị health của provider, sender, complaint/bounce posture và tín hiệu throttling.
- Xem quota hoặc trạng thái hạn chế đang áp dụng.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/deliverability`

- Query gợi ý: `window`, `provider`, `sender_domain_id`.
- Response chính: bounce rate, complaint rate, deferred rate, sender/provider health, `last_updated_at`.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "window": "7d",
    "bounce_rate": 0.012,
    "complaint_rate": 0.001,
    "deferred_rate": 0.031,
    "last_updated_at": "2026-05-31T10:03:00Z"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 deliverability.read_denied` không có quyền đọc deliverability

`GET /api/v1/workspaces/{workspace_id}/rate-limits`

- Response chính: throttle scopes đang active theo workspace/provider/domain/message type, quota usage snapshot.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "scope": "workspace",
      "scope_key": "ws_123",
      "message_type": "transactional",
      "quota_per_minute": 600,
      "current_usage": 142
    }
  ]
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 rate_limit.read_denied` không có quyền đọc rate limits

### Ghi chú contract

- Đây là projection/operations view, không phải nơi quyết định business state nguồn.
