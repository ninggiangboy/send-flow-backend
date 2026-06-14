# Repository Guidelines

## Project Structure & Module Organization

This Go backend is a modular monolith. Entrypoints live in `cmd/api` and `cmd/worker`; runtime orchestration is in `internal/apps/api` and `internal/apps/worker`. Business capabilities live under `internal/modules/<module>` with `domain`, `app`, `ports`, `contracts`, and `infrastructure`. Shared foundations live in `internal/platform`. Migrations are in `migrations`, deployment assets in `deployments`, and docs in `docs`.

Tests are colocated with implementation files as `*_test.go`, for example `internal/modules/identity/domain/auth_test.go` and `internal/apps/api/auth_test.go`.

## Build, Test, and Development Commands

- `make dev-up`: start local infrastructure with Docker Compose and register the outbox connector.
- `make dev-down`: stop local infrastructure and remove volumes.
- `make api`: run the API service from `./cmd/api`.
- `make worker`: run the worker service from `./cmd/worker`.
- `make start`: run API and worker together.
- `make build`: compile binaries into `./bin/api` and `./bin/worker`.
- `make format`: run `gofmt` over `cmd/` and `internal/`.
- `make vet`: run `go vet ./...`.
- `make test`: run `go test ./...`.
- `make migrate-up` / `make migrate-down`: apply or roll back Goose migrations using `DATABASE_URL` from `.env`.

## Coding Style & Naming Conventions

Use standard Go formatting with tabs via `gofmt`. Keep controllers, consumers, repositories, and mappers thin: business rules belong in `domain` or `app`. Do not expose domain entities directly in HTTP responses; use DTOs and mappers. Prefer use case package names such as `signup`, `login`, or `oauthstart`.

## Comments, API Docs & Logging

Write comments only for non-obvious business rules, trade-offs, or operational constraints. Avoid comments that repeat code.

When adding or changing HTTP APIs, update Huma OpenAPI registration in `internal/apps/api/openapi.go`: operation ID, method, path, tags, summary, status, input/output docs, auth middleware, and `documentedErrorStatuses()`. Match the handler pattern in `internal/apps/api/auth.go`: decode DTO, boundary validation, call `identityapp.Service` or the relevant app service, map errors through existing response helpers, and return envelopes.

Use structured `log/slog` in use cases, following `internal/modules/identity/app/login/handler.go`: create `deps.Logger.With("usecase", "<name>")`, log meaningful state changes at `Info`, expected denials or invalid input at `Warn`, and infrastructure failures at `Error` with `"error", err`. Include IDs such as `user_id`, `workspace_id`, or `session_id`; never log passwords, tokens, secrets, or raw personal data.

## Testing Guidelines

Use Go’s built-in testing package. Name files `*_test.go` and test functions `Test...`. Add focused tests for domain invariants, use case behavior, API validation, persistence, and important error paths. Run `make test`; use targeted commands such as `go test ./internal/modules/identity/...` while iterating.

## Commit & Pull Request Guidelines

This branch has no Git commit history yet, so use concise, imperative commit messages such as `Add workspace invitation API`. For PRs, include purpose, API or migration changes, linked issues, commands run, assumptions, and risks.

## Security & Configuration Tips

Copy `.env.example` to `.env` for local development. Never commit secrets or log sensitive values such as tokens, passwords, or credentials. Keep transaction boundaries in application/use case code, and avoid holding transactions open across external calls.
