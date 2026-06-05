# Feature Map

Bộ tài liệu này liệt kê toàn bộ feature của `send-flow` theo **capability sản phẩm**. Mục tiêu là giúp người đọc trả lời nhanh: dự án này có những tính năng gì, nhóm nào là user-facing, nhóm nào là capability nền, và nên đọc sâu ở đâu.

## Cách đọc nhanh

Graph tổng và capability trace nằm ở [../00-doc-graph.md](../00-doc-graph.md).

| Cần trả lời | Đọc file |
|---|---|
| Hệ thống quản trị user, workspace, quyền, sender và cấu hình thế nào? | `01-control-plane.md` |
| Sản phẩm quản lý audience, contacts và template ra sao? | `02-audience-and-content.md` |
| Hệ thống gửi campaign và transactional email thế nào? | `03-delivery-and-messaging.md` |
| Webhook, tracking, suppression và các tín hiệu hậu gửi được xử lý ra sao? | `04-ingestion-tracking-suppression.md` |
| Dashboard, analytics và capability vận hành gồm những gì? | `05-analytics-and-operations.md` |

## Feature inventory tổng quan

| Nhóm | Mục tiêu | Ví dụ feature chính |
|---|---|---|
| Control plane | Quản trị danh tính, tenant, quyền và cấu hình gửi | auth, social sign-in, session, workspace, membership, role/permission, API key, sender/domain, webhook config, settings, audit |
| Audience and content | Quản lý người nhận và nội dung gửi | contacts, lists, segments, import/export, send eligibility, templates, preview/render |
| Delivery and messaging | Điều phối và thực thi gửi email | campaigns, transactional email, scheduling, queueing, routing, throttling, retry, delivery state, logs |
| Ingestion, tracking, suppression | Nhận tín hiệu từ provider và recipient sau khi gửi | provider webhook, normalization, dedupe, bounce/complaint, unsubscribe, suppression list, open/click tracking |
| Analytics and operations | Quan sát hệ thống và vận hành ở quy mô lớn | dashboards, projections, email analytics, queue/retry ops, DLQ/replay, health/readiness, observability, feature flags |

## Boundary của sản phẩm

`send-flow` không chỉ là một API gửi email. Theo kiến trúc hiện tại, phạm vi sản phẩm bao gồm cả:

- Capability user-facing trên dashboard và public API.
- Capability nền bắt buộc để email delivery hoạt động đúng: idempotency, rate limiting, throttling, retry, dedupe, event publishing, projection.
- Capability operations để hệ thống an toàn khi tải cao: DLQ, replay, health/readiness, observability, feature flags, tenant fairness, backpressure.

## Actor chính

| Actor | Tương tác chính |
|---|---|
| Dashboard user | Quản lý workspace, audience, template, campaign, analytics, settings |
| Public API client | Gửi transactional email, dùng API key để thao tác resource |
| Operator / developer | Theo dõi metric, logs, traces, queue lag, xử lý DLQ/replay, rollout feature flag |
| Email provider | Nhận send request, trả webhook delivery/tracking/suppression event |
| Recipient | Mở email, click link, unsubscribe, phát sinh bounce/complaint |

## Nguyên tắc đọc bộ feature

- Feature được nhóm theo **năng lực sản phẩm**, không theo package Go hay runtime API/Worker.
- Một feature lớn có thể bao gồm cả feature phụ và capability nền nếu chúng là một phần có chủ đích của hành vi sản phẩm.
- Nếu một capability xuất hiện ở nhiều nhóm, tài liệu sẽ đặt một nơi chính và chỗ khác chỉ nhắc theo góc nhìn phụ thuộc.
- Bộ này bám theo `docs/architecture/*`; không tự mở rộng thành roadmap mới.
