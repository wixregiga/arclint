# Container build for arclint. The binary embeds cmd/arclint/VERSION;
# the Makefile reads that same file for the image tag.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG GRAMMARS="grammar_subset grammar_subset_typescript grammar_subset_tsx grammar_subset_python"
RUN CGO_ENABLED=0 go build -tags "$GRAMMARS" -trimpath \
    -ldflags "-s -w" -o /arclint ./cmd/arclint

# The binary is static and self-contained (esbuild and the extension
# runtime are in-process), so scratch works: no shell, no libc, nothing
# to patch.
FROM scratch
COPY --from=build /arclint /usr/local/bin/arclint
WORKDIR /repo
ENTRYPOINT ["/usr/local/bin/arclint"]
CMD ["check", "."]
