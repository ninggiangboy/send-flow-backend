# send-flow backend

Backend service for `send-flow`, organized as a modular monolith.

## Prerequisites

- Go `1.24+`
- Docker + Docker Compose

## Quick start

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
The worker does not run migrations by default; keep migrations owned by `make migrate-up` or the API startup path.

For auth flows in local development, keep `FRONTEND_BASE_URL` aligned with your frontend app and note that refresh tokens live in the `sf_refresh_token` HttpOnly cookie. Cookie `Secure` is disabled only when `APP_ENV=local`.

## Common commands

```bash
make dev-up        # start local infra
make dev-down      # stop and remove local infra
make dev-logs      # tail docker compose logs
make api           # run API locally
make worker        # run worker locally
make build         # build ./bin/api and ./bin/worker
make format        # gofmt for cmd/ and internal/
make vet           # go vet ./...
make test          # go test ./...
make migrate-up    # apply migrations
make migrate-down  # rollback one migration step
```

## Local service endpoints

- API: `http://localhost:8081`
- OpenAPI JSON: `http://localhost:8081/openapi.json`
- Worker health/metrics: `http://localhost:8082`
- Kafka UI (Redpanda Console): `http://localhost:8080`
- Kafka Connect: `http://localhost:8083`
- Mailpit UI: `http://localhost:8025`
- MinIO Console: `http://localhost:9002`
- Grafana: `http://localhost:3001` (`admin` / `admin`)
- Prometheus: `http://localhost:9090`

## Health check

```bash
curl http://localhost:8081/api/readyz
curl http://localhost:8082/api/readyz
```
