# Module Map

Bộ tài liệu này mô tả cách tách **bounded context thành module** cho `send-flow`.

Mục tiêu chính là giúp project bắt đầu bằng **modular monolith** nhưng vẫn giữ đường tách sang microservice sau này:

- Mỗi module có ownership rõ về business capability, dữ liệu, invariant và contract.
- Runtime API/Worker chỉ orchestration, không làm mờ boundary nghiệp vụ.
- Cross-module dependency đi qua public API hoặc event contract, không import sâu vào implementation của nhau.
- Database/schema có owner, tránh shared table vô chủ.

## Cách đọc nhanh

Graph tổng và capability trace nằm ở [../00-doc-graph.md](../00-doc-graph.md).

| Cần trả lời | Đọc file |
|---|---|
| Vì sao chia module, module khác bounded context thế nào? | `01-bounded-contexts.md` |
| Module nói chuyện với nhau bằng gì, public API gồm những gì? | `02-communication-and-public-api.md` |
| Project nên có những module nào, scope và contract từng module ra sao? | `03-module-catalog.md` |

## Module là gì trong project này?

Một module là một bounded context triển khai được bằng code trong:

```text
internal/modules/<module>
├── domain
├── app
├── ports
├── contracts
├── infrastructure
└── module.go
```

Module không chỉ là folder. Module là boundary của:

- Ngôn ngữ nghiệp vụ.
- Transaction state chính.
- Invariant và lifecycle model.
- Public command/query API cho module khác hoặc runtime gọi.
- Integration event mà module publish/consume.
- Migration/schema do module sở hữu.

## Module candidate tổng quan

| Module | Vai trò chính | Tách service sau này? |
|---|---|---|
| `identity` | User, workspace, membership, role, session | Có, nhưng thường tách muộn vì nhiều module cần auth context |
| `access` | Permission evaluation, API key, credential scope | Có thể gộp trong `identity` giai đoạn đầu |
| `sender` | Sender domain, DNS verification, sending identity | Có, nếu deliverability/provider ownership lớn |
| `audience` | Contact, list, segment, import/export audience | Có, nếu data volume/contact processing lớn |
| `content` | Template, template version, render/preview | Có, nếu rendering scale hoặc editor độc lập |
| `campaign` | Campaign planning, scheduling, message candidate | Có, khi campaign orchestration cần scale riêng |
| `delivery` | Message lifecycle, provider routing, attempts, retry | Có, đây là ứng viên tách sớm khi tải gửi tăng |
| `ingestion` | Provider webhook raw ingest, normalize, dedupe | Có, đây là ứng viên tách sớm do traffic burst |
| `suppression` | Block/unblock recipient, unsubscribe guardrail | Có, nếu cần compliance/service boundary riêng |
| `tracking` | Open/click tracking, tracking links/events | Có, nếu tracking endpoint có volume lớn |
| `webhooks` | Customer webhook outbound delivery | Có, vì retry/failure profile khác delivery email |
| `analytics` | Event facts, dashboard/read model analytics | Có, vì storage/compute thường khác hẳn |
| `operations` | Outbox, DLQ, replay, rate limit/circuit state views | Có thể là platform/runtime trước, rồi tách capability |
| `audit` | Audit entry cho action nhạy cảm | Có thể là module nhỏ hoặc capability dùng chung |
| `notification` | Internal/user notification như welcome/system email | Có thể trì hoãn, không phải core send-flow đầu tiên |

## Gợi ý phase triển khai

Giai đoạn đầu nên giữ số module vừa đủ để tránh over-engineering:

| Phase | Module ưu tiên |
|---|---|
| Phase 1 | `identity`, `access` nếu không gộp, `sender`, `audience`, `content`, `campaign`, `delivery`, `ingestion`, `suppression`, `analytics` |
| Phase 2 | `tracking`, `webhooks`, `operations`, `audit` tách rõ hơn nếu code bắt đầu phình |
| Phase 3 | Tách service vật lý cho `delivery`, `ingestion`, `analytics`, `tracking` nếu tải hoặc failure profile khác biệt |

Nếu chưa chắc boundary nào ổn định, giữ module trong monolith nhưng vẫn áp dụng contract/import rule như thể ngày mai có thể tách process.
