SHELL := /bin/bash
.DEFAULT_GOAL := help

BACKEND := backend
FRONTEND := frontend
COMPOSE := docker compose -f deploy/docker-compose.yaml
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/xchats?sslmode=disable
GORUN := go run ./cmd/xchats -env ../.env -config ../config.yaml

.PHONY: help up down logs migrate seed webhook-set dev-backend dev-frontend \
        test test-backend test-frontend test-e2e build smoke

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

up: ## Build + run the whole stack (Postgres + backend + frontend)
	$(COMPOSE) up --build

down: ## Stop the stack
	$(COMPOSE) down

logs: ## Tail stack logs
	$(COMPOSE) logs -f

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

test-e2e: ## Full demo loop vs a real Postgres (set DATABASE_URL)
	cd $(BACKEND) && DATABASE_URL="$(DATABASE_URL)" go test ./internal/httpapi/ -count=1 -v

build: ## Build backend binary + frontend bundle
	cd $(BACKEND) && go build -o bin/xchats ./cmd/xchats
	cd $(FRONTEND) && npm ci && npm run build

smoke: webhook-set ## Manual: register webhook for a live round-trip test
	@echo "Webhook registered. Send a WhatsApp message to the connected number."
