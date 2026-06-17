SHELL := /bin/sh

LOCAL_COMPOSE := ./deployments/local/docker-compose.yml
LOG_DIR := ./deployments/local/logs
LOGROTATE_CONF := ./deployments/local/logrotate.sendflow.conf
ENV_FILE := ./.env
START_COUNT := $(or $(word 2,$(MAKECMDGOALS)),1)
VERSION ?= dev
GIT_SHA ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/ninggiangboy/send-flow/backend/internal/platform/buildinfo.Version=$(VERSION) -X github.com/ninggiangboy/send-flow/backend/internal/platform/buildinfo.GitSHA=$(GIT_SHA) -X github.com/ninggiangboy/send-flow/backend/internal/platform/buildinfo.BuildTime=$(BUILD_TIME)

ifeq ($(firstword $(MAKECMDGOALS)),start)
START_EXTRA_GOALS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
ifneq ($(START_EXTRA_GOALS),)
$(eval $(START_EXTRA_GOALS):; @:)
endif
endif

.PHONY: dev-up dev-down dev-stop dev-logs connector-up consul-clean start api api-instance worker worker-instance build format vet test check test-race test-integration migrate-up migrate-down clickhouse-migrate-up clickhouse-migrate-down logs-clean logs-rotate

dev-up:
	docker compose -f $(LOCAL_COMPOSE) up -d
	$(MAKE) consul-clean
	$(MAKE) connector-up

dev-down:
	docker compose -f $(LOCAL_COMPOSE) down -v

dev-stop:
	docker compose -f $(LOCAL_COMPOSE) stop

dev-logs:
	docker compose -f $(LOCAL_COMPOSE) logs -f --tail=200

connector-up:
	./scripts/register-outbox-connector.sh

consul-clean:
	sh -c 'addr="$${CONSUL_HTTP_ADDR:-http://localhost:8500}"; for base in sendflow-api-local sendflow-worker-local; do curl -fsS -X PUT "$$addr/v1/agent/service/deregister/$$base" >/dev/null 2>&1 || true; i=1; while [ $$i -le 20 ]; do curl -fsS -X PUT "$$addr/v1/agent/service/deregister/$$base-$$i" >/dev/null 2>&1 || true; i=$$((i + 1)); done; done'

start:
	$(MAKE) consul-clean
	sh -c 'count="$(START_COUNT)"; case "$$count" in ""|*[!0-9]*|0) echo "usage: make start [replicas], e.g. make start 2"; exit 2;; esac; pids=""; used_ports=""; i=1; trap "trap - INT TERM EXIT; kill $$pids 2>/dev/null; wait" INT TERM EXIT; while [ $$i -le $$count ]; do api_port=$$(./scripts/pick-local-port.sh $$used_ports); used_ports="$$used_ports $$api_port"; worker_port=$$(./scripts/pick-local-port.sh $$used_ports); used_ports="$$used_ports $$worker_port"; echo "starting api[$$i] on :$$api_port and worker[$$i] on :$$worker_port"; $(MAKE) --no-print-directory api-instance INSTANCE=$$i HTTP_ADDR=:$$api_port CONSUL_SERVICE_ID=sendflow-api-local-$$i CONSUL_SERVICE_PORT=$$api_port & pids="$$pids $$!"; $(MAKE) --no-print-directory worker-instance INSTANCE=$$i WORKER_HTTP_ADDR=:$$worker_port CONSUL_WORKER_SERVICE_ID=sendflow-worker-local-$$i CONSUL_WORKER_SERVICE_PORT=$$worker_port & pids="$$pids $$!"; i=$$((i + 1)); done; wait'

api:
	sh -c 'mkdir -p $(LOG_DIR); if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; SERVICE_NAME=api INSTANCE_ID="$${INSTANCE_ID:-$${CONSUL_SERVICE_ID:-sendflow-api-local}}" INSTANCE_ADDR="$${INSTANCE_ADDR:-$${HTTP_ADDR:-:8081}}" go run ./cmd/api 2>&1 | tee -a $(LOG_DIR)/api.log'

api-instance:
	sh -c 'mkdir -p $(LOG_DIR); if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; SERVICE_NAME=api INSTANCE_ID="$${INSTANCE_ID:-sendflow-api-local-$(INSTANCE)}" INSTANCE_ADDR="$${INSTANCE_ADDR:-$(HTTP_ADDR)}" HTTP_ADDR="$(HTTP_ADDR)" CONSUL_SERVICE_ID="$(CONSUL_SERVICE_ID)" CONSUL_SERVICE_PORT="$(CONSUL_SERVICE_PORT)" go run ./cmd/api 2>&1 | tee -a $(LOG_DIR)/api-$(INSTANCE).log'

worker:
	sh -c 'mkdir -p $(LOG_DIR); if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; SERVICE_NAME=worker INSTANCE_ID="$${INSTANCE_ID:-$${CONSUL_WORKER_SERVICE_ID:-sendflow-worker-local}}" INSTANCE_ADDR="$${INSTANCE_ADDR:-$${WORKER_HTTP_ADDR:-:8082}}" CONSUL_WORKER_SERVICE_ID="$${CONSUL_WORKER_SERVICE_ID:-sendflow-worker-local}" go run ./cmd/worker 2>&1 | tee -a $(LOG_DIR)/worker.log'

worker-instance:
	sh -c 'mkdir -p $(LOG_DIR); if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; SERVICE_NAME=worker INSTANCE_ID="$${INSTANCE_ID:-sendflow-worker-local-$(INSTANCE)}" INSTANCE_ADDR="$${INSTANCE_ADDR:-$(WORKER_HTTP_ADDR)}" WORKER_HTTP_ADDR="$(WORKER_HTTP_ADDR)" CONSUL_WORKER_SERVICE_ID="$(CONSUL_WORKER_SERVICE_ID)" CONSUL_WORKER_SERVICE_PORT="$(CONSUL_WORKER_SERVICE_PORT)" go run ./cmd/worker 2>&1 | tee -a $(LOG_DIR)/worker-$(INSTANCE).log'

build:
	mkdir -p ./bin
	go build -ldflags "$(LDFLAGS)" -o ./bin/api ./cmd/api
	go build -ldflags "$(LDFLAGS)" -o ./bin/worker ./cmd/worker
	go build -ldflags "$(LDFLAGS)" -o ./bin/clickhouse-migrate ./cmd/clickhouse-migrate

format:
	gofmt -w ./cmd ./internal

vet:
	go vet ./...

test:
	go test ./...

check: format vet test

test-race:
	go test -race ./internal/...

test-integration:
	go test -tags=integration ./...

migrate-up:
	sh -c 'if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; go run github.com/pressly/goose/v3/cmd/goose@v3.22.1 -dir ./migrations postgres "$$DATABASE_URL" up'

migrate-down:
	sh -c 'if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; go run github.com/pressly/goose/v3/cmd/goose@v3.22.1 -dir ./migrations postgres "$$DATABASE_URL" down'

clickhouse-migrate-up:
	sh -c 'if [ -f $(ENV_FILE) ]; then set -a; . $(ENV_FILE); set +a; fi; go run ./cmd/clickhouse-migrate 2>&1'

clickhouse-migrate-down:
	@echo "ClickHouse rollback not yet implemented; drop and re-run migrate-up to rebuild schema"


logs-clean:
	sh -c 'mkdir -p $(LOG_DIR); : > $(LOG_DIR)/api.log; : > $(LOG_DIR)/worker.log; rm -f $(LOG_DIR)/api-*.log $(LOG_DIR)/worker-*.log'
