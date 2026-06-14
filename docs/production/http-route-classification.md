# HTTP Route Classification

Huma runtime OpenAPI generation is canonical. This classification is a production baseline for route exposure and auth posture.

## Public

- `GET /api/healthz`
- `GET /api/readyz`
- `POST /api/v1/auth/signup`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/login/mfa`
- `POST /api/v1/auth/refresh`
- `GET /api/v1/auth/providers`
- `POST /api/v1/auth/oauth/{provider}/start`
- `POST /api/v1/auth/oauth/{provider}/exchange`
- `POST /api/v1/auth/email/verify`
- `POST /api/v1/auth/password/forgot`
- `POST /api/v1/auth/password/reset`
- `GET /openapi.json`

## Authenticated User API

- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`
- `POST /api/v1/auth/email/verify/request`
- `POST /api/v1/auth/mfa/totp/setup`
- `POST /api/v1/auth/mfa/totp/enable`
- `POST /api/v1/auth/mfa/totp/disable`
- `POST /api/v1/auth/mfa/recovery/regenerate`
- `GET /api/v1/sessions`
- `DELETE /api/v1/sessions/{session_id}`
- `GET /api/v1/permissions`
- `GET,POST /api/v1/workspaces`
- `GET /api/v1/workspaces/{workspace_id}`
- `GET /api/v1/workspaces/{workspace_id}/access`
- `POST /api/v1/invitations/{token}/accept`
- `GET /api/v1/workspaces/{workspace_id}/members`
- `DELETE /api/v1/workspaces/{workspace_id}/members/{membership_id}`
- `PUT /api/v1/workspaces/{workspace_id}/members/{membership_id}/role`
- `PUT /api/v1/workspaces/{workspace_id}/members/{membership_id}/roles`
- `GET,POST /api/v1/workspaces/{workspace_id}/invitations`
- `GET,POST /api/v1/workspaces/{workspace_id}/roles`
- `PATCH /api/v1/workspaces/{workspace_id}/roles/{role_id}`
- `GET,POST /api/v1/workspaces/{workspace_id}/sender-domains`
- `GET /api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}`
- `POST /api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}/verify`
- `POST /api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}/disable`
- `GET,POST /api/v1/workspaces/{workspace_id}/contacts`
- `GET,PATCH,DELETE /api/v1/workspaces/{workspace_id}/contacts/{contact_id}`
- `GET,POST /api/v1/workspaces/{workspace_id}/lists`
- `POST /api/v1/workspaces/{workspace_id}/lists/{list_id}/contacts`
- `GET,POST /api/v1/workspaces/{workspace_id}/segments`
- `PATCH,DELETE /api/v1/workspaces/{workspace_id}/segments/{segment_id}`
- `GET,POST /api/v1/workspaces/{workspace_id}/audience/imports`
- `GET /api/v1/workspaces/{workspace_id}/audience/imports/{import_id}`
- `GET,POST /api/v1/workspaces/{workspace_id}/audience/exports`
- `GET /api/v1/workspaces/{workspace_id}/audience/exports/{export_id}`
- `GET,POST /api/v1/workspaces/{workspace_id}/templates`
- `GET,PATCH,DELETE /api/v1/workspaces/{workspace_id}/templates/{template_id}`
- `POST /api/v1/workspaces/{workspace_id}/templates/{template_id}/publish`
- `POST /api/v1/workspaces/{workspace_id}/templates/{template_id}/preview`
- `GET /api/v1/workspaces/{workspace_id}/templates/{template_id}/versions`
- `POST /api/v1/workspaces/{workspace_id}/render`
- `GET,POST /api/v1/workspaces/{workspace_id}/suppression`
- `DELETE /api/v1/workspaces/{workspace_id}/suppression/{suppression_id}`
- `GET,POST /api/v1/workspaces/{workspace_id}/campaigns`
- `GET,PATCH,DELETE /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}`
- `POST /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/schedule`
- `POST /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/pause`
- `POST /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/resume`
- `POST /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/cancel`
- `GET /api/v1/workspaces/{workspace_id}/campaigns/{campaign_id}/candidates`
- `GET /api/v1/workspaces/{workspace_id}/messages`
- `GET /api/v1/workspaces/{workspace_id}/messages/{message_id}`
- `GET,POST /api/v1/workspaces/{workspace_id}/api-keys`
- `DELETE /api/v1/workspaces/{workspace_id}/api-keys/{api_key_id}`
- `GET,POST /api/v1/workspaces/{workspace_id}/webhooks`
- `GET,PATCH,DELETE /api/v1/workspaces/{workspace_id}/webhooks/{webhook_id}`
- `POST /api/v1/workspaces/{workspace_id}/webhooks/{webhook_id}/rotate-secret`
- `GET /api/v1/workspaces/{workspace_id}/webhook-deliveries`
- `GET /api/v1/workspaces/{workspace_id}/webhook-deliveries/{delivery_id}`
- `POST /api/v1/workspaces/{workspace_id}/webhook-deliveries/{delivery_id}/retry`
- `GET /api/v1/workspaces/{workspace_id}/notifications`
- `GET /api/v1/workspaces/{workspace_id}/notifications/{notification_id}`
- `POST /api/v1/workspaces/{workspace_id}/notifications/alert`
- `GET,PATCH /api/v1/workspaces/{workspace_id}/settings`
- `GET /api/v1/workspaces/{workspace_id}/audit-logs`
- `GET /api/v1/workspaces/{workspace_id}/analytics/**`
- `GET /api/v1/analytics/sync/{stream_name}/status`

## Authenticated API-Key API

- `POST /api/v1/transactional/send`
- `GET /api/v1/transactional/messages/{message_id}`

## Provider Webhook

- `POST /api/v1/webhooks/providers/{provider}`

## Tracking Endpoint

- `GET /o/{tracking_id}`
- `GET /t/{tracking_id}`
- `GET /u/{token}`

## Internal Operations Endpoint

- `GET /api/metrics`
- `GET /api/events/stream`
- `GET /api/v1/workspaces/{workspace_id}/operations/outbox/summary`
- `GET /api/v1/workspaces/{workspace_id}/operations/outbox`
- `GET /api/v1/workspaces/{workspace_id}/operations/outbox/{outbox_id}`
- `GET /api/v1/workspaces/{workspace_id}/operations/dead-letter-records`
- `GET /api/v1/workspaces/{workspace_id}/operations/dead-letter-records/{dead_letter_id}`
- `GET,POST /api/v1/workspaces/{workspace_id}/operations/replay-jobs`
- `GET /api/v1/workspaces/{workspace_id}/operations/replay-jobs/{replay_job_id}`
