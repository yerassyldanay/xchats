SHELL := /bin/bash
.DEFAULT_GOAL := help

BACKEND := backend
FRONTEND := frontend
# Use the root .env and the local host-port override when present (this dev box
# remaps backend→8090, frontend→8081, db→5433 to dodge port conflicts) under a
# stable project name; a clean checkout without them falls back to compose
# defaults (backend→8080, frontend→8081, db→5432).
COMPOSE := docker compose -p xchats $(if $(wildcard .env),--env-file .env,) -f deploy/docker-compose.yaml $(if $(wildcard deploy/docker-compose.override.yaml),-f deploy/docker-compose.override.yaml,)
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/xchats?sslmode=disable
GORUN := go run ./cmd/xchats -env ../.env -config ../config.yaml

# Ports kill-ports frees (override: make kill-ports PORTS="8080 5173")
PORTS ?= 8080 8090 5173 8081

.PHONY: help up up-fg down logs ps kill-ports migrate seed webhook-set dev-backend dev-frontend \
        test test-backend test-frontend test-e2e build smoke

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Rebuild + run the whole stack detached (Postgres + backend + frontend)
	$(COMPOSE) up -d
	@echo "✅ up — frontend: http://localhost:8081 · backend: http://localhost:8090 · logs: make logs"

up-fg: ## Same as up but foreground (Ctrl-C to stop, attached logs)
	$(COMPOSE) up --build

down: ## Stop + remove the stack
	$(COMPOSE) down

logs: ## Tail stack logs (Ctrl-C to detach)
	$(COMPOSE) logs -f

ps: ## Show stack container status
	$(COMPOSE) ps

kill-ports: ## Free backend/frontend ports (default 8080 8090 5173 8081; override PORTS=). Docker stack: prefer 'make down'.
	@for p in $(PORTS); do \
	  if fuser $$p/tcp >/dev/null 2>&1; then \
	    fuser -k $$p/tcp >/dev/null 2>&1 && echo "  killed listener on :$$p" \
	      || echo "  :$$p in use but not killed (Docker-owned? use 'make down', or sudo)"; \
	  else echo "  :$$p free"; fi; \
	done

migrate: ## Apply DB migrations
	cd $(BACKEND) && DATABASE_URL="$(DATABASE_URL)" $(GORUN) migrate

seed: ## Seed org + admin + the single WhatsApp account
	cd $(BACKEND) && DATABASE_URL="$(DATABASE_URL)" $(GORUN) seed

webhook-set: ## Register our webhook on the live Evolution instance
	cd $(BACKEND) && $(GORUN) webhook-set

dev-backend: ## Run the backend (go) on :8080
	cd $(BACKEND) && DATABASE_URL="$(DATABASE_URL)" $(GORUN) serve

dev-frontend: ## Run the frontend (vite) on :5173
	cd $(FRONTEND) && npm run dev

test: test-backend test-frontend ## Unit + component (offline, deterministic)

test-backend: ## Go unit tests (DB tests skip unless DATABASE_URL is set)
	cd $(BACKEND) && go test ./...

test-frontend: ## Frontend typecheck + build
	cd $(FRONTEND) && npm run build

test-e2e: ## Full demo loop + KB/playground vs a real Postgres (set DATABASE_URL)
	# -p 1: these packages each reset the shared xchats schema, so they must not
	# run concurrently against the same database.
	cd $(BACKEND) && DATABASE_URL="$(DATABASE_URL)" go test -p 1 -count=1 \
		./internal/httpapi/ ./internal/kbstore/ ./internal/playground/

build: ## Build backend binary + frontend bundle
	cd $(BACKEND) && go build -o bin/xchats ./cmd/xchats
	cd $(FRONTEND) && npm ci && npm run build

smoke: webhook-set ## Manual: register webhook for a live round-trip test
	@echo "Webhook registered. Send a WhatsApp message to the connected number."
