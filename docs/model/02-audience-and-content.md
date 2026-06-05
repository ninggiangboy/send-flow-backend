# Audience And Content Model

## Mục tiêu nhóm model

Nhóm model này sở hữu dữ liệu người nhận và nội dung dùng cho các flow gửi email.

## Module ownership

| Model | Module owner chính |
|---|---|
| `Contact`, `AudienceList`, `AudienceListMembership`, `Segment`, `AudienceImportJob`, `AudienceExportJob`, `SendEligibilityDecision` | `audience` |
| `Template`, `TemplateVersion`, `RenderedTemplateSnapshot` | `content` |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/02-audience-and-content.md` |
| API contract | `../api/02-audience-and-content.md` |
| Database schema | `../database/02-audience-and-content.md` |
| Module catalog | `../modules/03-module-catalog.md` |

## Aggregate / entity chính

### `Contact`

- Vai trò: recipient identity trong một workspace.
- Thuộc tính chính: `id`, `workspace_id`, `email_normalized`, profile fields, status, timestamps.
- Invariant chính:
  - Contact uniqueness thường được scope theo workspace.

### `AudienceList`

- Vai trò: nhóm contact tĩnh do user quản lý.
- Thuộc tính chính: `id`, `workspace_id`, `name`, metadata, timestamps.

### `AudienceListMembership`

- Vai trò: mapping contact vào list.
- Thuộc tính chính: `workspace_id`, `list_id`, `contact_id`, timestamps.

### `Segment`

- Vai trò: audience động theo tiêu chí.
- Thuộc tính chính: `id`, `workspace_id`, `name`, `definition`, `status`, timestamps.
- Invariant chính:
  - Segment definition phải parse/evaluate được bởi engine tương ứng.

### `AudienceImportJob`

- Vai trò: job import dữ liệu audience.
- Thuộc tính chính: `id`, `workspace_id`, `source`, `status`, progress, error summary, timestamps.

### `AudienceExportJob`

- Vai trò: job export dữ liệu audience.
- Thuộc tính chính: `id`, `workspace_id`, `filters`, `status`, artifact reference, timestamps.

### `Template`

- Vai trò: email template gốc.
- Thuộc tính chính: `id`, `workspace_id`, `name`, `status`, timestamps.

### `TemplateVersion`

- Vai trò: version bất biến hoặc gần bất biến của template.
- Thuộc tính chính: `id`, `template_id`, `workspace_id`, `version_number`, source/content, `published_at`.

### `RenderedTemplateSnapshot`

- Vai trò: snapshot render reproducible dùng cho send flow.
- Thuộc tính chính: `template_version_id`, render metadata, output html/text, timestamps.

## Value object chính

- `ContactID`
- `ListID`
- `SegmentID`
- `TemplateID`
- `TemplateVersionID`
- `SegmentDefinition`
- `RenderPayload`

## Capability model quan trọng

### `SendEligibilityDecision`

- Vai trò: kết quả kiểm tra recipient có được gửi hay không.
- Input chính: contact, workspace, message type, suppression signal.
- Output chính: `eligible`, `reason`, optional scope metadata.

## Event model chính

- `contact imported`
- `segment changed`
- `audience import completed`
- `template published`
- `rendering failed`

## Read model / projection quan trọng

- Contact list view
- Segment membership view
- Audience import/export job status view
- Template library view
- Template preview/render result view
