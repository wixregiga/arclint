package vocab_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// TestSchemaIsDeterministicIndentedJSON proves the published bytes are
// reproducible: identical across calls, valid JSON, indented, and
// newline-terminated so the committed file compares byte-for-byte.
// Schema acceptance against a JSON Schema validator lives in
// infrastructure (yamlvocab), keeping this package stdlib-only.
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
