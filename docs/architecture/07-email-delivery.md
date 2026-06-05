# Email Delivery Platform

Email delivery là domain trung tâm của `send-flow`, nên tài liệu này đọc theo capability thay vì theo tool kỹ thuật.

## Liên kết liên quan

| Góc nhìn | File |
|---|---|
| Delivery feature | `../features/03-delivery-and-messaging.md` |
| Ingestion/tracking/suppression feature | `../features/04-ingestion-tracking-suppression.md` |
| Delivery API | `../api/03-messaging-and-delivery.md` |
| Events/webhooks API | `../api/04-events-webhooks-and-operations.md` |
| Messaging model | `../model/03-messaging-and-delivery.md` |
| Delivery/events schema | `../database/03-delivery-and-events.md` |

## Capability map

```text
sender authentication
  -> verify domain/DNS/provider credential

template rendering
  -> render content safely and reproducibly

campaign orchestration
  -> decide who should receive what and when

delivery pipeline
  -> queue, throttle, send, retry, classify result

provider webhook ingestion
  -> normalize external provider events

suppression
  -> protect recipients and sender reputation

tracking
  -> capture open/click with privacy and abuse controls

analytics
  -> aggregate delivery health and campaign performance
```

Vì `send-flow` là **email delivery platform**, domain email phải được xem là core architecture. Đây không chỉ là chuyện gửi một email qua provider; hệ thống cần quản lý sender reputation, authentication, suppression, event ingestion, throttling, tracking và compliance.

## Module đề xuất cho email domain

Khi domain lớn dần, nên tách capability theo module:

```text
internal/modules
├── identity          # user, tenant, membership, auth/session
├── sender           # sending domain, sender identity, DNS verification
├── template         # template, version, rendering, preview
├── campaign         # campaign/flow, audience, schedule
├── delivery         # message, recipient, provider routing, send orchestration
├── suppression      # unsubscribe, bounce, complaint, block/allow list
├── tracking         # open/click/unsubscribe tracking
├── webhook          # provider event ingestion
└── analytics        # delivery metrics, aggregate reporting
```

Không cần tạo tất cả module ngay từ đầu. Nhưng nên thiết kế event/contract để sau này tách được mà không đổi toàn bộ flow.

## Email provider adapter

Giai đoạn đầu nên dùng provider có sẵn, không tự vận hành MTA.

Khuyến nghị:

```text
Primary provider   AWS SES
Adapter-ready      Mailgun, Postmark, SendGrid
Local/dev          fake provider hoặc Mailpit
```

Không nên tự host Postfix/MTA sớm vì deliverability, IP reputation, feedback loop, bounce classification và abuse handling rất nặng.

Port đề xuất:

```go
type EmailProvider interface {
    Send(ctx context.Context, msg Message) (*SendResult, error)
}

type SendResult struct {
    ProviderMessageID string
    Provider          string
    AcceptedAt        time.Time
}
```

Adapter implementation:

```text
internal/modules/delivery/infrastructure/ses
internal/modules/delivery/infrastructure/mailgun
internal/modules/delivery/infrastructure/postmark
```

Provider routing nên nằm trong `delivery/app`, không nằm trong provider adapter.

## Sender authentication và DNS verification

Deliverability phụ thuộc mạnh vào domain authentication.

Bắt buộc cho sending domain:

```text
SPF
DKIM
DMARC
MX check nếu nhận inbound/reply
Return-Path / MAIL FROM domain alignment nếu provider hỗ trợ
```

Nên cân nhắc sau:

```text
BIMI
MTA-STS
TLS-RPT
DMARC aggregate report ingestion
Google Postmaster Tools integration/manual monitoring
```

Domain verification workflow:

```text
tenant adds sending domain
  -> system generates DNS records
  -> tenant adds records
  -> scheduled verifier checks DNS
  -> mark domain verified/authenticated
  -> enable sending gradually
```

DNS record model nên lưu:

```text
record_type
host/name
expected_value
current_value
status
last_checked_at
failure_reason
```

Không cho gửi production traffic từ domain chưa pass authentication tối thiểu.

## Suppression service

Suppression là core capability của email delivery platform.

Các scope suppression:

```text
global              # không gửi bất kỳ tenant/campaign nào
tenant              # không gửi trong tenant này
list/audience       # unsubscribe khỏi một list cụ thể
campaign/category   # unsubscribe khỏi một category
provider            # provider-level suppression mirror nếu có
```

Các reason:

```text
unsubscribe
hard_bounce
complaint
manual_block
invalid_recipient
policy_violation
```

Rule:

- Complaint và hard bounce phải suppress trước khi retry campaign.
- Marketing email phải check unsubscribe/suppression trước khi enqueue send.
- Transactional email vẫn phải tôn trọng complaint/hard bounce/global block.
- Suppression write phải idempotent.
- Suppression decision nên được audit log.

Schema gợi ý:

```text
suppression_entries
├── id
├── tenant_id
├── email_hash
├── email_normalized
├── scope
├── reason
├── source
├── created_at
└── metadata
```

Với PII, cân nhắc hash email để lookup và hạn chế lộ dữ liệu trong log/export.

## Provider webhook ingestion

Provider events là nguồn sự thật cho delivery outcome sau khi provider nhận message.

Event nên nhận:

```text
accepted/sent
delivered
delivery_delayed
bounced
complained
rejected
opened
clicked
unsubscribed
rendering_failed
```

Webhook endpoint làm:

```text
receive provider webhook
  -> verify signature
  -> normalize provider event
  -> idempotency check by provider event id/message id/event type
  -> persist raw event
  -> publish normalized event to Kafka/outbox
  -> update delivery state/projection
```

Không xử lý business side effect phức tạp trực tiếp trong HTTP webhook request. Request nên nhanh, idempotent và retry-safe.

Nên lưu cả:

```text
raw_provider_payload
normalized_event
provider_message_id
message_id
tenant_id
event_type
occurred_at
received_at
```

## Delivery pipeline

Flow gửi email nên đi qua queue/scheduler thay vì gửi trực tiếp trong HTTP request.

```text
API creates campaign/message
  -> validate template/audience/suppression
  -> create message records
  -> enqueue delivery jobs

delivery worker
  -> pick due jobs
  -> check suppression again
  -> check rate limit/throttle
  -> render template version
  -> choose provider
  -> send via provider adapter
  -> store provider_message_id
  -> wait for provider webhook outcome
```

State machine gợi ý:

```text
queued
scheduled
sending
accepted
delivered
delayed
bounced
complained
rejected
suppressed
failed
cancelled
```

Không coi provider `accepted/sent` là delivered. Delivered chỉ đến từ provider/recipient server event nếu provider hỗ trợ.

## Rate limiting và throttling

Email sending cần throttle theo nhiều chiều để bảo vệ reputation.

Dimension:

```text
tenant
sending_domain
provider
recipient_domain      # gmail.com, yahoo.com, outlook.com
campaign
message_type          # transactional vs marketing
```

Redis token bucket phù hợp cho fast path. PostgreSQL giữ quota/config bền.

Throttle config ví dụ:

```text
tenant: 10_000/day
domain: 1_000/hour
gmail.com: 100/minute
new_domain_warmup_day_1: 50/day
```

Nên có warm-up policy cho domain/IP mới. Blast traffic lớn từ domain mới là cách rất nhanh để hỏng reputation.

## Tracking service

Tracking là capability riêng, không nhét vào provider adapter.

Tracking gồm:

```text
open pixel
click redirect
unsubscribe endpoint
preference center
link signing
bot filtering heuristic
```

Rule:

- Tracking link phải có signed token, không expose raw recipient id dễ đoán.
- Click endpoint ghi event rồi redirect nhanh.
- Open tracking không đáng tin tuyệt đối vì image proxy/privacy protection.
- Không dùng open rate làm tín hiệu duy nhất cho engagement.
- Unsubscribe endpoint phải hoạt động ổn định và idempotent.

Marketing/subscription email nên có:

```text
List-Unsubscribe: <https://.../unsubscribe/...>
List-Unsubscribe-Post: List-Unsubscribe=One-Click
```

One-click unsubscribe xử lý bằng POST, không yêu cầu login, không đưa người dùng qua nhiều bước.

## Template rendering

Template cần versioning vì email đã gửi phải trace được nội dung tại thời điểm gửi.

Stack:

```text
simple transactional email   html/template
component-like Go templates  templ
marketing email layout       MJML render service
CSS inline                   premailer-compatible tool/service nếu cần
local preview                Mailpit
```

Rule:

- Lưu template version immutable khi campaign bắt đầu gửi.
- Validate required variables trước khi enqueue.
- Render failure là event riêng, không retry vô hạn nếu data/template sai.
- Asset public URL phải stable.
- Không cho user-injected HTML/script nếu không sanitize.

## Analytics storage

PostgreSQL đủ cho giai đoạn đầu nếu volume nhỏ.

Khi event volume tăng, cân nhắc:

```text
ClickHouse       delivery/open/click/bounce analytics
PostgreSQL       source of truth, campaign/message state
Redis            hot counters/cache
Object storage   raw event archive/export
```

Chỉ chuyển analytics sang ClickHouse khi PostgreSQL bắt đầu bị kéo bởi event scan/aggregate nặng. Đừng thêm OLAP quá sớm nếu chưa có volume.

ClickHouse ingestion:

```text
normalized email event
  -> Kafka topic: analytics.email.event.recorded
  -> analytics worker batches events
  -> insert into ClickHouse email_events
  -> materialized views update campaign/domain/provider stats
```

ClickHouse nên nhận event đã normalize, không nhận raw webhook payload trực tiếp. Raw payload có thể lưu PostgreSQL ngắn hạn hoặc object storage để debug/replay.

Query dashboard nên đọc:

```text
campaign_daily_stats
recipient_domain_hourly_stats
provider_delivery_stats
```

Không nên scan raw `email_events` cho mọi dashboard nếu traffic lớn.

Metrics quan trọng:

```text
delivery rate
bounce rate
complaint rate
deferral/delay rate
unsubscribe rate
open/click rate
provider latency
recipient-domain latency
queue age
suppression hit rate
```

Alert quan trọng:

```text
complaint rate tăng bất thường
hard bounce rate vượt ngưỡng
gmail/yahoo/outlook deferral tăng
queue age tăng
provider error tăng
webhook ingestion fail
DNS authentication fail
```

## Compliance baseline

Compliance không nên để cuối.

Nên có:

- Consent/audit trail cho marketing recipient.
- Unsubscribe/preference center.
- One-click unsubscribe cho marketing/subscription email.
- Suppression áp dụng trước khi send.
- Physical sender/company info nếu gửi marketing theo luật áp dụng.
- Data retention policy cho event/raw payload.
- Export/delete workflow nếu phục vụ thị trường có yêu cầu privacy.

Không nên gửi marketing cho recipient không có consent rõ hoặc đã complaint/unsubscribe.

## Email-specific local tooling

Local/dev stack nên thêm:

```text
Mailpit hoặc MailHog     capture email local
fake provider adapter    simulate accepted/delivered/bounced/complained
DNS verifier fake mode   test domain verification flow
webhook replay tool      replay provider events
```

Test quan trọng:

- Suppressed recipient không được enqueue.
- Hard bounce tạo suppression.
- Complaint tạo suppression.
- Webhook duplicate không double count.
- Click tracking redirect đúng và ghi event một lần.
- One-click unsubscribe POST tạo suppression đúng scope.
- Template thiếu variable không gửi email lỗi.

## Những thứ chưa nên làm sớm

Chưa nên build ở phase đầu:

- Self-host MTA/Postfix.
- Dedicated IP pool management.
- Automatic IP/domain warmup quá phức tạp.
- Inbox placement testing platform.
- Full DMARC report analytics.
- Multi-provider routing quá thông minh.
- Event sourcing toàn bộ email events.

Nên thiết kế để mở rộng được, nhưng chỉ implement khi có nhu cầu thật và data volume thật.

Nguồn tham khảo chính:

- Google Email sender guidelines: https://support.google.com/mail/answer/81126
- Google sender guidelines FAQ: https://support.google.com/a/answer/14229414
- Amazon SES event publishing: https://docs.aws.amazon.com/ses/latest/dg/event-publishing-retrieving-sns-contents.html
- Amazon SES suppression list: https://docs.aws.amazon.com/ses/latest/DeveloperGuide/sending-email-suppression-list.html
- Mailgun webhooks: https://documentation.mailgun.com/docs/mailgun/user-manual/webhooks/webhooks

---
