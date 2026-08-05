SHELL := /bin/bash
.DEFAULT_GOAL := help

BACKEND := backend
FRONTEND := frontend
# docker compose auto-loads a root .env for ${VAR:-default} interpolation
# (BACKEND_PORT, FRONTEND_PORT, CORS_ORIGINS, ...) with no --env-file flag
# needed; the local host-port override applies when present (this dev box
# remaps backend→8090 to dodge a port conflict) under a stable project name —
# a clean checkout without either falls back to the compose file's own
# defaults (backend→8080, frontend→8081).
COMPOSE := docker compose -p xchats -f deploy/docker-compose.yaml $(if $(wildcard deploy/docker-compose.override.yaml),-f deploy/docker-compose.override.yaml,)
GORUN := go run ./cmd/xchats -env ../.env -config ../config.yaml

# Ports kill-ports frees (override: make kill-ports PORTS="8080 5173")
PORTS ?= 8080 8090 5173 8081

.PHONY: help up up-fg down logs ps kill-ports migrate seed seed-kb-demo dev-backend dev-frontend \
        test test-backend test-frontend test-e2e build

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Rebuild + run the whole stack detached (SQLite — backend + frontend, no separate database service)
	$(COMPOSE) up -d --build
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
	cd $(BACKEND) && $(GORUN) migrate

seed: ## Seed the default organization + admin login
	cd $(BACKEND) && $(GORUN) seed

seed-kb-demo: ## Seed demo KB content (topics/products/tariffs/zones/contacts/policies) — opt-in, for test cases only; no-ops if the org already has KB content
	cd $(BACKEND) && $(GORUN) seed-kb-demo

dev-backend: ## Run the backend (go) on :8080
	cd $(BACKEND) && $(GORUN) serve

dev-frontend: ## Run the frontend (vite) on :5173
	cd $(FRONTEND) && npm run dev

test: test-backend test-frontend ## Unit + component (offline, deterministic)

test-backend: ## Go unit tests (SQLite — every DB test gets its own fresh database, see internal/dbtest)
	cd $(BACKEND) && go test ./...

test-frontend: ## Frontend typecheck + build
	cd $(FRONTEND) && npm run build

test-e2e: ## Full demo loop + KB/response service DB-backed suites (subset of test-backend, run in isolation)
	cd $(BACKEND) && go test -count=1 \
		./internal/httpapi/ ./internal/kbstore/ \
		./internal/responsestore/ ./internal/store/

build: ## Build backend binary + frontend bundle
	cd $(BACKEND) && go build -o bin/xchats ./cmd/xchats
	cd $(FRONTEND) && npm ci && npm run build
