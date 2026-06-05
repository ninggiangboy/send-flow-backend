# Architecture Style

Backend dùng bốn ý tưởng chính: **Clean Architecture**, **DDD nhẹ**, **Modular Monolith** và **Event-driven**. File này định nghĩa cách hiểu thực dụng cho `send-flow`, để team không áp dụng các pattern này quá máy móc.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Architecture principles | `02-architecture-principles.md` |
| Code architecture | `04-code-architecture.md` |
| Module map | `../modules/00-module-map.md` |
| Module communication | `../modules/02-communication-and-public-api.md` |
| Event/outbox flow | `05-data-events-and-flows.md` |

## Executive summary

| Style | Dùng để giải quyết | Cách áp dụng trong project |
|---|---|---|
| Clean Architecture | Business logic không phụ thuộc framework/hạ tầng | Dependency đi vào trong: API/Worker gọi use case, use case gọi domain/ports, infrastructure implement ports |
| DDD nhẹ | Đặt business rule đúng chỗ, tránh service layer toàn transaction script | Aggregate/entity/value object cho invariant quan trọng; bounded context theo capability |
| Modular Monolith | Giữ deploy đơn giản nhưng code có boundary rõ | Một repo/một binary set ban đầu, nhiều module trong `internal/modules`, giao tiếp qua contract/use case/event |
| Event-driven | Tách side effect dài và cross-module propagation | Transactional outbox, Kafka event, idempotent consumer, projection eventually consistent |

## Clean Architecture

Clean Architecture trong project này không có nghĩa là tạo thật nhiều layer trừu tượng. Nó có nghĩa là business rule không bị phụ thuộc vào HTTP, Kafka, PostgreSQL, Redis hoặc framework.

Dependency direction:

```text
apps/api or apps/worker
  -> modules/<module>/app
     -> modules/<module>/domain
     -> modules/<module>/ports
        <- modules/<module>/infrastructure or platform implementations
```

Mapping vào package:

| Clean Architecture concept | Package |
|---|---|
| Interface adapter inbound | `internal/apps/api/modules/<module>`, `internal/apps/worker/modules/<module>` |
| Application/use case | `internal/modules/<module>/app/<usecase>` |
| Enterprise/domain rule | `internal/modules/<module>/domain` |
| Outbound port | `internal/modules/<module>/ports` hoặc domain repository interface |
| Outbound adapter | `internal/modules/<module>/infrastructure`, `internal/platform/*` |
| Composition root | `internal/apps/api/bootstrap.go`, `internal/apps/worker/bootstrap.go`, `module.go` |

Rule thực tế:

- Handler/consumer decode input, validate transport shape, map sang command/query rồi gọi use case.
- Use case orchestrate transaction, gọi domain behavior, gọi port, map domain error sang app error.
- Domain không biết JSON, SQL, Kafka topic, Redis key hoặc HTTP status.
- Infrastructure có thể biết DB/Kafka/Redis SDK, nhưng không quyết định business invariant.
- Interface đặt ở nơi dùng. Đừng tạo package `interfaces` chung chỉ vì muốn “clean”.

Ví dụ flow đúng:

```text
HTTP POST /campaigns
  -> api campaign handler
  -> campaign/app/createcampaign.Handler
  -> campaign/domain.Campaign.Schedule(...)
  -> campaign repository port
  -> outbox port
  -> postgres/outbox infrastructure
```

Anti-pattern:

```text
api handler
  -> postgres campaign table
  -> kafka producer
  -> if/else business state transition
```

Nếu handler đang tự quyết định state transition, code đó đang vượt boundary.

## DDD nhẹ

DDD ở đây là **domain modeling vừa đủ**, không phải áp dụng toàn bộ tactical DDD ceremony.

Dùng DDD mạnh hơn khi có:

- Invariant cần bảo vệ, ví dụ campaign không được send nếu sender domain chưa verified.
- State transition có rule, ví dụ message queued -> accepted -> delivered/bounced.
- Value object có validation rõ, ví dụ email address, tenant id, provider message id.
- Domain event cần sinh từ thay đổi state, ví dụ `CampaignScheduled`, `MessageBounced`.

Không cần DDD nặng cho:

- CRUD settings đơn giản.
- Admin list/search/filter không có invariant phức tạp.
- Projection/report read model.
- Adapter thuần kỹ thuật như config loader hoặc logging.

Tactical building blocks:

| Building block | Khi dùng | Ví dụ |
|---|---|---|
| Value object | Cần validation/meaning ổn định | `EmailAddress`, `TenantID`, `DomainName` |
| Entity | Có identity và lifecycle | `Contact`, `Template`, `Message` |
| Aggregate | Cần transaction boundary và invariant | `Campaign`, `SenderDomain`, `SuppressionEntry` |
| Domain service | Rule thật sự không thuộc một aggregate | suppression eligibility checker nếu cần nhiều source |
| Repository | Load/save aggregate hoặc query domain cần thiết | `CampaignRepository`, `SenderDomainRepository` |
| Domain event | Sự thật đã xảy ra trong domain | `SenderDomainVerified`, `MessageQueued` |

Aggregate rule:

- Một use case chỉ nên thay đổi một aggregate root chính trong một transaction.
- Nếu cần phản ứng ở module khác, publish event thay vì update trực tiếp state của module đó.
- Aggregate không gọi repository, Kafka producer hoặc Redis client.
- Domain event được record trong aggregate/use case, rồi persist qua outbox trong cùng transaction.

## Bounded context

Bounded context là boundary của ngôn ngữ và ownership, không chỉ là folder.

Ví dụ cùng chữ `Message` có thể mang nghĩa khác nhau:

| Context | Meaning |
|---|---|
| `campaign` | planned message candidate từ audience selection |
| `delivery` | provider dispatch state và retry lifecycle |
| `analytics` | event fact hoặc metric row |

Quy tắc:

- Context owner quyết định schema transaction chính của mình.
- Context khác dùng event hoặc public read model, không update table owner trực tiếp.
- Nếu hai context cần cùng dữ liệu, cân nhắc projection thay vì shared table.
- Tên event nên dùng ngôn ngữ của context producer.

## Modular Monolith

Project bắt đầu là modular monolith vì nó cho phép:

- Một repo, một deployment story đơn giản hơn microservice.
- Transaction nội bộ rõ khi cần strong consistency.
- Refactor module boundary rẻ hơn khi domain còn đang học.
- Vẫn giữ đường tách service sau này nếu boundary và contract sạch.

Điều kiện để modular monolith không biến thành big ball of mud:

- `internal/modules/<module>` không import infrastructure của module khác.
- Cross-module call synchronous phải đi qua interface/use case đã được composition root wire rõ.
- Cross-module async nên đi qua event trong `contracts`.
- Shared kernel nhỏ và ổn định.
- Mỗi module có owner của migration/schema/event contract.

Khi nào tách thành service riêng:

- Module cần scale hoặc deploy cadence khác hẳn.
- Failure của module đó cần isolate khỏi phần còn lại.
- Resource profile khác, ví dụ analytics ingestion nặng CPU/IO.
- Boundary/event contract đã ổn định đủ lâu.
- Observability/runbook đã đủ để vận hành độc lập.

Không tách service chỉ vì folder đã là module.

## Event-driven architecture

Event-driven dùng cho propagation và side effect async, không thay thế toàn bộ use case synchronous.

Event types:

| Loại event | Ý nghĩa | Nơi đặt |
|---|---|---|
| Domain event | Sự thật nội bộ sinh ra từ aggregate/use case | `domain` hoặc mapping nội bộ |
| Integration event | Contract public để runtime/module khác consume | `contracts` |
| Provider event | Event external từ email provider webhook | API ingress DTO + normalized internal event |
| Analytics event | Event/projection phục vụ reporting | analytics consumer/schema |

Domain event có thể giàu ngữ nghĩa nội bộ. Integration event phải ổn định, versioned và backward-compatible.

Event publishing rule:

```text
use case transaction
  -> save aggregate state
  -> save outbox integration event
  -> commit

outbox publisher
  -> publish Kafka event
  -> mark outbox row published

hoặc Debezium/Kafka Connect
  -> capture outbox row từ WAL
  -> route sang business topic

consumer
  -> dedupe by event_id
  -> handle idempotently
  -> ack or retry/DLQ
```

Không publish Kafka trực tiếp trong use case sau khi ghi DB. Nếu DB commit thành công nhưng publish fail, hệ thống mất event.

Trong local/dev stack của repo này, Debezium là lựa chọn mặc định để bridge `outbox_events` sang Kafka. Poller riêng vẫn là fallback hợp lệ khi môi trường chưa có Kafka Connect hoặc cần replay logic riêng.

## Event naming

Event nên mô tả sự thật đã xảy ra, không phải command mong muốn.

Good:

```text
campaign.scheduled.v1
delivery.message.queued.v1
delivery.message.bounced.v1
suppression.recipient.suppressed.v1
sender.domain.verified.v1
```

Avoid:

```text
send_email_now
update_campaign_status
process_webhook
do_retry
```

Command là intent. Event là fact.

## Read models and projections

Event-driven không có nghĩa mọi query đọc thẳng từ Kafka. Dashboard và operations screen nên đọc từ read model/projection phù hợp.

| Screen | Read model/projection |
|---|---|
| Dashboard | campaign/delivery aggregate metrics |
| Email Logs | delivery message log projection |
| Queue / Retry | outbox + delivery retry state + DLQ projection |
| Bounces / Complaints | provider event/suppression projection |
| Analytics | ClickHouse aggregate/materialized view |
| Deliverability | sender/domain/provider health projection |

Projection là bản sao eventually consistent. Nếu UI cần hiển thị trạng thái “đang xử lý”, API nên trả rõ `pending`, `processing`, `last_updated_at`.

## Practical checklist

Khi thêm một feature:

- Feature thuộc bounded context nào?
- Có aggregate/invariant thật không, hay chỉ là CRUD/projection?
- Command synchronous cần trả gì ngay cho API?
- Có side effect async nào cần outbox event không?
- Event public nào cần version trong `contracts`?
- Consumer nào cần idempotency key?
- Read model nào phục vụ UI?
- Module nào sở hữu bảng transaction và migration?

Nếu trả lời được các câu này, feature thường đã nằm đúng theo Clean Architecture + DDD nhẹ + Event-driven.
