# Module Communication And Public API

File này định nghĩa module trong modular monolith được phép giao tiếp với nhau bằng gì.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Module catalog | `03-module-catalog.md` |
| Bounded context rule | `01-bounded-contexts.md` |
| Code architecture | `../architecture/04-code-architecture.md` |
| Event/outbox flow | `../architecture/05-data-events-and-flows.md` |
| API map | `../api/00-api-map.md` |

## Public API của module

Public API của một module gồm:

| Thành phần | Package gợi ý | Người dùng |
|---|---|---|
| Application command/query handler | `internal/modules/<module>/app/<usecase>` | API runtime, Worker runtime, module khác qua interface đã wire |
| Public query/read interface | `internal/modules/<module>/app/public` hoặc `contracts` nếu chỉ là read contract | Module khác cần decision/read model ổn định |
| Integration event | `internal/modules/<module>/contracts` | Outbox publisher, worker consumer, module khác |
| Event topic/name/schema | `internal/modules/<module>/contracts` | Producer/consumer |
| Error code ổn định | `app` hoặc `contracts` | API mapper, caller module |

Không coi các phần sau là public API:

- `domain` của module khác.
- `infrastructure` của module khác.
- SQL table private của module khác.
- Redis key private của module khác.
- DTO HTTP trong `internal/apps/api/modules/<module>`.

## Rule import

Trong monolith, compile-time import vẫn cần được kiểm soát như service boundary.

Allowed:

```text
internal/apps/api
  -> internal/modules/<module>/app
  -> internal/modules/<module>/contracts

internal/apps/worker
  -> internal/modules/<module>/app
  -> internal/modules/<module>/contracts

internal/modules/<module>/app
  -> internal/modules/<module>/domain
  -> internal/modules/<module>/ports
  -> internal/sharedkernel
```

Restricted:

```text
internal/modules/a
  -x-> internal/modules/b/domain
  -x-> internal/modules/b/infrastructure
  -x-> internal/apps/api/modules/b
```

Nếu module A cần dữ liệu hoặc quyết định từ module B, dùng một trong các cách sau:

| Nhu cầu | Cách giao tiếp |
|---|---|
| Cần quyết định ngay trong command path | Public query/service interface của B, wire qua composition root |
| Cần phản ứng sau khi state của B đổi | Consume integration event của B |
| Cần dữ liệu để hiển thị dashboard | Projection/read model, owner rõ |
| Cần side effect dài hoặc retryable | Publish event hoặc enqueue job qua outbox |

## Synchronous call

Synchronous call được phép khi command không thể ra quyết định nếu thiếu câu trả lời ngay.

Ví dụ hợp lý:

- `campaign` hỏi `sender`: sender domain đã verified chưa?
- `campaign` hỏi `content`: template version có published/renderable không?
- `delivery` hỏi `suppression`: recipient hiện có bị suppress không?
- `delivery` hỏi `content`: render template snapshot cho message.

Rule:

- Gọi qua interface đặt ở module caller hoặc public app API của module callee.
- Caller không import repository/domain/infrastructure của callee.
- Callee trả DTO/decision nhỏ, không expose aggregate private.
- Transaction không nên bao trùm nhiều module nếu sau này muốn tách service.
- Nếu call có thể chậm hoặc fail độc lập, cân nhắc async/projection/cache.

Ví dụ interface ở caller:

```go
type SenderReadinessChecker interface {
    IsSenderReady(ctx context.Context, workspaceID sharedkernel.WorkspaceID, senderID SenderID) (SenderReadiness, error)
}
```

Composition root wire implementation từ module `sender` vào module `campaign`.

## Asynchronous event

Async event dùng khi module khác chỉ cần biết sự thật đã xảy ra và có thể xử lý eventually consistent.

Ví dụ:

| Producer | Event | Consumer |
|---|---|---|
| `identity` | `identity.workspace.created.v1` | `audit`, `analytics`, setup defaults |
| `sender` | `sender.domain.verified.v1` | `campaign`, `delivery`, `audit` |
| `content` | `content.template.published.v1` | `campaign`, `analytics` |
| `campaign` | `campaign.scheduled.v1` | `delivery`, `analytics`, `audit` |
| `delivery` | `delivery.message.queued.v1` | `analytics`, `webhooks` |
| `delivery` | `delivery.message.delivered.v1` | `analytics`, `webhooks` |
| `delivery` | `delivery.message.bounced.v1` | `suppression`, `analytics`, `webhooks` |
| `suppression` | `suppression.recipient.suppressed.v1` | `delivery`, `analytics`, `webhooks` |
| `tracking` | `tracking.link.clicked.v1` | `analytics`, `webhooks` |

Rule event contract:

- Event name là fact đã xảy ra, không phải command.
- Event type phải versioned.
- Payload chỉ chứa dữ liệu consumer cần và có thể ổn định.
- Consumer phải idempotent theo `event_id`.
- Producer không biết consumer cụ thể nào đang chạy.
- Consumer không được yêu cầu producer update event cũ theo schema private của consumer.

## Public read model

Public read model dùng khi nhiều module/screen cần đọc dữ liệu đã tổng hợp.

Ví dụ:

| Read model | Owner | Người đọc |
|---|---|---|
| Current workspace/session view | `identity` | API runtime, dashboard modules |
| Sender readiness view | `sender` | `campaign`, `delivery` |
| Template published/render preview view | `content` | `campaign`, `delivery`, dashboard |
| Campaign status view | `campaign` | dashboard, analytics |
| Message log projection | `delivery` hoặc `analytics` tùy storage | dashboard, support/operator |
| Suppression lookup view | `suppression` | `delivery`, audience UI |
| Dashboard overview | `analytics` | dashboard |

Read model có thể được implement bằng query trực tiếp trong monolith, nhưng contract nên không phụ thuộc table private để sau này thay bằng HTTP/gRPC hoặc replicated projection.

## Error và idempotency contract

Public API của module nên trả error ổn định theo dạng:

```text
<module>.<reason>
```

Ví dụ:

- `identity.permission_denied`
- `sender.domain_not_verified`
- `content.template_not_published`
- `campaign.invalid_state_transition`
- `delivery.idempotency_key_conflict`
- `suppression.recipient_suppressed`

Command quan trọng cần idempotency:

- Transactional send.
- Provider webhook ingest.
- Recipient unsubscribe.
- Customer webhook delivery.
- Import/export job trigger.
- Replay/DLQ operation.

## Microservice extraction checklist

Trước khi tách một module thành service riêng, kiểm tra:

- Module khác chỉ dùng public command/query/event contract.
- Không có import vào `domain`/`infrastructure` của module đó.
- Không có write trực tiếp vào bảng của module đó từ module khác.
- Event payload có version và consumer idempotent.
- Public query có thể đổi từ in-process call sang network call.
- Có timeout/retry/circuit breaker cho synchronous dependency.
- Có migration ownership và seed/config riêng.
- Có metric/log/trace theo module boundary.
