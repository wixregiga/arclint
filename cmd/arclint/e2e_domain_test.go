package main

// End-to-end coverage of the arclint domain command family against the
// compiled binary. Assertions follow the contexts-shaped ubiquitous
// language model and the domain CLI contract.

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
	t.Run("context", testDomainContext)
	t.Run("ambiguity", testDomainAmbiguity)
}

func testDomainInit(t *testing.T) {
	root := domainFixture(t)
	modelPath := filepath.Join(root, "ubiquitous-language.yaml")

	stdout := mustRunDomain(t, root, "domain", "init")
	if stdout != "Initialized ubiquitous-language.yaml.\n" {
		t.Fatalf("first init output = %q", stdout)
	}
	created, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read initialized model: %v", err)
	}
	text := string(created)
	if !strings.Contains(text, "yaml-language-server: $schema=") || !strings.Contains(text, "version: 1") {
		t.Fatalf("initialized model lacks schema hint or version:\n%s", text)
	}

	existing := `# project-owned comment
version: 1
contexts:
  - name: Ordering
    entities:
      - name: Order
        definition: A purchase request.
`
	write(t, root, "ubiquitous-language.yaml", existing)
	stdout = mustRunDomain(t, root, "domain", "init")
	if stdout != "ubiquitous-language.yaml already exists; left unchanged.\n" {
		t.Fatalf("repeated init output = %q", stdout)
	}
	unchanged, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read existing model: %v", err)
	}
	if string(unchanged) != existing {
		t.Fatalf("repeated init changed existing model:\n%s", unchanged)
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
	if !strings.Contains(stdout, "--context") {
		t.Fatalf("missing-model guidance should mention --context:\n%s", stdout)
	}

	mustRunDomain(t, root,
		"domain", "define", "entity", "Order",
		"--context", "Ordering",
		"--definition", "A customer's request to purchase products.",
	)
	modelPath := filepath.Join(root, "ubiquitous-language.yaml")
	if _, err := os.Stat(modelPath); err != nil {
		t.Fatalf("define must create ubiquitous-language.yaml beside rules.yaml: %v", err)
	}

	other := t.TempDir()
	stdout, stderr, code = runBin(t, other, os.Environ(),
		"--rules", filepath.Join(root, "rules.yaml"),
		"domain", "show", "entity", "Order",
	)
	if code != 0 {
		t.Fatalf("--rules domain show: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Entity: Order") || !strings.Contains(stdout, "Context: Ordering") {
		t.Fatalf("--rules show missed Order:\n%s", stdout)
	}
}

func testDomainFullFlow(t *testing.T) {
	root := domainFixture(t)

	out := mustRunDomain(t, root,
		"domain", "define", "entity", "Order",
		"--context", "Ordering",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
	)
	if out != "Defined entity Order.\n" {
		t.Fatalf("define entity create:\n got: %q", out)
	}
	out = mustRunDomain(t, root, "domain", "define", "aggregate", "Order", "--context", "Ordering")
	if !strings.HasPrefix(out, "Updated aggregate Order.\n") || !strings.Contains(out, "aggregate: designated") {
		t.Fatalf("define aggregate designation:\n%s", out)
	}

	for _, args := range [][]string{
		{"domain", "define", "value_object", "OrderID", "--context", "Ordering", "--definition", "The stable identity of an Order."},
		{"domain", "define", "value_object", "Money", "--context", "Ordering", "--definition", "A monetary amount expressed in a particular currency."},
		{"domain", "define", "invariant", "Every Order must identify its Customer.", "--context", "Ordering", "--owner", "Order"},
		{"domain", "define", "invariant", "An Order's total monetary amount cannot be negative.", "--context", "Ordering", "--owner", "Order"},
		{"domain", "define", "domain_event", "OrderPlaced", "--context", "Ordering", "--definition", "An Order has been accepted for processing."},
		{"domain", "define", "domain_event", "OrderCancelled", "--context", "Ordering", "--definition", "An Order has been cancelled and will not be processed."},
	} {
		out = mustRunDomain(t, root, args...)
		if !strings.HasPrefix(out, "Defined ") {
			t.Fatalf("%v: got %q", args, out)
		}
	}

	wantShow := "" +
		"Entity: Order\n" +
		"Context: Ordering\n" +
		"Aggregate: yes\n" +
		"Definition: A customer's request to purchase products.\n" +
		"Aliases:\n" +
		"  Purchase Order\n"
	out = mustRunDomain(t, root, "domain", "show", "entity", "Order")
	if out != wantShow {
		t.Fatalf("show entity Order:\n got: %q\nwant: %q", out, wantShow)
	}

	out = mustRunDomain(t, root, "domain", "show", "invariant", "Every Order must identify its Customer.")
	if !strings.Contains(out, "Invariant: Every Order must identify its Customer.") ||
		!strings.Contains(out, "Owner: Order") {
		t.Fatalf("show invariant:\n%s", out)
	}

	out = mustRunDomain(t, root, "domain", "list", "aggregates")
	if !strings.Contains(out, "Context Ordering") || !strings.Contains(out, "Order") {
		t.Fatalf("list aggregates:\n%s", out)
	}

	out = mustRunDomain(t, root, "domain", "list")
	for _, want := range []string{
		"Context Ordering",
		"Order [aggregate]",
		"Money",
		"OrderID",
		"Every Order must identify its Customer.",
		"OrderPlaced",
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
		"1 Context",
		"1 Entity",
		"1 Aggregate",
		"2 Value Objects",
		"2 Invariants",
		"2 Events",
		"Context Ordering",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("overview missing %q:\n%s", want, out)
		}
	}

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

	mustRunDomain(t, root, "domain", "define", "value_object", "LegacyOrderID",
		"--context", "Ordering", "--definition", "retired")
	wantRmVO := "" +
		"Removed value_object LegacyOrderID from the project domain model.\n" +
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
	mustRunDomain(t, root,
		"domain", "define", "entity", "Order",
		"--context", "Ordering",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
	)

	stdout, stderr, code := runBin(t, root, os.Environ(),
		"domain", "define", "entity", "Order",
		"--context", "Ordering",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
	)
	if code != 0 || stdout != "Unchanged entity Order.\n" {
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
		"domain", "define", "entity", "Order",
		"--context", "Ordering",
		"--alias", "X", "--clear-aliases",
	)
	if code != 2 {
		t.Fatalf("alias+clear-aliases: exit %d stderr %q", code, stderr)
	}

	// missing definition at create → 2
	_, stderr, code = runBin(t, root, os.Environ(),
		"domain", "define", "entity", "Customer", "--context", "Ordering",
	)
	if code != 2 || !strings.Contains(stderr, "definition") {
		t.Fatalf("missing definition: exit %d stderr %q", code, stderr)
	}

	// missing owner at create → 2
	_, stderr, code = runBin(t, root, os.Environ(),
		"domain", "define", "invariant", "must hold", "--context", "Ordering",
	)
	if code != 2 || !strings.Contains(stderr, "owner") {
		t.Fatalf("missing owner: exit %d stderr %q", code, stderr)
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
	mustRunDomain(t, root,
		"domain", "define", "entity", "Order",
		"--context", "Ordering",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
	)
	mustRunDomain(t, root, "domain", "define", "aggregate", "Order", "--context", "Ordering")
	mustRunDomain(t, root,
		"domain", "define", "value_object", "Money",
		"--context", "Ordering",
		"--definition", "A monetary amount expressed in a particular currency.",
	)
	mustRunDomain(t, root,
		"domain", "define", "invariant", "Every Order must identify its Customer.",
		"--context", "Ordering", "--owner", "Order",
	)
	mustRunDomain(t, root,
		"domain", "define", "domain_event", "OrderPlaced",
		"--context", "Ordering",
		"--definition", "An Order has been accepted for processing.",
	)

	stdout := mustRunDomain(t, root, "domain", "overview", "--format", "json")
	var overview struct {
		Source string `json:"source"`
		Found  bool   `json:"found"`
		Counts struct {
			Contexts     int `json:"contexts"`
			Entities     int `json:"entities"`
			Aggregates   int `json:"aggregates"`
			ValueObjects int `json:"valueObjects"`
			Invariants   int `json:"invariants"`
			Events       int `json:"events"`
		} `json:"counts"`
		Contexts []struct {
			Name     string `json:"name"`
			Entities []struct {
				Name      string `json:"name"`
				Aggregate bool   `json:"aggregate"`
			} `json:"entities"`
			ValueObjects []map[string]any `json:"valueObjects"`
			Invariants   []struct {
				Statement string `json:"statement"`
				Owner     string `json:"owner"`
			} `json:"invariants"`
			Events []map[string]any `json:"events"`
		} `json:"contexts"`
	}
	if err := json.Unmarshal([]byte(stdout), &overview); err != nil {
		t.Fatalf("overview json: %v\n%s", err, stdout)
	}
	if overview.Source != "ubiquitous-language.yaml" || !overview.Found {
		t.Fatalf("overview envelope: %+v", overview)
	}
	if overview.Counts.Contexts != 1 || overview.Counts.Entities != 1 || overview.Counts.Aggregates != 1 ||
		overview.Counts.ValueObjects != 1 || overview.Counts.Invariants != 1 ||
		overview.Counts.Events != 1 {
		t.Fatalf("overview counts: %+v", overview.Counts)
	}
	if len(overview.Contexts) != 1 || overview.Contexts[0].Name != "Ordering" {
		t.Fatalf("overview contexts: %+v", overview.Contexts)
	}
	if len(overview.Contexts[0].Entities) != 1 || overview.Contexts[0].Entities[0].Name != "Order" ||
		!overview.Contexts[0].Entities[0].Aggregate {
		t.Fatalf("overview entities: %+v", overview.Contexts[0].Entities)
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

	stdout = mustRunDomain(t, root, "domain", "list", "aggregates", "--format", "json")
	if err := json.Unmarshal([]byte(stdout), &listing); err != nil {
		t.Fatalf("list aggregates json: %v\n%s", err, stdout)
	}
	if _, ok := listing["contexts"]; !ok {
		t.Fatalf("list aggregates json missing contexts: %+v", listing)
	}

	stdout = mustRunDomain(t, root, "domain", "show", "entity", "Order", "--format", "json")
	var show map[string]any
	if err := json.Unmarshal([]byte(stdout), &show); err != nil {
		t.Fatalf("show json: %v\n%s", err, stdout)
	}
	if show["type"] != "entity" || show["name"] != "Order" || show["aggregate"] != true || show["context"] != "Ordering" {
		t.Fatalf("show json: %+v", show)
	}

	stdout = mustRunDomain(t, root,
		"domain", "define", "entity", "Order",
		"--context", "Ordering",
		"--definition", "A customer's request to purchase products.",
		"--alias", "Purchase Order",
		"--format", "json",
	)
	var defRes map[string]any
	if err := json.Unmarshal([]byte(stdout), &defRes); err != nil {
		t.Fatalf("define json: %v\n%s", err, stdout)
	}
	if defRes["result"] != "unchanged" || defRes["type"] != "entity" || defRes["context"] != "Ordering" {
		t.Fatalf("define unchanged json: %+v", defRes)
	}

	stdout = mustRunDomain(t, root,
		"domain", "define", "entity", "Customer",
		"--context", "Ordering",
		"--definition", "A person or organization that places Orders.",
		"--format", "json",
	)
	if err := json.Unmarshal([]byte(stdout), &defRes); err != nil {
		t.Fatalf("define create json: %v\n%s", err, stdout)
	}
	if defRes["result"] != "created" || defRes["name"] != "Customer" {
		t.Fatalf("define create json: %+v", defRes)
	}

	mustRunDomain(t, root, "domain", "define", "aggregate", "Order", "--context", "Ordering")
	stdout = mustRunDomain(t, root, "domain", "remove", "aggregate", "Order", "--format", "json")
	var rm map[string]any
	if err := json.Unmarshal([]byte(stdout), &rm); err != nil {
		t.Fatalf("remove aggregate json: %v\n%s", err, stdout)
	}
	if rm["type"] != "aggregate" || rm["name"] != "Order" ||
		rm["result"] != "removed" || rm["entityPreserved"] != true {
		t.Fatalf("remove aggregate json: %+v", rm)
	}

	stdout = mustRunDomain(t, root, "domain", "remove", "value_object", "Money", "--format", "json")
	if err := json.Unmarshal([]byte(stdout), &rm); err != nil {
		t.Fatalf("remove vo json: %v\n%s", err, stdout)
	}
	if rm["type"] != "value_object" || rm["name"] != "Money" ||
		rm["result"] != "removed" || rm["sourceFilesChanged"] != false {
		t.Fatalf("remove vo json: %+v", rm)
	}
}

func testDomainCommentPreservation(t *testing.T) {
	root := domainFixture(t)
	write(t, root, "ubiquitous-language.yaml", `# project domain model
version: 1
contexts:
  - name: Ordering
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
		"domain", "define", "value_object", "Money",
		"--context", "Ordering",
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
	if iOrder, iMoney := strings.Index(body, "name: Order"), strings.Index(body, "name: Money"); iOrder < 0 || iMoney < 0 || iOrder > iMoney {
		t.Fatalf("entry order disturbed:\n%s", body)
	}
}

func testDomainGuided(t *testing.T) {
	root := domainFixture(t)
	// 1=Entity, context, term, definition, aliases, aggregate y, confirm y
	stdin := strings.Join([]string{
		"1",
		"Ordering",
		"Order",
		"A customer's request to purchase products.",
		"Purchase Order",
		"y",
		"y",
		"",
	}, "\n")
	stdout, stderr, code := runBinStdin(t, root, os.Environ(), stdin, "domain", "define", "--guided")
	if code != 0 {
		t.Fatalf("guided yes: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
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

	rootNo := domainFixture(t)
	stdinNo := strings.Join([]string{
		"Entity",
		"Ordering",
		"Customer",
		"A person or organization that places Orders.",
		"",
		"n",
		"n",
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
	committed, err := os.ReadFile(filepath.Join(repoRoot(t), ".agents", "skills", "domain-librarian", "library.schema.json"))
	if err != nil {
		t.Fatalf("read committed schema: %v", err)
	}
	if stdout != string(committed) {
		t.Fatalf("domain schema output differs from .agents/skills/domain-librarian/library.schema.json (%d vs %d bytes)",
			len(stdout), len(committed))
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
	mustRunDomain(t, root, "domain", "define", "entity", "Order",
		"--context", "Ordering",
		"--definition", "A customer's request to purchase products.")
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
    for (const bound of domain.contexts) {
      for (const entity of bound.entities) {
        ctx.report({
          path: "src/ok.go",
          line: 1,
          message: "entity:" + entity.name,
        });
      }
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
contexts:
  - name: Ordering
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
contexts:
  - name: Ordering
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
	// Absent model -> no project-domain block / no domain JSON key.
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
		t.Fatalf("context json with bare model should omit domain: %+v", bare)
	}

	// Present model -> text block + json domain key with contexts.
	root := domainFixture(t)
	write(t, root, "ubiquitous-language.yaml", `version: 1
contexts:
  - name: Ordering
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
    invariants:
      - statement: Every Order must identify its Customer.
        owner: Order
      - statement: An Order's total monetary amount cannot be negative.
        owner: Order
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
	if !strings.Contains(stdout, "project domain (ubiquitous-language.yaml)") {
		t.Fatalf("context text missing project-domain header:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Order [aggregate]") {
		t.Fatalf("context text missing aggregate mark:\n%s", stdout)
	}
	if !strings.Contains(stdout, "context Ordering:") ||
		!strings.Contains(stdout, "entities:") ||
		!strings.Contains(stdout, "value objects:") ||
		!strings.Contains(stdout, "invariants:") {
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
	contexts, ok := domain["contexts"].([]any)
	if !ok || len(contexts) != 1 {
		t.Fatalf("domain.contexts = %+v", domain["contexts"])
	}
	ctx0, _ := contexts[0].(map[string]any)
	ents, _ := ctx0["entities"].([]any)
	if len(ents) != 3 {
		t.Fatalf("domain.entities = %+v", ents)
	}
}

func testDomainAmbiguity(t *testing.T) {
	root := domainFixture(t)
	write(t, root, "ubiquitous-language.yaml", `version: 1
contexts:
  - name: Ordering
    entities:
      - name: Order
        definition: ordering order
  - name: Billing
    entities:
      - name: Order
        definition: billing order
`)
	_, stderr, code := runBin(t, root, os.Environ(), "domain", "show", "entity", "Order")
	if code != 2 || !strings.Contains(stderr, "multiple contexts") {
		t.Fatalf("ambiguous show: exit %d stderr %q", code, stderr)
	}
	stdout := mustRunDomain(t, root, "domain", "show", "entity", "Order", "--context", "Billing")
	if !strings.Contains(stdout, "Context: Billing") || !strings.Contains(stdout, "billing order") {
		t.Fatalf("explicit context show:\n%s", stdout)
	}

	// define without --context when multiple contexts → usage
	_, stderr, code = runBin(t, root, os.Environ(),
		"domain", "define", "entity", "Invoice",
		"--definition", "A bill.",
	)
	if code != 2 || !strings.Contains(stderr, "--context") {
		t.Fatalf("define without context: exit %d stderr %q", code, stderr)
	}
}
