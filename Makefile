# Single source of truth for build + the CI gate set. CI runs `make check`, so
# a green `make check` locally is the same checks CI runs. Lint/vuln tools are
# fetched on demand via `go run <tool>@<version>` — no separate install step.

BINARY := lookit
MODULE := github.com/jonathandeamer/lookit
GOLANGCI_LINT_VERSION := v2.12.2
GORELEASER_VERSION := v2.16.0
GO_LICENSES_VERSION := v1.6.0
GORELEASER := go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

REVIEW_CHROME_TAPES := \
	docs/tui-review/chrome-80-dark.tape \
	docs/tui-review/chrome-100-dark.tape \
	docs/tui-review/chrome-60-dark.tape \
	docs/tui-review/chrome-80-light.tape

REVIEW_RESPONSES_TAPES := \
	docs/tui-review/responses-80-dark.tape \
	docs/tui-review/responses-100-dark.tape \
	docs/tui-review/responses-60-dark.tape \
	docs/tui-review/responses-80-light.tape

REVIEW_FINGERD := docs/tui-review/fixtures/fingerd/fingerd

.PHONY: build test race vet fmt fmt-check lint vuln check hooks tidy clean \
	notices release-check release-snapshot release review-tui review-fingerd \
	review-sheet

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

review-fingerd: ## build the loopback finger server used by responses tapes
	go build -o $(REVIEW_FINGERD) ./docs/tui-review/fixtures/fingerd

review-tui: build review-fingerd ## record docs/tui-review stills (not part of check)
	@command -v vhs >/dev/null || { echo "vhs not on PATH (brew install vhs ffmpeg ttyd)"; exit 1; }
	@command -v ttyd >/dev/null || { echo "ttyd not on PATH (brew install vhs ffmpeg ttyd)"; exit 1; }
	@command -v ffmpeg >/dev/null || { echo "ffmpeg not on PATH (brew install vhs ffmpeg ttyd)"; exit 1; }
	@mkdir -p docs/tui-review/frames
	@$(REVIEW_FINGERD) & echo $$! > docs/tui-review/frames/fingerd.pid
	@trap 'kill $$(cat docs/tui-review/frames/fingerd.pid) 2>/dev/null; rm -f docs/tui-review/frames/fingerd.pid' EXIT; \
	ready=0; \
	i=0; while [ $$i -lt 50 ]; do \
		if $(REVIEW_FINGERD) -ping >/dev/null 2>&1; then ready=1; break; fi; \
		sleep 0.1; i=$$((i+1)); \
	done; \
	if [ $$ready -eq 0 ]; then \
		echo "loopback fingerd is not serving the fixture bodies on 2479/2480"; \
		echo "(port already bound by something else? stale fingerd? run: $(REVIEW_FINGERD) -ping)"; \
		exit 1; \
	fi; \
	for tape in $(REVIEW_CHROME_TAPES) $(REVIEW_RESPONSES_TAPES); do \
		name=$$(basename "$$tape" .tape); \
		tour=$$(awk '/^Source /{print $$2}' "$$tape"); \
		want=$$(grep -c '^Screenshot ' "$$tour"); \
		echo "recording $$name ($$want stills)"; \
		rm -f docs/tui-review/frames/*.png docs/tui-review/frames/_render.txt; \
		vhs "$$tape" || exit 1; \
		got=$$(ls docs/tui-review/frames/*.png 2>/dev/null | wc -l | tr -d ' '); \
		if [ "$$got" != "$$want" ]; then \
			echo "$$name: recorded $$got stills, $$tour asks for $$want"; \
			echo "(a Screenshot with no following Sleep writes nothing)"; \
			exit 1; \
		fi; \
		dest=docs/tui-review/frames/$$name; \
		mkdir -p "$$dest"; \
		rm -f "$$dest"/*.png; \
		mv docs/tui-review/frames/*.png "$$dest/"; \
		rm -f docs/tui-review/frames/_render.txt; \
		sh docs/tui-review/verify-frames.sh "$$dest" || exit 1; \
	done
	@$(MAKE) --no-print-directory review-sheet
	@echo "wrote docs/tui-review/frames/{chrome,responses}-{80-dark,100-dark,60-dark,80-light}/"

review-sheet: ## tile each recorded directory into one contact sheet
	@command -v ffmpeg >/dev/null || { echo "ffmpeg not on PATH (brew install ffmpeg)"; exit 1; }
	@for dir in docs/tui-review/frames/*/; do \
		name=$$(basename "$$dir"); \
		case "$$name" in xdg) continue ;; esac; \
		n=$$(ls "$$dir"*.png 2>/dev/null | wc -l | tr -d ' '); \
		[ "$$n" -gt 0 ] || continue; \
		rows=$$(( ($$n + 3) / 4 )); \
		ffmpeg -y -loglevel error -pattern_type glob -i "$$dir*.png" \
			-filter_complex "scale=iw/2:ih/2,tile=4x$$rows:padding=6:margin=6:color=0x101014" \
			-frames:v 1 "docs/tui-review/frames/$$name-sheet.png" || exit 1; \
		echo "$$name-sheet.png: $$(ls "$$dir" | tr '\n' ' ')"; \
	done

hooks: ## install git hooks (commit-msg: Conventional Commits); run once per clone
	git config core.hooksPath .githooks
	@echo "git hooks installed (core.hooksPath -> .githooks)"

tidy: ## tidy go.mod/go.sum
	go mod tidy

clean: ## remove build artifacts
	rm -f $(BINARY) $(REVIEW_FINGERD)
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
