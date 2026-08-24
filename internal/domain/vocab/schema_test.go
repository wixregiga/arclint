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

// TestSchemaMatchesLitmusLibrarySchema is the drift half of the
// domain-librarian schema invariant: vocab.Schema() must equal the
// committed litmus library.schema.json byte-for-byte.
//
// On failure: regenerate the committed skill artifacts via arclint
// agents skill, or fix the generator — never edit fixtures by hand.
func TestSchemaMatchesLitmusLibrarySchema(t *testing.T) {
	wantPath := filepath.Join(repoRoot(t), ".agents", "skills", "domain-librarian", "library.schema.json")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read litmus library.schema.json: %v", err)
	}
	got, err := vocab.Schema()
	if err != nil {
		t.Fatalf("vocab.Schema: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("vocab.Schema() drifted from litmus library.schema.json; regenerate the committed skill artifacts via arclint agents skill, or fix the generator — never edit fixtures by hand\n--- got (%d bytes) ---\n%s\n--- want (%d bytes) ---\n%s",
			len(got), truncate(got, 400), len(want), truncate(want, 400))
	}
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
