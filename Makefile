# Single source of truth for build + the CI gate set. CI runs `make check`, so
# a green `make check` locally is the same checks CI runs. Lint/vuln tools are
# fetched on demand via `go run <tool>@<version>` — no separate install step.

BINARY := lookit
GOLANGCI_LINT_VERSION := v2.12.2
GORELEASER_VERSION := v2.16.0
GORELEASER := go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

.PHONY: build test race vet fmt fmt-check lint vuln check hooks tidy clean \
	release-check release-snapshot release

build: ## build the binary
	go build -o $(BINARY) .

test: ## run tests
	go test ./...

race: ## run tests with the race detector (the variant CI runs)
	go test ./... -race

vet:
	go vet ./...

fmt: ## reformat all Go files in place
	gofmt -w .

fmt-check: ## fail if any file needs gofmt (mirrors CI)
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "gofmt would reformat the following files:"; \
		echo "$$out"; \
		exit 1; \
	fi

lint: ## run golangci-lint (config in .golangci.yml)
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

vuln: ## scan dependencies for known vulnerabilities
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

check: vet fmt-check lint race ## run the full CI gate set

hooks: ## install git hooks (commit-msg: Conventional Commits); run once per clone
	git config core.hooksPath .githooks
	@echo "git hooks installed (core.hooksPath -> .githooks)"

tidy: ## tidy go.mod/go.sum
	go mod tidy

clean: ## remove build artifacts
	rm -f $(BINARY)
	rm -rf dist

release-check: ## validate the GoReleaser config
	$(GORELEASER) check

release-snapshot: ## build a local snapshot release into dist/ (no publish)
	$(GORELEASER) release --snapshot --clean

release: ## build + publish a release (CI runs this on a vX.Y.Z tag; needs GITHUB_TOKEN)
	$(GORELEASER) release --clean
