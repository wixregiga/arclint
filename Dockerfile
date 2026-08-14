# Container build for arclint. The grammar tags and version come from
# the Makefile docker target, so the Makefile stays the single source
# (make docker); building this file bare still produces a correct
# image with the default args below.
FROM golang:1.26 AS build
ARG GRAMMARS="grammar_subset grammar_subset_typescript grammar_subset_tsx grammar_subset_javascript grammar_subset_python"
ARG VERSION=0.1.0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -tags "$GRAMMARS" -trimpath \
    -ldflags "-s -w -X main.version=$VERSION" -o /arclint ./cmd/arclint

# The binary is static and self-contained (esbuild and the extension
# runtime are in-process), so scratch works: no shell, no libc, nothing
# to patch.
FROM scratch
COPY --from=build /arclint /usr/local/bin/arclint
WORKDIR /repo
ENTRYPOINT ["/usr/local/bin/arclint"]
CMD ["check", "."]
