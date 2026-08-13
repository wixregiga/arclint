package config

import (
	"bytes"
	"os"
	"testing"
)

// TestPublishedSchemaCurrent fails when docs/rules.schema.json drifts
// from the reflected schema; `go generate ./internal/config` refreshes
// it. Runtime validation compiles from reflection and cannot drift, so
// this guards the editor-completion copy only — the same treatment the
// generated docs reference page gets.
func TestPublishedSchemaCurrent(t *testing.T) {
	committed, err := os.ReadFile("../../docs/rules.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	generated, err := SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(committed, generated) {
		t.Fatal("docs/rules.schema.json is stale; run `go generate ./internal/config`")
	}
}
