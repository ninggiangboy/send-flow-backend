# Documentation Graph

File này là bản đồ liên kết tổng của bộ tài liệu `docs/`.

Mục tiêu:

- Chỉ ra file nào là entry point.
- Cho thấy cùng một capability được mô tả qua feature, module, API, model và database như thế nào.
- Giúp người đọc đi từ sản phẩm -> architecture -> module boundary -> contract -> schema mà không phải đoán file kế tiếp.

## Entry points

| Cần đọc | Bắt đầu từ |
|---|---|
| Tổng quan hệ thống và quyết định kiến trúc | [architecture/00-reading-map.md](architecture/00-reading-map.md) |
| Feature/capability sản phẩm | [features/00-feature-map.md](features/00-feature-map.md) |
| Bounded context và module boundary | [modules/00-module-map.md](modules/00-module-map.md) |
| Cách lên implementation note cho feature mới | [00-implementation-note.md](00-implementation-note.md) |
| Testing strategy và writing tests | [testing/00-testing-guide.md](testing/00-testing-guide.md) |
| API contract | [api/00-api-map.md](api/00-api-map.md) |
| Tích hợp API theo hành trình sản phẩm | [api/05-use-cases-and-flows.md](api/05-use-cases-and-flows.md) |
| Business model | [model/00-model-map.md](model/00-model-map.md) |
| Logical database schema | [database/00-database-map.md](database/00-database-map.md) |

## Graph tổng

```mermaid
flowchart TD
  G[docs/00-doc-graph.md]

  A0[architecture/00-reading-map.md]
  A1[architecture/01-system-overview.md]
  A2[architecture/02-architecture-principles.md]
  A3[architecture/03-architecture-style.md]
  A4[architecture/04-code-architecture.md]
  A5[architecture/05-data-events-and-flows.md]
  A6[architecture/06-infrastructure.md]
  A7[architecture/07-email-delivery.md]
  A8[architecture/08-scale-and-resilience.md]
  A9[architecture/09-engineering-practices.md]

  F0[features/00-feature-map.md]
  F1[features/01-control-plane.md]
  F2[features/02-audience-and-content.md]
  F3[features/03-delivery-and-messaging.md]
  F4[features/04-ingestion-tracking-suppression.md]
  F5[features/05-analytics-and-operations.md]

  M0[modules/00-module-map.md]
  M1[modules/01-bounded-contexts.md]
  M2[modules/02-communication-and-public-api.md]
  M3[modules/03-module-catalog.md]

  API0[api/00-api-map.md]
  API1[api/01-auth-and-control-plane.md]
  API2[api/02-audience-and-content.md]
  API3[api/03-messaging-and-delivery.md]
  API4[api/04-events-webhooks-and-operations.md]
  API5[api/05-use-cases-and-flows.md]

  MOD0[model/00-model-map.md]
  MOD1[model/01-identity-and-access.md]
  MOD2[model/02-audience-and-content.md]
  MOD3[model/03-messaging-and-delivery.md]
  MOD4[model/04-events-analytics-and-operations.md]

  DB0[database/00-database-map.md]
  DB1[database/01-core-identity-and-access.md]
  DB2[database/02-audience-and-content.md]
  DB3[database/03-delivery-and-events.md]
  DB4[database/04-analytics-and-operations.md]

  G --> A0
  G --> F0
  G --> M0
  G --> API0
  G --> MOD0
  G --> DB0

  A0 --> A1 --> A2 --> A3 --> A4 --> A5 --> A6 --> A7 --> A8 --> A9
  F0 --> F1
  F0 --> F2
  F0 --> F3
  F0 --> F4
  F0 --> F5
  M0 --> M1
  M0 --> M2
  M0 --> M3
  API0 --> API1
  API0 --> API2
  API0 --> API3
  API0 --> API4
  API0 --> API5
  API5 --> API1
  API5 --> API2
  API5 --> API3
  API5 --> API4
  MOD0 --> MOD1
  MOD0 --> MOD2
  MOD0 --> MOD3
  MOD0 --> MOD4
  DB0 --> DB1
  DB0 --> DB2
  DB0 --> DB3
  DB0 --> DB4

  A2 --> M1
  A3 --> M0
  A4 --> M2
  A5 --> M2
  A5 --> API0
  A6 --> DB0
  A7 --> F3
  A7 --> F4
  A8 --> F5
  A9 --> M3

  F1 --> API1
  F1 --> MOD1
  F1 --> DB1
  F2 --> API2
  F2 --> MOD2
  F2 --> DB2
  F3 --> API3
  F3 --> MOD3
  F3 --> DB3
  F4 --> API4
  F4 --> MOD3
  F4 --> DB3
  F5 --> API4
  F5 --> MOD4
  F5 --> DB4

  API1 --> MOD1
  API1 --> DB1
  API2 --> MOD2
  API2 --> DB2
  API3 --> MOD3
  API3 --> DB3
  API4 --> MOD4
  API4 --> DB4

  M3 --> API0
  M3 --> MOD0
  M3 --> DB0
```

## Capability trace

| Capability | Feature | Module | API | Model | Database |
|---|---|---|---|---|---|
| Control plane | [features/01-control-plane.md](features/01-control-plane.md) | [modules/03-module-catalog.md](modules/03-module-catalog.md) (`identity`, `access`, `sender`, `webhooks`, `audit`) | [api/01-auth-and-control-plane.md](api/01-auth-and-control-plane.md) | [model/01-identity-and-access.md](model/01-identity-and-access.md) | [database/01-core-identity-and-access.md](database/01-core-identity-and-access.md) |
| Audience and content | [features/02-audience-and-content.md](features/02-audience-and-content.md) | [modules/03-module-catalog.md](modules/03-module-catalog.md) (`audience`, `content`) | [api/02-audience-and-content.md](api/02-audience-and-content.md) | [model/02-audience-and-content.md](model/02-audience-and-content.md) | [database/02-audience-and-content.md](database/02-audience-and-content.md) |
| Messaging and delivery | [features/03-delivery-and-messaging.md](features/03-delivery-and-messaging.md) | [modules/03-module-catalog.md](modules/03-module-catalog.md) (`campaign`, `delivery`) | [api/03-messaging-and-delivery.md](api/03-messaging-and-delivery.md) | [model/03-messaging-and-delivery.md](model/03-messaging-and-delivery.md) | [database/03-delivery-and-events.md](database/03-delivery-and-events.md) |
| Ingestion, tracking, suppression | [features/04-ingestion-tracking-suppression.md](features/04-ingestion-tracking-suppression.md) | [modules/03-module-catalog.md](modules/03-module-catalog.md) (`ingestion`, `tracking`, `suppression`) | [api/04-events-webhooks-and-operations.md](api/04-events-webhooks-and-operations.md) | [model/03-messaging-and-delivery.md](model/03-messaging-and-delivery.md) | [database/03-delivery-and-events.md](database/03-delivery-and-events.md) |
| Analytics and operations | [features/05-analytics-and-operations.md](features/05-analytics-and-operations.md) | [modules/03-module-catalog.md](modules/03-module-catalog.md) (`analytics`, `operations`) | [api/04-events-webhooks-and-operations.md](api/04-events-webhooks-and-operations.md) | [model/04-events-analytics-and-operations.md](model/04-events-analytics-and-operations.md) | [database/04-analytics-and-operations.md](database/04-analytics-and-operations.md) |

## Recommended reading paths

### Implement một feature mới

```text
features/<capability>.md
  -> modules/03-module-catalog.md
  -> api/<matching-api>.md
  -> model/<matching-model>.md
  -> database/<matching-database>.md
  -> architecture/04-code-architecture.md
  -> architecture/09-engineering-practices.md
```

### Review boundary hoặc tách service

```text
architecture/02-architecture-principles.md
  -> architecture/03-architecture-style.md
  -> modules/01-bounded-contexts.md
  -> modules/02-communication-and-public-api.md
  -> modules/03-module-catalog.md
```

### Thiết kế event/async flow

```text
architecture/05-data-events-and-flows.md
  -> modules/02-communication-and-public-api.md
  -> api/00-api-map.md
  -> model/04-events-analytics-and-operations.md
  -> database/03-delivery-and-events.md
  -> database/04-analytics-and-operations.md
```

### Tích hợp API theo hành trình sản phẩm

```text
api/05-use-cases-and-flows.md
  -> api/00-api-map.md
  -> api/01-auth-and-control-plane.md
  -> api/02-audience-and-content.md
  -> api/03-messaging-and-delivery.md
  -> api/04-events-webhooks-and-operations.md
```
