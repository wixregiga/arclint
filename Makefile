GO      ?= go
VERSION ?= 0.1.0
BIN     ?= ./arclint

.PHONY: build test vet generate selfcheck bench oracle release ci clean

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/arclint

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

release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/arclint-linux-amd64 ./cmd/arclint
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o dist/arclint-linux-arm64 ./cmd/arclint

ci: vet test selfcheck

clean:
	rm -rf $(BIN) dist
