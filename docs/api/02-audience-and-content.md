# Audience And Content API

## Mục tiêu nhóm API

Nhóm API này phục vụ quản lý audience và nội dung email trước khi đi vào send pipeline.

## Module ownership

| API/resource | Module owner chính | Ghi chú boundary |
|---|---|---|
| Contacts | `audience` | Source of truth cho recipient identity trong workspace |
| Lists và list membership | `audience` | Campaign chỉ dùng public audience selection/read model |
| Segments | `audience` | Segment definition/evaluation thuộc audience |
| Import/export audience | `audience` | Job state và artifact reference thuộc audience |
| Templates | `content` | Template lifecycle không thuộc campaign |
| Template versions | `content` | Published version là contract để campaign/delivery tham chiếu |
| Preview/render | `content` | Delivery gọi public render API, không import template internals |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/02-audience-and-content.md` |
| Business model | `../model/02-audience-and-content.md` |
| Database schema | `../database/02-audience-and-content.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| API conventions | `00-api-map.md` |

## Resource: Contacts

### Actor chính

- Dashboard user
- Public API client nếu expose audience management

### Auth model

- Dashboard session với quyền `audience.read` hoặc `audience.write`.
- Nếu expose public API cho audience, phải giới hạn scope rõ ràng hơn transactional API.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/contacts`
- `POST /api/v1/workspaces/{workspace_id}/contacts`
- `GET /api/v1/workspaces/{workspace_id}/contacts/{contact_id}`
- `PATCH /api/v1/workspaces/{workspace_id}/contacts/{contact_id}`
- `DELETE /api/v1/workspaces/{workspace_id}/contacts/{contact_id}`

### Hành vi chính

- Tạo, xem, cập nhật và xóa contact.
- Query contact trong boundary workspace.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/contacts`

- Query gợi ý: `q`, `status`, `list_id`, `segment_id`, `limit`, `cursor`, `sort`.
- Response chính: contact list view và pagination metadata.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "id": "ct_123",
      "email": "alice@example.com",
      "status": "active"
    }
  ],
  "meta": {
    "next_cursor": "cursor_123"
  }
}
```

`POST /api/v1/workspaces/{workspace_id}/contacts`

- Request chính: `email`, profile fields, tags/custom attributes nếu có.
- Response chính: contact mới tạo.
- Status code: `201`, `409` nếu trùng email trong workspace.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "email": "alice@example.com",
  "first_name": "Alice",
  "tags": [
    "vip"
  ]
}
```

Response body gợi ý:

```json
{
  "data": {
    "id": "ct_123",
    "email": "alice@example.com",
    "status": "active"
  }
}
```

`GET /api/v1/workspaces/{workspace_id}/contacts/{contact_id}`

- Response chính: contact detail, list membership summary, suppression summary nếu expose.

Headers:

- `Authorization: Bearer <session_token>`

`PATCH /api/v1/workspaces/{workspace_id}/contacts/{contact_id}`

- Request chính: partial profile update, status, tags.
- Response chính: contact đã cập nhật.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

`DELETE /api/v1/workspaces/{workspace_id}/contacts/{contact_id}`

- Response chính: `204` hoặc deleted state.

Headers:

- `Authorization: Bearer <session_token>`

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 audience.read_denied` không có quyền đọc audience
- `403 audience.write_denied` không có quyền sửa audience
- `404 audience.contact_not_found` không tìm thấy contact
- `409 audience.contact_email_conflict` email contact đã tồn tại trong workspace
- `422 audience.contact_payload_invalid` payload contact không hợp lệ
- `422 audience.contact_status_invalid` trạng thái contact không hợp lệ

### Ghi chú contract

- Resource phải mang đủ identity để liên kết với lists, segments và suppression signal.

## Resource: Lists và segments

### Actor chính

- Dashboard user

### Auth model

- Yêu cầu permission audience read/write tương ứng.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/lists`
- `POST /api/v1/workspaces/{workspace_id}/lists`
- `GET /api/v1/workspaces/{workspace_id}/segments`
- `POST /api/v1/workspaces/{workspace_id}/segments`
- `PUT /api/v1/workspaces/{workspace_id}/lists/{list_id}/contacts`
- `PATCH /api/v1/workspaces/{workspace_id}/segments/{segment_id}`

### Hành vi chính

- Quản lý list.
- Gán contact vào list.
- Định nghĩa và cập nhật segment.
- Xem audience membership theo list/segment.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/lists`

- Trả list collection cùng count sơ bộ nếu có.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "id": "list_123",
      "name": "VIP customers",
      "contact_count": 124
    }
  ]
}
```

`POST /api/v1/workspaces/{workspace_id}/lists`

- Request chính: `name`, optional description/metadata.
- Response chính: list mới tạo.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "name": "VIP customers"
}
```

Response body gợi ý:

```json
{
  "data": {
    "id": "list_123",
    "name": "VIP customers"
  }
}
```

`PUT /api/v1/workspaces/{workspace_id}/lists/{list_id}/contacts`

- Request chính: danh sách `contact_ids`, mode `replace` hoặc `merge`.
- Response chính: membership summary như `added_count`, `removed_count`, `skipped_count`.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "mode": "merge",
  "contact_ids": [
    "ct_123",
    "ct_456"
  ]
}
```

Response body gợi ý:

```json
{
  "data": {
    "added_count": 2,
    "removed_count": 0,
    "skipped_count": 0
  }
}
```

`GET /api/v1/workspaces/{workspace_id}/segments`

- Query gợi ý: `status`, `limit`, `cursor`.
- Response chính: segment collection và evaluation status.

Headers:

- `Authorization: Bearer <session_token>`

`POST /api/v1/workspaces/{workspace_id}/segments`

- Request chính: `name`, `definition`.
- Response chính: segment mới và status ban đầu.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "name": "Recent buyers",
  "definition": {
    "rules": [
      {
        "field": "last_order_at",
        "op": "gte",
        "value": "2026-05-01"
      }
    ]
  }
}
```

`PATCH /api/v1/workspaces/{workspace_id}/segments/{segment_id}`

- Request chính: cập nhật `definition`, `name` hoặc status-level action.
- Response chính: segment sau cập nhật.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 audience.read_denied` không có quyền đọc lists/segments
- `403 audience.write_denied` không có quyền sửa lists/segments
- `404 audience.list_not_found` không tìm thấy list
- `404 audience.segment_not_found` không tìm thấy segment
- `409 audience.list_name_conflict` tên list đã tồn tại
- `409 audience.segment_name_conflict` tên segment đã tồn tại
- `422 audience.segment_definition_invalid` định nghĩa segment không hợp lệ
- `422 audience.list_membership_payload_invalid` payload membership list không hợp lệ

### Ghi chú contract

- Segment evaluation có thể eventual; API nên phản ánh `processing` nếu recompute chưa xong.

## Resource: Import/export audience

### Actor chính

- Dashboard user
- Operator hỗ trợ dữ liệu

### Auth model

- Yêu cầu permission audience write/read theo chiều import hoặc export.

### API capability

- `POST /api/v1/workspaces/{workspace_id}/audience/imports`
- `GET /api/v1/workspaces/{workspace_id}/audience/imports`
- `GET /api/v1/workspaces/{workspace_id}/audience/imports/{import_id}`
- `POST /api/v1/workspaces/{workspace_id}/audience/exports`
- `GET /api/v1/workspaces/{workspace_id}/audience/exports/{export_id}`

### Hành vi chính

- Bắt đầu import/export dưới dạng background job.
- Theo dõi progress, kết quả và lỗi.

### Request và response chính

`POST /api/v1/workspaces/{workspace_id}/audience/imports`

- Request chính: file reference, source metadata, mapping config, dedupe mode.
- Response chính: import job id và status `accepted` hoặc `queued`.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "source_uri": "s3://bucket/contacts.csv",
  "dedupe_mode": "by_email"
}
```

Response body gợi ý:

```json
{
  "data": {
    "import_id": "imp_123",
    "status": "queued"
  }
}
```

`GET /api/v1/workspaces/{workspace_id}/audience/imports`

- Query gợi ý: `status`, `from`, `to`, `cursor`.
- Response chính: job list view.

Headers:

- `Authorization: Bearer <session_token>`

`GET /api/v1/workspaces/{workspace_id}/audience/imports/{import_id}`

- Response chính: progress, processed counts, error summary, output artifact nếu có.

Headers:

- `Authorization: Bearer <session_token>`

`POST /api/v1/workspaces/{workspace_id}/audience/exports`

- Request chính: filters, selected fields, destination/export format.
- Response chính: export job id và status.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "filters": {
    "list_id": "list_123"
  },
  "format": "csv"
}
```

`GET /api/v1/workspaces/{workspace_id}/audience/exports/{export_id}`

- Response chính: status, artifact availability, expiry nếu có file output.

Headers:

- `Authorization: Bearer <session_token>`

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 audience.import_denied` không có quyền import audience
- `403 audience.export_denied` không có quyền export audience
- `404 audience.import_job_not_found` không tìm thấy import job
- `404 audience.export_job_not_found` không tìm thấy export job
- `409 audience.import_duplicate_submission` job import bị submit trùng semantic
- `422 audience.import_source_invalid` source import không hợp lệ
- `422 audience.export_filter_invalid` filter export không hợp lệ
- `422 audience.export_format_invalid` format export không hợp lệ

### Ghi chú contract

- Import/export nên là job resource thay vì request synchronous dài.

## Resource: Suppression list ở góc nhìn audience

### Actor chính

- Dashboard user
- Operator

### Auth model

- Read cần permission audience read hoặc operations read phù hợp.
- Write cần permission audience/settings write hoặc suppression manage tương đương.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/suppression`
- `POST /api/v1/workspaces/{workspace_id}/suppression`
- `DELETE /api/v1/workspaces/{workspace_id}/suppression/{suppression_id}`

### Hành vi chính

- Xem recipient đang bị suppress trong workspace.
- Block hoặc unsuppress thủ công.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/suppression`

- Query gợi ý: `email`, `scope`, `reason`, `from`, `to`, `cursor`.
- Response chính: suppression entries view.

Headers:

- `Authorization: Bearer <session_token>`

`POST /api/v1/workspaces/{workspace_id}/suppression`

- Request chính: `email`, `scope`, `reason`, optional operator note.
- Response chính: suppression entry vừa được áp dụng.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "email": "alice@example.com",
  "scope": "workspace",
  "reason": "manual_block"
}
```

Response body gợi ý:

```json
{
  "data": {
    "id": "sup_123",
    "email": "alice@example.com",
    "scope": "workspace",
    "reason": "manual_block"
  }
}
```

`DELETE /api/v1/workspaces/{workspace_id}/suppression/{suppression_id}`

- Response chính: unsuppressed state hoặc `204`.

Headers:

- `Authorization: Bearer <session_token>`

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 suppression.read_denied` không có quyền đọc suppression
- `403 suppression.manage_denied` không có quyền sửa suppression
- `404 suppression.entry_not_found` không tìm thấy suppression entry
- `409 suppression.unsuppress_conflict` entry không ở trạng thái có thể unsuppress
- `422 suppression.scope_invalid` scope suppression không hợp lệ
- `422 suppression.reason_invalid` reason suppression không hợp lệ

### Ghi chú contract

- Resource này là user-facing projection của suppression service.

## Resource: Templates

### Actor chính

- Dashboard user
- Developer dùng template trong send flow

### Auth model

- Read/write theo permission template read/write.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/templates`
- `POST /api/v1/workspaces/{workspace_id}/templates`
- `GET /api/v1/workspaces/{workspace_id}/templates/{template_id}`
- `PATCH /api/v1/workspaces/{workspace_id}/templates/{template_id}`
- `POST /api/v1/workspaces/{workspace_id}/templates/{template_id}/publish`
- `GET /api/v1/workspaces/{workspace_id}/templates/{template_id}/versions`

### Hành vi chính

- Quản lý template source.
- Tạo version và publish version để dùng cho send flow.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/templates`

- Query gợi ý: `status`, `q`, `cursor`, `limit`.
- Response chính: template list view.

Headers:

- `Authorization: Bearer <session_token>`

`POST /api/v1/workspaces/{workspace_id}/templates`

- Request chính: `name`, source fields, optional tags/category.
- Response chính: template mới tạo.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "name": "OTP Email",
  "subject": "Your OTP code",
  "html": "<p>Hello {{first_name}}, your OTP is {{otp_code}}</p>"
}
```

Response body gợi ý:

```json
{
  "data": {
    "id": "tpl_123",
    "name": "OTP Email",
    "status": "draft"
  }
}
```

`GET /api/v1/workspaces/{workspace_id}/templates/{template_id}`

- Response chính: template detail, current draft, published version summary.

Headers:

- `Authorization: Bearer <session_token>`

`PATCH /api/v1/workspaces/{workspace_id}/templates/{template_id}`

- Request chính: partial update source hoặc metadata.
- Response chính: template sau cập nhật.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

`POST /api/v1/workspaces/{workspace_id}/templates/{template_id}/publish`

- Request chính: optional target version hoặc publish note.
- Response chính: published version summary.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

`GET /api/v1/workspaces/{workspace_id}/templates/{template_id}/versions`

- Response chính: danh sách versions và publish state.

Headers:

- `Authorization: Bearer <session_token>`

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 template.read_denied` không có quyền đọc template
- `403 template.write_denied` không có quyền sửa/publish template
- `404 template.not_found` không tìm thấy template
- `404 template.version_not_found` không tìm thấy template version
- `409 template.publish_conflict` xung đột publish state
- `409 template.version_conflict` version conflict khi cập nhật
- `422 template.source_invalid` source template không hợp lệ
- `422 template.publish_payload_invalid` payload publish không hợp lệ

### Ghi chú contract

- Template API nên làm rõ `draft`, `published_version`, `render_snapshot_ready` hoặc trạng thái tương đương.

## Resource: Render và preview

### Actor chính

- Dashboard user
- Developer

### Auth model

- Thường yêu cầu template read/write tùy policy; với public API render thử nên tách riêng nếu có.

### API capability

- `POST /api/v1/workspaces/{workspace_id}/templates/{template_id}/preview`
- `POST /api/v1/workspaces/{workspace_id}/render`

### Hành vi chính

- Render preview cho template.
- Render thử với payload mẫu trước khi gửi.

### Request và response chính

`POST /api/v1/workspaces/{workspace_id}/templates/{template_id}/preview`

- Request chính: render payload mẫu, optional target version.
- Response chính: rendered HTML/text, warnings, asset issues nếu có.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "template_data": {
    "first_name": "Alice",
    "otp_code": "492113"
  }
}
```

Response body gợi ý:

```json
{
  "data": {
    "subject": "Your OTP code",
    "html": "<p>Hello Alice, your OTP is 492113</p>",
    "warnings": []
  }
}
```

`POST /api/v1/workspaces/{workspace_id}/render`

- Request chính: template ref hoặc raw content + payload.
- Response chính: render result độc lập, không tạo message.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "template_id": "tpl_123",
  "template_data": {
    "first_name": "Alice",
    "otp_code": "492113"
  }
}
```

Response body gợi ý:

```json
{
  "data": {
    "html": "<p>Hello Alice, your OTP is 492113</p>"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 template.render_denied` không có quyền render template
- `404 template.not_found` không tìm thấy template
- `422 template.render_payload_invalid` payload render không hợp lệ
- `422 template.render_context_invalid` context render không hợp lệ

### Ghi chú contract

- Render API là capability đồng hành cùng template, không thay thế send API.
