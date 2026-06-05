# API Map

Bộ tài liệu này mô tả **surface API** của `send-flow` ở mức sản phẩm và contract. Nó trả lời ba câu hỏi:

- Hệ thống có những nhóm API nào?
- Actor nào gọi từng nhóm API?
- Resource, action và event ingress/egress chính là gì?

Các tài liệu này bám theo `docs/architecture/*` và `docs/features/*`. Vì code backend chưa được triển khai đầy đủ, đây là **API blueprint** ở mức contract, không phải OpenAPI cuối cùng.

Nếu cần đọc theo **use case hoặc flow tổng thể**, bắt đầu từ `05-use-cases-and-flows.md` trước rồi quay lại từng file resource để lấy contract chi tiết.

## Cách đọc nhanh

Graph tổng và capability trace nằm ở [../00-doc-graph.md](../00-doc-graph.md).

| Cần trả lời | Đọc file |
|---|---|
| Auth, workspace, role, API key, sender, settings có API gì? | `01-auth-and-control-plane.md` |
| Audience, contacts, segments, templates có API gì? | `02-audience-and-content.md` |
| Campaign, transactional send, logs, queue có API gì? | `03-messaging-and-delivery.md` |
| Provider webhook, tracking, customer webhook, analytics, operations có API gì? | `04-events-webhooks-and-operations.md` |
| Muốn biết cần phối hợp nhiều API theo use case nào và theo thứ tự nào? | `05-use-cases-and-flows.md` |

## Đọc theo endpoint hay theo flow

- Các file `01` đến `04` trả lời câu hỏi: từng nhóm resource có API nào, actor nào gọi, auth model ra sao và request/response chính là gì.
- File `05-use-cases-and-flows.md` trả lời câu hỏi: cần gọi những API nào theo thứ tự nào để onboard tenant, chuẩn bị campaign, gửi transactional email hoặc theo dõi hậu gửi.
- Khi tích hợp thật, nên đọc flow trước để xác định tuyến gọi API, rồi mở lại file resource tương ứng để lấy contract chi tiết.

## Các loại API trong hệ thống

| Loại API | Người gọi chính | Mục đích |
|---|---|---|
| Dashboard API | Dashboard user | Quản trị workspace, audience, content, delivery, analytics, settings |
| Auth provider callback API | Dashboard app, external identity provider | Hoàn tất đăng nhập OAuth/OIDC như Google, GitHub |
| Public API | Public API client | Gửi transactional email và thao tác resource qua API key |
| Provider webhook ingress | Email provider | Đẩy delivery/tracking/suppression signal vào hệ thống |
| Recipient-facing endpoint | Recipient | Click tracking, unsubscribe, open tracking |
| Customer webhook egress | Hệ thống `send-flow` gọi ra | Đẩy event cho downstream system của tenant |
| Operations endpoint | Operator, load balancer, platform | Kiểm tra health/readiness, theo dõi trạng thái runtime, consume realtime runtime stream (SSE) |

## Module ownership theo nhóm API

API docs được nhóm theo trải nghiệm sản phẩm, còn owner module được dùng để triển khai bounded context trong `internal/modules`.

| Nhóm API / resource | Module owner chính | Module liên quan |
|---|---|---|
| Auth, session, workspace, membership | `identity` | `access`, `audit` |
| Role, permission, API key | `access` | `identity`, `audit` |
| Sender domain, DNS verification | `sender` | `delivery`, `campaign`, `audit` |
| Workspace settings | `identity` hoặc module settings riêng nếu tách sau | `audit` |
| Contacts, lists, segments, import/export | `audience` | `suppression`, `analytics` |
| Templates, versions, preview/render | `content` | `campaign`, `delivery`, `analytics` |
| Campaigns, scheduling, campaign candidates | `campaign` | `audience`, `content`, `sender`, `delivery`, `analytics` |
| Transactional send, message logs, delivery state | `delivery` | `content`, `sender`, `suppression`, `analytics`, `webhooks` |
| Provider webhook ingress, normalization | `ingestion` | `delivery`, `suppression`, `tracking`, `analytics` |
| Tracking open/click endpoints | `tracking` | `analytics`, `webhooks` |
| Unsubscribe endpoint | `suppression` | `tracking`, `analytics`, `webhooks` |
| Customer webhook config/delivery history | `webhooks` | `delivery`, `suppression`, `tracking`, `analytics` |
| Analytics dashboard/reporting | `analytics` | `campaign`, `delivery`, `tracking`, `suppression` |
| Queue, DLQ, replay, health/readiness, runtime stream (SSE) | `operations` | all event-producing/consuming modules |

## Nguyên tắc thiết kế API

- API phải luôn mang `workspace_id` theo request context hoặc resource scope đối với product data.
- Dashboard API và Public API có auth model khác nhau:
  - Dashboard API dùng user/session/permission.
  - Public API dùng API key scope theo workspace.
- Những thao tác dài hoặc side effect lớn trả kết quả chấp nhận ở request path, còn outcome chi tiết đi qua async flow.
- Provider webhook và recipient endpoint phải idempotent, nhanh và retry-safe.
- Delivery outcome, analytics và logs là eventual consistency; API đọc phải thể hiện rõ trạng thái `pending`, `processing` hoặc `last_updated_at` khi cần.

## API conventions chung

### Base path và versioning

- Dashboard và public API đi qua prefix `/api/v1`.
- Tracking/open/unsubscribe endpoint có thể đi ngoài `/api/v1` để giữ URL ngắn trong email.
- Event contract nội bộ version bằng `event_type`, còn HTTP contract version ở URL.

### Authentication

| API type | Cơ chế auth | Header/context chính |
|---|---|---|
| Dashboard API | Session hoặc bearer token của user | `Authorization`, session cookie hoặc token tương đương |
| Auth provider callback API | OAuth2/OIDC state + PKCE + provider token exchange | `state`, `code`, `redirect_uri` hoặc backend-managed redirect |
| Public API | Workspace-scoped API key | `Authorization: Bearer <api_key>` hoặc header tương đương |
| Provider webhook ingress | Provider signature verification | Header chữ ký provider-specific |
| Recipient endpoint | Không yêu cầu login | Signed token hoặc opaque tracking id trong URL |
| Health/readiness/SSE stream | Thường nội bộ | Có thể không cần auth sau gateway nội bộ |

### Request context

Dashboard request sau auth nên resolve được tối thiểu:

```json
{
  "actor_user_id": "user_123",
  "workspace_id": "ws_123",
  "membership_id": "m_123",
  "effective_permissions": [
    "campaign.read",
    "campaign.write"
  ],
  "request_id": "req_123"
}
```

Với resource product data, `workspace_id` phải được xác định tường minh từ path, query hoặc request context cùng membership tương ứng; API không dựa vào workspace state được nhớ trong session.

### Response envelope gợi ý

Response không bắt buộc phải bọc mọi nơi, nhưng để nhất quán tài liệu này dùng convention:

```json
{
  "data": {},
  "meta": {
    "request_id": "req_123"
  }
}
```

List response:

```json
{
  "data": [],
  "meta": {
    "next_cursor": "cursor_123",
    "total_estimate": 120
  }
}
```

Async command response:

```json
{
  "data": {
    "id": "job_or_message_id",
    "status": "accepted"
  },
  "meta": {
    "request_id": "req_123"
  }
}
```

### Error contract gợi ý

```json
{
  "error": {
    "code": "identity.email_already_registered",
    "message": "email already registered",
    "details": {
      "field": "email"
    }
  },
  "meta": {
    "request_id": "req_123"
  }
}
```

Quy ước `error.code`:

- Dùng dạng `domain.reason` hoặc `domain.resource_reason`.
- Ổn định hơn `message`; frontend và client SDK nên dựa vào `code` để xử lý nhánh logic.
- `message` có thể đổi cách diễn đạt, nhưng `code` nên được coi là contract.

Ví dụ:

- `auth.invalid_credentials`
- `auth.invalid_token`
- `identity.email_already_registered`
- `identity.permission_denied`
- `api_key.scope_denied`
- `sender.domain_not_verified`
- `template.render_payload_invalid`
- `campaign.invalid_state_transition`
- `delivery.idempotency_key_conflict`
- `webhook.invalid_signature`
- `tracking.invalid_tracking_id`
- `analytics.projection_not_ready`

Error class nên map nhất quán:

| HTTP status | Khi dùng |
|---|---|
| `400` | Request shape sai hoặc thiếu field |
| `401` | Chưa xác thực |
| `403` | Có auth nhưng thiếu quyền hoặc lệch workspace |
| `404` | Không tìm thấy resource trong scope hiện tại |
| `409` | Xung đột trạng thái, uniqueness hoặc idempotency semantic |
| `422` | Dữ liệu hợp lệ về shape nhưng vi phạm business rule |
| `429` | Rate limit hoặc throttling ở request boundary |
| `500` | Lỗi nội bộ không phân loại được |
| `503` | Tạm thời không sẵn sàng do dependency/runtime issue |

Một HTTP status có thể chứa nhiều `error.code` khác nhau. Ví dụ cùng là `422` nhưng có thể là:

- `sender.domain_not_verified`
- `campaign.audience_not_ready`
- `template.render_payload_invalid`
- `delivery.recipient_suppressed`

### Idempotency và eventual consistency

- Command quan trọng như transactional send, import/export trigger, webhook ingest nên hỗ trợ hoặc tự có idempotency.
- Đọc projection như analytics, message logs, queue/retry phải có `last_updated_at` hoặc status tương đương nếu dữ liệu đi qua async pipeline.
- Webhook ingress và recipient-facing endpoint phải an toàn khi bị retry hoặc double-submit.

### Filtering, pagination và time window

- List endpoint nên hỗ trợ `limit`, `cursor`, `sort`.
- Màn hình operations/analytics nên hỗ trợ `from`, `to`, `status`, `provider`, `campaign_id`, `recipient`, `domain` khi phù hợp.
- Không giả định offset pagination cho log/event volume lớn; cursor hoặc time-window an toàn hơn.
