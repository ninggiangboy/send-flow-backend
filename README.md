# send-flow

A modular monolith email delivery platform built in Go. send-flow provides a complete SaaS backend for building, operating, and observing email delivery flows — from sender domain authentication and audience management to campaign orchestration, transactional email, delivery analytics, and provider webhook ingestion.

## Architecture

send-flow follows a **Clean Architecture** + **Modular Monolith** + **DDD-lite** + **Event-driven** approach. Two runtimes share the same business modules:

| Runtime | Role |
|---|---|
| **API service** (`cmd/api`) | HTTP API — authentication, workspace management, CRUD operations, webhook ingress, health/readiness |
| **Worker service** (`cmd/worker`) | Background processing — Kafka consumers, outbox publisher, delivery pipeline, scheduled jobs, analytics projection |

### System Context

```mermaid
flowchart LR
    Client["Dashboard / API client"]
    Recipient["Recipient"]
    Provider["Email provider"]
    Operator["Operator / Developer"]

    Gateway["Gateway / Load Balancer"]
    API["API service"]
    Worker["Worker service"]

    Postgres[("PostgreSQL")]
    Redis[("Redis")]
    Kafka[("Kafka / Redpanda")]
    ClickHouse[("ClickHouse")]
    Observability["Observability stack"]
    ObjectStorage[("MinIO / S3")]

    Client --> Gateway --> API
    Recipient --> Gateway
    Provider --> Gateway

    API --> Postgres
    API --> Redis
    API --> Kafka
    API --> ObjectStorage
    API --> Observability

    Worker --> Kafka
    Worker --> Postgres
    Worker --> Redis
    Worker --> ClickHouse
    Worker --> Provider
    Worker --> ObjectStorage
    Worker --> Observability

    Operator --> Observability
```

### Data Flow

The system's data flow is decomposed into focused sequence diagrams by concern. Each diagram shows only the participants relevant to that flow.

---

#### 1. Authentication & Identity Flow

User registration, login, MFA, OAuth, and session management.

```mermaid
sequenceDiagram
    autonumber
    participant Client as Dashboard/API client
    participant API as API service
    participant Auth as Identity/Access
    participant DB as PostgreSQL

    Client->>API: POST /api/v1/auth/signup or /auth/login
    API->>Auth: signup/login, MFA, OAuth, refresh session
    Auth->>DB: Persist user, session, tokens, workspace membership
    Auth->>DB: Save outbox event: user.registered.v1
    API-->>Client: JWT session + workspace context
```

---

#### 2. Sender Setup & Audience Management

Sender domain verification, contact/list/segment management, and template publishing.

```mermaid
sequenceDiagram
    autonumber
    participant Client as Dashboard/API client
    participant API as API service
    participant Setup as Sender/Audience/Content
    participant DB as PostgreSQL

    rect rgba(70, 130, 180, 0.08)
        Note over Client,DB: Sender Domain
        Client->>API: POST /workspaces/{id}/sender-domains
        API->>Setup: Create sender domain and DNS records
        Setup->>DB: Persist sender domain
        Client->>API: POST /sender-domains/{domain_id}/verify
        API->>Setup: Refresh DNS readiness
        Setup->>DB: Mark verified/failed
    end

    rect rgba(60, 179, 113, 0.08)
        Note over Client,DB: Audience Management
        Client->>API: POST /contacts, /lists, /segments, /audience/imports
        API->>Setup: Manage audience and import/export jobs
        Setup->>DB: Persist contacts, lists, segments, jobs
    end

    rect rgba(218, 165, 32, 0.08)
        Note over Client,DB: Template Management
        Client->>API: POST /templates and /templates/{id}/publish
        API->>Setup: Validate, render, publish template version
        Setup->>DB: Persist template and immutable version
    end
```

---

#### 3. Campaign Lifecycle

Campaign creation, scheduling, outbox publishing, and message queuing via the worker.

```mermaid
sequenceDiagram
    autonumber
    participant Client as Dashboard/API client
    participant API as API service
    participant Campaign as Campaign app
    participant Delivery as Delivery app
    participant DB as PostgreSQL
    participant Outbox as Outbox publisher
    participant Kafka as Kafka/Redpanda
    participant Worker as Worker service

    rect rgba(100, 100, 200, 0.08)
        Note over Client,DB: Campaign Draft
        Client->>API: POST /workspaces/{id}/campaigns
        API->>Campaign: Create/update campaign draft
        Campaign->>DB: Save campaign, audience ref, template ref
    end

    rect rgba(100, 100, 200, 0.08)
        Note over Client,Kafka: Scheduling & Outbox
        Client->>API: POST /campaigns/{campaign_id}/schedule
        API->>Campaign: Validate audience, published template, sender readiness
        Campaign->>DB: Save scheduled campaign
        Campaign->>DB: Save campaign.scheduled.v1 outbox event
        API-->>Client: Campaign scheduled

        Outbox->>DB: Poll unpublished outbox_events
        Outbox->>Kafka: Publish campaign.scheduled.v1
        Outbox->>DB: Mark published
    end

    rect rgba(100, 100, 200, 0.08)
        Note over Kafka,Worker: Consumer & Message Queue
        Kafka->>Worker: CampaignScheduledConsumer
        Worker->>Delivery: Queue campaign messages
        Delivery->>DB: Resolve candidates and create delivery messages
        Delivery->>DB: Save delivery.message_queued.v1 outbox events
    end
```

---

#### 4. API Key Quotas & Transactional Send

API keys can carry optional per-minute, per-hour, per-day, and per-month email quota limits. Transactional sends authenticate with API-key scope, resolve quota limits through cache-aside reads, consume quota before durable acceptance, and refund consumed units when acceptance fails.

```mermaid
sequenceDiagram
    autonumber
    participant Client as Dashboard/API client
    participant API as API service
    participant Auth as Identity/Access
    participant Delivery as Delivery app
    participant Redis as Redis
    participant DB as PostgreSQL
    participant ObjectStorage as MinIO/S3
    participant Outbox as Outbox publisher
    participant Kafka as Kafka/Redpanda

    rect rgba(70, 130, 180, 0.08)
        Note over Client,Redis: API Key Quota Configuration
        Client->>API: POST/PATCH /workspaces/{id}/api-keys
        API->>Auth: Validate scopes and email_quota_limits
        Auth->>DB: Persist API key and quota limits
        API->>Redis: Invalidate cached quota limits after update
        API-->>Client: API key metadata
    end

    rect rgba(60, 179, 113, 0.08)
        Note over Client,Outbox: Transactional Send Acceptance
        Client->>API: POST /transactional/send (with API key)
        API->>Auth: Authenticate API key and require transactional.send
        API->>Delivery: Accept transactional send
        Delivery->>DB: Check durable idempotency record
        alt Existing matching idempotency key
            Delivery-->>API: Existing request and message IDs
            API-->>Client: 202 Accepted + existing IDs
        else New request
            Delivery->>DB: Validate sender, template, suppression rules
            Delivery->>Redis: Load cached quota limits or fetch from DB
            Delivery->>Redis: Atomically consume API-key quota buckets
            opt Attachments
                Delivery->>ObjectStorage: Upload attachment objects
            end
            Delivery->>DB: Create request, messages, events, outbox rows
            alt Accept failed after quota consume
                Delivery->>Redis: Best-effort quota refund
                Delivery->>ObjectStorage: Best-effort uploaded attachment cleanup
                Delivery-->>API: Error or recovered idempotency result
            else Accepted
                API-->>Client: 202 Accepted + request/message IDs
            end
        end
        Outbox->>DB: Poll unpublished delivery outbox rows
        Outbox->>Kafka: Publish delivery events
    end
```

---

#### 5. Email Delivery & ESP Interaction

Outbox-driven message processing, suppression checks, ESP sending, and retry state management.

```mermaid
sequenceDiagram
    autonumber
    participant Outbox as Outbox publisher
    participant Kafka as Kafka/Redpanda
    participant Worker as Worker service
    participant Delivery as Delivery app
    participant DB as PostgreSQL
    participant ESP as Email provider

    Outbox->>Kafka: Publish delivery.message_queued.v1
    Kafka->>Worker: DueMessageProcessor / delivery consumer
    Worker->>Delivery: Check suppression, sender readiness, retry state
    Delivery->>ESP: Send email
    ESP-->>Delivery: Accepted or failed
    Delivery->>DB: Persist attempt, message state, retry state
    Delivery->>DB: Save accepted/bounced/retry outbox event
```

---

#### 6. Webhook Ingestion & Tracking

Provider webhook receipt, normalization, and recipient interaction tracking (opens, clicks, unsubscribes).

```mermaid
sequenceDiagram
    autonumber
    participant ESP as Email provider
    participant Recipient as Recipient
    participant API as API service
    participant Ingestion as Ingestion/Tracking
    participant DB as PostgreSQL

    rect rgba(255, 100, 100, 0.08)
        Note over ESP,DB: Provider Webhook Ingestion
        ESP->>API: POST /webhooks/providers/{provider}
        API->>Ingestion: Verify, deduplicate, normalize provider webhook
        Ingestion->>DB: Store raw webhook and normalized event
        Ingestion->>DB: Save ingestion.provider_event.normalized.v1 outbox event
    end

    rect rgba(100, 200, 100, 0.08)
        Note over Recipient,DB: Open / Click / Unsubscribe
        Recipient->>API: GET /o/{tracking_id} or /t/{tracking_id} or /u/{token}
        API->>Ingestion: Record open, click, or unsubscribe
        Ingestion->>DB: Persist tracking event and suppression update
        Ingestion->>DB: Save tracking event outbox event
    end
```

---

#### 7. Analytics, Notifications & Operations

Downstream event consumption, analytics projections, customer webhook delivery, and operational queries.

```mermaid
sequenceDiagram
    autonumber
    participant Outbox as Outbox publisher
    participant Kafka as Kafka/Redpanda
    participant Worker as Worker service
    participant Analytics as ClickHouse
    participant Webhooks as Customer webhooks
    participant ESP as Email provider
    participant DB as PostgreSQL
    participant Client as Dashboard/API client
    participant API as API service
    participant Ops as Operations app

    rect rgba(180, 100, 200, 0.08)
        Note over Outbox,DB: Event Consumption & Projection
        Outbox->>Kafka: Publish delivery, ingestion, tracking, identity events
        Kafka->>Worker: Analytics, notification, webhook consumers
        Worker->>Analytics: Map events into facts, projections, timelines
        Worker->>Webhooks: Create and deliver customer webhook attempts
        Worker->>ESP: Send welcome, invitation, system notifications
        Worker->>DB: Persist notification and webhook delivery state
    end

    rect rgba(180, 100, 200, 0.08)
        Note over Client,DB: Operational Queries
        Client->>API: GET /analytics/*, /messages/*, /operations/*
        API->>Analytics: Query funnels, timeseries, timelines, incidents
        API->>Ops: Query outbox, DLQ, replay jobs, sync lag
        Ops->>DB: Read operational state or create replay job
        API-->>Client: Reports, delivery status, operations views
    end
```

## Tech Stack

| Component | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP Framework | Chi router, Huma OpenAPI |
| Database | PostgreSQL (via pgx), ClickHouse (analytics) |
| Cache | Redis (session, rate-limit, idempotency, locks) |
| Event Bus | Kafka / Redpanda + Debezium (CDC outbox) |
| Object Storage | MinIO (dev) / AWS S3 (prod) |
| Email Sending | AWS SES (prod), SMTP / Mailpit (dev) |
| Auth | JWT, MFA/TOTP, OAuth 2.0, API keys, RBAC |
| Observability | Prometheus, Grafana, Loki, Tempo |

## Business Capabilities

| Plane | Capabilities |
|---|---|
| **Control Plane** | Auth, user/session, workspace, membership/role, API keys, sender domain verification, templates, campaigns, suppression rules |
| **Delivery Plane** | Message queue, throttle (per workspace/provider), email provider calls, retry, delivery result classification |
| **Audience Plane** | Contacts, lists, segments, import/export, send eligibility |
| **Ingestion Plane** | Provider webhook receipt, verification, deduplication, normalization |
| **Analytics Plane** | Event storage (ClickHouse), projections, reports, dashboards |
| **Operations Plane** | Outbox lag, DLQ/replay, rate-limit state, circuit breakers, health/readiness |

## Codebase Structure

```
cmd/
├── api/                    # API entry point
└── worker/                 # Worker entry point

internal/
├── apps/
│   ├── api/                # HTTP handlers, route wiring, OpenAPI registration
│   └── worker/             # Consumer wiring, job scheduling
├── modules/                # Business capabilities (domain/app/ports/infrastructure)
│   ├── identity/           # Auth, sessions, workspaces, memberships, RBAC
│   ├── access/             # API keys, permission evaluation
│   ├── audience/           # Contacts, lists, segments, import/export
│   ├── content/            # Email templates, versioning, rendering
│   ├── campaign/           # Campaign CRUD, scheduling, lifecycle
│   ├── delivery/           # Message lifecycle, provider routing, retry
│   ├── sender/             # Sender domain management, DNS verification
│   ├── suppression/        # Recipient block list
│   ├── tracking/           # Open/click tracking
│   ├── ingestion/          # Provider webhook handling
│   ├── analytics/          # Event facts, reports, projections
│   ├── webhooks/           # Outbound customer webhooks
│   ├── notification/       # Internal product notifications
│   ├── audit/              # Append-only audit log
│   └── operations/         # Outbox lag, DLQ, replay, runtime health
└── platform/               # Shared foundations
    ├── postgres/           # pgx connection pool, transactions
    ├── kafka/              # Producer/consumer abstraction
    ├── redis/              # Client wrapper
    ├── outbox/             # Transactional outbox orchestration
    ├── email/              # SES/SMTP/Mailpit senders
    ├── auth/               # JWT, middleware
    ├── config/             # Env-based configuration
    └── ...                 # clock, id, logger, validator, etc.
```

## Prerequisites

- Go 1.25+
- Docker + Docker Compose

## Quick Start

Use this flow for local development.

1. Create local env file:

```bash
cp .env.example .env
```

2. Start local infrastructure:

```bash
make dev-up
```

3. Run database migrations:

```bash
make migrate-up
```

4. Run API:

```bash
make api
```

5. Run worker in a separate terminal:

```bash
make worker
```

Optional: set `AUTO_MIGRATE=true` in `.env` to run migrations automatically on API startup.

## Common Commands

```bash
make dev-up        # Start local infra
make dev-down      # Stop and remove local infra
make dev-logs      # Tail Docker Compose logs
make api           # Run API locally
make worker        # Run worker locally
make start 2       # Run 2 API and 2 worker instances with automatic ports
make build         # Build ./bin/api and ./bin/worker
make format        # gofmt for cmd/ and internal/
make vet           # go vet ./...
make test          # go test ./...
make check         # Format, vet and test
make test-race     # Run race detector over internal packages
make test-integration # Run Docker-backed integration tests
make openapi-generate # Regenerate docs/api/openapi.yaml
make migrate-up    # Apply migrations
make migrate-down  # Rollback one migration step
```

## Local Service Endpoints

| Service | URL |
|---|---|
| API | http://localhost:8081 |
| API Gateway | http://localhost:8088 |
| OpenAPI JSON | http://localhost:8081/openapi.json |
| Gateway OpenAPI JSON | http://localhost:8088/openapi.json |
| Worker health/metrics | http://localhost:8082 |
| Consul | http://localhost:8500 |
| Traefik Dashboard | http://localhost:8089/dashboard/ |
| Redpanda Console | http://localhost:8080 |
| Kafka Connect | http://localhost:8083 |
| Mailpit UI | http://localhost:8025 |
| MinIO Console | http://localhost:9002 |
| Grafana | http://localhost:3001 (admin / admin) |
| Prometheus | http://localhost:9090 |

## Health Check

```bash
curl http://localhost:8081/api/readyz
curl http://localhost:8088/api/readyz
curl http://localhost:8082/api/readyz
```

Health responses include `version`, `git_sha`, and `build_time` when binaries are built through `make build`.

## Local API Gateway

`make dev-up` starts Consul and Traefik with the rest of the local infrastructure. When `SERVICE_DISCOVERY_PROVIDER=consul` is set, API processes register in Consul, and Traefik discovers them through Consul Catalog. Worker processes also register as `sendflow-worker` so Prometheus and Grafana can discover every local worker instance without hard-coded scrape ports.

```bash
make dev-up
make api
curl http://localhost:8500/v1/catalog/service/sendflow-api
curl http://localhost:8088/api/readyz
```

Use `make start N` to run N API and N worker instances. Local ports are selected randomly from available high ports and skipped if they are already used by another process or by an earlier instance in the same run. Override the range with `LOCAL_RANDOM_PORT_MIN` and `LOCAL_RANDOM_PORT_MAX` when needed. API and worker service IDs are unique in Consul. Traefik load balances the registered API instances, while Prometheus discovers both `sendflow-api` and `sendflow-worker`; Grafana dashboards expose an `instance` filter for checking each process.

## Staging Deployment

Staging should mirror production dependencies with lower capacity and isolated credentials.

1. Build release artifacts:

```bash
make check
make build VERSION=staging
```

2. Provision staging Postgres, Redis, Kafka/Redpanda, ClickHouse when analytics is enabled, object storage when import/export is enabled, and an email provider suitable for staging.
3. Apply migrations with staging `DATABASE_URL`.
4. Deploy API and worker with staging environment variables.
5. Smoke test health, auth, transactional send, provider webhook ingestion, worker lag, and analytics freshness.

## Production Deployment

Production deployment follows the launch checklist in `docs/production/launch-checklist.md`.

Before promoting a release:

- Review `docs/production/readiness-matrix.md`.
- Confirm module contracts in `docs/production/module-contracts.md` still match the release.
- Confirm route exposure in `docs/production/http-route-classification.md`.
- Run `make check` and build with an explicit version:

```bash
make build VERSION=<release-version>
```
