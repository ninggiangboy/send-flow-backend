SHELL := /bin/sh

LOCAL_COMPOSE := ./deployments/local/docker-compose.yml
LOG_DIR := ./deployments/local/logs
LOGROTATE_CONF := ./deployments/local/logrotate.sendflow.conf
ENV_FILE := ./.env

.PHONY: dev-up dev-down dev-logs connector-up start api worker build format test migrate-up migrate-down clickhouse-migrate-up clickhouse-migrate-down clickhouse-backfill logs-clean logs-rotate

dev-up:
	docker compose -f $(LOCAL_COMPOSE) up -d
	$(MAKE) connector-up

dev-down:
	docker compose -f $(LOCAL_COMPOSE) down -v

dev-logs:
	docker compose -f $(LOCAL_COMPOSE) logs -f --tail=200

connector-up:
	./scripts/register-outbox-connector.sh

start:
	sh -c '$(MAKE) api & api_pid=$$!; $(MAKE) worker & worker_pid=$$!; trap "trap - INT TERM EXIT; kill $$api_pid $$worker_pid 2>/dev/null; wait" INT TERM EXIT; wait'

api:
	sh -c 'mkdir -p $(LOG_DIR); if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; go run ./cmd/api 2>&1 | tee -a $(LOG_DIR)/api.log'

worker:
	sh -c 'mkdir -p $(LOG_DIR); if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; go run ./cmd/worker 2>&1 | tee -a $(LOG_DIR)/worker.log'

build:
	mkdir -p ./bin
	go build -o ./bin/api ./cmd/api
	go build -o ./bin/worker ./cmd/worker
	go build -o ./bin/clickhouse-migrate ./cmd/clickhouse-migrate
	go build -o ./bin/backfill-clickhouse ./cmd/backfill-clickhouse

format:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

test:
	go test ./...

migrate-up:
	sh -c 'if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; go run github.com/pressly/goose/v3/cmd/goose@v3.22.1 -dir ./migrations postgres "$$DATABASE_URL" up'

migrate-down:
	sh -c 'if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; go run github.com/pressly/goose/v3/cmd/goose@v3.22.1 -dir ./migrations postgres "$$DATABASE_URL" down'

clickhouse-migrate-up:
	sh -c 'if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; go run ./cmd/clickhouse-migrate 2>&1'

clickhouse-migrate-down:
	@echo "ClickHouse rollback not yet implemented; drop and re-run migrate-up to rebuild schema"

clickhouse-backfill:
	sh -c 'if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; go run ./cmd/backfill-clickhouse 2>&1'

logs-clean:
	sh -c 'mkdir -p $(LOG_DIR); : > $(LOG_DIR)/api.log; : > $(LOG_DIR)/worker.log'
