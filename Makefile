# nitpub task runner
#
# Make wraps common dev and deploy commands. VPS-specific logic stays in
# deploy/*.sh (sudo, systemd, nitpub user). Override SSH host/path:
#   make deploy SSH_HOST=nitpub REPO_PATH=/var/lib/nitpub/src

BINARY := nitpub
BUILD_DIR := .build
PKG := ./cmd/nitpub
WEB_DIR := web
DEPLOY_SCRIPT := deploy/update.sh
SSH_HOST ?= nitpub
REPO_PATH ?= /var/lib/nitpub/src
# Nearest release tag (no +dirty suffix), falling back to the short commit
# hash pre-tag. Override: VERSION=v0.3.1 make build
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS := -X github.com/newtosh/nitpub/internal/version.Version=$(VERSION)
DEV_PORT ?= 8080
DEV_DOMAIN ?= localhost
DEV_DATA_DIR ?= ./data
DEV_PIDFILE := $(BUILD_DIR)/nitpub.pid
DEV_LOG := $(BUILD_DIR)/nitpub.log
DEV_ENV := NITPUB_HTTP=1 NITPUB_DOMAIN=$(DEV_DOMAIN) NITPUB_DATA_DIR=$(DEV_DATA_DIR)

.PHONY: help web web-install build test run dev dev-backend dev-web dev-stop dev-start dev-reload install deploy deploy-local clean

.DEFAULT_GOAL := help

help: ## List targets
	@printf "nitpub — common tasks\n\n"
	@grep -E '^[a-zA-Z0-9_.-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  %-16s %s\n", $$1, $$2}'

web-install: ## Install web dependencies (npm ci)
	cd $(WEB_DIR) && npm ci

web: ## Build embedded PWA (vite → cmd/nitpub/dist)
	cd $(WEB_DIR) && npm run build

build: web ## Build PWA + Go binary to .build/nitpub
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(PKG)

test: ## Run Go tests
	go test ./...

run: build ## Run server locally over HTTP (NITPUB_HTTP=1)
	NITPUB_HTTP=1 ./$(BUILD_DIR)/$(BINARY)

dev-backend: ## API server for local UI dev (:8080)
	NITPUB_HTTP=1 NITPUB_DOMAIN=localhost NITPUB_DATA_DIR=./data go run $(PKG)

dev-web: ## Vite dev server (proxies API to :8080)
	cd $(WEB_DIR) && npm run dev

dev-stop: ## Stop local backend on :8080
	@if [ -f $(DEV_PIDFILE) ]; then \
		kill $$(cat $(DEV_PIDFILE)) 2>/dev/null || true; \
		rm -f $(DEV_PIDFILE); \
	fi
	@lsof -ti :$(DEV_PORT) 2>/dev/null | xargs -r kill 2>/dev/null || true

dev-start: ## Start built backend in background (:8080)
	@mkdir -p $(BUILD_DIR)
	@test -x $(BUILD_DIR)/$(BINARY) || { echo "Run make build first"; exit 1; }
	@$(MAKE) dev-stop
	@nohup env $(DEV_ENV) $(BUILD_DIR)/$(BINARY) >$(DEV_LOG) 2>&1 & echo $$! > $(DEV_PIDFILE)
	@sleep 0.5
	@curl -sf "http://127.0.0.1:$(DEV_PORT)/healthz" >/dev/null \
		&& printf "nitpub → http://127.0.0.1:%s (pid %s)\n" "$(DEV_PORT)" "$$(cat $(DEV_PIDFILE))" \
		|| { echo "Backend failed to start — see $(DEV_LOG)"; exit 1; }

dev-reload: build dev-start ## Rebuild embedded UI + binary, restart :8080

dev: ## Local UI dev — run dev-backend and dev-web in two terminals, then Playwright MCP
	@echo "Start in two terminals:"
	@echo "  make dev-backend"
	@echo "  make dev-web"
	@echo "Open http://127.0.0.1:5173 and verify with Playwright MCP (see .cursor/rules/ui-testing.mdc)"
	@echo ""
	@echo "To test the embedded production bundle on :8080 instead:"
	@echo "  make dev-reload"

install: build ## Install binary to $$(go env GOPATH)/bin
	go install $(PKG)

deploy: ## Deploy on VPS: pull, build, install, restart (via SSH)
	ssh $(SSH_HOST) 'bash $(REPO_PATH)/$(DEPLOY_SCRIPT)'

deploy-local: ## Run deploy/update.sh on this machine (on the VPS as root)
	bash $(DEPLOY_SCRIPT)

clean: ## Remove local build artifacts
	rm -rf $(BUILD_DIR)
