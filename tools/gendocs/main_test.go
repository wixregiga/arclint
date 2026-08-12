package main

import (
	"bytes"
	"os"
	"testing"
)

// TestReferencePageCurrent fails when docs/site/content/docs/rules.md
// drifts from the doc table; `go generate ./tools/gendocs` refreshes it.
func TestReferencePageCurrent(t *testing.T) {
	committed, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(committed, render()) {
		t.Fatalf("%s is stale; run `go generate ./tools/gendocs`", outPath)
	}
}
