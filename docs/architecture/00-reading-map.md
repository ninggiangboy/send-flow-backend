# Architecture Reading Map

Bộ tài liệu này không còn đi theo thứ tự section ban đầu. Nó được tổ chức theo câu hỏi mà người đọc thường có khi cần hiểu sâu hệ thống.

## Cách đọc nhanh

Graph tổng của toàn bộ docs nằm ở [../00-doc-graph.md](../00-doc-graph.md).

| Cần trả lời | Đọc file |
|---|---|
| Hệ thống này làm gì, có service/runtime/infra nào, actor nào tương tác? | `01-system-overview.md` |
| Hệ thống tối ưu cho điều gì, consistency/data ownership ra sao? | `02-architecture-principles.md` |
| Clean Architecture, DDD nhẹ, Modular Monolith và Event-driven được áp dụng thế nào? | `03-architecture-style.md` |
| Code nên nằm ở đâu, layer nào được import layer nào? | `04-code-architecture.md` |
| Bounded context nên tách thành module nào, module giao tiếp ra sao? | `../modules/00-module-map.md` |
| Request, event, transaction, retry và idempotency chạy thế nào? | `05-data-events-and-flows.md` |
| PostgreSQL, Redis, Kafka, ClickHouse, observability dùng vai trò gì? | `06-infrastructure.md` |
| Email delivery platform gồm những capability nào? | `07-email-delivery.md` |
| Khi tải cao thì áp dụng pattern nào, lúc nào chưa nên dùng? | `08-scale-and-resilience.md` |
| Team implement, test, đặt tên, config và ship feature ra sao? | `09-engineering-practices.md` |

## Quyết định gộp và tách

| Quyết định | Lý do |
|---|---|
| Tách system overview khỏi architecture principles | Overview cần trả lời “service/infra nào đang chạy”; principles trả lời “vì sao hệ thống được thiết kế như vậy”. |
| Tách architecture style thành `03-architecture-style.md` | Clean Architecture, DDD nhẹ, Modular Monolith và Event-driven cần giải thích bằng rule riêng, không chỉ nhắc ở tiêu đề. |
| Gộp runtime, module layout, dependency direction và DI vào `04-code-architecture.md` | Đây đều là câu hỏi về vị trí code và hướng dependency; tách riêng làm người đọc phải nhảy file liên tục. |
| Gộp outbox, event schema, idempotency, error handling và sample flow vào `05-data-events-and-flows.md` | Những phần này cùng trả lời một câu hỏi: một command đi qua hệ thống ra sao và consistency được bảo vệ thế nào. |
| Tách `06-infrastructure.md` khỏi code architecture | Tech stack là quyết định hạ tầng, không nên lẫn với rule import/package. |
| Giữ email delivery thành file riêng | Đây là domain sâu nhất của sản phẩm, cần đọc như một bounded-context map chứ không phải một mục trong tech stack. |
| Giữ high-load thành file riêng nhưng thêm taxonomy | Pattern chịu tải cao rất nhiều; cần phân loại theo symptom để tránh áp dụng như checklist máy móc. |
| Đưa roadmap, ADR, naming, coding rules, config, DoD và checklist vào `09-engineering-practices.md` | Đây là working agreement của team, không phải core architecture model. |

## Chỗ đã viết rõ hơn

- Thêm mental model: `API/Worker` là runtime, `modules` là business capability, `platform` là kỹ thuật dùng chung.
- Thêm system overview: actor bên ngoài, API/Worker/Gateway/DB/Redis/Kafka/monitoring, capability map và runtime flow chính.
- Thêm architecture style: cách áp dụng Clean Architecture, DDD nhẹ, Modular Monolith và Event-driven vào package/use case/event cụ thể.
- Thêm bounded context map để thấy module nào sở hữu dữ liệu và module nào chỉ consume event.
- Thêm consistency model: strong consistency nằm trong transaction của aggregate; cross-module là eventual consistency qua event/outbox.
- Thêm data ownership rule: module owner mới được ghi bảng nghiệp vụ của mình.
- Thêm flow đọc theo boundary: inbound request, use case, transaction, outbox, worker, projection/analytics.
- Thêm guidance khi nào pattern chịu tải cao nên hoặc chưa nên dùng.

## Chỗ nên tiếp tục bổ sung khi hệ thống lớn hơn

- Sequence diagram thật cho các flow `signup`, `send email`, `provider webhook`, `unsubscribe`, `campaign analytics`.
- Data model chi tiết theo module: aggregate, table owner, index, retention, migration owner.
- Event catalog: topic, payload version, producer, consumer, ordering key, idempotency key, retention.
- SLO theo runtime: API latency, worker lag, outbox age, delivery success rate, webhook ingestion delay.
- Threat model cho auth, provider webhook, tenant isolation, PII/logging và secret rotation.
- Runbook vận hành: xử lý DLQ, replay event, outbox stuck, Kafka lag, provider outage, ClickHouse insert lag.
