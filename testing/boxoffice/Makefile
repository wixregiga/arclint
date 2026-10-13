# boxoffice: build, test, lint, and architecture gates.
# The arclint binary is rebuilt from the parent repository every time a
# target uses it, so it never lags the source; Go's build cache keeps an
# unchanged rebuild quick.

ARCLINT := .bin/arclint

.PHONY: build web build-full run test vet lint arclint rules-test check clean $(ARCLINT)

build:
	go build -o .bin/boxoffice ./cmd/boxoffice

web:
	npm --prefix web install
	npm --prefix web run typecheck
	npm --prefix web run build

build-full: web
	go build -tags embedweb -o .bin/boxoffice ./cmd/boxoffice

# One port, always current: rebuild the web app and the embedded
# binary, replace any running instance, serve on :8080. The bracket
# in the pkill pattern keeps it from matching its own shell.
run: build-full
	-pkill -f '[.]bin/boxoffice' 2>/dev/null || true
	.bin/boxoffice -seed -organizer-token dev-organizer

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run

$(ARCLINT):
	go -C ../.. build -o $(abspath $(ARCLINT)) ./cmd/arclint

arclint: $(ARCLINT)
	$(ARCLINT) check

rules-test: $(ARCLINT)
	$(ARCLINT) rules test

check: vet lint test arclint rules-test

clean:
	rm -rf .bin
