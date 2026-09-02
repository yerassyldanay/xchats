SHELL := /bin/bash
.DEFAULT_GOAL := help

BACKEND := backend
FRONTEND := frontend
# One compose file, no override file, no .env: application settings live in
# deploy/config.docker.yaml (mounted into the container), so there is nothing
# to source or export before `make up`. The only interpolated values left are
# the two host port mappings, which default to 8080/8081 when unset — export
# BACKEND_PORT/FRONTEND_PORT for a run if either collides. -p pins a stable
# project name so the volumes survive across invocations.
COMPOSE := docker compose -p xchats -f deploy/docker-compose.yaml
GORUN := go run ./cmd/xchats -config ../config.yaml

# --- Wails desktop packaging (see docs/desktop.md) --------------------------
# The desktop app is the SAME cmd/xchats binary with the `desktop` build tag,
# so wails.json lives next to its main package rather than at the repo root.
DESKTOP := $(BACKEND)/cmd/xchats
# Pinned, like every other tool this repo shells out to: `wails build` runs
# `go build` with tags and ldflags it chooses, so a surprise CLI upgrade is a
# surprise change to the shipped binary.
WAILS_VERSION := v2.15.0
# On Linux the WebKitGTK ABI is a build tag, and the default (untagged) one is
# a legacy path where the WebView delivers every request as a bodyless GET —
# which would break every POST the app makes. webkit2_41 is libwebkit2gtk-4.1,
# the current ABI. macOS/Windows need no extra tag.
WAILS_TAGS := $(if $(filter Linux,$(shell uname -s)),-tags webkit2_41,)
# -skipbindings: nothing is bound to the frontend, and the generator would
# otherwise compile and RUN the server binary mid-build (see
# backend/cmd/xchats/bindings.go).
WAILS_FLAGS := -skipbindings $(WAILS_TAGS)

# Ports kill-ports frees (override: make kill-ports PORTS="8080 5173").
# 8090 is still listed deliberately: this box used to publish the backend
# there via a local compose override, so a stale container may still hold it.
PORTS ?= 8080 8090 5173 8081

.PHONY: help up up-fg down logs ps kill-ports migrate seed seed-demo seed-kb-demo dev-backend dev-frontend \
        test test-backend test-frontend test-e2e build screenshots lint lint-backend lint-frontend notices ruleset-apply \
        desktop-tools desktop-assets desktop-dev desktop-build desktop-clean

help: ## Show this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Rebuild + run the whole stack detached (SQLite — backend + frontend, no separate database service)
	$(COMPOSE) up -d --build
	@echo "✅ up — frontend: http://localhost:$${FRONTEND_PORT:-8081} · backend: http://localhost:$${BACKEND_PORT:-8080}"
	@echo "   (run 'make ps' to confirm the actual bound ports)"

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

seed-demo: ## Seed full demo dataset (KB + products/images + draft + customers + followups + campaigns)
	cd $(BACKEND) && $(GORUN) seed-demo

seed-kb-demo: ## Alias for seed-demo (legacy)
	cd $(BACKEND) && $(GORUN) seed-demo

dev-backend: ## Run the backend (go) on :8080
	cd $(BACKEND) && $(GORUN) serve

dev-frontend: ## Run the frontend (vite) on :5173
	cd $(FRONTEND) && npm run dev

test: test-backend test-frontend ## Unit + component (offline, deterministic)

test-backend: ## Go unit tests (SQLite — every DB test gets its own fresh database, see internal/dbtest)
	cd $(BACKEND) && go test -race -count=1 ./...

test-frontend: ## Frontend typecheck + unit tests + build
	cd $(FRONTEND) && npm run typecheck && npm run test:unit && npm run build

test-e2e: ## Full demo loop + KB/response service DB-backed suites (subset of test-backend, run in isolation)
	cd $(BACKEND) && go test -count=1 \
		./internal/httpapi/ ./internal/kbstore/ \
		./internal/responsestore/ ./internal/store/

build: ## Build backend binary + frontend bundle
	cd $(BACKEND) && go build -o bin/xchats ./cmd/xchats
	cd $(FRONTEND) && npm ci && npm run build

screenshots: ## Regenerate docs/images/*.png from a running, seeded instance (needs `make up`/`dev-backend`+`dev-frontend` on :8081 AND `make seed-demo` first — see frontend/scripts/capture-screenshots.mjs)
	cd $(FRONTEND) && node scripts/capture-screenshots.mjs

desktop-tools: ## Install the pinned Wails CLI (one-off; see docs/desktop.md for the OS packages it needs)
	go install github.com/wailsapp/wails/v2/cmd/wails@$(WAILS_VERSION)
	@echo "✅ wails $(WAILS_VERSION) installed — run 'wails doctor' to check the OS-level prerequisites"

desktop-assets: ## Build the SPA and mirror it into the desktop binary's go:embed directory
	cd $(FRONTEND) && npm run build:desktop

desktop-dev: ## Run the desktop app against the Vite dev server (hot reload; Ctrl-C to stop)
	cd $(DESKTOP) && wails dev $(WAILS_FLAGS)

desktop-build: ## Build the packaged desktop app for THIS platform into backend/cmd/xchats/build/bin/
	cd $(DESKTOP) && wails build -clean $(WAILS_FLAGS)
	@echo "✅ desktop build → $(DESKTOP)/build/bin/"

desktop-clean: ## Remove desktop build output and the mirrored SPA bundle
	rm -rf $(DESKTOP)/build/bin $(DESKTOP)/wailsjs
	find $(BACKEND)/internal/desktop/dist -mindepth 1 ! -name .gitkeep -exec rm -rf {} +

lint: lint-backend lint-frontend ## Run every linter (same checks as CI's lint jobs)

lint-backend: ## golangci-lint over the backend module (see .golangci.yml)
	cd $(BACKEND) && golangci-lint run ./...

lint-frontend: ## eslint over the frontend (see frontend/eslint.config.js)
	cd $(FRONTEND) && npx eslint .

notices: ## Regenerate THIRD_PARTY_LICENSES.txt from the shipped dependency graph (pinned tools — see scripts/notices.sh)
	./scripts/notices.sh
	@echo "If the dependency set changed, also refresh the inventory tables in"
	@echo "THIRD_PARTY_NOTICES.md (see its 'Maintaining this file' section)."

ruleset-apply: ## Apply .github/rulesets/*.json to the repo via the GitHub API (does nothing until they exist — see the release plan's ruleset stage)
	@for f in .github/rulesets/*.json; do \
		[ -f "$$f" ] || continue; \
		echo "applying $$f"; \
		gh api -X POST repos/yerassyldanay/xchats/rulesets --input "$$f" >/dev/null || \
		gh api -X PUT  repos/yerassyldanay/xchats/rulesets/$$(gh api repos/yerassyldanay/xchats/rulesets --jq ".[] | select(.name==\"$$(basename $$f .json)\") | .id") --input "$$f" >/dev/null; \
	done
