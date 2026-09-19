GO      ?= go
PKGS    := ./...
GOFILES := $(shell find . -name '*.go' -not -path './.git/*')

.DEFAULT_GOAL := help

.PHONY: help
help: ## List available targets
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  %-12s %s\n", $$1, $$2}'

.PHONY: build
build: ## Compile every package
	$(GO) build $(PKGS)

.PHONY: test
test: ## Run tests
	$(GO) test $(PKGS)

.PHONY: race
race: ## Run tests under the race detector
	$(GO) test -race $(PKGS)

.PHONY: vet
vet: ## Run go vet
	$(GO) vet $(PKGS)

.PHONY: fmt
fmt: ## Rewrite sources with gofmt
	gofmt -w $(GOFILES)

.PHONY: fmt-check
fmt-check: ## Fail if any source is not gofmt-clean
	@out=$$(gofmt -l $(GOFILES)); \
	if [ -n "$$out" ]; then \
		echo "not gofmt-clean:"; echo "$$out"; exit 1; \
	fi

.PHONY: tidy-check
tidy-check: ## Fail if go.mod or go.sum would change
	@cp go.mod go.mod.bak; [ -f go.sum ] && cp go.sum go.sum.bak || true; \
	$(GO) mod tidy; \
	status=0; \
	cmp -s go.mod go.mod.bak || { echo "go.mod is not tidy"; status=1; }; \
	if [ -f go.sum.bak ]; then cmp -s go.sum go.sum.bak || { echo "go.sum is not tidy"; status=1; }; fi; \
	mv go.mod.bak go.mod; [ -f go.sum.bak ] && mv go.sum.bak go.sum || true; \
	exit $$status

.PHONY: ci
ci: fmt-check tidy-check vet build race ## Everything CI runs
