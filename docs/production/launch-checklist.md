# Production Launch Checklist

Use this checklist before each production launch or major rollout.

## Deploy Readiness

- [ ] Production environment variables are reviewed and secrets are loaded from the approved secret store.
- [ ] `make check` passes on the release commit.
- [ ] `make build VERSION=<release>` produces API and worker binaries/images with git SHA and build time metadata.
- [ ] Database migrations have been reviewed for lock risk, reversibility, and rollback strategy.
- [ ] Kafka topics, object storage buckets, ClickHouse migrations, and provider credentials are provisioned.
- [ ] API, tracking, unsubscribe, provider webhook, and customer webhook domains resolve through the intended gateway.

## Release Steps

- [ ] Announce maintenance or deploy window if required.
- [ ] Apply Postgres migrations.
- [ ] Apply ClickHouse migrations when analytics is enabled.
- [ ] Deploy API.
- [ ] Deploy worker with the intended `WORKER_ENABLED_CONSUMERS`.
- [ ] Confirm `/api/healthz`, `/api/readyz`, `/api/metrics`, and `/openapi.json`.
- [ ] Confirm health responses expose the expected `version`, `git_sha`, and `build_time`.

## Smoke Tests

- [ ] Signup/login or known test-user login works.
- [ ] Workspace access and permission checks work for a non-admin user.
- [ ] Sender readiness path works for a test domain or fixture.
- [ ] Transactional send accepts a test request and produces one message.
- [ ] Worker processes outbox and due-message work without growing lag.
- [ ] Provider webhook ingestion accepts a signed test fixture.
- [ ] Tracking open/click/unsubscribe endpoints return expected responses.
- [ ] Analytics overview works; ClickHouse-backed analytics either work or return typed unavailable responses.
- [ ] Customer webhook test delivery succeeds or records a clear terminal failure.

## Rollback

- [ ] Previous API and worker artifacts are available.
- [ ] Rollback order is documented for this release.
- [ ] Migration rollback or forward-fix decision is documented.
- [ ] Feature flags or consumer toggles are ready to disable risky subsystems.
- [ ] Replay/DLQ actions are paused during rollback unless explicitly required.

## Dashboards And Alerts

- [ ] API latency, status rate, and error-code dashboards are live.
- [ ] Worker lag, retry, and DLQ dashboards are live.
- [ ] Postgres, Redis, Kafka, ClickHouse, object storage, and email provider alerts are live.
- [ ] Delivery failure rate, webhook delivery failure rate, and analytics sync lag alerts are live.
- [ ] Logs include request IDs and relevant workspace/user/message IDs without secrets.

## Incident Contacts

- [ ] Primary on-call: TBD.
- [ ] Backup on-call: TBD.
- [ ] Product/communications contact: TBD.
- [ ] Infrastructure/provider escalation contact: TBD.
- [ ] Customer support contact: TBD.
