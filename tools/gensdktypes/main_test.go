package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Regenerate from the current Go wire structs, not the already-generated TS.
// Together with TestPublishedSDKMatchesContract this checks the entire chain.
func TestGeneratedWireTypesAreCurrent(t *testing.T) {
	output := filepath.Join(t.TempDir(), "types_gen.ts")
	if err := generate(output); err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("../../internal/infrastructure/extension/sobek/sdk/types_gen.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, committed) {
		t.Fatal("SDK wire types are stale; run go generate ./internal/infrastructure/extension/sobek and go run ./cmd/arclint sdk init")
	}
}

func TestGeneratedAPIUsesTheSDKInitContract(t *testing.T) {
	output := filepath.Join(t.TempDir(), "api_gen.ts")
	const source = "../../internal/infrastructure/extension/sobek/sdkinit.go"
	if err := generateAPI(source, output); err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	usedByRuntime, err := os.ReadFile("../../internal/infrastructure/extension/sobek/sdk/api_gen.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, usedByRuntime) {
		t.Fatal("SDK runtime contract is stale; run go generate ./internal/infrastructure/extension/sobek")
	}
}
