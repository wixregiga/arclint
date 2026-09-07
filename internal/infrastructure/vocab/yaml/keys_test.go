package yamlvocab

import (
	"slices"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// The keys the adapter reads and writes for each building block are
// the meta-model's recorded properties, less the property the entry is
// keyed by in the file. A property added to the meta-model fails here
// until the adapter reads it, and the adapter cannot accept a key the
// meta-model does not record.
func TestBlockKeysAreTheMetaModelRecordings(t *testing.T) {
	model := vocab.DDD()
	cases := []struct {
		term  string
		keyed string
		keys  []string
	}{
		{"domain", "", documentKeys[1:]},
		{"bounded_context", "name", contextKeys},
		{"aggregate", "name", aggregateKeys},
		{"entity", "name", entityKeys},
		{"value_object", "name", valueObjectKeys},
		{"assertion", "key", assertionKeys},
		{"domain_event", "name", eventKeys},
		{"domain_service", "name", serviceKeys},
		{"specification", "name", specificationKeys},
		{"context_relation", "", relationKeys},
	}
	for _, c := range cases {
		block, ok := model.Block(c.term)
		if !ok {
			t.Fatalf("block %q missing from the meta-model", c.term)
		}
		var want []string
		for _, p := range block.Records.Properties {
			if p.Name != c.keyed {
				want = append(want, p.Name)
			}
		}
		if !slices.Equal(c.keys, want) {
			t.Errorf("%s: adapter keys %v, the meta-model records %v", c.term, c.keys, want)
		}
	}
	if documentKeys[0] != keyVersion {
		t.Errorf("the document's first key is %q, want version", documentKeys[0])
	}
	// invariant and question are keyed statements: key and one text.
	for _, term := range []string{"invariant", "question"} {
		block, ok := model.Block(term)
		if !ok {
			t.Fatalf("block %q missing from the meta-model", term)
		}
		if len(block.Records.Properties) != 2 || block.Records.Properties[0].Name != "key" {
			t.Errorf("%s records %+v; the adapter writes it as key: text", term, block.Records.Properties)
		}
	}
}
