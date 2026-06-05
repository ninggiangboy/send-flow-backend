# Database Map

Bộ tài liệu này mô tả **database design ở mức logical schema** cho `send-flow`, dựa trên kiến trúc hiện tại. Nó không phải migration cuối cùng, nhưng là bản đồ để team triển khai PostgreSQL, Redis và ClickHouse nhất quán.

## Cách đọc nhanh

Graph tổng và capability trace nằm ở [../00-doc-graph.md](../00-doc-graph.md).

| Cần trả lời | Đọc file |
|---|---|
| Bảng lõi cho identity, access, sender, settings là gì? | `01-core-identity-and-access.md` |
| Audience, template và content lưu thế nào? | `02-audience-and-content.md` |
| Campaign, message, suppression, webhook, tracking, outbox lưu thế nào? | `03-delivery-and-events.md` |
| Analytics, audit, queue/DLQ, operational state lưu thế nào? | `04-analytics-and-operations.md` |

## Vai trò từng loại storage

| Storage | Vai trò |
|---|---|
| PostgreSQL | Source of truth cho business state và transactional outbox |
| Redis | Cache, session, idempotency TTL, rate limit, lock ngắn hạn, runtime state có thể rebuild |
| ClickHouse | Event analytics volume lớn và reporting aggregate |

## Module ownership theo nhóm schema

Database docs hiện nhóm table theo khu vực sản phẩm. Khi triển khai migration và repository, dùng module owner dưới đây để xác định ai được ghi bảng transaction chính.

| Table / schema group | Module owner chính |
|---|---|
| `users`, `external_auth_accounts`, `workspaces`, `workspace_memberships`, `workspace_invitations`, `sessions` | `identity` |
| `roles`, `membership_roles`, `permission_registry`, `api_keys` | `access` |
| `sender_domains`, `sender_domain_dns_records` | `sender` |
| `customer_webhooks`, `customer_webhook_deliveries` | `webhooks` |
| `workspace_settings` | `identity` hoặc `settings` nếu tách sau |
| `audit_entries` | `audit` |
| `contacts`, `audience_lists`, `audience_list_memberships`, `segments`, import/export jobs | `audience` |
| `templates`, `template_versions`, `template_render_snapshots` | `content` |
| `campaigns`, `campaign_message_candidates` | `campaign` |
| `transactional_send_requests`, `messages`, `delivery_attempts`, `retry_states` | `delivery` |
| `suppression_entries` | `suppression` |
| `provider_webhook_events`, `normalized_provider_events` | `ingestion` |
| `tracking_links`, `tracking_events` | `tracking` |
| PostgreSQL analytics projections và ClickHouse analytics facts/aggregates | `analytics` |
| `outbox_events`, `processed_event_markers`, `dead_letter_records`, `replay_jobs`, runtime operational state | `operations` |

## Nguyên tắc database

- Mỗi module/context sở hữu bảng giao dịch chính của mình.
- Product data mặc định phải có `workspace_id`.
- Unique constraint thường scope theo `workspace_id`.
- Redis không giữ business state không thể mất.
- ClickHouse không dùng cho workflow state hoặc business constraint.
