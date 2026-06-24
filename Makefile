# A Self-Documenting Makefile: http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
#
# Quality gate for the yup-cut CLI wrapper. `make check` must pass with zero
# findings before any change is considered complete. Tooling is declared in the
# go.mod `tool` stanza and run via `go tool` (no global installs).
.DEFAULT_GOAL := build

.PHONY: help
help: ## Show this help
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}'

# Project variables
NAME      ?= yup-cut
GO        ?= go
PKGS       = .
# Production (non-test) Go files, used by the format and complexity gates.
SRC        = $(shell find . -name '*.go' -not -name '*_test.go' -not -path './vendor/*')

export CGO_ENABLED ?= 0

## Build

.PHONY: build
build: ## Build a snapshot binary for the current platform (dist/)
	$(GO) tool goreleaser build --single-target --snapshot --clean

.PHONY: build-all
build-all: ## Build snapshot binaries for all platforms (dist/)
	$(GO) tool goreleaser build --snapshot --clean

.PHONY: release
release: ## Publish a release with goreleaser (requires a git tag)
	$(GO) tool goreleaser release --clean

.PHONY: release-snapshot
release-snapshot: ## Build a snapshot release without a git tag
	$(GO) tool goreleaser release --snapshot --clean

.PHONY: clean
clean: ## Remove build and coverage artifacts
	rm -rf dist/ coverage.out *.test

## Integration

IMAGE ?= $(NAME)-integration

.PHONY: integration
integration: ## Build the Linux binary in Docker and check it against the GNU reference
	docker build -f integration/Dockerfile -t $(IMAGE) .
	docker run --rm $(IMAGE)

## Code Quality

.PHONY: check
check: fmt-check vet staticcheck cognit cover vuln goreleaser-check ## Full gate: gofumpt, vet, staticcheck, complexity<=7, 100% coverage, vuln scan, goreleaser config

.PHONY: fmt
fmt: ## Rewrite all files with the strict formatter
	$(GO) tool gofumpt -w .

.PHONY: fmt-check
fmt-check: ## Fail if any file is not gofumpt-clean
	@out="$$($(GO) tool gofumpt -l .)"; \
	if [ -n "$$out" ]; then echo "gofumpt findings:"; echo "$$out"; exit 1; fi

.PHONY: vet
vet: ## Run go vet
	$(GO) vet $(PKGS)

.PHONY: staticcheck
staticcheck: ## Run staticcheck static analysis (zero findings)
	$(GO) tool staticcheck $(PKGS)

.PHONY: cognit
cognit: ## Assert cognitive complexity <= 7 for every production function
	$(GO) tool gocognit -over 7 $(SRC)

.PHONY: vuln
vuln: ## Scan dependencies for known vulnerabilities
	$(GO) tool govulncheck ./...

.PHONY: goreleaser-check
goreleaser-check: ## Validate the goreleaser release configuration
	$(GO) tool goreleaser check

.PHONY: test
test: ## Run tests
	$(GO) test $(PKGS)

.PHONY: cover
cover: ## Run tests and assert 100.0% statement coverage
	@$(GO) test -coverprofile=coverage.out $(PKGS) >/dev/null
	@total="$$($(GO) tool cover -func=coverage.out | awk '/^total:/ {print $$3}')"; \
	echo "coverage: $$total"; \
	if [ "$$total" != "100.0%" ]; then \
		echo "FAIL: coverage $$total is below 100.0%"; \
		$(GO) tool cover -func=coverage.out | awk '$$3 != "100.0%"'; \
		exit 1; \
	fi

## Dependencies

.PHONY: tidy
tidy: ## Tidy and verify module dependencies
	$(GO) mod tidy
	$(GO) mod verify
