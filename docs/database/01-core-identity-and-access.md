# Core Identity And Access Database

## Mục tiêu nhóm schema

Nhóm schema này lưu source of truth cho identity, tenant boundary, quyền truy cập và cấu hình nền của workspace.

## Module ownership

| Table | Module owner chính |
|---|---|
| `users`, `external_auth_accounts`, `workspaces`, `workspace_memberships`, `workspace_invitations`, `sessions` | `identity` |
| `roles`, `membership_roles`, `permission_registry`, `api_keys` | `access` |
| `sender_domains`, `sender_domain_dns_records` | `sender` |
| `customer_webhooks` | `webhooks` |
| `workspace_settings` | `identity` hoặc `settings` nếu tách sau |
| `audit_entries` | `audit` |

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Feature | `../features/01-control-plane.md` |
| API contract | `../api/01-auth-and-control-plane.md` |
| Business model | `../model/01-identity-and-access.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| Infrastructure | `../architecture/06-infrastructure.md` |

## PostgreSQL tables

### `users`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | user id toàn cục |
| `email` | email đăng nhập |
| `hashed_password` | credential đã hash, có thể null với social-only account |
| `status` | trạng thái user |
| `primary_auth_method` | `password`, `oauth_google`, `oauth_github`, `oidc`... |
| `email_verified_at` | thời điểm email nội bộ được verify |
| `mfa_enabled_at` | thời điểm TOTP MFA được bật |
| timestamps | lifecycle metadata |

Ràng buộc gợi ý:

- unique: `(email)`

### `external_auth_accounts`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | external auth account id |
| `user_id` | user nội bộ được link |
| `provider` | `google`, `github`, `oidc`... |
| `provider_user_id` | user id phía provider |
| `provider_email` | email provider trả về |
| `provider_email_verified` | cờ verified email từ provider |
| `access_token_ref` hoặc equivalent | optional reference nếu cần lưu token an toàn |
| `refresh_token_ref` hoặc equivalent | optional reference nếu cần |
| `linked_at` | thời điểm link |
| `last_login_at` | lần login gần nhất |
| timestamps | lifecycle metadata |

Ràng buộc gợi ý:

- unique: `(provider, provider_user_id)`
- index: `(user_id, provider)`

### `workspaces`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | workspace id |
| `name` | tên workspace |
| `status` | trạng thái workspace |
| timestamps | lifecycle metadata |

### `workspace_memberships`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | membership id |
| `workspace_id` | workspace owner |
| `user_id` | user member |
| `status` | active/invited/disabled... |
| timestamps | lifecycle metadata |

Ràng buộc gợi ý:

- unique: `(workspace_id, user_id)`

### `workspace_invitations`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | invitation id |
| `workspace_id` | workspace target |
| `email` | email được mời |
| `invited_by_user_id` | actor mời |
| `token_hash` | token đã băm |
| `expires_at` | hạn dùng |
| `accepted_at` | thời điểm đã accept |

### `sessions`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | session id |
| `user_id` | user owner |
| `membership_id` | membership tương ứng nếu có |
| `auth_method` | password/oauth/provider... |
| `expires_at` | thời điểm hết hạn |
| `access_jti` | JTI hiện tại của access token |
| `refresh_jti` | JTI hiện tại của refresh token |
| `revoked_at` | thời điểm revoke nếu có |
| metadata | user agent, IP hash hoặc context an toàn |

### `auth_tokens`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | token record id |
| `user_id` | user owner |
| `purpose` | `email_verification`, `password_reset`, `mfa_challenge` |
| `token_hash` | SHA-256 hash của token raw |
| `expires_at` | hạn dùng |
| `consumed_at` | thời điểm đã dùng |
| `created_at` | thời điểm tạo |

### `user_mfa_totp`

| Cột chính | Ý nghĩa |
|---|---|
| `user_id` | user owner |
| `secret` | TOTP secret hiện tại hoặc pending |
| timestamps | lifecycle metadata |

### `user_mfa_recovery_codes`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | recovery code record id |
| `user_id` | user owner |
| `code_hash` | hash của recovery code raw |
| `consumed_at` | thời điểm đã dùng |
| `created_at` | thời điểm tạo |

### `permission_registry`

| Cột chính | Ý nghĩa |
|---|---|
| `bit` hoặc `id` | định danh permission ổn định |
| `name` | tên permission như `campaign.read` |
| `description` | mô tả quyền |
| `status` | active/deprecated |
| timestamps | lifecycle metadata |

Ràng buộc gợi ý:

- unique: `(name)`
- không reuse/reorder bit đã release

### `roles`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | role id |
| `workspace_id` | workspace owner |
| `name` | tên role |
| `permissions_mask` | bitmask quyền |
| `version` | version để invalidate cache |
| timestamps | lifecycle metadata |

Ràng buộc gợi ý:

- unique: `(workspace_id, name)`

### `membership_roles`

| Cột chính | Ý nghĩa |
|---|---|
| `workspace_id` | workspace owner |
| `membership_id` | membership target |
| `role_id` | assigned role |

Ràng buộc gợi ý:

- unique: `(workspace_id, membership_id, role_id)`

### `api_keys`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | api key id |
| `workspace_id` | workspace owner |
| `name` | tên hiển thị |
| `hashed_secret` | secret đã hash |
| `scope` | phạm vi dùng |
| `status` | active/revoked |
| timestamps | lifecycle metadata |

### `sender_domains`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | sender domain id |
| `workspace_id` | workspace owner |
| `domain` | tên domain |
| `provider` | provider liên quan |
| `status` | pending/verified/disabled |
| timestamps | lifecycle metadata |

Ràng buộc gợi ý:

- unique: `(workspace_id, domain)`

### `sender_domain_dns_records`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | record id |
| `sender_domain_id` | sender domain owner |
| `record_type` | SPF/DKIM/DMARC... |
| `host` | DNS host |
| `expected_value` | giá trị mong muốn |
| `current_value` | giá trị hiện tại |
| `status` | verified/missing/mismatch |
| `last_checked_at` | lần check gần nhất |
| `failure_reason` | lý do lỗi |

### `customer_webhooks`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | webhook config id |
| `workspace_id` | workspace owner |
| `target_url` | URL downstream |
| `signing_secret_hash` hoặc equivalent | secret lưu an toàn |
| `status` | enabled/disabled |
| `subscribed_events` | tập event đăng ký |
| timestamps | lifecycle metadata |

### `workspace_settings`

| Cột chính | Ý nghĩa |
|---|---|
| `workspace_id` | workspace owner |
| `settings_json` hoặc structured columns | config snapshot |
| `version` | version thay đổi |
| timestamps | lifecycle metadata |

### `audit_entries`

| Cột chính | Ý nghĩa |
|---|---|
| `id` | audit id |
| `workspace_id` | workspace liên quan |
| `actor_user_id` | ai thực hiện |
| `action_type` | hành động |
| `target_type` / `target_id` | đối tượng bị tác động |
| `payload_summary` | metadata tóm tắt |
| `occurred_at` | thời điểm |

## Redis keys

- `identity:refresh:<refresh_jti>`
- `identity:oauth:state:<state>`
- `ratelimit:auth:<scope>:...`
- `perm:<workspace_id>:<membership_id>`
- `idem:api:<workspace_id>:<idempotency_key>`
- `cache:sender:domain-auth:v1:<workspace_id>:<domain>`

## Index và truy vấn quan trọng

- `workspace_memberships(workspace_id, user_id)`
- `sessions(user_id, expires_at)`
- `external_auth_accounts(provider, provider_user_id)`
- `external_auth_accounts(user_id, provider)`
- `permission_registry(name)`
- `roles(workspace_id, name)`
- `api_keys(workspace_id, status)`
- `sender_domains(workspace_id, domain, status)`
- `audit_entries(workspace_id, occurred_at desc)`
