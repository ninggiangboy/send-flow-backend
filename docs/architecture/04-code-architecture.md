# Code Architecture

File này trả lời: code đặt ở đâu, package nào được biết package nào, và runtime wire dependency như thế nào.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Architecture style | `03-architecture-style.md` |
| Module map | `../modules/00-module-map.md` |
| Module communication | `../modules/02-communication-and-public-api.md` |
| Module catalog | `../modules/03-module-catalog.md` |
| Engineering practices | `09-engineering-practices.md` |

## Source layout

```text
.
├── cmd
│   ├── api
│   └── worker
├── internal
│   ├── apps
│   │   ├── api
│   │   └── worker
│   ├── modules
│   │   └── <module>
│   │       ├── domain
│   │       ├── app
│   │       ├── ports
│   │       ├── contracts
│   │       ├── infrastructure
│   │       └── module.go
│   ├── platform
│   └── sharedkernel
├── migrations
├── ops
│   ├── deployments
│   ├── configs
│   └── scripts
└── docs
```

## Bốn vùng chính trong `internal`

| Vùng | Vai trò | Được phép biết |
|---|---|---|
| `internal/apps` | Runtime orchestration: HTTP, Kafka consumer, scheduler, shutdown | `modules/*/app`, `modules/*/contracts`, `platform` |
| `internal/modules` | Business capability/bounded context | `sharedkernel`, interface nội bộ, platform qua abstraction |
| `internal/platform` | Technical foundation dùng chung | thư viện ngoài, driver, SDK |
| `internal/sharedkernel` | Domain concept dùng chung và ổn định | standard library là chính |

Quy tắc nền:

- `apps` gọi use case, không chứa business rule.
- `modules` chứa nghiệp vụ và use case, không biết HTTP/Kafka framework.
- `platform` không import `apps` hoặc `modules`.
- `sharedkernel` phải nhỏ. Đưa vào đây càng nhiều thì coupling càng tăng.

## Runtime layer

`cmd/*/main.go` chỉ gọi bootstrap tương ứng.

| File | Gọi tới |
|---|---|
| `cmd/api/main.go` | `internal/apps/api/bootstrap.go` |
| `cmd/worker/main.go` | `internal/apps/worker/bootstrap.go` |

Không đặt business logic, router, DB query, config parsing chi tiết hoặc Kafka consumer logic trong `cmd`.

```text
internal/apps/api
├── app.go          # start server, graceful shutdown
├── bootstrap.go    # wire dependency graph cho API
├── router.go       # route group tổng
├── middleware.go   # auth, CORS, rate limit, logging
├── server.go       # http.Server config
├── errors.go       # map app error -> HTTP response
└── modules
    └── identity
        ├── handler.go
        ├── dto.go
        ├── mapper.go
        └── routes.go
```

API handler làm:

```text
HTTP request
  -> decode API DTO
  -> validate transport-level input
  -> map sang app command/query
  -> gọi use case
  -> map result/error sang HTTP response
```

API handler không:

- Ghi SQL trực tiếp vào business table.
- Publish Kafka trực tiếp.
- Tự quyết định business invariant.
- Import `modules/<name>/infrastructure`.

```text
internal/apps/worker
├── app.go          # start worker, graceful shutdown
├── bootstrap.go    # wire dependency graph cho worker
├── consumers.go    # đăng ký topic -> consumer
├── scheduler.go    # cron/scheduled job nếu cần
├── runner.go       # background runner
├── dlq.go          # dead letter queue handling
├── retry.go        # retry/backoff policy
└── modules
    └── identity
        ├── consumer.go
        ├── dto.go
        └── mapper.go
```

Worker consumer làm:

```text
Kafka message
  -> parse envelope
  -> validate event type/version
  -> map payload sang app command/query
  -> gọi use case
  -> commit offset sau khi xử lý thành công
```

Worker không update business table bằng SQL tùy tiện. Nếu cần thay đổi state, gọi use case trong `modules/<name>/app`.

---

## Module layer

Mỗi module là một bounded context.

```text
internal/modules/<module>
├── domain          # nghiệp vụ lõi
├── app             # use case/application service
├── ports           # outbound interfaces module cần từ bên ngoài
├── contracts       # public contract: events, topic names, public read models
├── infrastructure  # implementation kỹ thuật riêng của module
└── module.go       # DI set entry point
```

Ví dụ `identity`:

```text
internal/modules/identity
├── domain
│   ├── user.go
│   ├── workspace.go
│   ├── membership.go
│   ├── invitation.go
│   ├── role.go
│   ├── permission.go
│   ├── permission_set.go
│   ├── events.go
│   ├── errors.go
│   └── repository.go
│
├── app
│   ├── signup
│   │   ├── command.go
│   │   ├── handler.go
│   │   └── result.go
│   ├── login
│   ├── getuser
│   └── errors.go
│
├── ports
│   ├── outbox.go
│   ├── session_store.go
│   ├── password_hasher.go
│   └── tx_manager.go
│
├── contracts
│   ├── events.go
│   └── topics.go
│
├── infrastructure
│   ├── postgres
│   │   ├── user_repository.go
│   │   └── mapper.go
│   └── outbox
│       └── event_mapper.go
│
└── module.go
```

---

## Vai trò từng package trong module

`domain` chứa business model, invariant và domain event.

Nên có:

- Aggregate root, entity, value object riêng của module.
- Domain error.
- Domain event.
- Repository interface nếu repository là một phần của domain language.
- Behavior method: `Lock`, `Activate`, `ChangeEmail`, ...

Không có:

- HTTP DTO, JSON response.
- Kafka envelope.
- SQL query.
- Redis/Kafka/PostgreSQL client.
- `time.Now()` rải rác trong entity nếu logic cần test ổn định.

Ví dụ factory nên nhận thời gian từ use case:

```go
func NewUser(id UserID, email email.EmailAddress, hashedPassword string, now time.Time) (*User, error) {
    if id == "" {
        return nil, ErrEmptyUserID
    }
    if hashedPassword == "" {
        return nil, ErrEmptyPassword
    }

    user := &User{
        ID:             id,
        Email:          email,
        HashedPassword: hashedPassword,
        Status:         UserStatusActive,
        CreatedAt:      now,
        UpdatedAt:      now,
    }
    user.recordEvent(UserRegistered{UserID: string(id), Email: email.String(), OccurredAt: now})
    return user, nil
}
```

`app` chứa use case. Mỗi use case nên là một package riêng nếu logic đủ lớn.

```text
app/signup
├── command.go
├── handler.go
└── result.go
```

Handler làm orchestration:

- Validate command/query ở mức use case.
- Gọi domain behavior.
- Gọi repository/port.
- Quản lý transaction thông qua `ports.TxManager`.
- Map domain error sang app error.

Handler không:

- Import `infrastructure`.
- Biết HTTP status.
- Biết Kafka consumer group.
- Nuốt lỗi bằng `_`.

Lưu ý quan trọng về import cycle: không đặt interface trong `app/ports.go` rồi bắt interface đó nhận type từ `app/signup`, sau đó `signup` lại import `app`. Cách đơn giản hơn trong Go là runtime nhận trực tiếp concrete handler hoặc interface do runtime tự khai báo.

Ví dụ API handler có thể phụ thuộc trực tiếp vào `*signup.Handler`:

```go
type Handler struct {
    signUp *signup.Handler
}
```

Nếu thật sự cần interface để test runtime, đặt interface ở runtime layer:

```go
// internal/apps/api/modules/identity/handler.go
type signUpUseCase interface {
    Handle(ctx context.Context, cmd signup.Command) (*signup.Result, error)
}
```

`ports` là outbound interfaces mà use case cần từ bên ngoài.

Ví dụ:

```go
package ports

type TxManager interface {
    WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Outbox interface {
    Save(ctx context.Context, event any) error
}

type SessionStore interface {
    Save(ctx context.Context, userID string, token string, ttl time.Duration) error
    Get(ctx context.Context, token string) (userID string, err error)
    Delete(ctx context.Context, token string) error
}

type PasswordHasher interface {
    Hash(plain string) (string, error)
    Verify(plain, hashed string) bool
}
```

Interface đặt ở nơi dùng. Implementation có thể nằm ở `platform` hoặc `modules/<module>/infrastructure`, nhưng use case chỉ biết interface.

`contracts` là phần module công khai cho runtime hoặc module khác import.

Nên chứa:

- Event payload versioned.
- Topic name.
- Public read model nếu thật sự cần cross-module.

Không nên chứa:

- HTTP request/response DTO.
- Internal command/query.
- Business enum không dùng cross-module.

Ví dụ:

```go
package contracts

const TopicUserRegistered = "identity.user.registered"

type UserRegisteredV1 struct {
    EventID    string `json:"event_id"`
    EventType  string `json:"event_type"`
    UserID     string `json:"user_id"`
    Email      string `json:"email"`
    OccurredAt int64  `json:"occurred_at"`
}
```

API DTO nên ở `internal/apps/api/modules/<module>/dto.go`; Worker DTO hoặc message input riêng nên ở `internal/apps/worker/modules/<module>/dto.go`.

`infrastructure` implement repository/ports cho module.

Ví dụ:

```text
infrastructure/postgres/user_repository.go
infrastructure/postgres/mapper.go
infrastructure/outbox/event_mapper.go
```

Quy tắc:

- Repository implement interface ở `domain` hoặc `ports`.
- Mapper DB row -> domain chỉ nằm trong infrastructure.
- Không đặt HTTP handler hoặc Kafka consumer ở đây.
- Không để module khác import implementation trực tiếp.

`module.go` là entry point để runtime wire module.

```go
package identity

import "github.com/google/wire"

var ModuleSet = wire.NewSet(
    postgres.NewUserRepository,
    wire.Bind(new(domain.UserRepository), new(*postgres.UserRepository)),

    outbox.NewStore,
    wire.Bind(new(ports.Outbox), new(*outbox.Store)),

    signup.NewHandler,
    login.NewHandler,
    getuser.NewHandler,
)
```

Runtime import `identity.ModuleSet`; runtime không tự đi vào từng implementation nhỏ nếu không cần.

---

## Dependency direction

Dependency được phép:

```text
cmd
  -> apps

apps
  -> modules/*/app
  -> modules/*/contracts
  -> platform

modules/*/app
  -> modules/*/domain
  -> modules/*/ports
  -> sharedkernel

modules/*/infrastructure
  -> modules/*/domain
  -> modules/*/ports
  -> modules/*/contracts
  -> platform

modules/*/domain
  -> sharedkernel

platform
  -> external libraries
```

Dependency không được phép:

```text
domain -> app
domain -> infrastructure
domain -> apps
domain -> platform/postgres
app -> infrastructure
platform -> modules
module A domain -> module B infrastructure
```

Giao tiếp giữa module:

- Prefer event qua `contracts`.
- Nếu cần synchronous lookup, tạo interface ở module cần dùng, rồi bind implementation ở composition root. Không import implementation module khác trực tiếp.

---

## Dependency Injection

Khuyến nghị dùng **Google Wire** cho project Go này.

Lý do:

- Compile-time DI, lỗi wiring lộ khi build.
- Không dùng reflect cho object graph.
- Code generate dễ đọc.
- Hợp với style Go hơn service container runtime.

Cấu trúc:

```text
internal/modules/identity/module.go   # module set
internal/apps/api/bootstrap.go        # API graph
internal/apps/worker/bootstrap.go     # Worker graph
```

API và Worker có bootstrap riêng vì dependency runtime khác nhau. Ví dụ API cần router/server, Worker cần consumer group/scheduler.

---

## Khi tách worker nhỏ hơn

Nếu sau này worker lớn lên, có thể tách runtime mà không đổi core module:

```text
cmd
├── api
├── worker-identity
└── worker-notification

internal/apps
├── api
├── workeridentity
└── workernotification
```

`internal/modules` và `internal/platform` giữ nguyên. Đây là lý do tách `apps` khỏi `modules` ngay từ đầu.

---
