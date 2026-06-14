# Production Readiness Matrix

This matrix is the baseline production inventory for Sendflow modules. `Owner` is intentionally `TBD` until team ownership is assigned.

| Module | Owner | Critical flows | Dependencies | Data stores | Public APIs | Async events | Operational risks |
|---|---|---|---|---|---|---|---|
| identity | TBD | Signup, login, refresh, MFA, OAuth, workspace membership | Postgres, Redis, email, audit | users, sessions, workspaces, memberships, invitations | Auth and workspace user APIs | identity user/workspace/member events | Account takeover, token/session replay, workspace data leakage |
| access | TBD | API key auth, scope checks, role permission updates | Postgres, identity, audit | roles, membership_roles, permission_registry, api_keys | API key and permission APIs | access role/API key events | Over-broad scopes, key leakage, stale permission decisions |
| sender | TBD | Sender domain create, DNS verification, disable | Postgres, DNS resolver, identity access | sender_domains, domain_dns_record_statuses | Sender domain APIs | sender domain events | Bad DNS timeouts, unverified sender use, spoofing risk |
| audience | TBD | Contact/list/segment CRUD, import/export jobs | Postgres, object storage, workers, suppression | contacts, lists, segments, import/export jobs | Audience APIs | audience contact/import/export events | Large imports, duplicate contacts, consent/suppression drift |
| content | TBD | Template CRUD, publish, render, preview | Postgres, identity access | templates, template_versions, render snapshots | Template and render APIs | content template/render events | Unsafe rendering, mutable campaign content, render failures |
| campaign | TBD | Create, schedule, pause/resume/cancel, candidate handoff | Postgres, audience, content, sender, delivery, outbox | campaigns, campaign_message_candidates | Campaign APIs | campaign lifecycle events | Duplicate enqueue, invalid lifecycle transitions, oversized campaigns |
| delivery | TBD | Transactional send, queue, provider send, retry, state transitions | Postgres, Redis, email provider, content, sender, suppression, outbox | transactional_send_requests, messages, attempts, retry state | Delivery and transactional APIs | delivery message events | Duplicate sends, retry storms, provider outage, suppression bypass |
| ingestion | TBD | Provider webhook verify, persist, dedupe, normalize | Postgres, provider signature verifier, outbox | provider_webhook_events, normalized_provider_events | Provider webhook APIs | ingestion provider events | Signature bypass, duplicate/out-of-order events, slow webhook responses |
| tracking | TBD | Open pixel, click redirect, unsubscribe | Postgres, suppression, delivery resolver | tracking_links, tracking_events | Tracking endpoints | tracking open/click events | Open redirects, token leakage, bot traffic, unauthenticated abuse |
| suppression | TBD | Manual suppress/unsuppress, unsubscribe, complaint, bounce | Postgres, identity access | suppression_entries | Suppression APIs | suppression events | Sending to suppressed recipients, workspace isolation errors |
| analytics | TBD | Event fact ingest, projections, ClickHouse reports, sync status | Postgres, ClickHouse, workers | email_event_facts, projections, ClickHouse facts | Analytics APIs | analytics projection/backfill events | Stale reports, ClickHouse outage, duplicate facts |
| webhooks | TBD | Customer webhook config, signing, delivery, retry | Postgres, workers, HTTP client, audit | customer_webhooks, webhook_deliveries | Webhook config/delivery APIs | webhook config/delivery events | SSRF, retry storms, secret exposure, noisy customer endpoints |
| notification | TBD | Internal product email creation, send, retry | Postgres, email provider, workers | notification_messages, notification_attempts | Notification APIs | notification message events | Secret leakage in email, retry loops, disabled provider behavior |
| audit | TBD | Append sensitive action records, search logs | Postgres, identity access | audit_entries | Audit log APIs | audit entry events | Missing sensitive actions, PII overexposure, retention gaps |
| operations | TBD | Outbox visibility, DLQ, replay jobs, runtime health | Postgres, workers, Kafka/outbox | outbox_records, processed markers, DLQ, replay jobs | Operations APIs | operations DLQ/replay events | Unsafe replay, poor incident visibility, stuck outbox rows |
| platform | TBD | Config, migrations, DB/Redis/Kafka/email/storage clients, observability | All infrastructure services | Shared infra state and external stores | Health, readiness, metrics, OpenAPI | Outbox and platform envelopes | Unsafe config, missing telemetry, failed graceful shutdown |

## Baseline Decisions

- PostgreSQL remains the transactional source of truth.
- Redis remains required for sessions, rate limiting, idempotency, locks, and transient state.
- Kafka/Redpanda plus the transactional outbox remains the async backbone.
- ClickHouse is optional for core operation and required for advanced analytics.
- Object storage is optional only when import/export and large artifacts are disabled.
- Huma runtime OpenAPI generation is canonical; `docs/api/openapi.yaml` is the checked-in snapshot.
