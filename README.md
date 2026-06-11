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

```mermaid
sequenceDiagram
    autonumber
    participant Client as Dashboard/API client
    participant API as API service
    participant DB as PostgreSQL
    participant Outbox as Outbox publisher
    participant Kafka as Kafka/Redpanda
    participant Worker as Worker service
    participant ESP as Email provider
    participant Analytics as ClickHouse

    Client->>API: Create campaign
    API->>DB: Save campaign + outbox event
    API-->>Client: 202 Accepted

    Outbox->>DB: Poll unpublished events
    Outbox->>Kafka: Publish campaign event
    Outbox->>DB: Mark published

    Kafka->>Worker: Consume message queued event
    Worker->>DB: Check suppression + rate limit
    Worker->>ESP: Send email
    ESP-->>Worker: Accepted
    Worker->>DB: Persist delivery state
    Worker->>Kafka: Publish delivery event
    Kafka->>Analytics: Consume delivery event
    Analytics->>Analytics: Update projections
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
make build         # Build ./bin/api and ./bin/worker
make format        # gofmt for cmd/ and internal/
make vet           # go vet ./...
make test          # go test ./...
make migrate-up    # Apply migrations
make migrate-down  # Rollback one migration step
```

## Local Service Endpoints

| Service | URL |
|---|---|
| API | http://localhost:8081 |
| OpenAPI JSON | http://localhost:8081/openapi.json |
| Worker health/metrics | http://localhost:8082 |
| Redpanda Console | http://localhost:8080 |
| Kafka Connect | http://localhost:8083 |
| Mailpit UI | http://localhost:8025 |
| MinIO Console | http://localhost:9002 |
| Grafana | http://localhost:3001 (admin / admin) |
| Prometheus | http://localhost:9090 |

## Health Check

```bash
curl http://localhost:8081/api/readyz
curl http://localhost:8082/api/readyz
```
