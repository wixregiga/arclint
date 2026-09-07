package main

// End-to-end coverage of the arclint domain command family against the
// compiled binary: recording a Ubiquitous Language context by context,
// reading it back in text and JSON, and the built-in rules that apply
// to it the moment it is recorded.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalDomainRules is a ruleset that resolves a project root so
// domain.arclint.yaml is found/created beside rules.arclint.yaml.
const minimalDomainRules = `runtime: [go]
zones:
  src:
    paths: ["src/**"]
`

// orderingDomain is a recorded model in the shape the domain file
// takes: one context holding an aggregate with a member entity and an
// invariant, two value objects, and two events.
const orderingDomain = `version: 1
project: shop
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
            definition: One product on the order.
        invariants:
          customer-identified: Every Order identifies its Customer.
          total-never-negative: An Order's total is never negative.
    value_objects:
      OrderID:
        definition: The stable identity of an Order.
      Money:
        definition: A monetary amount expressed in a particular currency.
    events:
      OrderPlaced:
        definition: An Order has been accepted for processing.
        raised_by: Order
      OrderCancelled:
        definition: An Order has been cancelled and will not be processed.
`

func domainFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", minimalDomainRules)
	write(t, root, "src/ok.go", "package src\n")
	return root
}

func mustRunDomain(t *testing.T, root string, args ...string) string {
	t.Helper()
	stdout, stderr, code := runBin(t, root, os.Environ(), args...)
	if code != 0 {
		t.Fatalf("%v: exit %d\nstdout: %s\nstderr: %s", args, code, stdout, stderr)
	}
	return stdout
}

// recordOrdering records the ordering context of orderingDomain through
// the command line, one define at a time.
func recordOrdering(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{
		{"domain", "define", "bounded_context", "ordering", "--definition", "Taking and fulfilling customer orders."},
		{"domain", "define", "aggregate", "Order", "--identity", "OrderID", "--definition", "A customer's request to purchase products.", "--alias", "Purchase Order"},
		{"domain", "define", "entity", "OrderLine", "--owner", "Order", "--definition", "One product on the order."},
		{"domain", "define", "value_object", "OrderID", "--definition", "The stable identity of an Order."},
		{"domain", "define", "value_object", "Money", "--definition", "A monetary amount expressed in a particular currency."},
		{"domain", "define", "invariant", "customer-identified", "--owner", "Order", "--statement", "Every Order identifies its Customer."},
		{"domain", "define", "invariant", "total-never-negative", "--owner", "Order", "--statement", "An Order's total is never negative."},
		{"domain", "define", "domain_event", "OrderPlaced", "--definition", "An Order has been accepted for processing.", "--raised-by", "Order"},
		{"domain", "define", "domain_event", "OrderCancelled", "--definition", "An Order has been cancelled and will not be processed."},
	} {
		out := mustRunDomain(t, root, args...)
		if !strings.HasPrefix(out, "Defined ") {
			t.Fatalf("%v: got %q", args, out)
		}
	}
}

func TestDomainCommandFamily(t *testing.T) {
	t.Run("init", testDomainInit)
	t.Run("resolution", testDomainResolution)
	t.Run("fullFlow", testDomainFullFlow)
	t.Run("exitCodes", testDomainExitCodes)
	t.Run("json", testDomainJSON)
	t.Run("commentPreservation", testDomainCommentPreservation)
	t.Run("guided", testDomainGuided)
	t.Run("schema", testDomainSchema)
	t.Run("help", testDomainHelp)
	t.Run("exclusions", testDomainExclusions)
	t.Run("removeLeavesSources", testDomainRemoveLeavesSources)
	t.Run("extensionAccess", testDomainExtensionAccess)
	t.Run("builtInRules", testDomainBuiltInRules)
	t.Run("context", testDomainContext)
	t.Run("ambiguity", testDomainAmbiguity)
}

func testDomainInit(t *testing.T) {
	root := domainFixture(t)
	modelPath := filepath.Join(root, "domain.arclint.yaml")
	project := filepath.Base(root)

	stdout := mustRunDomain(t, root, "domain", "init")
	if stdout != "Initialized domain.arclint.yaml for project "+project+".\n" {
		t.Fatalf("first init output = %q", stdout)
	}
	created, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read initialized model: %v", err)
	}
	text := string(created)
	for _, want := range []string{"yaml-language-server: $schema=", "version: 1\n", "contexts: {}\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("initialized model lacks %q:\n%s", want, text)
		}
	}
	// A temp directory's basename may be numeric, which YAML quotes.
	if !strings.Contains(text, "project: "+project+"\n") && !strings.Contains(text, "project: \""+project+"\"\n") {
		t.Fatalf("initialized model does not name project %s:\n%s", project, text)
	}

	existing := "# project-owned comment\n" + orderingDomain
	write(t, root, "domain.arclint.yaml", existing)
	stdout = mustRunDomain(t, root, "domain", "init")
	if stdout != "domain.arclint.yaml already exists (project shop); left unchanged.\n" {
		t.Fatalf("repeated init output = %q", stdout)
	}
	unchanged, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read existing model: %v", err)
	}
	if string(unchanged) != existing {
		t.Fatalf("repeated init changed existing model:\n%s", unchanged)
	}

	named := domainFixture(t)
	stdout = mustRunDomain(t, named, "domain", "init", "--project", "boxoffice")
	if stdout != "Initialized domain.arclint.yaml for project boxoffice.\n" {
		t.Fatalf("named init output = %q", stdout)
	}

	_, stderr, code := runBin(t, root, os.Environ(), "domain", "init", "extra")
	if code != 2 {
		t.Fatalf("init with argument: exit %d stderr %q", code, stderr)
	}
}

func testDomainResolution(t *testing.T) {
	root := domainFixture(t)

	stdout, stderr, code := runBin(t, root, os.Environ(), "domain")
	if code != 0 {
		t.Fatalf("domain with no model: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "No recorded Ubiquitous Language found") {
		t.Fatalf("missing-model overview:\n%s", stdout)
	}
	if !strings.Contains(stdout, "arclint domain define bounded_context <name> --definition <text>") {
		t.Fatalf("missing-model guidance should say how to record the first context:\n%s", stdout)
	}

	// An entry cannot be recorded before its context is; the refusal
	// says what to run.
	_, stderr, code = runBin(t, root, os.Environ(),
		"domain", "define", "aggregate", "Order",
		"--identity", "OrderID",
		"--definition", "A customer's request to purchase products.",
	)
	if code != 2 || !strings.Contains(stderr, "no bounded context is recorded yet; record one first with: domain define bounded_context <name> --definition <text>") {
		t.Fatalf("define before any context: exit %d stderr %q", code, stderr)
	}
	_, stderr, code = runBin(t, root, os.Environ(),
		"domain", "define", "aggregate", "Order",
		"--context", "ordering",
		"--identity", "OrderID",
		"--definition", "A customer's request to purchase products.",
	)
	if code != 2 || !strings.Contains(stderr, `context "ordering" is not recorded; record it first with: domain define bounded_context ordering --definition <text>`) {
		t.Fatalf("define in an unrecorded context: exit %d stderr %q", code, stderr)
	}

	// The first define creates the file beside rules.arclint.yaml.
	out := mustRunDomain(t, root, "domain", "define", "bounded_context", "ordering", "--definition", "Taking and fulfilling customer orders.")
	if out != "Defined bounded context ordering.\n" {
		t.Fatalf("define bounded_context: %q", out)
	}
	modelPath := filepath.Join(root, "domain.arclint.yaml")
	if _, err := os.Stat(modelPath); err != nil {
		t.Fatalf("define must create domain.arclint.yaml beside rules.arclint.yaml: %v", err)
	}
	mustRunDomain(t, root,
		"domain", "define", "aggregate", "Order",
		"--identity", "OrderID",
		"--definition", "A customer's request to purchase products.",
	)

	other := t.TempDir()
	stdout, stderr, code = runBin(t, other, os.Environ(),
		"--rules", filepath.Join(root, "rules.arclint.yaml"),
		"domain", "show", "aggregate", "Order",
	)
	if code != 0 {
		t.Fatalf("--rules domain show: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Aggregate: Order\n") || !strings.Contains(stdout, "Context: ordering\n") {
		t.Fatalf("--rules show missed Order:\n%s", stdout)
	}
}

func testDomainFullFlow(t *testing.T) {
	root := domainFixture(t)

	out := mustRunDomain(t, root, "domain", "define", "bounded_context", "ordering", "--definition", "Taking and fulfilling customer orders.")
	if out != "Defined bounded context ordering.\n" {
		t.Fatalf("define bounded_context: %q", out)
	}
	out = mustRunDomain(t, root,
		"domain", "define", "aggregate", "Order",
		"--identity", "OrderID",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
	)
	if out != "Defined aggregate Order in context ordering.\n" {
		t.Fatalf("define aggregate: %q", out)
	}
	out = mustRunDomain(t, root, "domain", "define", "entity", "OrderLine", "--owner", "Order", "--definition", "One product on the order.")
	if out != "Defined entity OrderLine under Order in context ordering.\n" {
		t.Fatalf("define entity: %q", out)
	}
	for _, args := range [][]string{
		{"domain", "define", "value_object", "OrderID", "--definition", "The stable identity of an Order."},
		{"domain", "define", "value_object", "Money", "--definition", "A monetary amount expressed in a particular currency."},
		{"domain", "define", "invariant", "customer-identified", "--owner", "Order", "--statement", "Every Order identifies its Customer."},
		{"domain", "define", "invariant", "total-never-negative", "--owner", "Order", "--statement", "An Order's total is never negative."},
		{"domain", "define", "domain_event", "OrderPlaced", "--definition", "An Order has been accepted for processing.", "--raised-by", "Order"},
		{"domain", "define", "domain_event", "OrderCancelled", "--definition", "An Order has been cancelled and will not be processed."},
	} {
		out = mustRunDomain(t, root, args...)
		if !strings.HasPrefix(out, "Defined ") {
			t.Fatalf("%v: got %q", args, out)
		}
	}

	// An update names each property that changed.
	out = mustRunDomain(t, root, "domain", "define", "aggregate", "Order", "--alias", "Purchase Order", "--alias", "Booking")
	if out != "Updated aggregate Order in context ordering.\n  aliases: Purchase Order, Booking\n" {
		t.Fatalf("update aggregate aliases: %q", out)
	}

	wantShow := "" +
		"Aggregate: Order\n" +
		"Context: ordering\n" +
		"Identity: OrderID\n" +
		"Definition: A customer's request to purchase products.\n" +
		"Aliases: Purchase Order, Booking\n" +
		"Entities:\n" +
		"  OrderLine  One product on the order.\n" +
		"Invariants:\n" +
		"  customer-identified  Every Order identifies its Customer.\n" +
		"  total-never-negative  An Order's total is never negative.\n"
	out = mustRunDomain(t, root, "domain", "show", "aggregate", "Order")
	if out != wantShow {
		t.Fatalf("show aggregate Order:\n got: %q\nwant: %q", out, wantShow)
	}

	wantEntity := "" +
		"Entity: OrderLine\n" +
		"Context: ordering\n" +
		"Aggregate: Order\n" +
		"Definition: One product on the order.\n"
	out = mustRunDomain(t, root, "domain", "show", "entity", "OrderLine")
	if out != wantEntity {
		t.Fatalf("show entity OrderLine:\n got: %q\nwant: %q", out, wantEntity)
	}

	out = mustRunDomain(t, root, "domain", "show", "invariant", "customer-identified")
	if !strings.Contains(out, "Invariant: customer-identified\n") ||
		!strings.Contains(out, "Owner: Order\n") ||
		!strings.Contains(out, "Statement: Every Order identifies its Customer.\n") {
		t.Fatalf("show invariant:\n%s", out)
	}

	out = mustRunDomain(t, root, "domain", "show", "bounded_context", "ordering")
	for _, want := range []string{
		"Bounded Context: ordering\n",
		"Definition: Taking and fulfilling customer orders.\n",
		"Aggregates: Order\n",
		"Value objects: OrderID, Money\n",
		"Domain events: OrderPlaced, OrderCancelled\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("show bounded_context missing %q:\n%s", want, out)
		}
	}

	out = mustRunDomain(t, root, "domain", "list", "aggregates")
	if !strings.Contains(out, "Context ordering\n") || !strings.Contains(out, "    Order (OrderID)\n      OrderLine\n") {
		t.Fatalf("list aggregates:\n%s", out)
	}

	out = mustRunDomain(t, root, "domain", "list")
	for _, want := range []string{
		"Context ordering\n",
		"  Aggregates\n    Order (OrderID)\n      OrderLine\n",
		"  Value objects\n    OrderID\n    Money\n",
		"  Invariants\n    customer-identified (Order)\n    total-never-negative (Order)\n",
		"  Domain events\n    OrderPlaced (raised by Order)\n    OrderCancelled\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("list all missing %q:\n%s", want, out)
		}
	}

	out = mustRunDomain(t, root, "domain")
	outOverview := mustRunDomain(t, root, "domain", "overview")
	if out != outOverview {
		t.Fatalf("domain vs domain overview differ")
	}
	for _, want := range []string{
		"Project domain: " + filepath.Base(root) + "\n",
		"Source: domain.arclint.yaml\n",
		"1 context · 1 aggregate · 1 entity · 2 value objects · 2 invariants · 2 events\n",
		"Context ordering\n  Taking and fulfilling customer orders.\n",
		"  Aggregate Order\n    identity: OrderID\n",
		"      customer-identified  Every Order identifies its Customer.\n        source: missing\n",
		"    OrderPlaced (raised by Order)  An Order has been accepted for processing.\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("overview missing %q:\n%s", want, out)
		}
	}

	// Removing the aggregate takes its members and contracts with it and
	// says so; the event that named it as its raiser keeps its entry.
	wantRmAgg := "" +
		"Removed aggregate Order from the project domain model.\n" +
		"  entity OrderLine removed with it\n" +
		"  invariant customer-identified removed with it\n" +
		"  invariant total-never-negative removed with it\n" +
		"  event OrderPlaced no longer names what raises it\n" +
		"Source files were not changed.\n"
	out = mustRunDomain(t, root, "domain", "remove", "aggregate", "Order")
	if out != wantRmAgg {
		t.Fatalf("remove aggregate:\n got: %q\nwant: %q", out, wantRmAgg)
	}
	out = mustRunDomain(t, root, "domain", "show", "domain_event", "OrderPlaced")
	if strings.Contains(out, "Raised by") {
		t.Fatalf("event should no longer name its raiser:\n%s", out)
	}

	mustRunDomain(t, root, "domain", "define", "value_object", "LegacyOrderID", "--definition", "retired")
	wantRmVO := "" +
		"Removed value object LegacyOrderID from the project domain model.\n" +
		"Source files were not changed.\n"
	out = mustRunDomain(t, root, "domain", "remove", "value_object", "LegacyOrderID")
	if out != wantRmVO {
		t.Fatalf("remove value_object:\n got: %q\nwant: %q", out, wantRmVO)
	}

	out = mustRunDomain(t, root, "domain", "schema")
	if !strings.Contains(out, `"$schema"`) || !strings.Contains(out, "contexts") {
		snippet := out
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		t.Fatalf("schema output does not look like the published schema:\n%s", snippet)
	}
}

func testDomainExitCodes(t *testing.T) {
	root := domainFixture(t)
	mustRunDomain(t, root, "domain", "define", "bounded_context", "ordering", "--definition", "Taking and fulfilling customer orders.")
	mustRunDomain(t, root,
		"domain", "define", "aggregate", "Order",
		"--identity", "OrderID",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
	)

	stdout, stderr, code := runBin(t, root, os.Environ(),
		"domain", "define", "aggregate", "Order",
		"--identity", "OrderID",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
	)
	if code != 0 || stdout != "Unchanged aggregate Order in context ordering.\n" {
		t.Fatalf("unchanged define: exit %d\nstdout: %q\nstderr: %s", code, stdout, stderr)
	}

	_, stderr, code = runBin(t, root, os.Environ(), "domain", "show", "entity", "Missing")
	if code != 1 || !strings.Contains(stderr, `no entity named "Missing"`) {
		t.Fatalf("show missing: exit %d stderr %q", code, stderr)
	}

	_, stderr, code = runBin(t, root, os.Environ(), "domain", "remove", "entity", "Missing")
	if code != 1 || !strings.Contains(stderr, `no entity named "Missing"`) {
		t.Fatalf("remove missing: exit %d stderr %q", code, stderr)
	}

	_, stderr, code = runBin(t, root, os.Environ(), "domain", "show", "widget", "Order")
	if code != 2 {
		t.Fatalf("unknown type: exit %d stderr %q", code, stderr)
	}

	_, stderr, code = runBin(t, root, os.Environ(),
		"domain", "define", "aggregate", "Order",
		"--alias", "X", "--clear-aliases",
	)
	if code != 2 {
		t.Fatalf("alias+clear-aliases: exit %d stderr %q", code, stderr)
	}

	// A first recording that lacks a required property is usage.
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "define", "value_object", "Money")
	if code != 2 || !strings.Contains(stderr, `value object "Money" is not recorded; recording one needs definition`) {
		t.Fatalf("missing definition: exit %d stderr %q", code, stderr)
	}
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "define", "aggregate", "Cart", "--definition", "A cart.")
	if code != 2 || !strings.Contains(stderr, `aggregate "Cart" is not recorded; recording one needs identity`) {
		t.Fatalf("missing identity: exit %d stderr %q", code, stderr)
	}
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "define", "entity", "Customer", "--definition", "A buyer.")
	if code != 2 || !strings.Contains(stderr, `entity "Customer" is not recorded; recording one names the aggregate it belongs to`) {
		t.Fatalf("missing owner: exit %d stderr %q", code, stderr)
	}
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "define", "invariant", "must-hold", "--owner", "Order")
	if code != 2 || !strings.Contains(stderr, `invariant "must-hold" is not recorded; recording one needs statement`) {
		t.Fatalf("missing statement: exit %d stderr %q", code, stderr)
	}

	// A recording the meta-model refuses fails under the invariant's id.
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "define", "value_object", "Order", "--definition", "clash")
	if code != 1 || !strings.Contains(stderr, "ubiquitous_language/one-meaning-per-name") {
		t.Fatalf("one meaning per name: exit %d stderr %q", code, stderr)
	}
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "define", "bounded_context", "Billing", "--definition", "Money in.")
	if code != 1 || !strings.Contains(stderr, `context "Billing"`) {
		t.Fatalf("context spelled like a type: exit %d stderr %q", code, stderr)
	}

	_, stderr, code = runBin(t, root, os.Environ(), "domain", "show", "entity")
	if code != 2 {
		t.Fatalf("missing name: exit %d stderr %q", code, stderr)
	}

	_, stderr, code = runBin(t, root, os.Environ(), "domain", "list", "--format", "yaml")
	if code != 2 {
		t.Fatalf("bad format: exit %d stderr %q", code, stderr)
	}
}

func testDomainJSON(t *testing.T) {
	root := domainFixture(t)
	recordOrdering(t, root)

	stdout := mustRunDomain(t, root, "domain", "overview", "--format", "json")
	var overview struct {
		Source  string `json:"source"`
		Found   bool   `json:"found"`
		Project string `json:"project"`
		Counts  struct {
			Contexts     int `json:"contexts"`
			Aggregates   int `json:"aggregates"`
			Entities     int `json:"entities"`
			ValueObjects int `json:"valueObjects"`
			Invariants   int `json:"invariants"`
			Events       int `json:"events"`
		} `json:"counts"`
		Contexts []struct {
			Name       string `json:"name"`
			Definition string `json:"definition"`
			Aggregates []struct {
				Name     string `json:"name"`
				Identity string `json:"identity"`
				Aliases  []string
				Entities []struct {
					Name string `json:"name"`
				} `json:"entities"`
				Invariants []struct {
					Key       string `json:"key"`
					Statement string `json:"statement"`
				} `json:"invariants"`
			} `json:"aggregates"`
			ValueObjects []map[string]any `json:"valueObjects"`
			Events       []struct {
				Name     string `json:"name"`
				RaisedBy string `json:"raisedBy"`
			} `json:"events"`
		} `json:"contexts"`
	}
	if err := json.Unmarshal([]byte(stdout), &overview); err != nil {
		t.Fatalf("overview json: %v\n%s", err, stdout)
	}
	if overview.Source != "domain.arclint.yaml" || !overview.Found || overview.Project != filepath.Base(root) {
		t.Fatalf("overview envelope: %+v", overview)
	}
	if overview.Counts.Contexts != 1 || overview.Counts.Aggregates != 1 || overview.Counts.Entities != 1 ||
		overview.Counts.ValueObjects != 2 || overview.Counts.Invariants != 2 || overview.Counts.Events != 2 {
		t.Fatalf("overview counts: %+v", overview.Counts)
	}
	if len(overview.Contexts) != 1 || overview.Contexts[0].Name != "ordering" {
		t.Fatalf("overview contexts: %+v", overview.Contexts)
	}
	aggs := overview.Contexts[0].Aggregates
	if len(aggs) != 1 || aggs[0].Name != "Order" || aggs[0].Identity != "OrderID" ||
		len(aggs[0].Entities) != 1 || aggs[0].Entities[0].Name != "OrderLine" ||
		len(aggs[0].Invariants) != 2 || aggs[0].Invariants[0].Key != "customer-identified" {
		t.Fatalf("overview aggregates: %+v", aggs)
	}
	if len(overview.Contexts[0].Events) != 2 || overview.Contexts[0].Events[0].RaisedBy != "Order" {
		t.Fatalf("overview events: %+v", overview.Contexts[0].Events)
	}

	stdout = mustRunDomain(t, root, "domain", "list", "--format", "json")
	var listing map[string]any
	if err := json.Unmarshal([]byte(stdout), &listing); err != nil {
		t.Fatalf("list json: %v\n%s", err, stdout)
	}
	contexts, ok := listing["contexts"].([]any)
	if !ok || len(contexts) != 1 {
		t.Fatalf("list contexts: %+v", listing)
	}

	stdout = mustRunDomain(t, root, "domain", "list", "invariants", "--format", "json")
	if err := json.Unmarshal([]byte(stdout), &listing); err != nil {
		t.Fatalf("list invariants json: %v\n%s", err, stdout)
	}
	if listing["listing"] != "invariants" {
		t.Fatalf("list invariants json names its listing: %+v", listing)
	}
	contexts, _ = listing["contexts"].([]any)
	if len(contexts) != 1 {
		t.Fatalf("list invariants json contexts: %+v", listing)
	}
	invariants, _ := contexts[0].(map[string]any)["invariants"].([]any)
	if len(invariants) != 2 {
		t.Fatalf("list invariants json: %+v", contexts[0])
	}
	if first, _ := invariants[0].(map[string]any); first["key"] != "customer-identified" || first["owner"] != "Order" {
		t.Fatalf("list invariants json first entry: %+v", invariants[0])
	}

	stdout = mustRunDomain(t, root, "domain", "show", "entity", "OrderLine", "--format", "json")
	var show map[string]any
	if err := json.Unmarshal([]byte(stdout), &show); err != nil {
		t.Fatalf("show json: %v\n%s", err, stdout)
	}
	if show["type"] != "entity" || show["name"] != "OrderLine" || show["owner"] != "Order" || show["context"] != "ordering" {
		t.Fatalf("show entity json: %+v", show)
	}

	stdout = mustRunDomain(t, root, "domain", "show", "value_object", "Money", "--format", "json")
	if err := json.Unmarshal([]byte(stdout), &show); err != nil {
		t.Fatalf("show vo json: %v\n%s", err, stdout)
	}
	if invs, ok := show["invariants"].([]any); !ok || len(invs) != 0 {
		t.Fatalf("show value_object json carries an empty invariants list, not null: %+v", show)
	}

	stdout = mustRunDomain(t, root,
		"domain", "define", "aggregate", "Order",
		"--identity", "OrderID",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
		"--format", "json",
	)
	var defRes map[string]any
	if err := json.Unmarshal([]byte(stdout), &defRes); err != nil {
		t.Fatalf("define json: %v\n%s", err, stdout)
	}
	if defRes["result"] != "unchanged" || defRes["type"] != "aggregate" || defRes["context"] != "ordering" {
		t.Fatalf("define unchanged json: %+v", defRes)
	}

	stdout = mustRunDomain(t, root,
		"domain", "define", "entity", "Customer",
		"--owner", "Order",
		"--definition", "A person or organization that places Orders.",
		"--format", "json",
	)
	if err := json.Unmarshal([]byte(stdout), &defRes); err != nil {
		t.Fatalf("define create json: %v\n%s", err, stdout)
	}
	if defRes["result"] != "created" || defRes["name"] != "Customer" || defRes["owner"] != "Order" {
		t.Fatalf("define create json: %+v", defRes)
	}
	values, _ := defRes["values"].(map[string]any)
	if values["definition"] != "A person or organization that places Orders." {
		t.Fatalf("define create json values: %+v", defRes)
	}

	stdout = mustRunDomain(t, root, "domain", "remove", "aggregate", "Order", "--format", "json")
	var rm map[string]any
	if err := json.Unmarshal([]byte(stdout), &rm); err != nil {
		t.Fatalf("remove aggregate json: %v\n%s", err, stdout)
	}
	if rm["type"] != "aggregate" || rm["name"] != "Order" || rm["result"] != "removed" || rm["sourceFilesChanged"] != false {
		t.Fatalf("remove aggregate json: %+v", rm)
	}
	also, _ := rm["also"].([]any)
	if len(also) != 5 || also[0] != "entity OrderLine removed with it" {
		t.Fatalf("remove aggregate json consequences: %+v", rm["also"])
	}

	stdout = mustRunDomain(t, root, "domain", "remove", "value_object", "Money", "--format", "json")
	var rmVO map[string]any
	if err := json.Unmarshal([]byte(stdout), &rmVO); err != nil {
		t.Fatalf("remove vo json: %v\n%s", err, stdout)
	}
	if rmVO["type"] != "value_object" || rmVO["name"] != "Money" || rmVO["result"] != "removed" || rmVO["sourceFilesChanged"] != false {
		t.Fatalf("remove vo json: %+v", rmVO)
	}
	if _, ok := rmVO["also"]; ok {
		t.Fatalf("remove vo json carries consequences it has none of: %+v", rmVO)
	}
}

func testDomainCommentPreservation(t *testing.T) {
	root := domainFixture(t)
	write(t, root, "domain.arclint.yaml", `# project domain model
version: 1
project: shop
contexts:
  ordering:
    definition: Taking and fulfilling customer orders.
    aggregates:
      # the consistency boundary for purchases
      Order: # primary aggregate
        definition: A customer's request to purchase products.
        identity: OrderID
`)
	mustRunDomain(t, root,
		"domain", "define", "value_object", "Money",
		"--definition", "A monetary amount expressed in a particular currency.",
	)
	after, err := os.ReadFile(filepath.Join(root, "domain.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(after)
	for _, want := range []string{
		"# project domain model\n",
		"      # the consistency boundary for purchases\n",
		"      Order: # primary aggregate\n",
		"    value_objects:\n      Money:\n        definition: A monetary amount expressed in a particular currency.\n",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("file lacks %q after define:\n%s", want, body)
		}
	}
	if iOrder, iMoney := strings.Index(body, "Order:"), strings.Index(body, "Money:"); iOrder < 0 || iMoney < 0 || iOrder > iMoney {
		t.Fatalf("entry order disturbed:\n%s", body)
	}
}

func testDomainGuided(t *testing.T) {
	root := domainFixture(t)
	mustRunDomain(t, root, "domain", "define", "bounded_context", "ordering", "--definition", "Taking and fulfilling customer orders.")
	mustRunDomain(t, root, "domain", "define", "aggregate", "Order", "--identity", "OrderID", "--definition", "A customer's request to purchase products.")

	// 3=Entity, context, owning aggregate, name, definition, aliases, confirm.
	stdin := strings.Join([]string{
		"3",
		"ordering",
		"Order",
		"OrderLine",
		"One product on the order.",
		"Line, Item",
		"y",
		"",
	}, "\n")
	stdout, stderr, code := runBinStdin(t, root, os.Environ(), stdin, "domain", "define", "--guided")
	if code != 0 {
		t.Fatalf("guided yes: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	for _, want := range []string{
		"What are you defining?\n  1) Bounded Context\n  2) Aggregate\n  3) Entity\n",
		"Bounded context:\n",
		"Aggregate it belongs to:\n",
		"Proposed definition:\n  Entity: OrderLine\n  Owner: Order\n  Definition: One product on the order.\n  Aliases: Line, Item\n",
		"Write this definition to domain.arclint.yaml? [y/N]\n",
		"Defined entity OrderLine under Order in context ordering.\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("guided stdout missing %q:\n%s", want, stdout)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "domain.arclint.yaml"))
	if err != nil {
		t.Fatalf("guided write missing file: %v", err)
	}
	if !strings.Contains(string(data), "        entities:\n          OrderLine:\n            definition: One product on the order.\n            aliases: [Line, Item]\n") {
		t.Fatalf("guided file content:\n%s", data)
	}

	// The concept may be named instead of numbered; a decline writes
	// nothing, and a project with no file gets none.
	rootNo := domainFixture(t)
	stdinNo := strings.Join([]string{
		"Bounded Context",
		"ordering",
		"Taking and fulfilling customer orders.",
		"n",
		"",
	}, "\n")
	stdout, stderr, code = runBinStdin(t, rootNo, os.Environ(), stdinNo, "domain", "define", "--guided")
	if code != 0 {
		t.Fatalf("guided no: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Proposed definition:\n  Bounded Context: ordering\n") || !strings.HasSuffix(stdout, "Nothing written.\n") {
		t.Fatalf("guided decline:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(rootNo, "domain.arclint.yaml")); !os.IsNotExist(err) {
		t.Fatalf("guided decline must not create domain.arclint.yaml, err=%v", err)
	}

	// Exhausted input aborts without writing.
	stdout, stderr, code = runBinStdin(t, rootNo, os.Environ(), "2\nordering\n", "domain", "define", "--guided")
	if code != 1 || !strings.Contains(stderr, "guided authoring aborted") {
		t.Fatalf("guided abort: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	_, stderr, code = runBin(t, root, os.Environ(), "domain", "define", "--guided", "entity", "X")
	if code != 2 || !strings.Contains(stderr, "--guided cannot be combined") {
		t.Fatalf("guided with arguments: exit %d stderr %q", code, stderr)
	}
}

func testDomainSchema(t *testing.T) {
	root := domainFixture(t)
	stdout := mustRunDomain(t, root, "domain", "schema")
	committed, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(domainSchemaLitmus)))
	if err != nil {
		t.Fatalf("read committed schema: %v", err)
	}
	if stdout != string(committed) {
		t.Fatalf("domain schema output differs from %s (%d vs %d bytes)",
			domainSchemaLitmus, len(stdout), len(committed))
	}

	// --write lands the schema under the project's schema directory by
	// default and reports the write; a second run reports it unchanged.
	out := mustRunDomain(t, root, "domain", "schema", "--write")
	written, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(domainSchemaProjectPath)))
	if err != nil {
		t.Fatalf("domain schema --write did not create %s: %v", domainSchemaProjectPath, err)
	}
	if string(written) != string(committed) {
		t.Fatalf("domain schema --write bytes differ from %s", domainSchemaLitmus)
	}
	if !strings.Contains(out, "domain.arclint.schema.json") {
		t.Fatalf("domain schema --write did not report the written path:\n%s", out)
	}
	if _, stderr, code := runBin(t, root, os.Environ(), "domain", "schema", "--dir", "elsewhere"); code == 0 {
		t.Fatalf("domain schema --dir without --write must fail; stderr: %s", stderr)
	}
	out = mustRunDomain(t, root, "domain", "schema", "--write", "--dir", "elsewhere")
	if _, err := os.Stat(filepath.Join(root, "elsewhere", "domain.arclint.schema.json")); err != nil {
		t.Fatalf("domain schema --write --dir elsewhere did not create the file: %v", err)
	}
	if !strings.Contains(out, "elsewhere") {
		t.Fatalf("domain schema --write --dir did not report the written path:\n%s", out)
	}

	// A file initialized after the schema is written points at the
	// local copy, so editors validate offline.
	mustRunDomain(t, root, "domain", "init")
	model, err := os.ReadFile(filepath.Join(root, "domain.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(model), "# yaml-language-server: $schema="+domainSchemaProjectPath+"\n") {
		t.Fatalf("initialized model does not point at the local schema:\n%s", model)
	}
}

func testDomainHelp(t *testing.T) {
	root := domainFixture(t)
	stdout, stderr, code := runBin(t, root, os.Environ(), "--help")
	if code != 0 {
		t.Fatalf("--help exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "domain") || !strings.Contains(stdout, "inspect and maintain the project's ubiquitous language") {
		t.Fatalf("top-level help missing domain entry:\n%s", stdout)
	}
	stdout, stderr, code = runBin(t, root, os.Environ(), "domain", "--help")
	if code != 0 {
		t.Fatalf("domain --help exit %d\nstderr: %s", code, stderr)
	}
	for _, sub := range []string{"init", "overview", "list", "show", "explain", "define", "remove", "schema"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("domain --help missing %q", sub)
		}
	}
	if !strings.Contains(stdout, "Running arclint domain without a subcommand") {
		t.Error("domain --help missing Long prose")
	}
	stdout, stderr, code = runBin(t, root, os.Environ(), "domain", "remove", "--help")
	if code != 0 {
		t.Fatalf("domain remove --help exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "rm") {
		t.Errorf("remove --help missing rm alias:\n%s", stdout)
	}
	stdout, stderr, code = runBin(t, root, os.Environ(), "domain", "define", "--help")
	if code != 0 {
		t.Fatalf("domain define --help exit %d\nstderr: %s", code, stderr)
	}
	for _, flag := range []string{"--identity", "--owner", "--statement", "--on", "--repository", "--factory", "--raised-by", "--guided"} {
		if !strings.Contains(stdout, flag) {
			t.Errorf("define --help missing %s:\n%s", flag, stdout)
		}
	}
	for _, cmd := range [][]string{
		{"domain", "init", "--help"},
		{"domain", "overview", "--help"},
		{"domain", "define", "--help"},
		{"domain", "show", "--help"},
	} {
		stdout, stderr, code = runBin(t, root, os.Environ(), cmd...)
		if code != 0 {
			t.Fatalf("%v exit %d\nstderr: %s", cmd, code, stderr)
		}
		if !strings.Contains(stdout, "Examples") || !strings.Contains(strings.ToLower(stdout), "arclint domain") {
			t.Errorf("%v help missing examples:\n%s", cmd, stdout)
		}
	}
}

func testDomainExclusions(t *testing.T) {
	root := domainFixture(t)
	for _, args := range [][]string{
		{"entities"},
		{"aggregates"},
		{"add"},
		{"edit"},
		{"missing"},
		{"domain", "check"},
		{"domain", "get"},
		{"domain", "describe"},
		{"domain", "apply"},
		{"domain", "delete"},
	} {
		stdout, stderr, code := runBin(t, root, os.Environ(), args...)
		if code == 0 {
			t.Errorf("%v: exit 0, want nonzero unknown-command\nstdout: %s", args, stdout)
			continue
		}
		msg := stdout + stderr
		if !strings.Contains(strings.ToLower(msg), "unknown") &&
			!strings.Contains(msg, "unknown command") &&
			!strings.Contains(msg, "Error") {
			t.Errorf("%v: exit %d without unknown-command signal\n%s", args, code, msg)
		}
	}
	for _, args := range [][]string{
		{"domain", "define", "entity", "X", "--path", "y"},
		{"domain", "define", "entity", "X", "--entity", "y"},
		{"domain", "define", "entity", "X", "--type", "y"},
		{"domain", "define", "entity", "X", "--aggregate"},
	} {
		_, stderr, code := runBin(t, root, os.Environ(), args...)
		if code == 0 {
			t.Errorf("%v: exit 0, want unknown-flag failure", args)
			continue
		}
		if !strings.Contains(stderr, "unknown flag") && !strings.Contains(stderr, "unknown shorthand") {
			t.Errorf("%v: exit %d stderr %q (want unknown flag)", args, code, stderr)
		}
	}
}

func testDomainRemoveLeavesSources(t *testing.T) {
	root := domainFixture(t)
	write(t, root, "order.go", "package main\n// stray source the remove command must not touch\n")
	before, err := os.ReadFile(filepath.Join(root, "order.go"))
	if err != nil {
		t.Fatal(err)
	}
	mustRunDomain(t, root, "domain", "define", "bounded_context", "ordering", "--definition", "Taking and fulfilling customer orders.")
	mustRunDomain(t, root, "domain", "define", "aggregate", "Order", "--identity", "OrderID", "--definition", "A customer's request to purchase products.")
	mustRunDomain(t, root, "domain", "remove", "aggregate", "Order")
	after, err := os.ReadFile(filepath.Join(root, "order.go"))
	if err != nil {
		t.Fatalf("order.go disappeared: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("remove mutated order.go\nbefore: %q\nafter: %q", before, after)
	}
}

func testDomainExtensionAccess(t *testing.T) {
	// A consuming extension rule reads the recorded model through
	// ctx.domain() and reports what it finds.
	root := t.TempDir()
	write(t, root, ".arclint/extensions/domain-probe.ts", `
import { defineRule } from "arclint";
export default defineRule({
  type: "domain-probe",
  check(ctx) {
    const domain = ctx.domain();
    for (const bound of domain.contexts) {
      for (const aggregate of bound.aggregates) {
        ctx.report({
          path: "src/ok.go",
          line: 1,
          message: "aggregate:" + aggregate.name + "/" + aggregate.identity,
        });
        for (const entity of aggregate.entities) {
          ctx.report({ path: "src/ok.go", line: 1, message: "entity:" + entity.name });
        }
        for (const invariant of aggregate.invariants) {
          ctx.report({ path: "src/ok.go", line: 1, message: "invariant:" + invariant.key });
        }
      }
      for (const event of bound.events) {
        ctx.report({ path: "src/ok.go", line: 1, message: "event:" + event.name + "<-" + (event.raisedBy || "") });
      }
    }
  },
});
`)
	write(t, root, "rules.arclint.yaml", `runtime: [go]
zones:
  src: src/**
rules:
  src/domain-probe:
    on: src
    files: "src/**/*.go"
    uses: domain-probe
`)
	write(t, root, "src/ok.go", "package src\n")
	write(t, root, "domain.arclint.yaml", orderingDomain)

	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	if code != 1 {
		t.Fatalf("check with domain-probe: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var diagnostics []diagnosticDoc
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatalf("check json: %v\n%s", err, stdout)
	}
	var msgs []string
	for _, d := range diagnostics {
		if d.RuleID == "src/domain-probe" && d.Status == "active" {
			msgs = append(msgs, d.Message)
		}
	}
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{
		"aggregate:Order/OrderID",
		"entity:OrderLine",
		"invariant:customer-identified",
		"invariant:total-never-negative",
		"event:OrderPlaced<-Order",
		"event:OrderCancelled<-",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("domain-probe findings missing %q: %v\nall: %+v", want, msgs, diagnostics)
		}
	}
}

// The recorded model is judged by the built-in rules against the
// whole repository: a bounded context locates its code from the
// declarations that spell its recorded terms, with no Zone required.
// Nothing implements the model yet, so every declaration-backed rule
// finds nothing to anchor to; recording the code closes each finding.
func testDomainBuiltInRules(t *testing.T) {
	root := domainFixture(t)
	write(t, root, "domain.arclint.yaml", orderingDomain)

	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	if code != 1 {
		t.Fatalf("check with no code implementing the model: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var diagnostics []diagnosticDoc
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatalf("check json: %v\n%s", err, stdout)
	}
	rules := map[string]bool{}
	for _, d := range diagnostics {
		rules[d.RuleID] = true
		if d.Path != "domain.arclint.yaml" {
			t.Errorf("a finding about a term with no declaration anchors in the domain file, got %+v", d)
		}
	}
	for _, want := range []string{
		"aggregate/root-declared",
		"ubiquitous_language/terms-declared-in-code",
		"domain_event/declared",
	} {
		if !rules[want] {
			t.Errorf("check with no code lacks a %s finding:\n%s", want, stdout)
		}
	}

	// The code that speaks the language: the root and its constructor
	// enforcing every invariant, the value objects, and the events.
	write(t, root, "src/order/order.go", `package order

import "errors"

// OrderID identifies an Order.
type OrderID string

// Money is an amount in whole cents.
type Money struct{ cents int64 }

// NewMoney constructs Money.
func NewMoney(cents int64) (Money, error) {
	if cents < 0 {
		return Money{}, errors.New("money is never negative")
	}
	return Money{cents: cents}, nil
}

// OrderLine is one product on the order.
type OrderLine struct {
	product string
	price   Money
}

// Order is the aggregate root.
type Order struct {
	id       OrderID
	customer string
	lines    []OrderLine
	total    Money
}

// NewOrder builds an Order for a customer.
func NewOrder(id OrderID, customer string) (*Order, error) {
	o := &Order{id: id, customer: customer}
	if err := o.EnsureCustomerIdentified(); err != nil {
		return nil, err
	}
	if err := o.EnsureTotalNeverNegative(); err != nil {
		return nil, err
	}
	return o, nil
}

// Add puts a line on the order.
func (o *Order) Add(line OrderLine) error {
	o.lines = append(o.lines, line)
	o.total.cents += line.price.cents
	if err := o.EnsureCustomerIdentified(); err != nil {
		return err
	}
	return o.EnsureTotalNeverNegative()
}

// EnsureCustomerIdentified enforces customer-identified.
func (o *Order) EnsureCustomerIdentified() error {
	if o.customer == "" {
		return errors.New("every Order identifies its Customer")
	}
	return nil
}

// EnsureTotalNeverNegative enforces total-never-negative.
func (o *Order) EnsureTotalNeverNegative() error {
	if o.total.cents < 0 {
		return errors.New("an Order's total is never negative")
	}
	return nil
}

// OrderPlaced is raised when an order is accepted.
type OrderPlaced struct{ ID OrderID }

// OrderCancelled is raised when an order is cancelled.
type OrderCancelled struct{ ID OrderID }
`)
	stdout, stderr, code = runBin(t, root, os.Environ(), "check")
	if code != 0 {
		t.Fatalf("check with the model implemented must be clean: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "0 active finding(s)") {
		t.Fatalf("clean check summary:\n%s", stdout)
	}

	// The overview and the worksite context now anchor each invariant
	// at the method that enforces it.
	stdout = mustRunDomain(t, root, "domain", "overview")
	for _, want := range []string{
		"      customer-identified  Every Order identifies its Customer.\n        source: src/order/order.go:",
		"      total-never-negative  An Order's total is never negative.\n        source: src/order/order.go:",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("overview lacks %q:\n%s", want, stdout)
		}
	}
	stdout = mustRunDomain(t, root, "context", "src/order/order.go")
	if !strings.Contains(stdout, "      customer-identified (Order): Every Order identifies its Customer. src/order/order.go:") {
		t.Fatalf("context lacks the invariant anchor:\n%s", stdout)
	}
	if strings.Contains(stdout, "missing") {
		t.Fatalf("context reports a missing anchor on an implemented model:\n%s", stdout)
	}

	// Removing the enforcing method is caught at the root's declaration.
	source, err := os.ReadFile(filepath.Join(root, "src/order/order.go"))
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(source), "\tif err := o.EnsureTotalNeverNegative(); err != nil {\n\t\treturn nil, err\n\t}\n", "", 1)
	if broken == string(source) {
		t.Fatal("fixture edit did not apply")
	}
	write(t, root, "src/order/order.go", broken)
	stdout, _, code = runBin(t, root, os.Environ(), "check", "--format", "json")
	if code != 1 {
		t.Fatalf("check with an unenforced invariant: exit %d\n%s", code, stdout)
	}
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatalf("check json: %v\n%s", err, stdout)
	}
	if len(diagnostics) != 1 || diagnostics[0].RuleID != "invariant/enforced-at-every-mutation" ||
		diagnostics[0].Path != "src/order/order.go" || !strings.Contains(diagnostics[0].Message, "total-never-negative") {
		t.Fatalf("unenforced invariant finding: %+v", diagnostics)
	}
}

func testDomainContext(t *testing.T) {
	// Absent model -> no project-domain block / no domain JSON key.
	absent := domainFixture(t)
	stdout, stderr, code := runBin(t, absent, os.Environ(), "context")
	if code != 0 {
		t.Fatalf("context without model: exit %d\nstderr: %s", code, stderr)
	}
	if strings.Contains(stdout, "project domain") || strings.Contains(stdout, "domain.arclint.yaml") {
		t.Fatalf("context text leaked domain block without model:\n%s", stdout)
	}
	stdout, stderr, code = runBin(t, absent, os.Environ(), "context", "--format", "json")
	if code != 0 {
		t.Fatalf("context json without model: exit %d\nstderr: %s", code, stderr)
	}
	var bare map[string]any
	if err := json.Unmarshal([]byte(stdout), &bare); err != nil {
		t.Fatalf("context json: %v\n%s", err, stdout)
	}
	if _, ok := bare["domain"]; ok {
		t.Fatalf("context json with bare model should omit domain: %+v", bare)
	}

	// Present model -> text block + json domain key with contexts.
	root := domainFixture(t)
	write(t, root, "domain.arclint.yaml", orderingDomain)
	stdout, stderr, code = runBin(t, root, os.Environ(), "context")
	if code != 0 {
		t.Fatalf("context with model: exit %d\nstderr: %s", code, stderr)
	}
	for _, want := range []string{
		"project domain (domain.arclint.yaml): 1 context · 1 aggregate · 1 entity · 2 value objects · 2 invariants · 2 events\n",
		"  context ordering:\n",
		"    aggregates: Order (OrderID; OrderLine)\n",
		"    value objects: OrderID, Money\n",
		"    invariants:\n      customer-identified (Order): Every Order identifies its Customer. missing\n",
		"    events: OrderPlaced, OrderCancelled\n",
		"  unanchored contracts: 2 missing\n",
		"    missing: invariant customer-identified of Order (context ordering)\n      expected method EnsureCustomerIdentified on Order\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("context text missing %q:\n%s", want, stdout)
		}
	}
	stdout, stderr, code = runBin(t, root, os.Environ(), "context", "--format", "json")
	if code != 0 {
		t.Fatalf("context json with model: exit %d\nstderr: %s", code, stderr)
	}
	var with map[string]any
	if err := json.Unmarshal([]byte(stdout), &with); err != nil {
		t.Fatalf("context json: %v\n%s", err, stdout)
	}
	domain, ok := with["domain"].(map[string]any)
	if !ok {
		t.Fatalf("context json missing domain object: %+v", with)
	}
	if domain["source"] != "domain.arclint.yaml" {
		t.Fatalf("domain.source = %v", domain["source"])
	}
	counts, _ := domain["counts"].(map[string]any)
	if counts["aggregates"] != 1.0 || counts["entities"] != 1.0 || counts["invariants"] != 2.0 {
		t.Fatalf("domain.counts = %+v", counts)
	}
	contexts, ok := domain["contexts"].([]any)
	if !ok || len(contexts) != 1 {
		t.Fatalf("domain.contexts = %+v", domain["contexts"])
	}
	ctx0, _ := contexts[0].(map[string]any)
	aggs, _ := ctx0["aggregates"].([]any)
	if len(aggs) != 1 {
		t.Fatalf("domain.contexts[0].aggregates = %+v", aggs)
	}
	agg0, _ := aggs[0].(map[string]any)
	if agg0["name"] != "Order" || agg0["identity"] != "OrderID" {
		t.Fatalf("domain aggregate ref = %+v", agg0)
	}
	invs, _ := ctx0["invariants"].([]any)
	if len(invs) != 2 {
		t.Fatalf("domain.contexts[0].invariants = %+v", invs)
	}
	if inv0, _ := invs[0].(map[string]any); inv0["key"] != "customer-identified" || inv0["owner"] != "Order" || inv0["anchor"] != "missing" {
		t.Fatalf("domain invariant ref = %+v", invs[0])
	}
	unanchored, _ := domain["unanchored"].([]any)
	if len(unanchored) != 2 {
		t.Fatalf("domain.unanchored = %+v", domain["unanchored"])
	}
}

func testDomainAmbiguity(t *testing.T) {
	root := domainFixture(t)
	write(t, root, "domain.arclint.yaml", `version: 1
project: shop
contexts:
  ordering:
    definition: Taking orders.
    aggregates:
      Order:
        definition: ordering order
        identity: OrderID
  billing:
    definition: Collecting payment.
    aggregates:
      Order:
        definition: billing order
        identity: InvoiceID
`)
	_, stderr, code := runBin(t, root, os.Environ(), "domain", "show", "aggregate", "Order")
	if code != 2 || !strings.Contains(stderr, `aggregate "Order" is recorded in multiple contexts (ordering, billing); pass --context`) {
		t.Fatalf("ambiguous show: exit %d stderr %q", code, stderr)
	}
	stdout := mustRunDomain(t, root, "domain", "show", "aggregate", "Order", "--context", "billing")
	if !strings.Contains(stdout, "Context: billing\n") || !strings.Contains(stdout, "billing order") {
		t.Fatalf("explicit context show:\n%s", stdout)
	}

	// define without --context when multiple contexts → usage
	_, stderr, code = runBin(t, root, os.Environ(),
		"domain", "define", "value_object", "Money",
		"--definition", "An amount.",
	)
	if code != 2 || !strings.Contains(stderr, "--context is required when the project records several bounded contexts (ordering, billing)") {
		t.Fatalf("define without context: exit %d stderr %q", code, stderr)
	}

	// A listing scoped to one context shows that context alone.
	stdout = mustRunDomain(t, root, "domain", "list", "--context", "billing")
	if strings.Contains(stdout, "Context ordering") || !strings.Contains(stdout, "Context billing\n") {
		t.Fatalf("list --context billing:\n%s", stdout)
	}
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "list", "--context", "shipping")
	if code != 2 || !strings.Contains(stderr, `unknown context "shipping"; recorded: ordering, billing`) {
		t.Fatalf("list unknown context: exit %d stderr %q", code, stderr)
	}
}
