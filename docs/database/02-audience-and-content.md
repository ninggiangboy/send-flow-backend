# Audience And Content Database

## Mục tiêu nhóm schema

Nhóm schema này lưu audience, template và các job nội dung/dữ liệu liên quan.

## Module ownership

| Table | Module owner chính |
|---|---|
| `contacts`, `audience_lists`, `audience_list_memberships`, `segments`, `audience_import_jobs`, `audience_export_jobs` | `audience` |
| `templates`, `template_versions`, `template_render_snapshots` | `content` |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/02-audience-and-content.md` |
| API contract | `../api/02-audience-and-content.md` |
| Business model | `../model/02-audience-and-content.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| Infrastructure | `../architecture/06-infrastructure.md` |

## PostgreSQL tables

### `contacts`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | contact id |
| `workspace_id` | workspace owner |
| `email_normalized` | email chuẩn hóa |
| profile columns | dữ liệu contact |
| `status` | trạng thái contact |
| timestamps | lifecycle metadata |

Ràng buộc gợi ý:

- unique: `(workspace_id, email_normalized)`

### `audience_lists`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | list id |
| `workspace_id` | workspace owner |
| `name` | tên list |
| metadata | cấu hình phụ |
| timestamps | lifecycle metadata |

Ràng buộc gợi ý:

- unique: `(workspace_id, name)`

### `audience_list_memberships`

| Cột chính | Ý nghĩa |
|---|---|
| `workspace_id` | workspace owner |
| `list_id` | list target |
| `contact_id` | contact member |
| timestamps | lifecycle metadata |

Ràng buộc gợi ý:

- unique: `(workspace_id, list_id, contact_id)`

### `segments`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | segment id |
| `workspace_id` | workspace owner |
| `name` | tên segment |
| `definition_json` | định nghĩa filter/rule |
| `status` | draft/ready/recomputing |
| timestamps | lifecycle metadata |

### `audience_import_jobs`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | import job id |
| `workspace_id` | workspace owner |
| `source_uri` hoặc equivalent | nguồn dữ liệu |
| `status` | queued/running/completed/failed |
| `processed_count` | tiến độ |
| `error_summary` | lỗi chính |
| timestamps | lifecycle metadata |

### `audience_export_jobs`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | export job id |
| `workspace_id` | workspace owner |
| `filters_json` | bộ lọc export |
| `status` | queued/running/completed/failed |
| `artifact_uri` | nơi lưu kết quả |
| timestamps | lifecycle metadata |

### `templates`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | template id |
| `workspace_id` | workspace owner |
| `name` | tên template |
| `status` | draft/active/archived |
| `current_version_id` | version đang chọn nếu cần |
| timestamps | lifecycle metadata |

### `template_versions`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | version id |
| `workspace_id` | workspace owner |
| `template_id` | template owner |
| `version_number` | số version |
| `source_html` / `source_text` / metadata | nội dung source |
| `published_at` | thời điểm publish |
| timestamps | lifecycle metadata |

Ràng buộc gợi ý:

- unique: `(template_id, version_number)`

### `template_render_snapshots`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | snapshot id |
| `workspace_id` | workspace owner |
| `template_version_id` | version nguồn |
| `render_input_hash` | fingerprint input |
| rendered content columns | output render |
| timestamps | lifecycle metadata |

## Redis keys

- `cache:audience:segment-membership:v1:<workspace_id>:<segment_id>`
- `cache:template:preview:v1:<workspace_id>:<template_id>:<payload_hash>`

## Index và truy vấn quan trọng

- `contacts(workspace_id, email_normalized)`
- `audience_list_memberships(workspace_id, list_id, contact_id)`
- `segments(workspace_id, status)`
- `audience_import_jobs(workspace_id, created_at desc)`
- `templates(workspace_id, name)`
- `template_versions(template_id, version_number desc)`
