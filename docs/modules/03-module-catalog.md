# Module Catalog

File này là catalog module đề xuất cho `send-flow`: mục đích, scope, dữ liệu sở hữu, public API và cách giao tiếp.

Tên module là gợi ý triển khai trong `internal/modules/<module>`. Trong giai đoạn đầu có thể gộp một số module nhỏ, nhưng vẫn nên giữ boundary logic và contract như bên dưới.

Ghi chú: các file `docs/database/*` hiện đang nhóm schema theo khu vực sản phẩm để dễ đọc. Phần `Owned data` trong file này mô tả owner module mục tiêu; vì vậy một bảng có thể đang nằm trong file database khác nhưng vẫn thuộc owner module được ghi ở đây.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Module map | `00-module-map.md` |
| Bounded context rule | `01-bounded-contexts.md` |
| Public API/giao tiếp module | `02-communication-and-public-api.md` |
| API ownership | `../api/00-api-map.md` |
| Model ownership | `../model/00-model-map.md` |
| Database ownership | `../database/00-database-map.md` |

## `identity`

### Mục đích

Quản lý identity toàn cục, tenant boundary và session context.

### Scope

Trong scope:

- User registration/login/social account link.
- Workspace lifecycle.
- Membership và invitation.
- Session/current user context.

Ngoài scope:

- Delivery permission decision chi tiết của từng feature.
- Sender DNS verification.
- Campaign hoặc audience business rule.

### Owned data

- `users`
- `external_auth_accounts`
- `workspaces`
- `workspace_memberships`
- `workspace_invitations`
- `sessions`

### Public API

Commands:

- `RegisterUser`
- `Login`
- `LinkExternalAuthAccount`
- `CreateWorkspace`
- `InviteWorkspaceMember`
- `AcceptWorkspaceInvitation`
- `SwitchActiveWorkspace`
- `RevokeSession`

Queries:

- `GetCurrentSession`
- `GetWorkspaceMembership`
- `ListWorkspaceMembers`
- `ResolveWorkspaceContext`

Events:

- `identity.user.registered.v1`
- `identity.user.external_account_linked.v1`
- `identity.workspace.created.v1`
- `identity.workspace.member_invited.v1`
- `identity.workspace.member_joined.v1`

Communication:

- API runtime gọi command/query trực tiếp.
- Module khác đọc workspace/session context qua public query hoặc request context đã resolve.
- Audit/analytics consume event.

## `access`

### Mục đích

Quản lý permission evaluation, role và API key scope cho Dashboard API và Public API.

Giai đoạn đầu có thể gộp package này vào `identity`, nhưng nên giữ ngôn ngữ `access` riêng để không trộn auth với authorization.

### Scope

Trong scope:

- Role và permission registry.
- Membership role assignment.
- Effective permission calculation.
- API key lifecycle và scope.

Ngoài scope:

- Session issuance, nếu đã nằm ở `identity`.
- Business decision như campaign có được schedule không.

### Owned data

- `roles`
- `membership_roles`
- `permission_registry`
- `api_keys`

### Public API

Commands:

- `CreateRole`
- `UpdateRolePermissions`
- `AssignMemberRoles`
- `CreateAPIKey`
- `RevokeAPIKey`

Queries:

- `CheckPermission`
- `ResolveEffectivePermissions`
- `AuthenticateAPIKey`
- `CheckAPIKeyScope`

Events:

- `access.role.permissions_changed.v1`
- `access.api_key.created.v1`
- `access.api_key.revoked.v1`

Communication:

- API middleware dùng public query để resolve permission/API key.
- Module use case nhận actor context đã được runtime resolve, không tự parse token.
- Audit consume access change event.

## `sender`

### Mục đích

Sở hữu sending identity của workspace: sender domain, DNS readiness và trạng thái verified.

### Scope

Trong scope:

- Sender domain lifecycle.
- DNS record expected/current status.
- Domain verification check.
- Sender readiness public decision.

Ngoài scope:

- Provider send execution.
- Campaign scheduling.
- Suppression/compliance decision.

### Owned data

- `sender_domains`
- `domain_dns_record_statuses`

### Public API

Commands:

- `CreateSenderDomain`
- `StartSenderDomainVerification`
- `RefreshSenderDomainDNSStatus`
- `MarkSenderDomainVerified`
- `DisableSenderDomain`

Queries:

- `GetSenderDomain`
- `GetSenderReadiness`
- `ListSenderDomains`

Events:

- `sender.domain.created.v1`
- `sender.domain.verified.v1`
- `sender.domain.disabled.v1`

Communication:

- `campaign` và `delivery` gọi `GetSenderReadiness` khi cần quyết định ngay.
- `analytics` và `audit` consume sender event.

## `audience`

### Mục đích

Sở hữu recipient identity trong workspace và các cách chọn audience.

### Scope

Trong scope:

- Contact lifecycle.
- Audience list và membership.
- Segment definition và evaluation readiness.
- Import/export audience job.
- Audience selection read model cho campaign.

Ngoài scope:

- Template/content rendering.
- Campaign schedule.
- Delivery suppression final decision, dù audience có thể hiển thị suppression status.

### Owned data

- `contacts`
- `audience_lists`
- `audience_list_memberships`
- `segments`
- `audience_import_jobs`
- `audience_export_jobs`

### Public API

Commands:

- `CreateContact`
- `UpdateContact`
- `ArchiveContact`
- `CreateAudienceList`
- `AddContactToList`
- `CreateSegment`
- `StartAudienceImport`
- `StartAudienceExport`

Queries:

- `GetContact`
- `ListContacts`
- `ResolveAudienceSelection`
- `EstimateAudienceSize`
- `GetImportJobStatus`

Events:

- `audience.contact.created.v1`
- `audience.contact.updated.v1`
- `audience.segment.changed.v1`
- `audience.import.completed.v1`

Communication:

- `campaign` gọi query selection/estimate khi tạo hoặc schedule campaign.
- `analytics` consume contact/segment/import event nếu cần.
- `suppression` có thể publish event để audience UI update projection, nhưng không update contact owner trực tiếp.

## `content`

### Mục đích

Sở hữu template, template version và khả năng render nội dung email.

### Scope

Trong scope:

- Template lifecycle.
- Template version publish.
- Preview/render validation.
- Rendered snapshot dùng cho send flow.

Ngoài scope:

- Chọn audience.
- Gửi email qua provider.
- Tracking event sau khi email được gửi.

### Owned data

- `templates`
- `template_versions`
- `rendered_template_snapshots`

### Public API

Commands:

- `CreateTemplate`
- `UpdateTemplateDraft`
- `PublishTemplateVersion`
- `ArchiveTemplate`
- `RenderTemplatePreview`

Queries:

- `GetTemplate`
- `GetPublishedTemplateVersion`
- `ValidateTemplateRenderable`
- `RenderForMessage`

Events:

- `content.template.created.v1`
- `content.template.published.v1`
- `content.template.render_failed.v1`

Communication:

- `campaign` hỏi template readiness khi schedule.
- `delivery` gọi render public API khi chuẩn bị message.
- `analytics` consume render failure/published event.

## `campaign`

### Mục đích

Sở hữu kế hoạch gửi marketing/campaign: cấu hình campaign, schedule và message candidate.

### Scope

Trong scope:

- Campaign lifecycle và state transition.
- Audience/template/sender reference validation ở thời điểm schedule.
- Schedule campaign.
- Sinh campaign message candidate hoặc publish intent để delivery nhận.

Ngoài scope:

- Delivery attempt/provider execution.
- Contact lifecycle.
- Template lifecycle.
- Sender DNS verification.

### Owned data

- `campaigns`
- `campaign_message_candidates`

### Public API

Commands:

- `CreateCampaign`
- `UpdateCampaignDraft`
- `ScheduleCampaign`
- `CancelCampaign`
- `PauseCampaign`
- `ResumeCampaign`

Queries:

- `GetCampaign`
- `ListCampaigns`
- `GetCampaignStatus`
- `ListCampaignCandidates`

Events:

- `campaign.created.v1`
- `campaign.scheduled.v1`
- `campaign.cancelled.v1`
- `campaign.paused.v1`
- `campaign.resumed.v1`
- `campaign.message_candidate.created.v1`

Communication:

- Synchronous read từ `audience`, `content`, `sender` để validate schedule.
- Publish `campaign.scheduled.v1` cho `delivery` tạo message/queue.
- `analytics` consume campaign event để build dashboard.

## `delivery`

### Mục đích

Sở hữu message lifecycle, queueing, provider routing, send attempt, retry và delivery state.

### Scope

Trong scope:

- Transactional send request acceptance.
- Message creation và state machine.
- Delivery attempt.
- Provider routing decision.
- Retry state.
- Delivery logs projection nguồn.

Ngoài scope:

- Provider webhook raw ingest.
- Suppression policy source of truth.
- Analytics aggregation dài hạn.
- Customer webhook outbound retry.

### Owned data

- `transactional_send_requests`
- `messages`
- `delivery_attempts`
- `retry_states`

### Public API

Commands:

- `AcceptTransactionalSend`
- `QueueCampaignMessages`
- `QueueMessage`
- `StartDeliveryAttempt`
- `MarkMessageAccepted`
- `MarkMessageDelivered`
- `MarkMessageBounced`
- `MarkMessageComplained`
- `ScheduleRetry`
- `MoveMessageToDLQ`

Queries:

- `GetMessage`
- `GetMessageByProviderMessageID`
- `ListMessageLogs`
- `GetQueueState`
- `GetRetryState`

Events:

- `delivery.transactional_send.accepted.v1`
- `delivery.message.queued.v1`
- `delivery.message.accepted.v1`
- `delivery.message.delivered.v1`
- `delivery.message.bounced.v1`
- `delivery.message.complained.v1`
- `delivery.message.retry_scheduled.v1`

Communication:

- Consume `campaign.scheduled.v1`.
- Gọi `content.RenderForMessage`, `sender.GetSenderReadiness`, `suppression.CheckSuppression`.
- Publish delivery events cho `analytics`, `suppression`, `webhooks`.
- Provider adapter nằm sau port của module, không lộ ra module khác.

## `ingestion`

### Mục đích

Nhận, xác thực, lưu raw provider webhook và normalize thành event nội bộ idempotent.

### Scope

Trong scope:

- Provider signature verification.
- Raw provider event persistence.
- Dedupe theo provider event id.
- Normalize event taxonomy.
- Publish normalized provider event.

Ngoài scope:

- Message state transition cuối cùng.
- Suppression decision.
- Analytics aggregation.

### Owned data

- `provider_webhook_events`
- `normalized_provider_events`

### Public API

Commands:

- `IngestProviderWebhook`
- `NormalizeProviderEvent`
- `MarkProviderEventProcessed`

Queries:

- `GetRawProviderEvent`
- `GetNormalizedProviderEvent`

Events:

- `ingestion.provider_webhook.received.v1`
- `ingestion.provider_event.normalized.v1`

Communication:

- API webhook endpoint gọi command ingest và trả ack nhanh.
- `delivery`, `suppression`, `tracking`, `analytics` consume normalized event.

## `suppression`

### Mục đích

Sở hữu quyết định chặn gửi recipient theo scope và reason.

### Scope

Trong scope:

- Global/workspace/list/category suppression.
- Hard bounce, complaint, unsubscribe, manual block.
- Idempotent suppress/unsuppress.
- Public suppression lookup.

Ngoài scope:

- Contact profile lifecycle.
- Delivery attempt state.
- Tracking redirect.

### Owned data

- `suppression_entries`

### Public API

Commands:

- `SuppressRecipient`
- `UnsuppressRecipient`
- `HandleUnsubscribe`
- `HandleComplaint`
- `HandleHardBounce`

Queries:

- `CheckSuppression`
- `ListSuppressionEntries`
- `GetSuppressionReason`

Events:

- `suppression.recipient_suppressed.v1`
- `suppression.recipient_unsuppressed.v1`

Communication:

- `delivery` gọi `CheckSuppression` trước khi gửi.
- Consume `delivery.message.bounced.v1`, `delivery.message.complained.v1` hoặc normalized provider event.
- Publish event cho `analytics`, `webhooks`, audience UI projection.

## `tracking`

### Mục đích

Sở hữu tracking link, open/click endpoint và recipient behavior event.

### Scope

Trong scope:

- Generate tracking link.
- Open pixel endpoint.
- Click redirect endpoint.
- Tracking event persistence/dedupe.

Ngoài scope:

- Analytics aggregation.
- Suppression unsubscribe decision, trừ endpoint unsubscribe được route sang `suppression`.
- Message delivery state source of truth.

### Owned data

- `tracking_links`
- `tracking_events`

### Public API

Commands:

- `CreateTrackingLink`
- `RecordEmailOpened`
- `RecordLinkClicked`

Queries:

- `ResolveTrackingLink`
- `GetTrackingEvent`

Events:

- `tracking.email_opened.v1`
- `tracking.link_clicked.v1`

Communication:

- Recipient endpoint gọi tracking command và redirect nếu cần.
- `analytics` và `webhooks` consume tracking event.

## `webhooks`

### Mục đích

Sở hữu outbound customer webhook config và delivery/retry tới downstream system của tenant.

### Scope

Trong scope:

- Customer webhook config.
- Event subscription.
- Signing secret.
- Outbound webhook delivery attempt.
- Retry/DLQ của webhook delivery.

Ngoài scope:

- Provider webhook ingress.
- Email provider send execution.
- Analytics dashboard.

### Owned data

- `customer_webhooks`
- `customer_webhook_deliveries`

### Public API

Commands:

- `CreateWebhookConfig`
- `UpdateWebhookConfig`
- `DisableWebhookConfig`
- `DeliverCustomerWebhook`
- `RetryCustomerWebhookDelivery`

Queries:

- `ListWebhookConfigs`
- `GetWebhookDeliveryHistory`
- `GetSubscribedWebhookTargets`

Events:

- `webhooks.config.created.v1`
- `webhooks.delivery.succeeded.v1`
- `webhooks.delivery.failed.v1`

Communication:

- Consume delivery/suppression/tracking events.
- Does not mutate producer module state.
- Publishes delivery result for audit/operations visibility.

## `analytics`

### Mục đích

Sở hữu event facts, reporting projections và dashboard analytics.

### Scope

Trong scope:

- Email event fact ingestion.
- Campaign delivery summary.
- Recipient domain/provider stats.
- Dashboard overview.
- Projection freshness metadata.

Ngoài scope:

- Business state transition của delivery/campaign.
- Operational retry decision.
- Source-of-truth command state.

### Owned data

- `email_event_facts`
- `campaign_delivery_summaries`
- `recipient_domain_hourly_stats`
- `provider_delivery_stats`
- `dashboard_overviews`

### Public API

Commands:

- `IngestEmailEventFact`
- `RebuildCampaignSummary`
- `BackfillAnalyticsWindow`

Queries:

- `GetDashboardOverview`
- `GetCampaignAnalytics`
- `GetProviderStats`
- `GetRecipientDomainStats`

Events:

- `analytics.projection.updated.v1`
- `analytics.backfill.completed.v1`

Communication:

- Consume event từ `campaign`, `delivery`, `suppression`, `tracking`, `ingestion`.
- Dashboard API đọc analytics query.
- Không là source of truth cho workflow state.

## `operations`

### Mục đích

Sở hữu capability vận hành event pipeline và runtime: outbox visibility, processed markers, DLQ, replay, rate limit/circuit state view.

Một phần implementation có thể nằm ở `internal/platform`, nhưng operational workflow nên có owner rõ.

### Scope

Trong scope:

- Outbox publisher state/view.
- Processed event marker.
- DLQ record.
- Replay job.
- Rate limit/circuit breaker state visibility.
- Health/readiness operational view.

Ngoài scope:

- Business decision của từng module.
- Analytics product dashboard.

### Owned data

- `outbox_records`
- `processed_event_markers`
- `dead_letter_records`
- `replay_jobs`
- `rate_limit_states`
- `circuit_breaker_states`

### Public API

Commands:

- `CreateReplayJob`
- `MoveDLQRecordForReplay`
- `MarkOutboxRecordStuck`
- `OpenCircuitBreaker`
- `CloseCircuitBreaker`

Queries:

- `GetOutboxLag`
- `ListDLQRecords`
- `GetReplayJobStatus`
- `GetRuntimeHealth`
- `GetQueueLag`

Events:

- `operations.dlq_record.created.v1`
- `operations.replay_job.created.v1`
- `operations.replay_job.completed.v1`
- `operations.circuit_breaker.opened.v1`

Communication:

- Runtime/worker writes operational state through platform/operations APIs.
- Operator dashboard reads queries.
- Replay emits original events or replay commands with explicit audit trail.

## `audit`

### Mục đích

Ghi lại hành động nhạy cảm để review, compliance và điều tra.

### Scope

Trong scope:

- Audit entry append-only.
- Actor/target/action summary.
- Audit query theo workspace/time/action.

Ngoài scope:

- Authorization decision.
- Business rollback.
- Analytics event fact.

### Owned data

- `audit_entries`

### Public API

Commands:

- `RecordAuditEntry`

Queries:

- `SearchAuditEntries`
- `GetAuditEntry`

Events:

- `audit.entry.recorded.v1`

Communication:

- Consume hoặc được gọi bởi module có action nhạy cảm.
- Không block critical path nếu audit sink temporary fail; dùng outbox/async cho action quan trọng.

## `notification`

### Mục đích

Gửi notification nội bộ của sản phẩm như welcome email, invite email hoặc system alert.

Module này khác `delivery`: `delivery` là sản phẩm gửi email cho tenant, còn `notification` là email/thông báo vận hành của chính `send-flow`.

### Scope

Trong scope:

- Welcome/invite/system notification.
- Notification template nội bộ.
- Retry gửi notification nội bộ.

Ngoài scope:

- Public transactional send của tenant.
- Campaign send.
- Customer webhook.

### Owned data

- `notification_messages`
- `notification_attempts`

### Public API

Commands:

- `SendWelcomeEmail`
- `SendWorkspaceInvitationEmail`
- `SendSystemAlert`

Queries:

- `GetNotificationStatus`

Events:

- `notification.message.queued.v1`
- `notification.message.sent.v1`
- `notification.message.failed.v1`

Communication:

- Consume `identity.user.registered.v1` hoặc `identity.workspace.member_invited.v1`.
- Có thể dùng provider adapter riêng hoặc đi qua hạ tầng gửi email nội bộ, nhưng không đi qua tenant `delivery` path nếu điều đó làm lẫn billing/quota/logs.
