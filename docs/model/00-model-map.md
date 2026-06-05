# Model Map

Bộ tài liệu này mô tả **business model** của `send-flow`: bounded context, aggregate, entity, value object, read model và event-facing model quan trọng.

Mục tiêu của nó là giúp người đọc trả lời:

- Sản phẩm có những model lõi nào?
- Model nào là source of truth, model nào là projection?
- Context nào sở hữu lifecycle và invariant của từng model?

## Cách đọc nhanh

Graph tổng và capability trace nằm ở [../00-doc-graph.md](../00-doc-graph.md).

| Cần trả lời | Đọc file |
|---|---|
| User, workspace, membership, role, API key, sender có model gì? | `01-identity-and-access.md` |
| Contact, list, segment, template có model gì? | `02-audience-and-content.md` |
| Campaign, message, suppression, webhook, tracking có model gì? | `03-messaging-and-delivery.md` |
| Analytics, audit, operations projection có model gì? | `04-events-analytics-and-operations.md` |

## Phân loại model trong hệ thống

| Loại model | Mục đích |
|---|---|
| Aggregate / entity giao dịch | Giữ invariant và lifecycle nghiệp vụ |
| Value object | Chuẩn hóa khái niệm và validation ổn định |
| Integration event model | Contract để context/runtime khác consume |
| Read model / projection | Phục vụ dashboard, logs, analytics, operations |
| Operational model | Phục vụ retry, queue, DLQ, replay, health, throttling |

## Module ownership theo nhóm model

Model docs được nhóm theo khu vực nghiệp vụ để dễ đọc. Owner module mới là nơi sở hữu lifecycle, invariant và transaction state chính.

| Model group | Module owner chính | Model tiêu biểu |
|---|---|---|
| User, external auth, workspace, membership, session | `identity` | `User`, `Workspace`, `WorkspaceMembership`, `Session` |
| Role, permission, API key | `access` | `Role`, `PermissionRegistry`, `APIKey` |
| Sender domain và DNS status | `sender` | `SenderDomain`, `DomainDNSRecordStatus` |
| Contact, list, segment, import/export | `audience` | `Contact`, `AudienceList`, `Segment`, `AudienceImportJob` |
| Template và render snapshot | `content` | `Template`, `TemplateVersion`, `RenderedTemplateSnapshot` |
| Campaign và candidate | `campaign` | `Campaign`, `CampaignMessageCandidate` |
| Transactional send, message, attempt, retry | `delivery` | `TransactionalSendRequest`, `Message`, `DeliveryAttempt`, `RetryState` |
| Provider webhook raw/normalized event | `ingestion` | `ProviderWebhookEvent`, `NormalizedProviderEvent` |
| Suppression và unsubscribe | `suppression` | `SuppressionEntry`, `SendEligibilityDecision` |
| Tracking link/event | `tracking` | `TrackingLink`, `TrackingEvent` |
| Customer webhook config/delivery | `webhooks` | `CustomerWebhookConfig`, `CustomerWebhookDelivery` |
| Analytics fact/projection | `analytics` | `EmailEventFact`, `CampaignDeliverySummary`, `DashboardOverview` |
| Operational state | `operations` | `OutboxRecord`, `ProcessedEventMarker`, `DeadLetterRecord`, `ReplayJob` |
| Audit trail | `audit` | `AuditEntry` |

## Nguyên tắc đọc model

- Một model chỉ có một owner context cho transaction state chính.
- Context khác có thể có bản sao read model nhưng không sở hữu lifecycle nguồn.
- Với product data, `workspace_id` là boundary mặc định.
- Những model dưới đây là blueprint dựa trên architecture hiện tại; chúng giúp team thiết kế code và database nhất quán khi triển khai.
