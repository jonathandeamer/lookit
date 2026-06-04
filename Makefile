# Single source of truth for build + the CI gate set. CI runs `make check`, so
# a green `make check` locally is the same checks CI runs. Lint/vuln tools are
# fetched on demand via `go run <tool>@<version>` — no separate install step.

BINARY := lookit
MODULE := github.com/jonathandeamer/lookit
GOLANGCI_LINT_VERSION := v2.12.2
GORELEASER_VERSION := v2.16.0
GO_LICENSES_VERSION := v1.6.0
GORELEASER := go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

.PHONY: build test race vet fmt fmt-check lint vuln check hooks tidy clean \
	notices release-check release-snapshot release

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

notices: ## regenerate THIRD_PARTY_NOTICES.md from dependency licenses (rerun after dep changes)
	@tmp=$$(mktemp -d); \
	go run github.com/google/go-licenses@$(GO_LICENSES_VERSION) save ./... --save_path=$$tmp --force --ignore $(MODULE); \
	{ \
		printf '# Third-party notices\n\n'; \
		printf 'The lookit binary statically links the Go modules below. Each is\n'; \
		printf 'distributed under the license reproduced here. Regenerate with `make notices`.\n\n'; \
		( cd $$tmp && find . \( -name 'LICENSE*' -o -name 'COPYING*' \) | LC_ALL=C sort | while IFS= read -r f; do \
			mod=$${f#./}; mod=$${mod%/LICENSE*}; mod=$${mod%/COPYING*}; \
			printf '## %s\n\n```\n' "$$mod"; \
			cat "$$f"; \
			printf '```\n\n'; \
		done ); \
	} > THIRD_PARTY_NOTICES.md; \
	rm -rf $$tmp; \
	echo "wrote THIRD_PARTY_NOTICES.md ($$(grep -c '^## ' THIRD_PARTY_NOTICES.md) modules)"

release-check: ## validate the GoReleaser config
	$(GORELEASER) check

release-snapshot: ## build a local snapshot release into dist/ (no publish)
	$(GORELEASER) release --snapshot --clean

release: ## build + publish a release (CI runs this on a vX.Y.Z tag; needs GITHUB_TOKEN)
	$(GORELEASER) release --clean
