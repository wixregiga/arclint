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
)

// multiContextExample is a representative domain file: two bounded
// contexts, every section kind, and one relation.
const multiContextExample = `version: 1
project: shop
description: Selling products to customers.

contexts:
  ordering:
    definition: Taking and fulfilling customer orders.
    aggregates:
      Order:
        definition: A customer's request to purchase products.
        identity: OrderID
        aliases: [Purchase Order]
        entities:
          OrderLine:
            definition: One product and quantity within an Order.
        invariants:
          names-its-customer: Every Order identifies its Customer.
        assertions:
          lines-present:
            on: Place
            statement: An Order is placed with at least one line.
        repository: OrderRepository
    value_objects:
      Money:
        definition: A monetary amount expressed in a particular currency.
        invariants:
          never-negative: Money is never negative.
    events:
      OrderPlaced:
        definition: An Order has been accepted for processing.
        raised_by: Order
    services:
      Pricing:
        definition: Prices an Order from the catalog.
    specifications:
      LargeOrder:
        definition: An Order above the wholesale threshold.
    questions:
      partial-shipment: Can an Order ship in parts?

  billing:
    definition: Invoicing accepted orders.
    aggregates:
      Invoice:
        definition: A bill issued for an accepted Order.
        identity: InvoiceID

relations:
  - from: ordering
    to: billing
    kind: customer_supplier
    description: Billing invoices what ordering accepts.
`

// TestDomainSchemaCompilesAsDraft202012 asserts vocab.Schema() is a
// valid JSON Schema draft 2020-12 document (santhosh-tekuri), matching
// the infrastructure differential-test approach.
func TestDomainSchemaCompilesAsDraft202012(t *testing.T) {
	compileDomainSchema(t)
}

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

// TestProjectSchemaMatchesDomain is the drift half of the Ubiquitous
// Language Schema invariant from the project's side: the copy under
// .arclint/schemas (what the domain file's modeline points at) is
// byte-for-byte what vocab.Schema() produces.
func TestProjectSchemaMatchesDomain(t *testing.T) {
	want, err := vocab.Schema()
	if err != nil {
		t.Fatalf("vocab.Schema: %v", err)
	}
	published := filepath.Join(repoRoot(t), filepath.FromSlash(vocab.SchemaPath))
	got, err := os.ReadFile(published)
	if err != nil {
		t.Fatalf("read project schema: %v", err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("%s drifted from vocab.Schema(); run make schemas", vocab.SchemaPath)
	}
}

// compileDomainSchema compiles vocab.Schema() under its published $id
// with the same validator the engine uses for extension parameter
// schemas.
func compileDomainSchema(t *testing.T) *sj.Schema {
	t.Helper()
	data, err := vocab.Schema()
	if err != nil {
		t.Fatalf("vocab.Schema: %v", err)
	}
	doc, err := sj.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}
	compiler := sj.NewCompiler()
	if err := compiler.AddResource(vocab.SchemaID, doc); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile(vocab.SchemaID)
	if err != nil {
		t.Fatalf("compile draft 2020-12 schema: %v", err)
	}
	return schema
}

// validateAgainstSchema parses the YAML document generically, converts
// it to the JSON data model, and validates it against the compiled
// schema, the editor-side half of the invariant.
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

// TestMultiContextExampleLoadsAndValidates proves the representative
// document both loads through Repository.RecordedLanguage and
// validates against vocab.Schema().
func TestMultiContextExampleLoadsAndValidates(t *testing.T) {
	schema := compileDomainSchema(t)
	repo, _ := repository(t, multiContextExample)
	lang, found, err := repo.RecordedLanguage()
	if err != nil {
		t.Fatalf("RecordedLanguage: %v", err)
	}
	if !found {
		t.Fatal("found = false")
	}
	want := vocab.Counts{
		Contexts: 2, Aggregates: 2, Entities: 1, ValueObjects: 1, Invariants: 2, Assertions: 1,
		Specifications: 1, Events: 1, Services: 1, Questions: 1, Relations: 1,
	}
	if got := lang.Counts(); got != want {
		t.Fatalf("counts = %+v, want %+v", got, want)
	}
	if err := validateAgainstSchema(t, schema, []byte(multiContextExample)); err != nil {
		t.Fatalf("schema rejected the example: %v", err)
	}
}

// TestSchemaAgreesWithLoader is the agreement half of the Ubiquitous
// Language Schema invariant: for every covered case the strict loader
// and JSON-Schema validation of the same document reach the same
// verdict, and that verdict is the expected one. Cross-entry
// invariants (a name recorded twice, a relation naming an undeclared
// context, one pair related twice, an event raised by an unrecorded
// aggregate) are loader-only: JSON Schema cannot express them, so
// those cases only require the loader to reject.
func TestSchemaAgreesWithLoader(t *testing.T) {
	schema := compileDomainSchema(t)
	const minimal = "version: 1\nproject: shop\ncontexts:\n  ordering:\n    definition: Taking orders.\n"

	cases := []struct {
		name       string
		document   string
		accepted   bool
		loaderOnly bool
	}{
		{"multi-context example", multiContextExample, true, false},
		{"minimal context", minimal, true, false},
		{"version 2", "version: 2\nproject: shop\ncontexts:\n  ordering:\n    definition: d\n", false, false},
		{"missing version", "project: shop\ncontexts:\n  ordering:\n    definition: d\n", false, false},
		{"missing project", "version: 1\ncontexts:\n  ordering:\n    definition: d\n", false, false},
		{"missing contexts", "version: 1\nproject: shop\n", false, false},
		{"unknown top-level key", minimal + "extra: true\n", false, false},
		{"contexts as a list", "version: 1\nproject: shop\ncontexts:\n  - name: ordering\n", false, false},
		{"context name not lowercase", "version: 1\nproject: shop\ncontexts:\n  Ordering:\n    definition: d\n", false, false},
		{"context without definition", "version: 1\nproject: shop\ncontexts:\n  ordering: {}\n", false, false},
		{"unknown key on a context", minimal + "    entities: {}\n", false, false},
		{"section without a value", minimal + "    aggregates:\n", false, false},
		{"zones is not a context key", minimal + "    zones: [ordering]\n", false, false},
		{
			"aggregate without identity", minimal +
				"    aggregates:\n      Order:\n        definition: A purchase.\n", false, false,
		},
		{
			"aggregate with a definition and identity", minimal +
				"    aggregates:\n      Order:\n        definition: A purchase.\n        identity: OrderID\n", true, false,
		},
		{
			"unknown key on an aggregate", minimal +
				"    aggregates:\n      Order:\n        definition: A purchase.\n        identity: OrderID\n        owner: x\n", false, false,
		},
		{
			"entity without definition", minimal +
				"    aggregates:\n      Order:\n        definition: A purchase.\n        identity: OrderID\n        entities:\n          Line: {}\n", false, false,
		},
		{
			"invariant key not kebab", minimal +
				"    aggregates:\n      Order:\n        definition: A purchase.\n        identity: OrderID\n        invariants:\n          Total_Sum: The total is the sum.\n", false, false,
		},
		{
			"invariant as a mapping", minimal +
				"    aggregates:\n      Order:\n        definition: A purchase.\n        identity: OrderID\n        invariants:\n          total-sum:\n            statement: The total is the sum.\n", false, false,
		},
		{
			"assertion without on", minimal +
				"    aggregates:\n      Order:\n        definition: A purchase.\n        identity: OrderID\n        assertions:\n          priced:\n            statement: Every line is priced.\n", false, false,
		},
		{
			"value object with identity", minimal +
				"    value_objects:\n      Money:\n        definition: An amount.\n        identity: MoneyID\n", false, false,
		},
		{"question as a mapping", minimal + "    questions:\n      open:\n        text: Why?\n", false, false},
		{
			"bad relation kind", minimal + "  billing:\n    definition: Invoicing.\nrelations:\n  - from: ordering\n    to: billing\n    kind: not_a_kind\n",
			false, false,
		},
		{
			"relation without kind", minimal + "  billing:\n    definition: Invoicing.\nrelations:\n  - from: ordering\n    to: billing\n",
			false, false,
		},
		{"relations as a mapping", minimal + "relations:\n  ordering: billing\n", false, false},
		{
			"same relation twice", minimal + "  billing:\n    definition: Invoicing.\nrelations:\n" +
				"  - from: ordering\n    to: billing\n    kind: conformist\n" +
				"  - from: ordering\n    to: billing\n    kind: conformist\n",
			false, false,
		},
		{
			"duplicate alias", minimal +
				"    aggregates:\n      Order:\n        definition: A purchase.\n        identity: OrderID\n        aliases: [Purchase Order, Purchase Order]\n",
			false, false,
		},
		{
			"same pair related with two kinds", minimal + "  billing:\n    definition: Invoicing.\nrelations:\n" +
				"  - from: ordering\n    to: billing\n    kind: conformist\n" +
				"  - from: ordering\n    to: billing\n    kind: customer_supplier\n",
			false, true,
		},
		{
			"relation to undeclared context", minimal + "relations:\n  - from: ordering\n    to: billing\n    kind: customer_supplier\n",
			false, true,
		},
		{
			"name recorded twice in a context", minimal +
				"    aggregates:\n      Order:\n        definition: A purchase.\n        identity: OrderID\n" +
				"    value_objects:\n      Order:\n        definition: An amount.\n",
			false, true,
		},
		{
			"event raised by an unrecorded aggregate", minimal +
				"    events:\n      OrderPlaced:\n        definition: An order was placed.\n        raised_by: Order\n",
			false, true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := repository(t, tc.document)
			_, _, loaderErr := repo.RecordedLanguage()
			schemaErr := validateAgainstSchema(t, schema, []byte(tc.document))
			loaderAccepts := loaderErr == nil
			schemaAccepts := schemaErr == nil

			if tc.loaderOnly {
				if loaderAccepts {
					t.Fatalf("loader accepted a loader-only document")
				}
				if !schemaAccepts {
					t.Fatalf("schema rejected a loader-only document; the case belongs with the agreeing ones: %v", schemaErr)
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
