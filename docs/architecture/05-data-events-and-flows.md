# Data, Events And Flows

File này là xương sống để hiểu hệ thống chạy thế nào: request vào, use case đổi state, event được publish, worker xử lý side effect, và lỗi được map theo boundary nào.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Module communication | `../modules/02-communication-and-public-api.md` |
| API conventions | `../api/00-api-map.md` |
| Event/operations model | `../model/04-events-analytics-and-operations.md` |
| Delivery/events schema | `../database/03-delivery-and-events.md` |
| Analytics/operations schema | `../database/04-analytics-and-operations.md` |
| Scale/resilience | `08-scale-and-resilience.md` |

## Command flow chuẩn

```text
Inbound boundary
  -> decode DTO/message
  -> validate transport shape
  -> map sang command/query
  -> gọi use case

Use case boundary
  -> validate command ở mức nghiệp vụ
  -> load aggregate/read model cần thiết
  -> gọi domain behavior
  -> ghi state trong transaction
  -> ghi outbox nếu có event

Async boundary
  -> Debezium/Kafka Connect hoặc outbox publisher publish event
  -> consumer xử lý idempotent
  -> retry hoặc DLQ theo loại lỗi
```

## Transactional Outbox

Nếu một use case vừa ghi business state vừa phát event, phải dùng outbox.

Không làm:

```text
userRepo.Save(user)
kafkaProducer.Publish(UserRegistered)
```

Vì nếu publish fail sau khi save thành công, hệ thống mất event.

Làm:

```text
txManager.WithinTx(ctx, func(ctx context.Context) error {
  userRepo.Save(ctx, user)
  outbox.Save(ctx, UserRegistered)
  return nil
})

outbox worker:
  read unpublished events
  publish to Kafka
  mark as published
```

Hoặc với Debezium:

```text
txManager.WithinTx(ctx, func(ctx context.Context) error {
  userRepo.Save(ctx, user)
  outbox.Save(ctx, UserRegistered)
  return nil
})

Debezium connector:
  read WAL change của outbox_events
  route sang business topic
  publish to Kafka
```

Trong repo này, local/dev stack mặc định ưu tiên Debezium CDC cho `outbox_events`. Polling outbox worker vẫn là fallback hợp lệ nếu một môi trường chưa có Kafka Connect hoặc cần replay logic riêng.

Flow chuẩn cho `SignUp`:

```text
POST /auth/signup
  -> API handler decode request
  -> map to signup.Command
  -> signup.Handler
       -> check email uniqueness
       -> hash password
       -> domain.NewUser(...)
       -> tx begin
          -> userRepo.Save(user)
          -> outbox.Save(user.Events())
       -> tx commit
  -> API handler return 201

Debezium outbox connector hoặc outbox worker
  -> publish identity.user.registered.v1
```

Event consumer phải idempotent. Kafka có thể deliver message nhiều hơn một lần.

---

## Event schema và versioning

Event là contract lâu dài. Một khi có consumer khác dùng, đổi event tùy tiện sẽ rất đau.

Quy ước event:

```text
topic:       identity.user.registered
event_type:  identity.user.registered.v1
key:         <aggregate_id>
```

Envelope tối thiểu:

```json
{
  "event_id": "01HX...",
  "event_type": "identity.user.registered.v1",
  "aggregate_id": "user_123",
  "tenant_id": "tenant_123",
  "occurred_at": "2026-05-30T10:00:00Z",
  "payload": {}
}
```

Quy tắc version:

- Chỉ thêm field optional thì giữ nguyên version.
- Đổi meaning, rename field, đổi type hoặc bỏ field thì tạo version mới.
- Consumer phải ignore field không biết.
- Producer có thể publish song song `v1` và `v2` trong giai đoạn migration.
- Không reuse `event_type` cũ cho payload mới.

Schema registry:

- Giai đoạn đầu có thể dùng JSON struct trong `contracts`.
- Khi nhiều team/consumer hơn, cân nhắc JSON Schema, Protobuf hoặc Avro.
- Dù chọn format nào, vẫn giữ naming/version convention ở trên.

## Idempotency

Idempotency cần cho cả API lẫn consumer vì request retry và Kafka redelivery là chuyện bình thường.

API idempotency dùng cho:

- Tạo payment/subscription/invoice.
- Tạo campaign/flow từ request dễ bị retry.
- Webhook inbound từ external provider.

Consumer idempotency dùng cho:

- Gửi email/SMS/webhook.
- Ghi audit/event projection.
- Sync external system.

Khuyến nghị:

- API nhận `Idempotency-Key` cho command quan trọng.
- Redis lưu key ngắn hạn cho request fast path.
- PostgreSQL unique constraint vẫn là lớp bảo vệ cuối cho business uniqueness.
- Consumer lưu `event_id` đã xử lý, bằng Redis TTL hoặc PostgreSQL table tùy mức độ cần bền.
- Side effect external nên có provider idempotency key nếu provider hỗ trợ.

Ví dụ key:

```text
idem:api:<tenant_id>:<idempotency_key>
idem:kafka:<consumer_name>:<event_id>
```

## Error handling

Error phân theo layer:

| Layer | Loại error | Ví dụ |
|---|---|---|
| Domain | Business invariant | `ErrEmailAlreadyRegistered`, `ErrInvalidStatusTransition` |
| App | Use case failure category | `KindConflict`, `KindValidation`, `KindNotFound` |
| Runtime | Protocol-specific response | HTTP 400/404/409/500, Kafka retry/DLQ |
| Platform | Technical fault | DB timeout, Redis unavailable, Kafka publish failed |

Domain không biết HTTP status. App không biết JSON response. Runtime là nơi map sang protocol.

Ví dụ:

```go
if errors.Is(err, domain.ErrEmailAlreadyRegistered) {
    return nil, app.NewConflictError(err)
}
if err != nil {
    return nil, app.NewInternalError(err)
}
```

API mapping:

```go
switch {
case app.IsValidation(err):
    writeJSON(w, http.StatusBadRequest, err)
case app.IsNotFound(err):
    writeJSON(w, http.StatusNotFound, err)
case app.IsConflict(err):
    writeJSON(w, http.StatusConflict, err)
default:
    writeJSON(w, http.StatusInternalServerError, ErrInternal)
}
```

Worker mapping:

```text
validation/non-retryable error -> ack + log/metric
temporary platform error       -> retry
permanent repeated failure     -> DLQ
```

---

## Sample flow: `identity/signup`

Flow này là blueprint cho use case đầu tiên.

```text
POST /api/v1/auth/signup
  -> apps/api/modules/identity/handler.go
       decode SignUpRequest
       validate request body
       map to signup.Command
       call signup.Handler.Handle(ctx, cmd)

  -> modules/identity/app/signup/handler.go
       validate command
       check email uniqueness
       hash password
       create User aggregate
       txManager.WithinTx(...)
         userRepo.Save(user)
         outbox.Save(user.Events())
       return Result

  -> apps/api/modules/identity/handler.go
       map Result to SignUpResponse
       return HTTP 201

  -> Debezium/Kafka Connect hoặc apps/worker/outbox publisher
       capture outbox row
       publish identity.user.registered.v1

  -> apps/worker/modules/notification/consumer.go
       consume UserRegisteredV1
       check idempotency
       send welcome email
       mark event processed
```

Điểm cần test:

- Signup thành công tạo user và outbox event.
- Duplicate email trả conflict.
- Password invalid trả validation error.
- Repository save fail rollback outbox.
- Outbox publish fail không mất event.
- Consumer nhận duplicate event không gửi email hai lần.

---

## Event catalog cần có khi hệ thống lớn hơn

Mỗi event public nên được ghi vào catalog riêng, tối thiểu có các cột sau:

| Field | Ý nghĩa |
|---|---|
| Topic | Kafka topic hoặc stream name |
| Event type | Tên event có version, ví dụ `identity.user.registered.v1` |
| Producer | Module/use case publish event |
| Consumers | Runtime/module đang consume |
| Ordering key | Aggregate id hoặc key giữ ordering |
| Idempotency key | Thường là `event_id`, đôi khi kết hợp consumer name |
| Retention | Thời gian giữ event/raw payload |
| Compatibility | Quy tắc thay đổi payload |

Chưa cần catalog hoành tráng từ ngày đầu, nhưng nên bắt đầu trước khi event có nhiều consumer.
