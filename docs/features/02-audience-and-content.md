# Audience And Content Features

## Mục tiêu nhóm feature

Nhóm này quản lý đối tượng nhận email và nội dung gửi. Đây là phần chuẩn bị dữ liệu đầu vào cho campaign, transactional messaging và analytics.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| API contract | `../api/02-audience-and-content.md` |
| Business model | `../model/02-audience-and-content.md` |
| Database schema | `../database/02-audience-and-content.md` |
| Module owner | `../modules/03-module-catalog.md` |
| Delivery dependency | `03-delivery-and-messaging.md` |

## Feature chính: Contacts

### Sub-feature / capability chi tiết

- Lưu contact theo workspace.
- Chuẩn hóa email/contact identity phục vụ gửi.
- Quản lý vòng đời contact.
- Tra cứu contact phục vụ audience selection.
- Gắn contact với list, segment và send eligibility.

### Actor sử dụng

- Marketer / operator của workspace
- Public API client nếu có quản lý audience

### Input / output hoặc hành vi chính

- Nhận dữ liệu contact.
- Trả hồ sơ contact và trạng thái liên quan đến audience.

### Phụ thuộc quan trọng

- Workspace boundary
- Suppression data
- Import/export pipeline

### Event / side effect nổi bật

- `contact imported`
- Projection update cho audience

### Ghi chú phạm vi

Đây là feature user-facing lõi của audience plane.

## Feature chính: Lists và segments

### Sub-feature / capability chi tiết

- Tạo list người nhận.
- Quản lý membership của contact trong list.
- Định nghĩa segment để chọn audience linh hoạt.
- Cập nhật segment khi dữ liệu contact thay đổi.
- Dùng segment/list làm đầu vào cho campaign orchestration.

### Actor sử dụng

- Marketer / workspace operator

### Input / output hoặc hành vi chính

- Nhận tiêu chí hoặc membership update.
- Trả tập người nhận đủ điều kiện theo list/segment.

### Phụ thuộc quan trọng

- Contacts
- Suppression/eligibility check
- Campaign planning

### Event / side effect nổi bật

- `segment changed`
- Audience projection refresh

### Ghi chú phạm vi

Feature này là user-facing, nhưng việc đồng bộ eligibility và projection là capability nền quan trọng.

## Feature chính: Import và export audience

### Sub-feature / capability chi tiết

- Import contacts quy mô lớn.
- Export audience data khi cần.
- Theo dõi job import/export.
- Idempotent xử lý import để tránh nhân đôi contact.
- Ghi kết quả import, lỗi dòng và completion state.

### Actor sử dụng

- Marketer / workspace operator
- Operator hỗ trợ dữ liệu

### Input / output hoặc hành vi chính

- Nhận file hoặc batch dữ liệu.
- Trả job state, kết quả nhập/xuất và lỗi nếu có.

### Phụ thuộc quan trọng

- Contacts store
- Background jobs
- Idempotency
- Audit nếu có thay đổi lớn

### Event / side effect nổi bật

- `audience import completed`
- `contact imported`

### Ghi chú phạm vi

Đây là feature có UI/API rõ, đồng thời phụ thuộc mạnh vào worker và processing pipeline.

## Feature chính: Send eligibility

### Sub-feature / capability chi tiết

- Kiểm tra contact có đủ điều kiện nhận email hay không.
- Kết hợp dữ liệu audience với suppression signal.
- Áp dụng rule khác nhau cho marketing và transactional traffic.
- Re-check eligibility trước khi enqueue/send.

### Actor sử dụng

- Campaign planner
- Delivery worker
- Public API flow

### Input / output hoặc hành vi chính

- Nhận contact, workspace context, loại email và audience context.
- Trả quyết định eligible/not eligible cùng lý do.

### Phụ thuộc quan trọng

- Contacts
- Suppression service
- Campaign/delivery rules

### Event / side effect nổi bật

- Eligibility change có thể làm mới projection audience

### Ghi chú phạm vi

Đây không phải một màn hình riêng, nhưng là capability nền bắt buộc của audience plane.

## Feature chính: Templates

### Sub-feature / capability chi tiết

- Tạo và quản lý email template.
- Lưu template source.
- Quản lý template version.
- Publish template version để campaign/transactional flow dùng lại.
- Gắn template với snapshot dùng cho gửi tái lập được.

### Actor sử dụng

- Marketer
- Content editor
- Developer dùng template trong transactional flow

### Input / output hoặc hành vi chính

- Nhận source template và metadata.
- Trả template/version phù hợp cho preview hoặc send pipeline.

### Phụ thuộc quan trọng

- Workspace boundary
- Rendering engine
- Campaign and transactional messaging

### Event / side effect nổi bật

- `template published`

### Ghi chú phạm vi

Đây là feature user-facing ở khu vực Deliver, nhưng còn đóng vai trò shared content asset cho nhiều capability khác.

## Feature chính: Rendering, preview và reproducible content

### Sub-feature / capability chi tiết

- Render nội dung email an toàn và nhất quán.
- Preview template trước khi gửi.
- Tạo rendered snapshot/version để delivery dùng lại.
- Hạn chế sai lệch giữa preview và send-time rendering.
- Hỗ trợ raw send hoặc template-based send trong transactional flow.

### Actor sử dụng

- Marketer
- Developer
- Delivery worker

### Input / output hoặc hành vi chính

- Nhận template/version cùng dữ liệu render.
- Trả preview hoặc rendered content snapshot.

### Phụ thuộc quan trọng

- Template versioning
- Delivery pipeline
- Validation/safety controls

### Event / side effect nổi bật

- `rendering failed`
- Template publish/update propagation

### Ghi chú phạm vi

Feature này nửa user-facing, nửa platform: người dùng thấy preview, còn hệ thống cần reproducibility cho delivery.
