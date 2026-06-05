# Identity And Access Model

## Mục tiêu nhóm model

Nhóm model này quản lý identity toàn cục, tenant boundary và access control của sản phẩm.

## Module ownership

| Model | Module owner chính |
|---|---|
| `User`, `ExternalAuthAccount`, `Workspace`, `WorkspaceMembership`, `WorkspaceInvitation`, `Session` | `identity` |
| `Role`, `MembershipRole`, `PermissionRegistry`, `APIKey` | `access` |
| `SenderDomain`, `DomainDNSRecordStatus` | `sender` |
| `CustomerWebhookConfig` | `webhooks` |
| `WorkspaceSettings` | `identity` hoặc `settings` nếu tách sau |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/01-control-plane.md` |
| API contract | `../api/01-auth-and-control-plane.md` |
| Database schema | `../database/01-core-identity-and-access.md` |
| Module catalog | `../modules/03-module-catalog.md` |

## Aggregate / entity chính

### `User`

- Vai trò: identity toàn cục dùng để đăng nhập.
- Thuộc tính chính: `id`, `email`, `hashed_password`, `status`, `primary_auth_method`, `email_verified_at`, `mfa_enabled_at`, `created_at`, `updated_at`.
- Invariant chính:
  - Email duy nhất trong hệ thống identity.
  - Nếu dùng password auth thì credential phải hợp lệ trước khi user active.
  - User social-first có thể chưa có `hashed_password` cho tới khi họ set password nội bộ.
  - `email_verified_at` chỉ được set qua flow verify email hoặc OAuth provider đã trả verified email.
  - `mfa_enabled_at` chỉ có giá trị khi user đã verify thành công một TOTP secret.

### `ExternalAuthAccount`

- Vai trò: liên kết user nội bộ với identity từ provider ngoài.
- Thuộc tính chính: `id`, `user_id`, `provider`, `provider_user_id`, `provider_email`, `provider_email_verified`, `linked_at`, `last_login_at`.
- Invariant chính:
  - Unique theo `(provider, provider_user_id)`.
  - Một external account chỉ link với đúng một `User`.

### `AuthToken`

- Vai trò: token một lần cho email verification, password reset, MFA challenge.
- Thuộc tính chính: `id`, `user_id`, `purpose`, `token_hash`, `expires_at`, `consumed_at`, `created_at`.
- Invariant chính:
  - Không lưu raw token.
  - Token chỉ dùng một lần và phải hết hạn rõ ràng theo từng purpose.

### `TOTPSecret` và `RecoveryCode`

- Vai trò: lưu state cho MFA TOTP.
- Thuộc tính chính:
  - `TOTPSecret`: `user_id`, `secret`, timestamps.
  - `RecoveryCode`: `id`, `user_id`, `code_hash`, `consumed_at`, `created_at`.
- Invariant chính:
  - Recovery code là single-use.
  - Login password với MFA enabled phải qua challenge token + TOTP/recovery code trước khi cấp session.

### `Workspace`

- Vai trò: boundary sở hữu của product data.
- Thuộc tính chính: `id`, `name`, `status`, `created_at`, `updated_at`.
- Invariant chính:
  - Workspace là scope mặc định của sender, contacts, templates, campaigns, logs, analytics.

### `WorkspaceMembership`

- Vai trò: nối `user` với `workspace`.
- Thuộc tính chính: `id`, `workspace_id`, `user_id`, `status`, timestamps.
- Invariant chính:
  - Permission chỉ có nghĩa trong membership + workspace context của request; session không nên là nơi giữ workspace active.

### `WorkspaceInvitation`

- Vai trò: model cho invite/join flow.
- Thuộc tính chính: `workspace_id`, `email`, `invited_by_user_id`, `role_ids`, `token_hash`, `expires_at`, `accepted_at`.
- Invariant chính:
  - Token chỉ được accept một lần hợp lệ trong thời hạn.

### `Role`

- Vai trò: nhóm permission trong workspace.
- Thuộc tính chính: `id`, `workspace_id`, `name`, `permissions_mask`, `version`.
- Invariant chính:
  - Role thuộc về đúng một workspace.
  - Thay đổi role phải làm mới effective permission.

### `MembershipRole`

- Vai trò: mapping membership -> role.
- Thuộc tính chính: `workspace_id`, `membership_id`, `role_id`.

### `PermissionRegistry`

- Vai trò: registry ổn định của permission bit.
- Thuộc tính chính: `bit`, `name`, `description` hoặc equivalent.
- Invariant chính:
  - Không reuse hoặc reorder bit đã release.

### `Session`

- Vai trò: phiên đăng nhập có thể revoke.
- Thuộc tính chính: `id`, `user_id`, `membership_id`, `expires_at`, `auth_method`, metadata.

### `APIKey`

- Vai trò: credential cho public API.
- Thuộc tính chính: `id`, `workspace_id`, `name`, `scope`, `hashed_secret`, `status`, timestamps.

### `SenderDomain`

- Vai trò: danh tính gửi email của tenant.
- Thuộc tính chính: `id`, `workspace_id`, `domain`, `status`, `provider`, timestamps.
- Invariant chính:
  - Domain chưa verified không được dùng cho production send path.

### `DomainDNSRecordStatus`

- Vai trò: model con mô tả trạng thái SPF/DKIM/DMARC.
- Thuộc tính chính: `record_type`, `host`, `expected_value`, `current_value`, `status`, `last_checked_at`, `failure_reason`.

### `CustomerWebhookConfig`

- Vai trò: cấu hình webhook outbound cho tenant.
- Thuộc tính chính: `id`, `workspace_id`, `target_url`, `signing_secret`, `status`, subscribed events.

### `WorkspaceSettings`

- Vai trò: cấu hình chung của workspace.
- Thuộc tính chính: key/value hoặc structured settings snapshot, version, updated metadata.

## Value object chính

- `UserID`
- `WorkspaceID`
- `MembershipID`
- `RoleID`
- `PermissionSet`
- `EmailAddress`
- `DomainName`
- `APIKeyScope`
- `AuthProvider`
- `ProviderUserID`

## Event model chính

- `identity.user.registered.v1`
- `identity.user.external_account_linked.v1`
- `identity.workspace.created.v1`
- `identity.workspace.member_invited.v1`
- `identity.workspace.member_joined.v1`
- `identity.workspace.member_roles_changed.v1`
- `identity.role.permissions_changed.v1`
- `sender.domain.verified.v1`
- `api key created/revoked`
- `workspace settings changed`

## Read model / projection quan trọng

- Current user/session view
- Workspace member list
- Role/permission assignment view
- Sender domain verification view
- Audit log view cho access control change
