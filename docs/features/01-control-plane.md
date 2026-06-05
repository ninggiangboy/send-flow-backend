# Control Plane Features

## Mục tiêu nhóm feature

Nhóm này bao phủ toàn bộ feature quản trị danh tính, tenant/workspace, quyền truy cập và cấu hình nền để các nhóm feature gửi email hoạt động đúng theo phạm vi workspace.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| API contract | `../api/01-auth-and-control-plane.md` |
| Business model | `../model/01-identity-and-access.md` |
| Database schema | `../database/01-core-identity-and-access.md` |
| Module owner | `../modules/03-module-catalog.md` |
| Boundary rule | `../modules/01-bounded-contexts.md` |

## Feature chính: Authentication và session

### Sub-feature / capability chi tiết

- Đăng ký tài khoản người dùng.
- Đăng nhập và thiết lập identity toàn cục.
- Đăng nhập bằng external identity provider như Google, GitHub.
- Hỗ trợ OAuth2/OIDC start, callback/exchange, `state` validation và PKCE khi client flow yêu cầu.
- Liên kết external account vào user nội bộ hoặc tự provision user mới theo policy.
- Quản lý session đang hoạt động.
- Revoke session nhanh khi role hoặc access bị thu hồi.
- Rate limit cho auth endpoint để giảm abuse.
- Password hashing và secret handling cho credential.
- Request context sau authentication gồm `actor_user_id`, `workspace_id`, `membership_id`, `effective_permissions_mask`, `request_id`.

### Actor sử dụng

- Dashboard user
- Public API client trong phần auth/API access
- Operator khi cần revoke hoặc audit

### Input / output hoặc hành vi chính

- Nhận credential hoặc token.
- Nhận authorization code/identity token từ provider ngoài.
- Xác thực danh tính và resolve membership/workspace theo dữ liệu request.
- Trả session/token cùng identity snapshot; permission hiệu lực được tính theo workspace context của từng request.

### Phụ thuộc quan trọng

- Identity store
- Session store/cache
- Permission registry
- Secret/config provider
- OAuth2/OIDC provider config

### Event / side effect nổi bật

- `identity.user.registered.v1`
- `identity.user.external_account_linked.v1`
- `session revoked`
- Invalidate permission/session cache khi role thay đổi

### Ghi chú phạm vi

Feature này vừa user-facing vừa là capability nền vì toàn bộ product data đều bị ràng buộc bởi workspace context. Với social sign-in, ưu tiên provider trả email đã verify để giảm rủi ro chiếm dụng account.

## Feature chính: Workspace và membership

### Sub-feature / capability chi tiết

- Tạo workspace.
- Một user thuộc nhiều workspace.
- Chọn workspace trong UI và truyền workspace context theo request khi thao tác sản phẩm.
- Mời user vào workspace.
- Join workspace qua invitation token.
- Quản lý membership status.
- Gán role cho membership thay vì gán global trên user.
- Scope toàn bộ product data theo `workspace_id`.

### Actor sử dụng

- Workspace admin
- Member được mời
- Operator

### Input / output hoặc hành vi chính

- Tạo hoặc cập nhật quan hệ giữa user và workspace.
- Trả membership và quyền hiệu lực trong workspace hiện tại.

### Phụ thuộc quan trọng

- Identity domain
- Invitation flow
- Role assignment
- Audit log

### Event / side effect nổi bật

- `identity.workspace.created.v1`
- `identity.workspace.member_invited.v1`
- `identity.workspace.member_joined.v1`

### Ghi chú phạm vi

Đây là tenant model lõi của SaaS; mọi feature sản phẩm khác đều phụ thuộc vào boundary này.

## Feature chính: Roles và permissions

### Sub-feature / capability chi tiết

- Workspace-scoped RBAC.
- Binary permission registry ổn định theo bitmask.
- Role có `permissions_mask`.
- Membership có thể mang nhiều role.
- Tính effective permission theo workspace context của request.
- Cache permission có TTL.
- Invalidate permission cache khi role, permission hoặc assignment thay đổi.
- Re-check permission cho action nhạy cảm.
- Audit mọi thay đổi role/permission.

### Actor sử dụng

- Workspace admin
- Operator

### Input / output hoặc hành vi chính

- Nhận thay đổi role hoặc permission assignment.
- Tính lại quyền hiệu lực cho membership trong workspace.

### Phụ thuộc quan trọng

- Permission registry
- Membership model
- Cache/session layer
- Audit trail

### Event / side effect nổi bật

- `identity.workspace.member_roles_changed.v1`
- `identity.role.permissions_changed.v1`
- Permission cache invalidation

### Ghi chú phạm vi

Feature này user-facing ở màn hình settings nhưng cũng là capability nền để bảo vệ mọi API và worker action.

## Feature chính: API access và credential cho public API

### Sub-feature / capability chi tiết

- Tạo API key theo workspace.
- Scope API key theo permission hoặc use case.
- Revoke API key.
- Audit vòng đời credential.
- Ràng buộc API key với workspace và permission model.
- Cho phép public API client gửi transactional email hoặc quản lý resource.

### Actor sử dụng

- Developer của tenant
- Workspace admin
- Operator

### Input / output hoặc hành vi chính

- Tạo, hiển thị metadata, xoay vòng hoặc thu hồi API credential.
- Áp dụng permission tương ứng khi public API gọi vào hệ thống.

### Phụ thuộc quan trọng

- Identity/workspace model
- Permission model
- Audit logging
- Secret handling

### Event / side effect nổi bật

- `api key created`
- `api key revoked`
- Audit entry recorded

### Ghi chú phạm vi

Đây là feature developer-facing, là cửa vào của transactional messaging và automation bên ngoài.

## Feature chính: Sender identity và sending domain

### Sub-feature / capability chi tiết

- Thêm sending domain theo workspace.
- Sinh record DNS cần thiết.
- Theo dõi trạng thái SPF, DKIM, DMARC.
- Kiểm tra DNS định kỳ.
- Lưu `expected_value`, `current_value`, `status`, `last_checked_at`, `failure_reason`.
- Chỉ cho phép gửi production traffic khi domain đạt authentication tối thiểu.
- Lưu metadata credential/provider liên quan đến sender.
- Pause sending khi domain auth fail hoặc provider disable.

### Actor sử dụng

- Workspace admin
- Deliverability operator

### Input / output hoặc hành vi chính

- Nhận domain mới.
- Trả hướng dẫn cấu hình DNS và trạng thái xác minh.
- Cập nhật sender readiness cho phần delivery.

### Phụ thuộc quan trọng

- DNS verification job
- Provider integration
- Delivery policy
- Workspace settings

### Event / side effect nổi bật

- `sender.domain.verified.v1`
- `provider disabled`
- Audit sender configuration changes

### Ghi chú phạm vi

Feature này user-facing ở settings nhưng đồng thời là deliverability control quan trọng.

## Feature chính: Customer webhook configuration

### Sub-feature / capability chi tiết

- Khai báo endpoint webhook của khách hàng.
- Quản lý signing secret.
- Theo dõi outbound webhook delivery state.
- Tách biệt webhook của khách hàng với provider webhook inbound.
- Cho phép downstream system nhận event delivery/tracking/suppression.

### Actor sử dụng

- Developer của tenant
- Workspace admin
- Operator

### Input / output hoặc hành vi chính

- Nhận cấu hình endpoint và secret.
- Phát outbound event tới endpoint khách hàng theo contract đã chọn.

### Phụ thuộc quan trọng

- Event source từ delivery/tracking/suppression
- Retry and DLQ for webhook delivery
- Secret storage

### Event / side effect nổi bật

- `customer webhook delivery succeeded`
- `customer webhook delivery failed`

### Ghi chú phạm vi

Feature này thuộc control plane vì nó là cấu hình tích hợp, còn việc thực thi delivery/retry thuộc operations.

## Feature chính: Workspace settings và feature controls

### Sub-feature / capability chi tiết

- Quản lý general settings của workspace.
- Lưu lựa chọn feature/config theo tenant.
- Feature flags theo environment hoặc tenant.
- Delivery policy hoặc behavior toggle ở mức workspace.
- Quản lý giới hạn hoặc lựa chọn vận hành liên quan đến workspace.

### Actor sử dụng

- Workspace admin
- Operator / developer

### Input / output hoặc hành vi chính

- Nhận thay đổi setting.
- Phản ánh setting vào API, worker và capability downstream.

### Phụ thuộc quan trọng

- Settings store
- Feature flag evaluator
- Audit log

### Event / side effect nổi bật

- `workspace settings changed`
- Config cache invalidation

### Ghi chú phạm vi

Nhiều setting là user-facing, nhưng feature flag và config propagation là capability vận hành của sản phẩm.

## Feature chính: Audit trail

### Sub-feature / capability chi tiết

- Ghi nhận hành động nhạy cảm của user/admin/developer.
- Audit thay đổi role, permission, API key, sender, settings.
- Giữ audit entry cho review và điều tra.
- Làm nguồn dữ liệu cho màn hình audit logs.

### Actor sử dụng

- Workspace admin
- Operator
- Security/compliance stakeholder

### Input / output hoặc hành vi chính

- Nhận action hoặc event từ nhiều nhóm feature.
- Ghi audit entry có actor, workspace, loại thay đổi và thời điểm.

### Phụ thuộc quan trọng

- Identity context
- Event pipeline
- Read model cho audit logs

### Event / side effect nổi bật

- `audit entry recorded`

### Ghi chú phạm vi

Đây là feature xuyên suốt toàn hệ thống, nhưng nơi sở hữu chính nằm ở control plane vì nó gắn chặt với identity và quản trị.
