# Developer Makefile for nebari-catalog-pack. Run targets from the repo root:
#   make -f dev/Makefile <target>

IMG ?= ghcr.io/nebari-dev/nebari-catalog-pack:dev
ROOT := $(shell git rev-parse --show-toplevel 2>/dev/null || pwd)

.PHONY: help web web-install web-typecheck build test vet run image helm-lint helm-template tidy clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

web-install: ## Install frontend dependencies
	cd $(ROOT)/web && npm ci

web: ## Build the React SPA into web/dist (embedded by the Go binary)
	cd $(ROOT)/web && npm run build

web-typecheck: ## Type-check the frontend
	cd $(ROOT)/web && npm run typecheck

build: web ## Build the catalog binary (with the embedded SPA) into ./bin
	cd $(ROOT) && go build -o bin/catalog ./cmd/catalog

test: ## Run Go unit tests
	cd $(ROOT) && go test ./...

vet: ## Run go vet
	cd $(ROOT) && go vet ./...

tidy: ## Tidy go.mod/go.sum
	cd $(ROOT) && go mod tidy

run: build ## Run the server locally (read-only against quay by default)
	cd $(ROOT) && CATALOG_DRY_RUN=true ./bin/catalog

image: ## Build the container image (builds the SPA in a Docker stage)
	cd $(ROOT) && docker build -t $(IMG) .

helm-lint: ## Lint the Helm chart
	cd $(ROOT) && helm lint chart

helm-template: ## Render the chart with NebariApp enabled
	cd $(ROOT) && helm template catalog chart \
		--set nebariapp.enabled=true \
		--set nebariapp.hostname=catalog.nebari.example.com

clean: ## Remove build artifacts
	cd $(ROOT) && rm -rf bin out web/dist/assets web/dist/index.html

# ── Local test cluster (see docs/dev-cluster.md) ───────────────────────────
cluster-quick: ## Tier 1: k3d cluster + helm install the catalog (read-only smoke)
	$(ROOT)/dev/cluster.sh quick

cluster-quick-down: ## Tier 1: delete the quick k3d cluster
	$(ROOT)/dev/cluster.sh quick-down

cluster-up: ## Tier 2: deploy a local Nebari platform (kind, via NIC)
	$(ROOT)/dev/cluster.sh up

catalog-up: ## Tier 2: run/deploy the catalog onto the platform (host|in-cluster)
	$(ROOT)/dev/cluster.sh catalog $(MODE)

cluster-status: ## Show ArgoCD apps + catalog pods
	$(ROOT)/dev/cluster.sh status

cluster-down: ## Tier 2: destroy the local Nebari platform
	$(ROOT)/dev/cluster.sh down
