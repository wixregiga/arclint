package vocab_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// repoRoot locates the repository root from this source file, keeping
// the tests independent of the working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller: no source location")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// TestSchemaIsDeterministicIndentedJSON proves the published bytes are
// reproducible: identical across calls, valid JSON, indented, and
// newline-terminated so the committed file compares byte-for-byte.
func TestSchemaIsDeterministicIndentedJSON(t *testing.T) {
	first, err := vocab.Schema()
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	second, err := vocab.Schema()
	if err != nil {
		t.Fatalf("Schema (second call): %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("Schema output differs between calls")
	}
	if !json.Valid(first) {
		t.Errorf("Schema output is not valid JSON")
	}
	if !bytes.HasPrefix(first, []byte("{\n  \"")) {
		t.Errorf("Schema output is not indented: starts %q", first[:min(len(first), 8)])
	}
	if !bytes.HasSuffix(first, []byte("}\n")) {
		t.Errorf("Schema output does not end with a newline-terminated object")
	}
}

// TestSchemaMatchesPublishedSchema is the drift half of the domain
// schema invariant: vocab.Schema() must equal the published
// docs/schemas/domain.arclint.schema.json byte-for-byte.
//
// On failure: regenerate the committed schemas via make schemas, or fix
// the generator; never edit the committed copies by hand.
func TestSchemaMatchesPublishedSchema(t *testing.T) {
	wantPath := filepath.Join(repoRoot(t), "docs", "schemas", vocab.SchemaFileName)
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read published %s: %v", vocab.SchemaFileName, err)
	}
	got, err := vocab.Schema()
	if err != nil {
		t.Fatalf("vocab.Schema: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("vocab.Schema() drifted from published %s; run make schemas, or fix the generator; never edit the committed copies by hand\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			vocab.SchemaFileName, len(got), truncate(got, 400), len(want), truncate(want, 400))
	}
}

// TestSchemaIdentifiesItself pins the $id to the published location and
// the file name the modeline and the skill vocabulary point at.
func TestSchemaIdentifiesItself(t *testing.T) {
	got, err := vocab.Schema()
	if err != nil {
		t.Fatalf("vocab.Schema: %v", err)
	}
	var doc struct {
		ID string `json:"$id"`
	}
	if err := json.Unmarshal(got, &doc); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	if doc.ID != vocab.SchemaID {
		t.Fatalf("$id = %q, want %q", doc.ID, vocab.SchemaID)
	}
	if want := "https://raw.githubusercontent.com/wixregiga/arclint/main/docs/schemas/" + vocab.SchemaFileName; vocab.SchemaID != want {
		t.Fatalf("SchemaID = %q, want %q", vocab.SchemaID, want)
	}
	if want := ".arclint/schemas/" + vocab.SchemaFileName; vocab.SchemaPath != want {
		t.Fatalf("SchemaPath = %q, want %q", vocab.SchemaPath, want)
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
