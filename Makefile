.PHONY: help tidy fmt lint test test-race build up up-go down logs migrate-up migrate-down smoke data research research-force

GO ?= go
COMPOSE ?= docker compose -f deployments/docker-compose.yaml
PG_DSN ?= postgres://alfa:alfa@localhost:15432/alfa?sslmode=disable

help:
	@echo "tidy          - go mod tidy"
	@echo "fmt           - gofmt -w по всему репозиторию"
	@echo "lint          - golangci-lint run"
	@echo "test          - юнит-тесты"
	@echo "test-race     - юнит-тесты с -race"
	@echo "build         - сборка трёх Go-сервисов в ./bin"
	@echo "up            - поднять весь стек (4 сервиса + gateway + postgres)"
	@echo "up-go         - поднять только Go-контур и БД (быстро)"
	@echo "down          - остановить стек и снести volume"
	@echo "logs          - логи стека"
	@echo "migrate-up    - применить миграции к локальной БД"
	@echo "migrate-down  - откатить одну миграцию"
	@echo "smoke         - быстрая проверка живого контура (healthz + вброс новости)"
	@echo "data          - скачать данные кейса в data/ (CSV и аналитику ЦМИ)"
	@echo "research      - загрузить аналитику ЦМИ в базу (OCR, повторно пропускает загруженное)"
	@echo "research-force - перезалить аналитику ЦМИ целиком"

tidy:
	$(GO) mod tidy

fmt:
	gofmt -w ./internal ./services

lint:
	golangci-lint run ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

build:
	$(GO) build -trimpath -o bin/go-ingest ./services/go-ingest/cmd
	$(GO) build -trimpath -o bin/llm-gateway ./services/llm-gateway/cmd
	$(GO) build -trimpath -o bin/go-api ./services/go-api/cmd

up:
	$(COMPOSE) up --build

up-go:
	$(COMPOSE) up --build postgres migrate go-ingest llm-gateway go-api

down:
	$(COMPOSE) down -v

logs:
	$(COMPOSE) logs -f --tail=100

migrate-up:
	docker run --rm --network host -v "$(CURDIR)/internal/infra/db/postgres/migrations:/migrations:ro" \
		migrate/migrate:v4.17.1 -path=/migrations -database="$(PG_DSN)" up

migrate-down:
	docker run --rm --network host -v "$(CURDIR)/internal/infra/db/postgres/migrations:/migrations:ro" \
		migrate/migrate:v4.17.1 -path=/migrations -database="$(PG_DSN)" down 1

smoke:
	./scripts/smoke.sh

data:
	bash scripts/fetch_case_data.sh

research:
	$(COMPOSE) run --rm --build research-loader

research-force:
	$(COMPOSE) run --rm --build -e RESEARCH_FORCE=true research-loader
