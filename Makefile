GO      ?= go
VERSION ?= 0.1.0
BIN     ?= ./arclint
# Embed only the tree-sitter grammars the declaration extractors use:
# without these tags every grammar embeds and the binary grows ~19 MB.
GRAMMARS := grammar_subset grammar_subset_typescript grammar_subset_tsx grammar_subset_python
.PHONY: build test vet fmt-check fmt lint lint-fix generate selfcheck verify check check-fix leak-check bench agentbench release ci clean docs docs-serve docker

build:
	CGO_ENABLED=0 $(GO) build -tags "$(GRAMMARS)" -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/arclint

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

# Container image; the version flows from this file so the Makefile
# stays the single source. Run a repo check with:
#   docker run --rm -v $(PWD):/repo arclint:$(VERSION)
docker:
	docker build --build-arg GRAMMARS="$(GRAMMARS)" --build-arg VERSION=$(VERSION) -t arclint:$(VERSION) .

release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -tags "$(GRAMMARS)" -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/arclint-linux-amd64 ./cmd/arclint
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -tags "$(GRAMMARS)" -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/arclint-linux-arm64 ./cmd/arclint

ci: check

clean:
	rm -rf $(BIN) dist
