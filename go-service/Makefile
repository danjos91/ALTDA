.PHONY: help install run test lint

OUT:="$(PWD)/bin"
TOOLS:="$(OUT)"

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install tools
	GOBIN="$(OUT)" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.2

run: ## Run the service locally (loads .env)
	set -a && . ./.env && set +a && go run ./cmd/app

test: ## Run unit tests
	go test -v -short ./...

lint: ## Run linter
	"$(TOOLS)"/golangci-lint run ./...
