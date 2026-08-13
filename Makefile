GO      ?= go
VERSION ?= 0.1.0
BIN     ?= ./arclint
# Embed only the tree-sitter grammars the fact providers use (M8 ADR):
# without these tags all 206 grammars embed and the binary grows ~19 MB.
GRAMMARS := grammar_subset grammar_subset_typescript grammar_subset_tsx grammar_subset_javascript grammar_subset_python

.PHONY: build test vet generate selfcheck bench oracle release ci clean docs docs-serve

build:
	CGO_ENABLED=0 $(GO) build -tags "$(GRAMMARS)" -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/arclint

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...
	$(GO) vet -tags oracle ./internal/oracle/
	$(GO) vet -tags bench ./internal/bench/

generate:
	$(GO) generate ./...

# M1 gate 3: arclint's own rules.yaml runs clean, CI-style.
selfcheck: build
	$(BIN) load rules.yaml
	$(BIN) check .

# M1 gate 4: cold start < 100ms; 5,000 files in low single-digit seconds.
bench: build
	ARCLINT_BIN=$(abspath $(BIN)) $(GO) test -tags bench -count=1 -v ./internal/bench/

# M1 gate 2: differential oracle over pinned real repositories.
# Network-permitted; clones cache under ~/.cache/arclint-oracle.
oracle:
	$(GO) test -tags oracle -timeout 30m -count=1 -v ./internal/oracle/

# Docs site (docs/site): markdown content, one zola binary to build.
# The rule reference page is generated (go generate ./tools/gendocs);
# a test fails when it drifts from the doc table.
docs:
	cd docs/site && zola build

docs-serve:
	cd docs/site && zola serve

release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -tags "$(GRAMMARS)" -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/arclint-linux-amd64 ./cmd/arclint
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -tags "$(GRAMMARS)" -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/arclint-linux-arm64 ./cmd/arclint

ci: vet test selfcheck

clean:
	rm -rf $(BIN) dist
