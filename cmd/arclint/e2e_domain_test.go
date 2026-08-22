package main

// End-to-end coverage of the arclint domain command family against the
// compiled binary. Assertions follow docs/domain-cli-recommendation.md
// (help text, example blocks, exit table) and the Project Domain Model plan.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalDomainRules is a ruleset that resolves a project root so
// ubiquitous-language.yaml is found/created beside rules.yaml.
const minimalDomainRules = `runtime: [go]
modules:
  src:
    paths: ["src/**"]
`

func domainFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "rules.yaml", minimalDomainRules)
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

func TestDomainCommandFamily(t *testing.T) {
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
	t.Run("context", testDomainContext)
}

func testDomainResolution(t *testing.T) {
	root := domainFixture(t)

	// Missing model is a soft empty state (exit 0), not a failure.
	stdout, stderr, code := runBin(t, root, os.Environ(), "domain")
	if code != 0 {
		t.Fatalf("domain with no model: exit %d\nstderr: %s", code, stderr)
	}
	wantMissing := "" +
		"No recorded Ubiquitous Language found at ubiquitous-language.yaml.\n" +
		"\n" +
		"Define one item:\n" +
		"  arclint domain define entity <name> --definition <text>\n" +
		"\n" +
		"Start guided authoring:\n" +
		"  arclint domain define --guided\n"
	if stdout != wantMissing {
		t.Fatalf("missing-model overview:\n got: %q\nwant: %q", stdout, wantMissing)
	}

	mustRunDomain(t, root,
		"domain", "define", "entity", "Order",
		"--definition", "A customer's request to purchase products.",
	)
	modelPath := filepath.Join(root, "ubiquitous-language.yaml")
	if _, err := os.Stat(modelPath); err != nil {
		t.Fatalf("define must create ubiquitous-language.yaml beside rules.yaml: %v", err)
	}

	// --rules from an unrelated cwd resolves the model beside that rules file.
	other := t.TempDir()
	stdout, stderr, code = runBin(t, other, os.Environ(),
		"--rules", filepath.Join(root, "rules.yaml"),
		"domain", "show", "entity", "Order",
	)
	if code != 0 {
		t.Fatalf("--rules domain show: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Entity: Order") {
		t.Fatalf("--rules show missed Order:\n%s", stdout)
	}
}

func testDomainFullFlow(t *testing.T) {
	root := domainFixture(t)

	// Complete examples: define Entity + Aggregate designation.
	out := mustRunDomain(t, root,
		"domain", "define", "entity", "Order",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
	)
	if out != "Defined entity Order.\n" {
		t.Fatalf("define entity create:\n got: %q\nwant: %q", out, "Defined entity Order.\n")
	}
	out = mustRunDomain(t, root, "domain", "define", "aggregate", "Order")
	if !strings.HasPrefix(out, "Updated aggregate Order.\n") || !strings.Contains(out, "aggregate: designated") {
		t.Fatalf("define aggregate designation:\n%s", out)
	}

	// Supporting Value Objects, Business Rules, Domain Events.
	for _, args := range [][]string{
		{"domain", "define", "value-object", "OrderID", "--definition", "The stable identity of an Order."},
		{"domain", "define", "value-object", "Money", "--definition", "A monetary amount expressed in a particular currency."},
		{"domain", "define", "business-rule", "OrderMustHaveCustomer", "--definition", "Every Order must identify its Customer."},
		{"domain", "define", "business-rule", "OrderTotalCannotBeNegative", "--definition", "An Order's total monetary amount cannot be negative."},
		{"domain", "define", "event", "OrderPlaced", "--definition", "An Order has been accepted for processing."},
		{"domain", "define", "event", "OrderCancelled", "--definition", "An Order has been cancelled and will not be processed."},
	} {
		out = mustRunDomain(t, root, args...)
		if !strings.HasPrefix(out, "Defined ") {
			t.Fatalf("%v: got %q", args, out)
		}
	}

	// show entity Order — exact recommendation example block.
	wantShow := "" +
		"Entity: Order\n" +
		"Aggregate: yes\n" +
		"Definition: A customer's request to purchase products.\n" +
		"Aliases:\n" +
		"  Purchase Order\n"
	out = mustRunDomain(t, root, "domain", "show", "entity", "Order")
	if out != wantShow {
		t.Fatalf("show entity Order:\n got: %q\nwant: %q", out, wantShow)
	}

	out = mustRunDomain(t, root, "domain", "show", "business-rule", "OrderMustHaveCustomer")
	if !strings.Contains(out, "Business Rule: OrderMustHaveCustomer") ||
		!strings.Contains(out, "Every Order must identify its Customer.") {
		t.Fatalf("show business-rule:\n%s", out)
	}

	// list aggregates — designated entities only.
	wantListAgg := "" +
		"Project domain\n" +
		"\n" +
		"Aggregates\n" +
		"  Order\n"
	out = mustRunDomain(t, root, "domain", "list", "aggregates")
	if out != wantListAgg {
		t.Fatalf("list aggregates:\n got: %q\nwant: %q", out, wantListAgg)
	}

	// list (all) — names only, sorted within groups, aggregates marked.
	wantList := "" +
		"Project domain\n" +
		"\n" +
		"Entities\n" +
		"  Order [aggregate]\n" +
		"\n" +
		"Value objects\n" +
		"  Money\n" +
		"  OrderID\n" +
		"\n" +
		"Business rules\n" +
		"  OrderMustHaveCustomer\n" +
		"  OrderTotalCannotBeNegative\n" +
		"\n" +
		"Domain events\n" +
		"  OrderCancelled\n" +
		"  OrderPlaced\n"
	out = mustRunDomain(t, root, "domain", "list")
	if out != wantList {
		t.Fatalf("list all:\n got: %q\nwant: %q", out, wantList)
	}

	// overview — counts + sections for the complete-examples model.
	// Singular/plural and interpunct separators match the recommendation.
	// File order is preserved (OrderID before Money; OrderPlaced before OrderCancelled).
	wantOverview := "" +
		"Project domain\n" +
		"Source: ubiquitous-language.yaml\n" +
		"\n" +
		"1 Entity · 1 Aggregate · 2 Value Objects · 2 Business Rules · 2 Events\n" +
		"\n" +
		"Aggregate\n" +
		"\n" +
		"  Order\n" +
		"  A customer's request to purchase products.\n" +
		"\n" +
		"Business rules\n" +
		"\n" +
		"  OrderMustHaveCustomer\n" +
		"  Every Order must identify its Customer.\n" +
		"\n" +
		"  OrderTotalCannotBeNegative\n" +
		"  An Order's total monetary amount cannot be negative.\n" +
		"\n" +
		"Value objects\n" +
		"\n" +
		"  OrderID — The stable identity of an Order.\n" +
		"  Money   — A monetary amount expressed in a particular currency.\n" +
		"\n" +
		"Domain events\n" +
		"\n" +
		"  OrderPlaced    — An Order has been accepted for processing.\n" +
		"  OrderCancelled — An Order has been cancelled and will not be processed.\n"
	out = mustRunDomain(t, root, "domain")
	outOverview := mustRunDomain(t, root, "domain", "overview")
	if out != outOverview {
		t.Fatalf("domain vs domain overview differ:\n domain: %q\n overview: %q", out, outOverview)
	}
	if out != wantOverview {
		t.Fatalf("overview:\n got: %q\nwant: %q", out, wantOverview)
	}

	// remove aggregate preserves the Entity.
	wantRmAgg := "" +
		"Removed the Aggregate designation from Order.\n" +
		"The Order Entity remains defined.\n"
	out = mustRunDomain(t, root, "domain", "remove", "aggregate", "Order")
	if out != wantRmAgg {
		t.Fatalf("remove aggregate:\n got: %q\nwant: %q", out, wantRmAgg)
	}
	out = mustRunDomain(t, root, "domain", "show", "entity", "Order")
	if !strings.Contains(out, "Aggregate: no") {
		t.Fatalf("entity should remain without aggregate designation:\n%s", out)
	}

	// remove a value object — exact recommendation wording.
	mustRunDomain(t, root, "domain", "define", "value-object", "LegacyOrderID",
		"--definition", "retired")
	wantRmVO := "" +
		"Removed value-object LegacyOrderID from the project domain model.\n" +
		"Source files were not changed.\n"
	out = mustRunDomain(t, root, "domain", "remove", "value-object", "LegacyOrderID")
	if out != wantRmVO {
		t.Fatalf("remove value-object:\n got: %q\nwant: %q", out, wantRmVO)
	}

	// schema prints JSON Schema bytes.
	out = mustRunDomain(t, root, "domain", "schema")
	if !strings.Contains(out, `"$schema"`) || !strings.Contains(out, "ubiquitous-language") {
		snippet := out
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		t.Fatalf("schema output does not look like the published schema:\n%s", snippet)
	}
}

func testDomainExitCodes(t *testing.T) {
	root := domainFixture(t)
	mustRunDomain(t, root,
		"domain", "define", "entity", "Order",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
	)

	// Unchanged define → 0.
	stdout, stderr, code := runBin(t, root, os.Environ(),
		"domain", "define", "entity", "Order",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
	)
	if code != 0 || stdout != "Unchanged entity Order.\n" {
		t.Fatalf("unchanged define: exit %d\nstdout: %q\nstderr: %s", code, stdout, stderr)
	}

	// show missing → 1, message on stderr.
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "show", "entity", "Missing")
	if code != 1 || !strings.Contains(stderr, `no entity named "Missing" is defined in the project domain model`) {
		t.Fatalf("show missing: exit %d stderr %q", code, stderr)
	}

	// remove missing → 1.
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "remove", "entity", "Missing")
	if code != 1 || !strings.Contains(stderr, `no entity named "Missing"`) {
		t.Fatalf("remove missing: exit %d stderr %q", code, stderr)
	}

	// unknown type → 2.
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "show", "widget", "Order")
	if code != 2 {
		t.Fatalf("unknown type: exit %d stderr %q", code, stderr)
	}

	// --alias + --clear-aliases → 2.
	_, stderr, code = runBin(t, root, os.Environ(),
		"domain", "define", "entity", "Order",
		"--alias", "X", "--clear-aliases",
	)
	if code != 2 {
		t.Fatalf("alias+clear-aliases: exit %d stderr %q", code, stderr)
	}

	// missing name → 2.
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "show", "entity")
	if code != 2 {
		t.Fatalf("missing name: exit %d stderr %q", code, stderr)
	}

	// bad --format → 2.
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "list", "--format", "yaml")
	if code != 2 {
		t.Fatalf("bad format: exit %d stderr %q", code, stderr)
	}
}

func testDomainJSON(t *testing.T) {
	root := domainFixture(t)
	mustRunDomain(t, root,
		"domain", "define", "entity", "Order",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
	)
	mustRunDomain(t, root, "domain", "define", "aggregate", "Order")
	mustRunDomain(t, root,
		"domain", "define", "value-object", "Money",
		"--definition", "A monetary amount expressed in a particular currency.",
	)
	mustRunDomain(t, root,
		"domain", "define", "business-rule", "OrderMustHaveCustomer",
		"--definition", "Every Order must identify its Customer.",
	)
	mustRunDomain(t, root,
		"domain", "define", "event", "OrderPlaced",
		"--definition", "An Order has been accepted for processing.",
	)

	// overview --format json
	stdout := mustRunDomain(t, root, "domain", "overview", "--format", "json")
	var overview struct {
		Source string `json:"source"`
		Found  bool   `json:"found"`
		Counts struct {
			Entities      int `json:"entities"`
			Aggregates    int `json:"aggregates"`
			ValueObjects  int `json:"valueObjects"`
			BusinessRules int `json:"businessRules"`
			Events        int `json:"events"`
		} `json:"counts"`
		Entities []struct {
			Name       string   `json:"name"`
			Definition string   `json:"definition"`
			Aliases    []string `json:"aliases"`
			Aggregate  bool     `json:"aggregate"`
		} `json:"entities"`
		ValueObjects  []map[string]any `json:"valueObjects"`
		BusinessRules []map[string]any `json:"businessRules"`
		Events        []map[string]any `json:"events"`
	}
	if err := json.Unmarshal([]byte(stdout), &overview); err != nil {
		t.Fatalf("overview json: %v\n%s", err, stdout)
	}
	if overview.Source != "ubiquitous-language.yaml" || !overview.Found {
		t.Fatalf("overview envelope: %+v", overview)
	}
	if overview.Counts.Entities != 1 || overview.Counts.Aggregates != 1 ||
		overview.Counts.ValueObjects != 1 || overview.Counts.BusinessRules != 1 ||
		overview.Counts.Events != 1 {
		t.Fatalf("overview counts: %+v", overview.Counts)
	}
	if len(overview.Entities) != 1 || overview.Entities[0].Name != "Order" ||
		!overview.Entities[0].Aggregate ||
		overview.Entities[0].Definition == "" ||
		len(overview.Entities[0].Aliases) != 1 {
		t.Fatalf("overview entities: %+v", overview.Entities)
	}

	// list --format json
	stdout = mustRunDomain(t, root, "domain", "list", "--format", "json")
	var listing map[string][]map[string]any
	if err := json.Unmarshal([]byte(stdout), &listing); err != nil {
		t.Fatalf("list json: %v\n%s", err, stdout)
	}
	ents := listing["entities"]
	if len(ents) != 1 || ents[0]["name"] != "Order" || ents[0]["aggregate"] != true {
		t.Fatalf("list entities: %+v", ents)
	}
	if _, ok := listing["valueObjects"]; !ok {
		t.Fatalf("list missing valueObjects: %+v", listing)
	}

	// filtered aggregates key
	stdout = mustRunDomain(t, root, "domain", "list", "aggregates", "--format", "json")
	var aggOnly map[string][]map[string]any
	if err := json.Unmarshal([]byte(stdout), &aggOnly); err != nil {
		t.Fatalf("list aggregates json: %v\n%s", err, stdout)
	}
	if len(aggOnly) != 1 || len(aggOnly["aggregates"]) != 1 || aggOnly["aggregates"][0]["name"] != "Order" {
		t.Fatalf("list aggregates json: %+v", aggOnly)
	}

	// show --format json
	stdout = mustRunDomain(t, root, "domain", "show", "entity", "Order", "--format", "json")
	var show map[string]any
	if err := json.Unmarshal([]byte(stdout), &show); err != nil {
		t.Fatalf("show json: %v\n%s", err, stdout)
	}
	if show["type"] != "entity" || show["name"] != "Order" || show["aggregate"] != true {
		t.Fatalf("show json: %+v", show)
	}
	if show["definition"] == nil || show["aliases"] == nil {
		t.Fatalf("show json missing definition/aliases: %+v", show)
	}

	// define unchanged --format json
	stdout = mustRunDomain(t, root,
		"domain", "define", "entity", "Order",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
		"--format", "json",
	)
	var defRes map[string]any
	if err := json.Unmarshal([]byte(stdout), &defRes); err != nil {
		t.Fatalf("define json: %v\n%s", err, stdout)
	}
	if defRes["result"] != "unchanged" || defRes["type"] != "entity" || defRes["name"] != "Order" {
		t.Fatalf("define unchanged json: %+v", defRes)
	}

	// define create --format json
	stdout = mustRunDomain(t, root,
		"domain", "define", "entity", "Customer",
		"--definition", "A person or organization that places Orders.",
		"--format", "json",
	)
	if err := json.Unmarshal([]byte(stdout), &defRes); err != nil {
		t.Fatalf("define create json: %v\n%s", err, stdout)
	}
	if defRes["result"] != "created" || defRes["name"] != "Customer" {
		t.Fatalf("define create json: %+v", defRes)
	}

	// remove --format json (aggregate preserves entity)
	mustRunDomain(t, root, "domain", "define", "aggregate", "Order")
	stdout = mustRunDomain(t, root, "domain", "remove", "aggregate", "Order", "--format", "json")
	var rm map[string]any
	if err := json.Unmarshal([]byte(stdout), &rm); err != nil {
		t.Fatalf("remove aggregate json: %v\n%s", err, stdout)
	}
	if rm["type"] != "aggregate" || rm["name"] != "Order" ||
		rm["result"] != "removed" || rm["entityPreserved"] != true {
		t.Fatalf("remove aggregate json: %+v", rm)
	}

	stdout = mustRunDomain(t, root, "domain", "remove", "value-object", "Money", "--format", "json")
	if err := json.Unmarshal([]byte(stdout), &rm); err != nil {
		t.Fatalf("remove vo json: %v\n%s", err, stdout)
	}
	if rm["type"] != "value-object" || rm["name"] != "Money" ||
		rm["result"] != "removed" || rm["sourceFilesChanged"] != false {
		t.Fatalf("remove vo json: %+v", rm)
	}
}

func testDomainCommentPreservation(t *testing.T) {
	root := domainFixture(t)
	// Hand-authored model with head/line comments on Order.
	write(t, root, "ubiquitous-language.yaml", `# project domain model
version: 1

entities:
  # the consistency boundary for purchases
  - name: Order # primary aggregate
    definition: A customer's request to purchase products.
    aggregate: true
`)
	before, err := os.ReadFile(filepath.Join(root, "ubiquitous-language.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(before), "# the consistency boundary for purchases") ||
		!strings.Contains(string(before), "# primary aggregate") {
		t.Fatalf("fixture comments missing:\n%s", before)
	}

	mustRunDomain(t, root,
		"domain", "define", "value-object", "Money",
		"--definition", "A monetary amount expressed in a particular currency.",
	)
	after, err := os.ReadFile(filepath.Join(root, "ubiquitous-language.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(after)
	if !strings.Contains(body, "# the consistency boundary for purchases") {
		t.Fatalf("head/entry comment lost after define:\n%s", body)
	}
	if !strings.Contains(body, "# primary aggregate") {
		t.Fatalf("line comment lost after define:\n%s", body)
	}
	if !strings.Contains(body, "name: Order") || !strings.Contains(body, "name: Money") {
		t.Fatalf("entries missing after define:\n%s", body)
	}
	// Order still precedes the newly appended Money section entries.
	if iOrder, iMoney := strings.Index(body, "name: Order"), strings.Index(body, "name: Money"); iOrder < 0 || iMoney < 0 || iOrder > iMoney {
		t.Fatalf("entry order disturbed:\n%s", body)
	}
}

func testDomainGuided(t *testing.T) {
	root := domainFixture(t)

	// Scripted guided session matching the recommendation flow.
	// Accepts number or title; use number "1" for Entity, y for yes.
	stdin := strings.Join([]string{
		"1", // Entity
		"Order",
		"A customer's request to purchase products.",
		"Purchase Order",
		"y", // aggregate
		"y", // confirm write
		"",
	}, "\n")
	stdout, stderr, code := runBinStdin(t, root, os.Environ(), stdin, "domain", "define", "--guided")
	if code != 0 {
		t.Fatalf("guided yes: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	// Preview block from the recommendation example.
	for _, want := range []string{
		"Proposed definition:",
		"Entity: Order",
		"Aggregate: yes",
		"Definition: A customer's request to purchase products.",
		"Aliases: Purchase Order",
		"Defined entity Order.",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("guided stdout missing %q:\n%s", want, stdout)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "ubiquitous-language.yaml"))
	if err != nil {
		t.Fatalf("guided write missing file: %v", err)
	}
	body := string(data)
	if !strings.Contains(body, "name: Order") || !strings.Contains(body, "aggregate: true") {
		t.Fatalf("guided file content:\n%s", body)
	}
	if !strings.Contains(body, "Purchase Order") {
		t.Fatalf("guided aliases missing:\n%s", body)
	}

	// Decline confirmation → nothing written (fresh fixture).
	rootNo := domainFixture(t)
	stdinNo := strings.Join([]string{
		"Entity",
		"Customer",
		"A person or organization that places Orders.",
		"",
		"n", // not aggregate
		"n", // do not write
		"",
	}, "\n")
	stdout, stderr, code = runBinStdin(t, rootNo, os.Environ(), stdinNo, "domain", "define", "--guided")
	if code != 0 {
		t.Fatalf("guided no: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Nothing written.") {
		t.Fatalf("guided decline missing Nothing written:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(rootNo, "ubiquitous-language.yaml")); !os.IsNotExist(err) {
		t.Fatalf("guided decline must not create ubiquitous-language.yaml, err=%v", err)
	}
}

func testDomainSchema(t *testing.T) {
	root := domainFixture(t)
	stdout := mustRunDomain(t, root, "domain", "schema")
	committed, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "ubiquitous-language.schema.json"))
	if err != nil {
		t.Fatalf("read committed schema: %v", err)
	}
	if stdout != string(committed) {
		t.Fatalf("domain schema output differs from docs/ubiquitous-language.schema.json (%d vs %d bytes)",
			len(stdout), len(committed))
	}
}

func testDomainHelp(t *testing.T) {
	root := domainFixture(t)

	stdout, stderr, code := runBin(t, root, os.Environ(), "--help")
	if code != 0 {
		t.Fatalf("--help exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "domain") ||
		!strings.Contains(stdout, "inspect and maintain the project's ubiquitous language") {
		t.Fatalf("top-level help missing domain entry:\n%s", stdout)
	}

	stdout, stderr, code = runBin(t, root, os.Environ(), "domain", "--help")
	if code != 0 {
		t.Fatalf("domain --help exit %d\nstderr: %s", code, stderr)
	}
	for _, sub := range []string{"overview", "list", "show", "explain", "define", "remove", "schema"} {
		if !strings.Contains(stdout, sub) {
			t.Errorf("domain --help missing %q", sub)
		}
	}
	if !strings.Contains(stdout, "Running arclint domain without a subcommand") {
		t.Errorf("domain --help missing Long prose")
	}

	stdout, stderr, code = runBin(t, root, os.Environ(), "domain", "remove", "--help")
	if code != 0 {
		t.Fatalf("domain remove --help exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "rm") {
		t.Errorf("remove --help missing rm alias:\n%s", stdout)
	}
	// Examples present on a few key commands.
	for _, cmd := range [][]string{
		{"domain", "overview", "--help"},
		{"domain", "define", "--help"},
		{"domain", "show", "--help"},
	} {
		stdout, stderr, code = runBin(t, root, os.Environ(), cmd...)
		if code != 0 {
			t.Fatalf("%v exit %d\nstderr: %s", cmd, code, stderr)
		}
		if !strings.Contains(stdout, "Examples:") && !strings.Contains(strings.ToLower(stdout), "arclint domain") {
			t.Errorf("%v help missing examples:\n%s", cmd, stdout)
		}
	}
}

func testDomainExclusions(t *testing.T) {
	root := domainFixture(t)

	for _, args := range [][]string{
		{"entities"},
		{"aggregates"},
		{"domain", "add"},
		{"domain", "edit"},
		{"domain", "check"},
		{"domain", "context"},
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
			!strings.Contains(msg, "Error:") {
			t.Errorf("%v: exit %d without unknown-command signal\n%s", args, code, msg)
		}
	}

	// Unknown flags on define.
	for _, args := range [][]string{
		{"domain", "define", "entity", "X", "--path", "y"},
		{"domain", "define", "entity", "X", "--entity", "y"},
		{"domain", "define", "entity", "X", "--type", "y"},
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
	write(t, root, "order.go", "package main\n\n// stray source the remove command must not touch\n")
	before, err := os.ReadFile(filepath.Join(root, "order.go"))
	if err != nil {
		t.Fatal(err)
	}
	mustRunDomain(t, root,
		"domain", "define", "entity", "Order",
		"--definition", "A customer's request to purchase products.",
	)
	mustRunDomain(t, root, "domain", "remove", "entity", "Order")
	after, err := os.ReadFile(filepath.Join(root, "order.go"))
	if err != nil {
		t.Fatalf("order.go disappeared: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("remove mutated order.go\nbefore: %q\nafter: %q", before, after)
	}
}

func testDomainExtensionAccess(t *testing.T) {
	// With a consuming extension rule, ctx.domain() surfaces entity names as findings.
	root := t.TempDir()
	write(t, root, ".arclint/extensions/domain-probe.ts", `
import { defineRule } from "arclint";
export default defineRule({
  type: "domain-probe",
  check(ctx) {
    const domain = ctx.domain();
    for (const entity of domain.entities) {
      ctx.report({
        path: "src/ok.go",
        line: 1,
        message: "entity:" + entity.name,
      });
    }
  },
});
`)
	write(t, root, "rules.yaml", `runtime: [go]
modules:
  src:
    paths: ["src/**"]
contracts:
  src:
    invariants:
      - id: "repo:src/domain-probe"
        kind: extension
        files: "src/**/*.go"
        uses: domain-probe
`)
	write(t, root, "src/ok.go", "package src\n")
	write(t, root, "ubiquitous-language.yaml", `version: 1
entities:
  - name: Order
    definition: A customer's request to purchase products.
    aggregate: true
  - name: Customer
    definition: A person or organization that places Orders.
`)

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
		if d.Kind == "violation" && d.Status == "active" {
			msgs = append(msgs, d.Message)
		}
	}
	joined := strings.Join(msgs, "\n")
	if !strings.Contains(joined, "entity:Order") || !strings.Contains(joined, "entity:Customer") {
		t.Fatalf("domain-probe findings missing entity names: %v\nall: %+v", msgs, diagnostics)
	}

	// Declaration alone never produces diagnostics: model present, no consuming rule.
	clean := t.TempDir()
	write(t, clean, "rules.yaml", minimalDomainRules)
	write(t, clean, "src/ok.go", "package src\n")
	write(t, clean, "ubiquitous-language.yaml", `version: 1
entities:
  - name: Order
    definition: A customer's request to purchase products.
    aggregate: true
`)
	_, stderr, code = runBin(t, clean, os.Environ(), "check")
	if code != 0 {
		t.Fatalf("check with model but no consuming rule must stay clean: exit %d\nstderr: %s", code, stderr)
	}
}

func testDomainContext(t *testing.T) {
	// Absent model → no project-domain block / no domain JSON key.
	absent := domainFixture(t)
	stdout, stderr, code := runBin(t, absent, os.Environ(), "context")
	if code != 0 {
		t.Fatalf("context without model: exit %d\nstderr: %s", code, stderr)
	}
	if strings.Contains(stdout, "project domain") || strings.Contains(stdout, "ubiquitous-language.yaml") {
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
		t.Fatalf("context json domain key present without model: %+v", bare["domain"])
	}

	// Present model → text block + json domain key.
	root := domainFixture(t)
	write(t, root, "ubiquitous-language.yaml", `version: 1
entities:
  - name: Customer
    definition: A person or organization that places Orders.
  - name: Order
    definition: A customer's request to purchase products.
    aggregate: true
  - name: Product
    definition: Something that can be purchased.
value_objects:
  - name: Money
    definition: A monetary amount expressed in a particular currency.
  - name: OrderID
    definition: The stable identity of an Order.
business_rules:
  - name: OrderMustHaveCustomer
    definition: Every Order must identify its Customer.
  - name: OrderTotalCannotBeNegative
    definition: An Order's total monetary amount cannot be negative.
events:
  - name: OrderPlaced
    definition: An Order has been accepted for processing.
  - name: OrderCancelled
    definition: An Order has been cancelled and will not be processed.
`)
	stdout, stderr, code = runBin(t, root, os.Environ(), "context")
	if code != 0 {
		t.Fatalf("context with model: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "project domain (ubiquitous-language.yaml):") {
		t.Fatalf("context text missing project-domain header:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Order [aggregate]") {
		t.Fatalf("context text missing aggregate mark:\n%s", stdout)
	}
	if !strings.Contains(stdout, "entities:") || !strings.Contains(stdout, "value objects:") {
		t.Fatalf("context text missing group lines:\n%s", stdout)
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
	if domain["source"] != "ubiquitous-language.yaml" {
		t.Fatalf("domain.source = %v", domain["source"])
	}
	ents, _ := domain["entities"].([]any)
	if len(ents) != 3 {
		t.Fatalf("domain.entities = %+v", ents)
	}
}
