GO      ?= go
VERSION := $(shell cat cmd/arclint/VERSION)
BIN     ?= ./arclint
# Embed only the tree-sitter grammars the declaration extractors use:
# without these tags every grammar embeds and the binary grows ~19 MB.
GRAMMARS := grammar_subset grammar_subset_typescript grammar_subset_tsx grammar_subset_python
.PHONY: build test vet fmt-check fmt lint lint-fix generate selfcheck verify check check-fix check-ro leak-check bench agentbench release ci clean docs docs-serve docker

build:
	CGO_ENABLED=0 $(GO) build -tags "$(GRAMMARS)" -trimpath -ldflags "-s -w" -o $(BIN) ./cmd/arclint

# The full run includes the toolchain ground truth (network, clones
# cache under ~/.cache); `go test -short ./...` is the quick loop.
test:
	$(GO) test -timeout 30m ./...

vet:
	$(GO) vet ./...
	$(GO) vet -tags bench ./internal/bench/

fmt-check:
	golangci-lint fmt --diff

fmt:
	golangci-lint fmt

lint:
	golangci-lint run --fix=false ./...

lint-fix:
	golangci-lint run --fix ./...

generate:
	$(GO) generate ./...

# M1 gate 3: arclint's own rules.yaml runs clean, CI-style.
selfcheck: build
	$(BIN) check .

# Behavioral and architecture verification after format and lint pass.
verify: vet test selfcheck

# hk pre-commit secret scan. Not a Make-owned linter; gitleaks is the hook.
leak-check:
	gitleaks detect --verbose

# hk pre-commit Make half: format, lint with fixes, then verify.
# Pair with check-leak. Run sequentially so verify sees the repaired tree.
check-fix:
	$(MAKE) fmt
	$(MAKE) lint-fix
	$(MAKE) verify
	$(MAKE) leak-check

# Canonical full local/CI gate. hk runs the same targets as separate
# steps so its fix mode can repair formatting and lint findings first.
check: fmt-check lint verify

# Read-only quick gate: no file writes (selfcheck via go run, no ./arclint
# rebuild), no network (-short skips the toolchain ground truth). For
# review sessions and agents that must not mutate the tree.
check-ro:
	$(GO) vet ./...
	golangci-lint run --fix=false ./...
	$(GO) test -short ./...
	$(GO) run -tags "$(GRAMMARS)" ./cmd/arclint check .

# M1 gate 4: cold start < 100ms; 5,000 files in low single-digit seconds.
bench: build
	ARCLINT_BIN=$(abspath $(BIN)) $(GO) test -tags bench -count=1 -v ./internal/bench/

# Agent convergence measurement: violations + diagnostics -> real coding
# agent -> re-check, with and without prompt-time context. Requires an
# agent CLI (default codex; override AGENTBENCH_AGENT_CMD) and network.
agentbench: build
	ARCLINT_BIN=$(abspath $(BIN)) $(GO) test -tags agentbench -timeout 60m -count=1 -v ./internal/agentbench/

# Docs site (docs/site): markdown content, one zola binary to build.
docs:
	cd docs/site && zola build

docs-serve:
	cd docs/site && zola serve

# Container image; its binary and image tag both use cmd/arclint/VERSION.
# Run a repo check with:
#   docker run --rm -v $(PWD):/repo arclint:$(VERSION)
docker:
	docker build --build-arg GRAMMARS="$(GRAMMARS)" -t arclint:$(VERSION) .

# Build the same Linux amd64/arm64 archives and checksums as a beta release,
# without publishing. Requires goreleaser (mise install / mise run release).
# Does not need credentials or a git tag.
release:
	ARCLINT_VERSION=$(VERSION) goreleaser check
	ARCLINT_VERSION=$(VERSION) goreleaser release --snapshot --clean

ci: check

clean:
	rm -rf $(BIN) dist
