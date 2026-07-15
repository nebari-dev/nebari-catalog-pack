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

# ── Local test cluster: kind + Helm, standalone (see docs/dev-cluster.md) ──
# Full-platform (operator/ArgoCD) testing runs in CI via integration.yml
# (action-nebari-sandbox); locally we deploy the chart standalone, like the
# other packs' dev/Makefiles.
KIND_CLUSTER ?= catalog-dev
NAMESPACE    ?= nebari-system

cluster-create: ## Create a local kind cluster
	kind create cluster --name $(KIND_CLUSTER)

cluster-load: image ## Build the image and load it into the kind cluster
	kind load docker-image $(IMG) --name $(KIND_CLUSTER)

cluster-deploy: cluster-load ## Deploy the catalog into kind (standalone, read-only)
	cd $(ROOT) && helm upgrade --install nebari-catalog-pack ./chart \
		--namespace $(NAMESPACE) --create-namespace \
		--set image.repository=$(word 1,$(subst :, ,$(IMG))) \
		--set image.tag=$(word 2,$(subst :, ,$(IMG))) \
		--set image.pullPolicy=Never \
		--set nebariapp.enabled=false \
		--set rbac.create=false \
		--set catalog.gitops.repoURL="" \
		--set catalog.argocd.enabled=false
	kubectl -n $(NAMESPACE) rollout status deploy/nebari-catalog-pack --timeout=120s
	@echo "Browse with: make -f dev/Makefile cluster-forward  (then open http://localhost:8080/)"

cluster-forward: ## Port-forward the catalog to http://localhost:8080
	kubectl -n $(NAMESPACE) port-forward svc/nebari-catalog-pack 8080:80

cluster-delete: ## Delete the local kind cluster
	kind delete cluster --name $(KIND_CLUSTER)
