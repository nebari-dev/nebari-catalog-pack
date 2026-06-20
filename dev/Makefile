# Developer Makefile for nebari-catalog-pack. Run targets from the repo root:
#   make -f dev/Makefile <target>

TEMPL_VERSION := v0.3.857
IMG ?= ghcr.io/nebari-dev/nebari-catalog-pack:dev
ROOT := $(shell git rev-parse --show-toplevel 2>/dev/null || pwd)

.PHONY: help generate build test vet lint run image helm-lint helm-template tidy clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

generate: ## Regenerate templ components
	cd $(ROOT) && go run github.com/a-h/templ/cmd/templ@$(TEMPL_VERSION) generate

build: generate ## Build the catalog binary into ./bin
	cd $(ROOT) && go build -o bin/catalog ./cmd/catalog

test: generate ## Run unit tests
	cd $(ROOT) && go test ./...

vet: generate ## Run go vet
	cd $(ROOT) && go vet ./...

tidy: ## Tidy go.mod/go.sum
	cd $(ROOT) && go mod tidy

run: build ## Run the server locally (read-only against quay by default)
	cd $(ROOT) && CATALOG_DRY_RUN=true ./bin/catalog

image: ## Build the container image
	cd $(ROOT) && docker build -t $(IMG) .

helm-lint: ## Lint the Helm chart
	cd $(ROOT) && helm lint chart

helm-template: ## Render the chart with NebariApp enabled
	cd $(ROOT) && helm template catalog chart \
		--set nebariapp.enabled=true \
		--set nebariapp.hostname=catalog.nebari.example.com

clean: ## Remove build artifacts
	cd $(ROOT) && rm -rf bin out
