# Auth And Control Plane API

## Mục tiêu nhóm API

Nhóm API này phục vụ quản trị danh tính, workspace, access control và cấu hình nền của tenant.

## Module ownership

| API/resource | Module owner chính | Ghi chú boundary |
|---|---|---|
| Auth và session | `identity` | Sở hữu user, external auth account, session |
| Workspace và membership | `identity` | Sở hữu tenant boundary và membership lifecycle |
| Role và permission | `access` | Có thể gộp vào `identity` giai đoạn đầu, nhưng giữ contract authorization riêng |
| API key | `access` | Public API credential và scope evaluation |
| Sender domain / DNS verification | `sender` | Campaign/delivery chỉ đọc readiness, không update trực tiếp |
| Customer webhook config | `webhooks` | Config và subscribed events thuộc outbound webhook module |
| Workspace settings | `identity` hoặc `settings` nếu tách sau | Nếu settings phình, tách module riêng sau |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/01-control-plane.md` |
| Business model | `../model/01-identity-and-access.md` |
| Database schema | `../database/01-core-identity-and-access.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| API conventions | `00-api-map.md` |

## Resource: Auth và session

### Actor chính

- Dashboard user

### Auth model

- `signup` và `login` là public endpoint.
- Hỗ trợ cả password credential và external identity provider qua OAuth2/OIDC.
- Provider đầu tiên nên hỗ trợ: `google`, `github`; contract cho phép mở rộng thêm provider như generic OIDC sau này.
- `logout`, `me`, `sessions` yêu cầu session/token hợp lệ của user.

### API capability

- `POST /api/v1/auth/signup`
- `POST /api/v1/auth/login`
- `GET /api/v1/auth/providers`
- `POST /api/v1/auth/oauth/{provider}/start`
- `POST /api/v1/auth/oauth/{provider}/exchange`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`
- `GET /api/v1/sessions`
- `DELETE /api/v1/sessions/{session_id}`

### Hành vi chính

- Tạo user mới.
- Xác thực user và trả session/token.
- Khởi tạo OAuth/OIDC authorization flow cho Google, GitHub hoặc provider tương đương.
- Đổi `authorization_code` lấy identity của provider, auto-link hoặc auto-provision user nếu policy cho phép.
- Trả identity hiện tại cùng session metadata.
- Revoke một hoặc nhiều session.

### Quy ước workspace context

- API phải resolve workspace context theo dữ liệu tường minh của request, như `workspace_id` trên path, input payload hoặc membership được chọn rõ ràng.

### Request và response chính

`POST /api/v1/auth/signup`

- Request chính: `email`, `password`, optional profile bootstrap fields.
- Response chính: `user`, `session`, `access_token`, `token_type`, `expires_at`.
- Đồng thời set `sf_refresh_token` vào HttpOnly cookie, `SameSite=Lax`, path `/api/v1/auth/refresh`.
- Status code: `201` khi tạo mới, `409` nếu email đã tồn tại, `422` nếu password/email không hợp lệ.

Headers:

- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "email": "owner@example.com",
  "password": "StrongPassword123!"
}
```

`POST /api/v1/auth/login`

- Request chính: `email`, `password`.
- Response chính: hoặc `AuthSessionResponse`, hoặc `{ mfa_required, mfa_challenge_token, user }` nếu user đã bật TOTP MFA.
- Status code: `200`, `401`, `429`.

Headers:

- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "email": "owner@example.com",
  "password": "StrongPassword123!"
}
```

`POST /api/v1/auth/login/mfa`

- Request chính: `mfa_challenge_token` và một trong `code` hoặc `recovery_code`.
- Response chính: `user`, `session`, `access_token`, `expires_at`, đồng thời rotate refresh cookie.

`POST /api/v1/auth/refresh`

- Không đọc refresh token từ request body/query/header.
- Chỉ dùng `sf_refresh_token` HttpOnly cookie để rotate token.
- Response chính: `user`, `session`, `access_token`, `expires_at`.

`GET /api/v1/auth/providers`

- Response chính: danh sách provider đang được bật cho dashboard login, capability và display metadata.

Response body gợi ý:

```json
{
  "data": [
    {
      "provider": "google",
      "type": "oidc",
      "display_name": "Google",
      "enabled": true
    },
    {
      "provider": "github",
      "type": "oauth2",
      "display_name": "GitHub",
      "enabled": true
    }
  ]
}
```

`POST /api/v1/auth/oauth/{provider}/start`

- Request chính: `redirect_uri`, optional `intent`, optional `invitation_token`.
- Response chính: `authorization_url`, `state`, optional PKCE challenge metadata nếu backend không tự redirect.
- Status code: `200`, `404`, `422`, `429`.

Headers:

- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "redirect_uri": "https://app.sendflow.dev/auth/callback/google",
  "intent": "login"
}
```

Response body gợi ý:

```json
{
  "data": {
    "provider": "google",
    "authorization_url": "https://accounts.google.com/o/oauth2/v2/auth?...",
    "state": "oauth_state_123",
    "code_challenge_method": "S256"
  }
}
```

`POST /api/v1/auth/oauth/{provider}/exchange`

- Request chính: `code`, `state`, `redirect_uri`, optional `code_verifier`.
- Response chính: `user`, `session`, `access_token`, `expires_at`, cùng metadata về provider đã dùng để login.
- Status code: `200`, `201`, `401`, `409`, `422`.
- `201` dùng khi lần đăng nhập đầu tạo mới user hoặc link account lần đầu theo policy.

Headers:

- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "code": "oauth_code_123",
  "state": "oauth_state_123",
  "redirect_uri": "https://app.sendflow.dev/auth/callback/google",
  "code_verifier": "pkce_verifier_123"
}
```

Response body gợi ý:

```json
{
  "data": {
    "user": {
      "id": "user_123",
      "email": "owner@example.com"
    },
    "session": {
      "id": "sess_123",
      "expires_at": "2026-06-01T00:00:00Z",
      "auth_method": "oauth_google"
    },
    "identity_provider": {
      "provider": "google",
      "provider_user_id": "google_abc_123"
    }
  }
}
```

`POST /api/v1/auth/logout`

- Request chính: current session context hoặc session id cần revoke nếu flow cho phép.
- Response chính: trạng thái revoke thành công và backend xóa `sf_refresh_token` cookie.
- Status code: `200` hoặc `204`.

Headers:

- `Authorization: Bearer <session_token>`

`GET /api/v1/auth/me`

- Response chính: identity snapshot hiện tại với `email_verified`, `mfa_enabled`.

Headers:

- `Authorization: Bearer <session_token>`

`GET /api/v1/sessions`

- Response chính: danh sách session đang hoạt động của user.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "id": "sess_123",
      "created_at": "2026-05-31T09:00:00Z",
      "expires_at": "2026-06-01T09:00:00Z",
      "ip_address": "203.0.113.10"
    }
  ]
}
```

`DELETE /api/v1/sessions/{session_id}`

- Response chính: kết quả revoke một session cụ thể.

Headers:

- `Authorization: Bearer <session_token>`

- `POST /api/v1/auth/email/verify/request`: gửi lại email verification cho user hiện tại.
- `POST /api/v1/auth/email/verify`: nhận token một lần để mark `email_verified`.
- `POST /api/v1/auth/password/forgot`: luôn trả `200`, chỉ gửi reset email nếu account tồn tại.
- `POST /api/v1/auth/password/reset`: nhận `token`, `new_password`; đổi mật khẩu và revoke session cũ.
- `POST /api/v1/auth/mfa/totp/setup`: tạo pending TOTP secret và `otpauth_url`.
- `POST /api/v1/auth/mfa/totp/enable`: verify TOTP code rồi bật MFA, trả recovery codes một lần.
- `POST /api/v1/auth/mfa/totp/disable`: dùng password hoặc TOTP code để tắt MFA.
- `POST /api/v1/auth/mfa/recovery/regenerate`: tạo mới recovery codes sau khi xác nhận TOTP code hiện tại.

### Refresh cookie contract

- Refresh token không xuất hiện trong response body.
- Client phải gửi request refresh với cookie enabled (`credentials: include` ở browser fetch nếu cần).
- Backend bỏ qua mọi refresh token mà client cố gửi qua body/query/header.

### Ví dụ shape gợi ý

```json
{
  "data": {
    "user": {
      "id": "user_123",
      "email": "owner@example.com"
    },
    "session": {
      "id": "sess_123",
      "expires_at": "2026-06-01T00:00:00Z",
      "auth_method": "password"
    }
  }
}
```

Error codes:

- `400 auth.invalid_request_body` request body sai shape hoặc thiếu field
- `401 auth.invalid_credentials` email/password sai
- `401 auth.oauth_state_invalid` `state` không hợp lệ, hết hạn hoặc không khớp
- `401 auth.provider_token_exchange_failed` không đổi được code lấy token/identity từ provider
- `401 auth.invalid_token` session token không hợp lệ hoặc hết hạn
- `404 auth.provider_not_supported` provider không được bật hoặc không tồn tại
- `409 identity.email_already_registered` email đã tồn tại khi signup
- `409 auth.identity_provider_conflict` external account đã link với user khác
- `422 identity.email_invalid` email không hợp lệ theo rule hệ thống
- `422 auth.identity_provider_email_not_verified` provider không trả email verified nhưng policy yêu cầu verified email
- `422 auth.identity_provider_account_linking_required` cần user xác nhận link account thay vì auto-link
- `422 auth.password_policy_violation` password không đạt policy
- `429 auth.rate_limited` vượt rate limit auth endpoint

### Ghi chú contract

- Auth response không cần nhúng workspace permission context; dùng workspace-scoped endpoint riêng để lấy access summary.
- Nếu provider trả `email_verified=true` và đã có `user` cùng email, hệ thống có thể auto-link theo policy; nếu không, trả error/linking flow rõ ràng.
- `signup` bằng password không bắt buộc với mọi user; user tạo từ Google/GitHub có thể không có `hashed_password` cho tới khi họ thiết lập credential nội bộ.
- Endpoint nhạy cảm cần rate limit và audit phù hợp.

## Resource: Workspaces, memberships, invitations

### Actor chính

- Workspace admin
- Member được mời

### Auth model

- Yêu cầu dashboard session hợp lệ.
- Hầu hết endpoint cần quyền quản trị workspace; riêng accept invitation có thể public theo token.

### API capability

- `GET /api/v1/workspaces`
- `POST /api/v1/workspaces`
- `GET /api/v1/workspaces/{workspace_id}`
- `GET /api/v1/workspaces/{workspace_id}/members`
- `POST /api/v1/workspaces/{workspace_id}/invitations`
- `GET /api/v1/workspaces/{workspace_id}/invitations`
- `POST /api/v1/invitations/{token}/accept`
- `DELETE /api/v1/workspaces/{workspace_id}/members/{membership_id}`

### Hành vi chính

- Tạo workspace.
- Mời member mới vào workspace.
- Xem danh sách member và invitation.
- Accept invitation và join workspace.
- Loại member khỏi workspace.

### Request và response chính

`GET /api/v1/workspaces`

- Trả danh sách workspace mà user đang là member.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "id": "ws_123",
      "name": "Acme",
      "membership_id": "m_123",
      "role_names": [
        "owner"
      ]
    }
  ]
}
```

`POST /api/v1/workspaces`

- Request chính: `name`, optional bootstrap settings.
- Response chính: workspace mới tạo cùng membership owner ban đầu.
- Status code: `201`.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "name": "Acme"
}
```

Response body gợi ý:

```json
{
  "data": {
    "workspace": {
      "id": "ws_123",
      "name": "Acme"
    },
    "membership": {
      "id": "m_123",
      "role_names": [
        "owner"
      ]
    }
  }
}
```

`GET /api/v1/workspaces/{workspace_id}/members`

- Query gợi ý: `status`, `limit`, `cursor`.
- Response chính: member list view, role summary, invitation state nếu cần.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "membership_id": "m_123",
      "user_email": "owner@example.com",
      "status": "active",
      "role_names": [
        "owner"
      ]
    }
  ]
}
```

`POST /api/v1/workspaces/{workspace_id}/invitations`

- Request chính: `email`, `role_ids`, optional message.
- Response chính: invitation metadata, `expires_at`.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "email": "member@example.com",
  "role_ids": [
    "role_editor"
  ]
}
```

Response body gợi ý:

```json
{
  "data": {
    "invitation_id": "inv_123",
    "email": "member@example.com",
    "expires_at": "2026-06-07T00:00:00Z"
  }
}
```

`POST /api/v1/invitations/{token}/accept`

- Request chính: optional profile completion fields.
- Response chính: membership vừa được kích hoạt để client dùng tiếp trong các request có `workspace_id` tương ứng.

Headers:

- `Content-Type: application/json`

Response body gợi ý:

```json
{
  "data": {
    "workspace_id": "ws_123",
    "membership_id": "m_456",
    "status": "active"
  }
}
```

`DELETE /api/v1/workspaces/{workspace_id}/members/{membership_id}`

- Response chính: trạng thái remove thành công.
- Status code: `204`, `404`, `409` nếu business rule không cho remove owner cuối cùng.

Headers:

- `Authorization: Bearer <session_token>`

Error codes:

- `401 auth.invalid_token` chưa xác thực hoặc session hết hạn
- `403 identity.workspace_access_denied` không có quyền truy cập workspace
- `403 identity.membership_manage_denied` không có quyền quản trị member/invitation
- `404 identity.workspace_not_found` không tìm thấy workspace trong scope hiện tại
- `404 identity.membership_not_found` không tìm thấy membership
- `404 identity.invitation_not_found` không tìm thấy invitation/token
- `409 identity.last_owner_cannot_be_removed` không thể remove owner cuối cùng
- `409 identity.invitation_already_accepted` invitation đã được accept trước đó
- `422 identity.invitation_payload_invalid` payload invite không hợp lệ
- `422 identity.invitation_token_expired` token invite đã hết hạn

### Ghi chú contract

- Product data luôn được scope bằng `workspace_id`.
- Membership là resource trung tâm cho permission và audit.

## Resource: Roles và permissions

### Actor chính

- Workspace admin

### Auth model

- Yêu cầu session hợp lệ và permission quản trị quyền trong workspace.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/access`
- `GET /api/v1/workspaces/{workspace_id}/roles`
- `POST /api/v1/workspaces/{workspace_id}/roles`
- `PATCH /api/v1/workspaces/{workspace_id}/roles/{role_id}`
- `GET /api/v1/permissions`
- `PUT /api/v1/workspaces/{workspace_id}/members/{membership_id}/roles`

### Hành vi chính

- Xem access summary của current user trong một workspace.
- Quản lý role trong workspace.
- Xem permission registry.
- Gán hoặc thay đổi role cho membership.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/access`

- Trả membership hiện tại của current user trong workspace, role assignments và effective permissions đã resolve.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "workspace_id": "ws_123",
    "membership_id": "m_123",
    "status": "active",
    "role_ids": [
      "role_owner"
    ],
    "role_names": [
      "owner"
    ],
    "effective_permissions": [
      "campaign.read",
      "campaign.write"
    ]
  }
}
```

`GET /api/v1/workspaces/{workspace_id}/roles`

- Trả danh sách role, `permissions_mask`, permission names resolved và version metadata.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "id": "role_owner",
      "name": "owner",
      "permissions_mask": 1023,
      "permission_names": [
        "campaign.read",
        "campaign.write"
      ],
      "version": 3
    }
  ]
}
```

`POST /api/v1/workspaces/{workspace_id}/roles`

- Request chính: `name`, `permission_names` hoặc `permissions_mask`.
- Response chính: role mới tạo.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "name": "editor",
  "permission_names": [
    "campaign.read",
    "campaign.write",
    "template.read",
    "template.write"
  ]
}
```

Response body gợi ý:

```json
{
  "data": {
    "id": "role_editor",
    "name": "editor",
    "version": 1
  }
}
```

`PATCH /api/v1/workspaces/{workspace_id}/roles/{role_id}`

- Request chính: thay đổi tên, permission set hoặc trạng thái.
- Response chính: role sau cập nhật và `version` mới.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

`GET /api/v1/permissions`

- Trả permission registry toàn cục hoặc snapshot được expose cho UI.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "bit": 0,
      "name": "campaign.read"
    }
  ]
}
```

`PUT /api/v1/workspaces/{workspace_id}/members/{membership_id}/roles`

- Request chính: danh sách `role_ids`.
- Response chính: role assignment mới và effective permission summary.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "role_ids": [
    "role_editor"
  ]
}
```

Response body gợi ý:

```json
{
  "data": {
    "membership_id": "m_123",
    "role_ids": [
      "role_editor"
    ],
    "effective_permissions": [
      "campaign.read",
      "campaign.write"
    ]
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 identity.workspace_access_denied` không có quyền truy cập workspace
- `403 identity.role_manage_denied` không có quyền quản trị roles/permissions
- `404 identity.role_not_found` không tìm thấy role trong workspace
- `404 identity.membership_not_found` không tìm thấy membership cần gán role
- `409 identity.role_name_conflict` tên role đã tồn tại trong workspace
- `409 identity.role_assignment_conflict` xung đột khi gán role
- `422 identity.permission_set_invalid` permission set không hợp lệ
- `422 identity.permission_registry_unknown` request chứa permission không tồn tại

### Ghi chú contract

- Permission contract nên có cả `name` lẫn `bit` hoặc representation tương đương.
- Thay đổi role cần kích hoạt permission invalidation và audit.

## Resource: API keys

### Actor chính

- Workspace admin
- Developer của tenant

### Auth model

- Yêu cầu session hợp lệ với quyền `api_key.manage` hoặc tương đương.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/api-keys`
- `POST /api/v1/workspaces/{workspace_id}/api-keys`
- `PATCH /api/v1/workspaces/{workspace_id}/api-keys/{api_key_id}`
- `DELETE /api/v1/workspaces/{workspace_id}/api-keys/{api_key_id}`

### Hành vi chính

- Tạo API key mới.
- Xem metadata key đã cấp.
- Rotate/revoke key.
- Quản lý scope hoặc trạng thái key.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/api-keys`

- Trả metadata key: `id`, `name`, `scope`, `status`, `created_at`, `last_used_at`.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "id": "key_123",
      "name": "backend-service",
      "scope": [
        "transactional.send"
      ],
      "status": "active",
      "last_used_at": "2026-05-31T08:00:00Z"
    }
  ]
}
```

`POST /api/v1/workspaces/{workspace_id}/api-keys`

- Request chính: `name`, `scope`, optional expiration/config.
- Response chính: metadata key và secret plaintext chỉ hiển thị một lần.
- Status code: `201`.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "name": "backend-service",
  "scope": [
    "transactional.send"
  ]
}
```

Response body gợi ý:

```json
{
  "data": {
    "id": "key_123",
    "name": "backend-service",
    "scope": [
      "transactional.send"
    ],
    "secret": "sk_live_xxx"
  }
}
```

`PATCH /api/v1/workspaces/{workspace_id}/api-keys/{api_key_id}`

- Request chính: đổi tên, scope hoặc rotate.
- Response chính: metadata đã cập nhật; nếu rotate thì secret mới chỉ trả trong response đó.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

`DELETE /api/v1/workspaces/{workspace_id}/api-keys/{api_key_id}`

- Response chính: revoked state hoặc `204`.

Headers:

- `Authorization: Bearer <session_token>`

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 api_key.manage_denied` không có quyền `api_key.manage`
- `404 api_key.not_found` không tìm thấy API key trong workspace
- `409 api_key.rotate_conflict` xung đột khi rotate/revoke key
- `422 api_key.scope_invalid` scope API key không hợp lệ
- `422 api_key.config_invalid` expiration hoặc config key không hợp lệ

### Ghi chú contract

- Secret value thường chỉ trả một lần lúc tạo.
- Audit credential lifecycle là bắt buộc.

## Resource: Sender domains

### Actor chính

- Workspace admin
- Deliverability owner

### Auth model

- Yêu cầu session hợp lệ với quyền quản lý sender/settings.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/sender-domains`
- `POST /api/v1/workspaces/{workspace_id}/sender-domains`
- `GET /api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}`
- `POST /api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}/verify`
- `POST /api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}/disable`

### Hành vi chính

- Thêm sending domain.
- Trả DNS records cần cấu hình.
- Kiểm tra và cập nhật trạng thái SPF/DKIM/DMARC.
- Disable sender khi cần.

### Request và response chính

`POST /api/v1/workspaces/{workspace_id}/sender-domains`

- Request chính: `domain`, optional provider binding metadata.
- Response chính: sender domain record và `desired_records`.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "domain": "mail.example.com",
  "provider": "ses"
}
```

Response body gợi ý:

```json
{
  "data": {
    "id": "sd_123",
    "domain": "mail.example.com",
    "status": "pending_verification",
    "desired_records": [
      {
        "record_type": "TXT",
        "host": "_dmarc.mail.example.com",
        "value": "v=DMARC1; p=none"
      }
    ]
  }
}
```

`GET /api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}`

- Response chính: domain status, DNS record status, `last_checked_at`, `failure_reason`, sending readiness summary.

Headers:

- `Authorization: Bearer <session_token>`

`POST /api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}/verify`

- Hành vi: trigger verify ngay hoặc mark recheck requested.
- Response chính: verification job/status snapshot.

Headers:

- `Authorization: Bearer <session_token>`

`POST /api/v1/workspaces/{workspace_id}/sender-domains/{domain_id}/disable`

- Hành vi: tắt sender khỏi send path mới.
- Response chính: domain status sau thay đổi.

Headers:

- `Authorization: Bearer <session_token>`

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 sender.manage_denied` không có quyền quản lý sender/domain
- `404 sender.domain_not_found` không tìm thấy sender domain
- `409 sender.invalid_state_transition` sender đang ở trạng thái không cho phép action này
- `422 sender.domain_invalid` domain không hợp lệ
- `422 sender.provider_config_invalid` provider binding/config không hợp lệ
- `422 sender.domain_not_verified` sender chưa đạt điều kiện verify/send readiness

### Ghi chú contract

- Response nên tách `desired_records` và `current_status`.
- Verification có thể là async workflow; API nên phản ánh `pending` hoặc `last_checked_at`.

## Resource: Customer webhook config

### Actor chính

- Workspace admin
- Developer của tenant

### Auth model

- Yêu cầu session hợp lệ với quyền quản trị developer integration.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/webhooks`
- `POST /api/v1/workspaces/{workspace_id}/webhooks`
- `PATCH /api/v1/workspaces/{workspace_id}/webhooks/{webhook_id}`
- `POST /api/v1/workspaces/{workspace_id}/webhooks/{webhook_id}/rotate-secret`
- `DELETE /api/v1/workspaces/{workspace_id}/webhooks/{webhook_id}`

### Hành vi chính

- Khai báo endpoint webhook downstream.
- Quản lý signing secret.
- Bật/tắt hoặc điều chỉnh event subscription.

### Request và response chính

`POST /api/v1/workspaces/{workspace_id}/webhooks`

- Request chính: `target_url`, `subscribed_events`, optional signing config.
- Response chính: webhook config metadata, trạng thái enablement.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "target_url": "https://example.com/webhooks/send-flow",
  "subscribed_events": [
    "delivery.message.delivered.v1",
    "delivery.message.bounced.v1"
  ]
}
```

Response body gợi ý:

```json
{
  "data": {
    "id": "wh_123",
    "target_url": "https://example.com/webhooks/send-flow",
    "status": "enabled"
  }
}
```

`PATCH /api/v1/workspaces/{workspace_id}/webhooks/{webhook_id}`

- Request chính: đổi URL, subscribed events, status.
- Response chính: config sau cập nhật.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

`POST /api/v1/workspaces/{workspace_id}/webhooks/{webhook_id}/rotate-secret`

- Response chính: metadata mới và secret plaintext nếu policy cho phép hiển thị lại một lần.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "id": "wh_123",
    "secret": "whsec_xxx"
  }
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 webhook.manage_denied` không có quyền quản trị webhook downstream
- `404 webhook.config_not_found` không tìm thấy webhook config
- `409 webhook.rotate_conflict` xung đột khi rotate secret hoặc thay đổi config
- `422 webhook.target_url_invalid` target URL không hợp lệ
- `422 webhook.subscription_invalid` subscribed events không hợp lệ

### Ghi chú contract

- Đây là customer webhook outbound config, không phải provider webhook ingress.

## Resource: Settings và audit logs

### Actor chính

- Workspace admin
- Operator

### Auth model

- `settings` cần quyền quản trị workspace.
- `audit-logs` cần quyền đọc audit hoặc role admin phù hợp.

### API capability

- `GET /api/v1/workspaces/{workspace_id}/settings`
- `PATCH /api/v1/workspaces/{workspace_id}/settings`
- `GET /api/v1/workspaces/{workspace_id}/audit-logs`

### Hành vi chính

- Xem và chỉnh sửa general settings.
- Lấy audit log đã ghi từ các action nhạy cảm.

### Request và response chính

`GET /api/v1/workspaces/{workspace_id}/settings`

- Response chính: structured settings snapshot, version, updated metadata.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": {
    "version": 3,
    "email_defaults": {
      "default_sender_domain_id": "sd_123"
    }
  }
}
```

`PATCH /api/v1/workspaces/{workspace_id}/settings`

- Request chính: partial update các field được phép chỉnh.
- Response chính: settings sau cập nhật và version mới.

Headers:

- `Authorization: Bearer <session_token>`
- `Content-Type: application/json`

Request body gợi ý:

```json
{
  "email_defaults": {
    "default_sender_domain_id": "sd_123"
  }
}
```

`GET /api/v1/workspaces/{workspace_id}/audit-logs`

- Query gợi ý: `actor_user_id`, `action_type`, `from`, `to`, `cursor`.
- Response chính: audit entries kèm actor, target, summary và thời điểm.

Headers:

- `Authorization: Bearer <session_token>`

Response body gợi ý:

```json
{
  "data": [
    {
      "id": "audit_123",
      "actor_user_id": "user_123",
      "action_type": "api_key.created",
      "target_id": "key_123",
      "occurred_at": "2026-05-31T08:00:00Z"
    }
  ]
}
```

Error codes:

- `401 auth.invalid_token` chưa xác thực
- `403 settings.manage_denied` không có quyền sửa settings workspace
- `403 audit.read_denied` không có quyền đọc audit logs
- `404 identity.workspace_not_found` không tìm thấy workspace
- `409 settings.version_conflict` settings bị conflict theo version/update race
- `422 settings.payload_invalid` payload settings không hợp lệ

### Ghi chú contract

- Settings API nên tách rõ user-configurable value và system-managed status.
- Audit log API là read model, không phải source of truth giao dịch.
