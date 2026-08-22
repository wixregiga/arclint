package yamlvocab_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	sj "github.com/santhosh-tekuri/jsonschema/v6"
	yamlv3 "gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

// recommendationExample is the Order/Customer ubiquitous-language.yaml
// from docs/domain-cli-recommendation.md (Initial file shape), copied
// verbatim as a test literal.
const recommendationExample = `version: 1

entities:
  - name: Order
    definition: A customer's request to purchase products.
    aliases:
      - Purchase Order
    aggregate: true

  - name: Customer
    definition: A person or organization that places Orders.

value_objects:
  - name: OrderID
    definition: The stable identity of an Order.

  - name: Money
    definition: A monetary amount expressed in a particular currency.

business_rules:
  - name: OrderMustHaveCustomer
    definition: Every Order must identify its Customer.

events:
  - name: OrderPlaced
    definition: An Order has been accepted for processing.
`

// repoRoot locates the repository root from this source file, keeping
// the tests independent of the working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller: no source location")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
}

// TestPublishedSchemaMatchesDomain is the drift half of the Ubiquitous
// Language Schema invariant: the committed
// docs/ubiquitous-language.schema.json is byte-for-byte what
// vocab.Schema() produces. Regenerate by writing Schema() output
// over the file.
func TestPublishedSchemaMatchesDomain(t *testing.T) {
	want, err := vocab.Schema()
	if err != nil {
		t.Fatalf("vocab.Schema: %v", err)
	}
	published := filepath.Join(repoRoot(t), "docs", "ubiquitous-language.schema.json")
	got, err := os.ReadFile(published)
	if err != nil {
		t.Fatalf("read published schema: %v", err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("docs/ubiquitous-language.schema.json drifted from vocab.Schema(); regenerate it from vocab.Schema() output")
	}
}

// compileUbiquitousSchema compiles vocab.Schema() with the same
// validator the engine uses for extension parameter schemas.
func compileUbiquitousSchema(t *testing.T) *sj.Schema {
	t.Helper()
	data, err := vocab.Schema()
	if err != nil {
		t.Fatalf("vocab.Schema: %v", err)
	}
	doc, err := sj.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	const url = "https://raw.githubusercontent.com/wixregiga/arclint/main/docs/ubiquitous-language.schema.json"
	compiler := sj.NewCompiler()
	if err := compiler.AddResource(url, doc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile(url)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return schema
}

// validateAgainstSchema parses the YAML document generically, converts
// it to the JSON data model, and validates it against the compiled
// schema — the editor-side half of the invariant.
func validateAgainstSchema(t *testing.T, schema *sj.Schema, source []byte) error {
	t.Helper()
	var value any
	if err := yamlv3.Unmarshal(source, &value); err != nil {
		t.Fatalf("generic YAML parse: %v", err)
	}
	data, err := json.Marshal(jsonify(value))
	if err != nil {
		t.Fatalf("marshal generic document: %v", err)
	}
	instance, err := sj.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unmarshal generic document: %v", err)
	}
	return schema.Validate(instance)
}

// jsonify converts YAML-decoded values into the JSON data model,
// stringifying any non-string map keys.
func jsonify(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, entry := range typed {
			out[key] = jsonify(entry)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, entry := range typed {
			out[fmt.Sprintf("%v", key)] = jsonify(entry)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, entry := range typed {
			out[i] = jsonify(entry)
		}
		return out
	default:
		return value
	}
}

// TestRecommendationExampleLoadsAndValidates proves the recommendation
// doc's example file both loads through Repository.RecordedLanguage and
// validates against vocab.Schema().
func TestRecommendationExampleLoadsAndValidates(t *testing.T) {
	schema := compileUbiquitousSchema(t)
	dir := t.TempDir()
	path := filepath.Join(dir, vocab.UbiquitousLanguageFileName)
	if err := os.WriteFile(path, []byte(recommendationExample), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	repo, err := yamlvocab.NewRepository(dir)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	lang, found, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	if !found {
		t.Fatal("found = false")
	}
	counts := lang.Counts()
	if counts.Entities != 2 || counts.Aggregates != 1 || counts.ValueObjects != 2 ||
		counts.BusinessRules != 1 || counts.Events != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	if err := validateAgainstSchema(t, schema, []byte(recommendationExample)); err != nil {
		t.Fatalf("schema rejected recommendation example: %v", err)
	}
}

// TestSchemaAgreesWithLoader is the agreement half of the Ubiquitous
// Language Schema invariant: for every covered case the strict loader
// and JSON-Schema validation of the same document reach the same
// verdict, and that verdict is the expected one. Duplicate names are
// a loader-only structural invariant (JSON Schema cannot express
// unique name-within-section cheaply), so that case only requires the
// loader to reject.
func TestSchemaAgreesWithLoader(t *testing.T) {
	schema := compileUbiquitousSchema(t)

	cases := []struct {
		name           string
		document       string
		accepted       bool
		loaderOnlyFail bool
	}{
		{"recommendation example", recommendationExample, true, false},
		{"version 2", "version: 2\nentities: []\n", false, false},
		{"missing version", "entities:\n  - name: Order\n", false, false},
		{
			"unknown key", `
version: 1
extra: true
entities:
  - name: Order
`, false, false,
		},
		{
			"aggregate on value_objects", `
version: 1
value_objects:
  - name: Money
    aggregate: true
`, false, false,
		},
		{
			"empty name", `
version: 1
entities:
  - name: ""
`, false, false,
		},
		{
			"duplicate name", `
version: 1
entities:
  - name: Order
  - name: Order
`, false, true,
		},
		{
			"minimal entity", `
version: 1
entities:
  - name: Order
`, true, false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, vocab.UbiquitousLanguageFileName)
			if err := os.WriteFile(path, []byte(tc.document), 0o600); err != nil {
				t.Fatalf("WriteFile: %v", err)
			}
			repo, err := yamlvocab.NewRepository(dir)
			if err != nil {
				t.Fatalf("NewRepository: %v", err)
			}
			_, _, loaderErr := repo.RecordedLanguage()
			schemaErr := validateAgainstSchema(t, schema, []byte(tc.document))
			loaderAccepts := loaderErr == nil
			schemaAccepts := schemaErr == nil

			if tc.loaderOnlyFail {
				if loaderAccepts {
					t.Fatalf("loader accepted duplicate-name document")
				}
				return
			}
			if loaderAccepts != schemaAccepts {
				t.Fatalf("divergence: loader accepts=%v (err: %v), schema accepts=%v (err: %v)",
					loaderAccepts, loaderErr, schemaAccepts, schemaErr)
			}
			if loaderAccepts != tc.accepted {
				t.Fatalf("both sides agree on accepts=%v, but the case expects %v (loader err: %v)",
					loaderAccepts, tc.accepted, loaderErr)
			}
		})
	}
}
