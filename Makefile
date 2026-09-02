GO      ?= go
VERSION := $(shell cat cmd/arclint/VERSION)
BIN     ?= ./arclint
GRAMMARS := grammar_subset grammar_subset_typescript grammar_subset_tsx grammar_subset_python

.DEFAULT_GOAL := check

# ==============================================================================
# Core SDLC Workflow
# These are the primary commands for daily development and CI.
# ==============================================================================
.PHONY: build test vet fmt lint generate check check-fix check-ro ci release clean

build:
	CGO_ENABLED=0 $(GO) build -tags "$(GRAMMARS)" -trimpath -ldflags "-s -w" -o $(BIN) ./cmd/arclint

test:
	$(GO) test -race -timeout 30m -coverprofile=coverage.out -covermode=atomic ./...

vet:
	$(GO) vet ./...
	$(GO) vet -tags bench ./internal/bench/

fmt:
	golangci-lint fmt --diff

lint:
	golangci-lint run --fix ./...

generate:
	$(GO) generate ./...

clean:
	rm -rf $(BIN) dist

# ------------------------------------------------------------------------------
# CI and Check Gates
# ------------------------------------------------------------------------------

# Variables for the check gate so CI can override them cleanly.
CHECK_LINT ?= lint
CHECK_LEAK ?= _leak
CHECK_VERIFY ?= _quick-verify

# Canonical local check: auto-fixes lint errors, runs fast tests, and checks staged secrets.
check: $(CHECK_LINT) $(CHECK_VERIFY) $(CHECK_LEAK)


# check-fix is required by the AGENTS.md contract. It aliases to check.
check-fix: check

# Read-only quick gate for review sessions and agents. Must not mutate the tree.
check-ro:
	$(MAKE) check CHECK_LINT=_lint-no-fix CHECK_VERIFY=_verify-ro CHECK_LEAK=_noop

# CI gate: calls check but overrides steps to be CI-appropriate and full.
ci:
	$(MAKE) check CHECK_LINT=_lint-no-fix CHECK_VERIFY=_verify CHECK_LEAK=_leak-ci

# Build the same Linux amd64/arm64 archives and checksums as a beta release,
# without publishing. Requires goreleaser (mise install / mise run release).
# Does not need credentials or a git tag.
release:
	ARCLINT_VERSION=$(VERSION) goreleaser check
	ARCLINT_VERSION=$(VERSION) goreleaser release --snapshot --clean

# ==============================================================================
# Internal / One-Off / Hidden Targets
# ==============================================================================
# Hiding targets from shell auto-completion is done by defining them via variables
# or prefixing with an underscore.

.PHONY: _quick-verify _verify _verify-ro _lint-no-fix _leak _leak-ci _leak-check _noop

_quick-verify:
	$(GO) vet ./...
	$(GO) test -short ./...

_verify: vet test
	$(MAKE) build
	$(BIN) check .

_verify-ro: _quick-verify
	$(GO) run -tags "$(GRAMMARS)" ./cmd/arclint check .

_noop:
	@true

_lint-no-fix:
	golangci-lint run --fix=false ./...

_leak:
	gitleaks protect --staged --verbose 

_leak-ci:
	gitleaks detect --no-git --verbose

_leak-check:
	gitleaks detect --verbose


# --- Hidden One-Offs ---
# We define these via variables so bash/zsh `make <tab>` auto-completion ignores them,
# keeping the main API clean, while still allowing them to be run explicitly.
DOCS_BUILD := docs-build
DOCS_SERVE := docs-serve
BENCH      := bench
AGENTBENCH := agentbench
DOCKER     := docker
COV_REP    := coverage-report
COV_HTML   := coverage-html
LINT_SCH_R := lint-schema-rule
LINT_SCH   := lint-schema

.PHONY: $(DOCS_BUILD) $(DOCS_SERVE) $(BENCH) $(AGENTBENCH) $(DOCKER) $(COV_REP) $(COV_HTML) $(LINT_SCH_R) $(LINT_SCH)

$(DOCS_BUILD):
	cd docs/site && zola build

$(DOCS_SERVE):
	cd docs/site && zola serve

$(BENCH): build
	ARCLINT_BIN=$(abspath $(BIN)) $(GO) test -tags bench -count=1 -v ./internal/bench/

$(AGENTBENCH): build
	ARCLINT_BIN=$(abspath $(BIN)) $(GO) test -tags agentbench -timeout 60m -count=1 -v ./internal/agentbench/

$(DOCKER):
	docker build --build-arg GRAMMARS="$(GRAMMARS)" -t arclint:$(VERSION) .

$(COV_REP):
	@if [ -f coverage.out ]; then \
		echo "Coverage summary:"; \
		$(GO) tool cover -func=coverage.out | tail -1; \
	else \
		echo "No coverage.out found. Run 'make test' first."; \
	fi

$(COV_HTML):
	@if [ -f coverage.out ]; then \
		$(GO) tool cover -html=coverage.out -o coverage.html; \
		echo "Coverage report generated: coverage.html"; \
	else \
		echo "No coverage.out found. Run 'make test' first."; \
	fi

SCHEMA_FILES := ./docs/rules.schema.json \
	.agents/skills/domain-librarian/library.schema.json \
	testing/boxoffice/.arclint/schemas/rules.arclint.schema.json

$(LINT_SCH_R):
	@spectral lint -F error -D --ruleset .spectral.yaml ./docs/rules.schema.json

$(LINT_SCH):
	@spectral lint -F error -D --ruleset .spectral.yaml $(SCHEMA_FILES)
	@echo "All schema files linted successfully."
