# Testing Guide

File này là source of truth cho cách viết test trong `send-flow`.

Mục tiêu:

- Giúp người implement biết nên test gì ở layer nào.
- Giúp reviewer đánh giá test coverage theo rủi ro thay vì theo số lượng test.
- Giữ cách viết test nhất quán với kiến trúc `Clean Architecture`, `DDD nhẹ`, `Modular Monolith` và `Event-driven`.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Documentation graph | `../00-doc-graph.md` |
| Implementation note | `../00-implementation-note.md` |
| Engineering practices | `../architecture/09-engineering-practices.md` |
| Code architecture | `../architecture/04-code-architecture.md` |
| Event/outbox flow | `../architecture/05-data-events-and-flows.md` |
| Infrastructure stack | `../architecture/06-infrastructure.md` |

## Mục tiêu và phạm vi

Docs này dùng khi:

- Bắt đầu implement feature mới.
- Viết hoặc review test cho một module/use case/adapter.
- Cần quyết định nên đặt effort test vào `unit`, `integration` hay `e2e`.
- Cần verify một flow có DB, outbox, worker, webhook hoặc external provider.

Docs này không thay thế:

- Verification checklist trong `../00-implementation-note.md`
- Quyết định kiến trúc trong `../architecture/*.md`
- Product flow/spec trong `../features`, `../api`, `../model`, `../database`

Khác biệt chính:

- `Verification checklist` trả lời: feature này đã đủ an toàn để merge/chạy chưa.
- `Testing guide` trả lời: nên viết test thế nào, ở đâu, với độ sâu nào.

## Testing principles

Nguyên tắc mặc định:

- Test theo behavior và risk, không test chỉ để tăng coverage số học.
- Test nên bám boundary thật của hệ thống: `domain`, `app`, `infrastructure`, `api`, `worker`.
- Domain thuần không mock nếu có thể giữ pure.
- App/use case được phép mock repository/ports để test orchestration.
- Integration test dùng dependency thật ở chỗ mapping/transaction/query/order/retry có rủi ro cao.
- E2E chỉ giữ cho vài flow critical, không thay unit/integration.

Những điều nên ưu tiên bắt lỗi:

- Invariant bị phá.
- State transition sai.
- Transaction không atomic.
- Event publish sai thời điểm.
- Consumer không idempotent.
- HTTP/worker mapping sai contract.
- Retry/DLQ/rollback không đúng kỳ vọng.

Những điều không nên là mục tiêu chính:

- Snapshot mọi branch implementation detail.
- Mock mọi thứ chỉ để test pass nhanh.
- Dùng e2e cho các case có thể bắt rẻ hơn ở unit/integration.
- Gộp quá nhiều assertion không liên quan vào một test.

## Test pyramid cho repo

Baseline của repo:

```text
unit          ~80%
integration  ~15%
e2e           ~5%
```

Tỷ lệ này là guideline, không phải quota cứng. Nếu một feature có nhiều rủi ro DB/outbox/consumer, phần integration có thể nhiều hơn.

### Unit test

Bắt lỗi nhanh nhất và rẻ nhất cho:

- Invariant.
- State transition.
- Validation.
- Error mapping.
- Decision logic.
- Idempotency decision ở mức use case.

### Integration test

Bắt lỗi boundary thật cho:

- SQL query/mapping/index/constraint.
- Transaction behavior.
- Outbox persistence.
- Kafka/Redis/PostgreSQL adapter.
- External adapter mapping với fake/local dependency.

### End-to-end test

Bắt lỗi wiring và flow critical cho:

- API -> DB -> outbox -> worker -> side effect.
- Webhook -> normalize -> state update.
- Retry/idempotency path quan trọng.

## Test theo layer

### Domain

Mục tiêu:

- Bảo vệ invariant và lifecycle của aggregate/entity/value object.
- Xác nhận domain event được phát đúng khi state đổi.

Nên test:

- Value object validation như `EmailAddress`, `TenantID`, `DomainName`.
- Aggregate transition như `Schedule`, `Activate`, `Suppress`, `Retry`.
- Domain error khi input hợp lệ về mặt transport nhưng sai về mặt business.
- Domain event emission khi hành vi thành công.

Không nên test:

- JSON DTO.
- SQL query.
- HTTP status.
- Kafka topic name ở level domain.

Ví dụ vị trí:

```text
internal/modules/identity/domain/user_test.go
internal/modules/delivery/domain/message_test.go
```

### App / use case

Mục tiêu:

- Test orchestration giữa domain, repository, `TxManager`, outbox và các port.

Nên test:

- Command/query validation ở mức application nếu có.
- Transaction bắt đầu và rollback đúng khi repository/outbox fail.
- Error mapping từ domain/infrastructure sang app error.
- Idempotency decision nếu use case là nơi quyết định.
- Chỉ ghi outbox sau khi aggregate thay đổi hợp lệ.

Dùng mock/fake:

- Repository/ports.
- Clock.
- ID generator.
- Provider gateway.

Ví dụ vị trí:

```text
internal/modules/identity/app/signup/handler_test.go
internal/modules/campaign/app/schedulecampaign/handler_test.go
```

### Infrastructure

Mục tiêu:

- Xác nhận adapter kỹ thuật hoạt động đúng với dependency thật.

Nên test:

- Repository CRUD/query/load aggregate.
- Transaction manager commit/rollback.
- Outbox save/load/mark published.
- Redis/Kafka/PostgreSQL adapter.
- Provider adapter mapping request/response/error.

Ưu tiên dependency thật khi bug chỉ lộ ra ở boundary thật:

- PostgreSQL qua testcontainers.
- Redis qua testcontainers.
- Kafka/Redpanda qua testcontainers.

Ví dụ vị trí:

```text
internal/modules/identity/infrastructure/postgres/user_repository_test.go
internal/platform/postgres/tx_test.go
internal/modules/delivery/infrastructure/outbox/publisher_test.go
```

### API

Mục tiêu:

- Xác nhận transport layer map đúng sang use case và trả đúng contract ra ngoài.

Nên test:

- Request validation.
- Auth/permission boundary.
- DTO -> command mapping.
- App error -> HTTP status/body mapping.
- Public response contract.

Không nên nhồi business rule sâu vào API test nếu domain/app đã cover.

Ví dụ vị trí:

```text
tests/api/identity_test.go
internal/apps/api/modules/campaign/handler_test.go
```

### Worker / consumer

Mục tiêu:

- Xác nhận event ingestion và background processing đúng contract và đúng failure mode.

Nên test:

- Envelope parsing.
- Event type/version validation.
- DTO -> command mapping.
- Idempotency khi message duplicate.
- Retry/DLQ behavior.
- Không commit/ack khi xử lý thất bại.

Ví dụ vị trí:

```text
tests/worker/outbox_test.go
internal/apps/worker/modules/ingestion/consumer_test.go
```

## Test theo feature shape

### CRUD / control plane

Tối thiểu nên có:

- Domain hoặc app test cho rule nghiệp vụ chính.
- API test cho validation và permission boundary.
- Integration test cho repository nếu có uniqueness/constraint đáng kể.

### Async flow qua outbox

Tối thiểu nên có:

- App test cho save aggregate + save outbox trong cùng transaction.
- Integration test cho outbox persistence.
- Connector/worker/consumer test cho idempotency.
- Ít nhất một e2e flow cho publish/consume path quan trọng.

Nếu local/dev dùng Debezium:

- Verify register connector thành công.
- Verify insert vào `outbox_events` publish đúng business topic, không chỉ topic CDC thô.
- Giữ consumer idempotency test như cũ; Debezium không thay thế retry/dedupe ở business layer.

### Webhook ingestion

Tối thiểu nên có:

- API/handler test cho signature/secret verification.
- App test cho normalize/dedupe decision.
- Integration hoặc e2e test cho duplicate provider event.
- Failure test cho malformed payload và replay path.

### Delivery / retry / throttling

Tối thiểu nên có:

- Domain/app test cho eligibility, retry eligibility, state transition.
- Integration test cho queue/outbox/provider adapter path quan trọng.
- Worker test cho backoff, max attempts, DLQ.

### Analytics / projection eventual consistency

Tối thiểu nên có:

- Consumer/projection test cho mapping event -> read model.
- Integration test cho insert/update projection.
- Test rõ chỗ nào eventual consistency được chấp nhận.

## Naming và placement convention

Quy tắc placement:

```text
Domain test           -> internal/modules/<module>/domain/*_test.go
App/use case test     -> internal/modules/<module>/app/<usecase>/*_test.go
Infrastructure test   -> internal/modules/<module>/infrastructure/.../*_test.go
Platform test         -> internal/platform/<capability>/*_test.go
API/worker e2e test   -> tests/api/*_test.go, tests/worker/*_test.go
```

Quy tắc naming:

- Tên test mô tả behavior, không mô tả implementation detail.
- Ưu tiên mẫu: `Test<Subject>_<Condition>_<ExpectedBehavior>`.

Ví dụ:

```text
TestUser_Activate_WhenPending_TransitionsToActive
TestSignupHandler_WhenEmailExists_ReturnsConflict
TestOutboxPublisher_WhenPublishFails_LeavesRowUnpublished
TestWebhookConsumer_WhenEventDuplicated_DoesNotApplyStateTwice
```

Nếu framework cho phép subtest:

- Dùng subtest để nhóm case cùng một behavior family.
- Không dùng subtest quá sâu khiến khó đọc output.

## Fixtures, fakes, test data

Ưu tiên theo thứ tự:

- Tạo object trực tiếp ngay trong test nếu nhỏ và rõ.
- Tạo helper/factory gần module nếu object phức tạp và lặp lại.
- Dùng fake nhỏ, có hành vi rõ ràng thay vì mock quá động.

Khi nào dùng fake:

- `fake clock` khi cần kiểm soát `time.Now()`.
- `fake id generator` khi cần assert event/order/id ổn định.
- `fake provider` khi cần mô phỏng accept/reject/timeout mà không phụ thuộc network.
- `fake DNS verifier` hoặc `fake webhook verifier` khi logic cần quyết định theo trạng thái xác thực.

Khi nào không nên dùng mock:

- Domain thuần không có side effect.
- Chỗ mapping SQL/JSON/driver behavior cần dependency thật.
- Chỗ retry/transaction cần boundary thật mới lộ bug.

Rule cho test data:

- Fixture có owner rõ: đặt gần module sử dụng.
- Tránh shared mutable fixture toàn repo.
- Mỗi test chỉ mang đúng lượng data đủ để giải thích behavior.
- Dữ liệu nhạy cảm như token/API key/email thật phải dùng dữ liệu giả rõ ràng.

## Local và CI workflow

Khi sửa `domain` hoặc `app`:

- Chạy unit test của module bị ảnh hưởng.
- Chạy lại app test nếu use case orchestration đổi.

Khi sửa `schema`, `repository`, `outbox`, `consumer`, `worker`:

- Chạy integration test của module liên quan.
- Chạy e2e test cho flow có rủi ro cao nhất.

Khi cần dependency thật:

- PostgreSQL: testcontainers hoặc docker-compose trong CI.
- Redis: testcontainers nếu logic phụ thuộc TTL/lock/idempotency store.
- Kafka/Redpanda: testcontainers cho outbox publish, consumer idempotency, retry/DLQ.

Heuristic thực dụng:

- Sửa mapper/DTO nhỏ ở API: unit + API-level test là đủ.
- Sửa query/index/transaction: bắt buộc có integration test.
- Sửa event flow, retry, idempotency: nên có cả app/integration và ít nhất một e2e.

## Verification checklist

Checklist này dùng cùng với implementation note:

```text
[ ] Happy path
[ ] Validation path
[ ] Permission/auth boundary
[ ] Duplicate or retry path
[ ] Rollback path
[ ] Async consumer path
[ ] Replay/idempotency path
[ ] Observability signal xuất hiện
```

Diễn giải:

- `Happy path`: flow chính thành công với input hợp lệ.
- `Validation path`: input sai bị chặn đúng boundary.
- `Permission/auth boundary`: actor không hợp lệ không vượt qua được.
- `Duplicate or retry path`: request/event bị gửi lại không gây double side effect.
- `Rollback path`: failure giữa chừng không để lại state nửa vời.
- `Async consumer path`: worker xử lý đúng message hợp lệ.
- `Replay/idempotency path`: replay hoặc duplicate delivery không áp state hai lần.
- `Observability signal`: có log/metric/trace đủ để debug.

## Definition of done về test

Một feature chưa được coi là đủ test nếu:

- Chưa cover path rủi ro chính của feature.
- Chưa rõ chỗ nào được cover bằng unit, integration, e2e.
- Chưa nói rõ phần nào chưa cover và vì sao.
- Có async flow nhưng chưa test idempotency hoặc rollback.
- Có schema/query mới nhưng chưa test boundary thật.

Một feature được coi là đạt baseline test khi:

- Có test cho business rule chính.
- Có test cho ít nhất một failure mode quan trọng.
- Có test đúng layer thay vì dồn tất cả vào e2e.
- Có verification checklist pass cho flow chính.
- Có note rõ nếu một phần rủi ro được defer.

## Mẫu quyết định nhanh khi viết test

```text
Nếu bug nằm ở business rule?         -> viết domain/app test
Nếu bug nằm ở transaction/query?     -> viết integration test
Nếu bug nằm ở API contract?          -> viết API test
Nếu bug nằm ở worker/event flow?     -> viết worker/integration/e2e test
Nếu bug chỉ lộ khi wire end-to-end?  -> thêm một e2e flow đại diện
```
