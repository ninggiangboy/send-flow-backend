# Architecture Principles

File này giữ các nguyên tắc nền giúp những phần còn lại nhất quán: mục tiêu kiến trúc, boundary giữa runtime/business/technical layer, consistency model, bounded context map và ownership dữ liệu.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Architecture style | `03-architecture-style.md` |
| Bounded contexts | `../modules/01-bounded-contexts.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| Model ownership | `../model/00-model-map.md` |
| Database ownership | `../database/00-database-map.md` |

## Mục tiêu kiến trúc

- Business logic không phụ thuộc HTTP, Kafka, PostgreSQL, Redis hoặc framework.
- API và Worker có orchestration riêng, deploy/runtime riêng, nhưng không duplicate use case.
- Module giao tiếp với nhau qua contract/event rõ ràng.
- Dependency direction đủ đơn giản để code Go không bị import cycle.
- Event publishing quan trọng phải đi qua transactional outbox, không publish trực tiếp sau khi ghi DB.

## Boundary model

```text
Runtime boundary
  API runtime      nhận HTTP, validate transport, gọi use case, trả response
  Worker runtime   nhận event/job, validate message, gọi use case, ack/retry/DLQ

Business boundary
  modules          sở hữu aggregate, use case, business rule, contract công khai

Technical boundary
  platform         cung cấp DB, transaction, Kafka, Redis, config, logging, tracing
  sharedkernel     value object thật sự dùng chung và ổn định
```

Rule quan trọng: runtime biết business, business không biết runtime. Business có thể dùng abstraction, implementation nằm ở platform hoặc infrastructure.

## Consistency model

| Loại thay đổi | Consistency | Cách làm |
|---|---|---|
| Thay đổi một aggregate trong một module | Strong consistency | PostgreSQL transaction qua `ports.TxManager` |
| Ghi business state và phát event | Atomic write, eventual publish | Transactional outbox |
| Giao tiếp giữa module | Eventual consistency | Event contract + idempotent consumer |
| Projection, analytics, reporting | Eventually consistent | Kafka consumer + PostgreSQL projection hoặc ClickHouse |
| Cache/session/rate limit | Ephemeral consistency | Redis, luôn có TTL và fallback rõ |

Không giả định Kafka exactly-once ở business level. Consumer phải idempotent.

## SaaS workspace model

`send-flow` là SaaS nhiều workspace:

- `User` là identity global để đăng nhập.
- `Workspace` là boundary ownership của product data.
- `WorkspaceMembership` nối `user_id` với `workspace_id`.
- Role/permission nằm trên membership hoặc role assignment trong workspace, không nằm global trên user.
- Một user có thể thuộc nhiều workspace và có role khác nhau ở từng workspace.

Request context tối thiểu sau authentication:

```text
actor_user_id
workspace_id
membership_id
effective_permissions_mask
request_id
```

Rule quan trọng:

- Product data mặc định phải có `workspace_id`: sender domains, API keys, contacts, lists, templates, campaigns, messages, webhooks, logs, analytics projections.
- Global tables chỉ dành cho thứ thật sự global: user credential, global session metadata, permission registry.
- Repository query product data phải filter theo `workspace_id`.
- Unique constraint thường phải scoped theo workspace, ví dụ `(workspace_id, email)` hoặc `(workspace_id, domain)`.
- Event public cho product data phải có `workspace_id` trong envelope hoặc payload.
- Job/worker khi xử lý event phải propagate `workspace_id` vào log, metrics, cache key, rate-limit key và audit.

## Bounded context map

| Context | Sở hữu | Có thể publish | Có thể consume |
|---|---|---|---|
| `identity` | user, credential, session, workspace, membership, role, permission | user registered, workspace created, member invited, member role changed, session revoked | workspace/user policy event nếu có |
| `apiaccess` | workspace API key, API key scope, public API credential audit | api key created, api key revoked | identity/member role changed |
| `sender` | workspace sending domain, DNS verification, provider credential metadata | sender domain verified, provider disabled | workspace settings changed |
| `audience` | contact, list, segment, import/export job, audience membership | contact imported, segment changed, audience import completed | suppression changed nếu cần projection eligibility |
| `template` | template source, rendered snapshot, template version | template published | asset/user/team event nếu có |
| `campaign` | campaign, audience selection, send plan | campaign scheduled, message queued | template published, sender verified |
| `transactional` | transactional send request, API request idempotency, template/raw send command | transactional message queued | api key revoked, template published, sender verified |
| `delivery` | message delivery state, provider dispatch, retry state, queue/retry state | message accepted/delivered/bounced/complained | campaign message queued, transactional message queued, suppression changed |
| `suppression` | unsubscribe, bounce/complaint suppression | recipient suppressed, recipient unsuppressed | provider webhook normalized |
| `webhook` | customer webhook endpoint, signing secret, outbound webhook delivery state | customer webhook delivery failed/succeeded | delivery/tracking/suppression events |
| `analytics` | report projection, aggregate metrics | normally no domain command event | delivery/tracking/suppression events |
| `tracking` | open/click events, tracking redirect state | email opened, link clicked | message delivered/sent snapshot |
| `audit` | audit trail for user/admin/developer/settings actions | audit entry recorded | identity/apiaccess/sender/settings events |
| `settings` | workspace settings, general settings, feature/config choices | workspace settings changed | identity/admin action events |

Bảng này là bản đồ định hướng, không bắt buộc phải tạo tất cả module ngay từ đầu. Module chỉ nên xuất hiện khi có state/rule đủ độc lập.

## Data ownership

- Module nào sở hữu aggregate thì module đó sở hữu bảng transaction chính của aggregate.
- Module khác không update trực tiếp bảng đó; nếu cần thay đổi state, gọi use case hoặc publish command/event đã thống nhất.
- Projection/read model có thể duplicate dữ liệu từ context khác, nhưng phải coi là bản sao eventually consistent.
- Cross-module foreign key nên dùng cẩn thận. Trong modular monolith có thể dùng FK nội bộ khi cùng database, nhưng đừng để FK làm mờ ownership.
- Với product data, `workspace_id` là ownership boundary mặc định. Nếu một bảng không có `workspace_id`, phải có lý do rõ vì sao nó global.
- Không truyền mỗi `user_id` xuống repository cho product query; truyền `workspace_id` hoặc request/workspace context rõ ràng.

## Identity permissions model

Identity dùng **binary permissions** cho RBAC workspace-scoped: mỗi permission là một bit ổn định, role trong workspace giữ permission mask, membership của user trong workspace nhận role, và effective permission được tính theo workspace context của request. API nên stateless theo request-scoped context thay vì server-side switching. Cách này nhanh, dễ cache và hợp với check permission lặp lại nhiều trong API.

Ví dụ permission registry:

```text
Permission bit  Name
0               campaign.read
1               campaign.write
2               campaign.send
3               template.read
4               template.write
5               audience.read
6               audience.write
7               settings.manage
8               api_key.manage
9               audit.read
```

Rule quan trọng:

- Permission bit là contract ổn định. Đã release thì không đổi nghĩa, không reorder, không reuse bit đã bỏ.
- Permission name vẫn phải tồn tại để audit/log/UI hiển thị; bitmask chỉ là representation để check nhanh.
- Nếu dùng PostgreSQL `BIGINT`, tránh dùng sign bit và chỉ dùng bit `0..62`; nếu cần nhiều hơn, tách mask theo domain hoặc dùng representation khác như `NUMERIC`, `bytea`, hoặc bảng permission mapping.
- Không hardcode magic number rải rác. Tất cả bit phải nằm trong registry/constant ở `identity/domain` hoặc generated từ một source rõ ràng.
- Role có `permissions_mask`; user effective permission = OR mask từ các role active của membership trong workspace context của request.
- Permission luôn scoped theo workspace. Không cache permission chỉ theo `user_id`.
- Access token/session snapshot có thể chứa `membership_id`, `role_version` hoặc `permission_version`; với action nhạy cảm nên re-check từ cache/store để role revocation có hiệu lực nhanh.
- Redis cache permission phải có TTL và invalidation khi `role changed`, `permission changed`, `user role assigned/revoked`.
- Mọi thay đổi role/permission/API key phải ghi audit log.

Gợi ý package:

```text
internal/modules/identity/domain
├── permission.go        # Permission type, bit constants, registry
├── permission_set.go    # mask operations: Has, Add, Remove, Union
├── role.go
└── user.go
```

Gợi ý DB tối thiểu:

```text
roles
├── id
├── workspace_id
├── name
├── permissions_mask
├── version
└── timestamps

workspace_memberships
├── id
├── workspace_id
├── user_id
├── status
└── timestamps

membership_roles
├── workspace_id
├── membership_id
└── role_id
```

Invite flow cần model riêng:

```text
workspace_invitations
├── workspace_id
├── email
├── invited_by_user_id
├── role_ids
├── token_hash
├── expires_at
└── accepted_at
```

Event nên có:

```text
identity.workspace.created.v1
identity.workspace.member_invited.v1
identity.workspace.member_joined.v1
identity.workspace.member_roles_changed.v1
identity.role.permissions_changed.v1
```

Các event này giúp API/Worker invalid permission cache, audit projection và các runtime khác phản ứng với thay đổi quyền trong workspace.

## Decision pressure

Khi một quyết định kiến trúc bị mơ hồ, ưu tiên theo thứ tự:

1. Giữ business invariant trong module owner.
2. Giữ transaction boundary nhỏ và rõ.
3. Publish event qua outbox nếu state change cần side effect async.
4. Làm consumer idempotent trước khi tối ưu throughput.
5. Chỉ tách runtime/service khi có lý do scale, isolation hoặc vận hành rõ ràng.
