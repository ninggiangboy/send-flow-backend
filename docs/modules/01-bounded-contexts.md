# Bounded Contexts And Module Boundaries

File này định nghĩa cách hiểu bounded context trong `send-flow` và tiêu chí tách module.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Module catalog | `03-module-catalog.md` |
| Public API/giao tiếp module | `02-communication-and-public-api.md` |
| Architecture style | `../architecture/03-architecture-style.md` |
| Architecture principles | `../architecture/02-architecture-principles.md` |
| Code architecture | `../architecture/04-code-architecture.md` |

## Mục đích chia module

Chia module không phải để có nhiều folder đẹp hơn. Mục đích là:

- Giữ business rule ở đúng nơi sở hữu ngôn ngữ nghiệp vụ.
- Giới hạn blast radius khi thay đổi một capability.
- Ngăn module này update trực tiếp state của module khác.
- Cho phép triển khai modular monolith trước, rồi tách microservice khi có lý do vận hành thật.
- Làm rõ contract cần ổn định: command/query, event, read model, migration ownership.

## Bounded context là gì?

Bounded context là boundary nơi một khái niệm có ý nghĩa nhất quán.

Ví dụ `Message`:

| Context | Ý nghĩa |
|---|---|
| `campaign` | Message candidate được lập kế hoạch từ audience |
| `delivery` | Đơn vị gửi có state machine, attempt, retry và provider id |
| `analytics` | Fact event hoặc metric row dùng để báo cáo |

Vì ý nghĩa khác nhau, không nên tạo một model `Message` dùng chung cho mọi nơi. Mỗi context có model riêng và trao đổi bằng contract.

## Tiêu chí tách module

Nên tách thành module riêng khi capability có ít nhất một trong các dấu hiệu:

- Có aggregate/lifecycle/invariant riêng.
- Có bảng transaction chính cần owner rõ.
- Có public event hoặc command được context khác dùng.
- Có scaling/failure profile khác phần còn lại.
- Có team hoặc cadence phát triển có thể độc lập về sau.
- Có ngôn ngữ nghiệp vụ đủ khác để model dùng chung gây nhầm lẫn.

Không nên tách chỉ vì:

- Có vài CRUD endpoint nhỏ.
- Muốn tránh file lớn nhưng chưa có boundary nghiệp vụ.
- Một helper kỹ thuật được nhiều nơi dùng.
- Module đó chỉ là adapter của hạ tầng như Redis/Kafka/PostgreSQL.

## Ownership rule

Mỗi module sở hữu:

- Aggregate/entity giao dịch chính.
- Repository và migration của bảng giao dịch chính.
- Domain invariant và state transition.
- Public application command/query mà runtime hoặc module khác được gọi.
- Integration event trong `contracts`.
- Consumer xử lý event nếu side effect thuộc module đó.

Module khác không được:

- Ghi trực tiếp bảng nghiệp vụ do module owner sở hữu.
- Import package `domain`, `infrastructure` hoặc repository implementation của module owner.
- Dựa vào schema private để thay thế public query/event.
- Publish event với tên producer context khác.

## Data ownership

Quy tắc mặc định:

| Dữ liệu | Owner |
|---|---|
| User, workspace, membership, session | `identity` |
| Permission evaluation, API key scope | `access` hoặc `identity` giai đoạn đầu |
| Sender domain, DNS record status | `sender` |
| Contact, list, segment, import/export | `audience` |
| Template, template version, render snapshot | `content` |
| Campaign, schedule, message candidate | `campaign` |
| Message, delivery attempt, retry state | `delivery` |
| Raw provider webhook, normalized provider event | `ingestion` |
| Suppression entry, unsubscribe decision | `suppression` |
| Tracking link, tracking event | `tracking` |
| Customer webhook delivery | `webhooks` |
| Email event fact, dashboard projections | `analytics` |
| Outbox, processed marker, DLQ, replay | `operations` hoặc platform-owned operational schema |
| Audit entry | `audit` |

Nếu một screen cần dữ liệu từ nhiều module, ưu tiên read model/projection thay vì join trực tiếp qua bảng private của nhiều owner.

## Strong consistency và eventual consistency

Trong modular monolith:

- Strong consistency nằm trong transaction của một module/aggregate.
- Cross-module state propagation mặc định là eventual consistency qua outbox + event.
- Synchronous cross-module call chỉ dùng cho decision cần trả lời ngay trong command path.

Ví dụ:

```text
campaign.Schedule
  -> gọi public query của content để kiểm tra template published
  -> gọi public query của sender để kiểm tra sender verified
  -> lưu campaign scheduled trong transaction của campaign
  -> ghi outbox event campaign.scheduled.v1
```

Không làm:

```text
campaign.Schedule
  -> update sender_domains
  -> update templates
  -> update messages
```

`campaign` có thể kiểm tra readiness của context khác, nhưng không sở hữu lifecycle của sender/template/message.

## Khi nào tách microservice?

Chỉ tách service vật lý khi module đã có boundary code sạch và có ít nhất một lý do vận hành:

- Cần scale độc lập.
- Cần deploy cadence độc lập.
- Failure cần isolate khỏi API chính hoặc worker khác.
- Resource profile khác hẳn, ví dụ analytics ingestion hoặc tracking endpoint traffic lớn.
- Contract đã ổn định và có consumer rõ.
- Có observability/runbook để vận hành service riêng.

Folder module sạch là điều kiện cần, không phải lý do đủ để tách microservice.
