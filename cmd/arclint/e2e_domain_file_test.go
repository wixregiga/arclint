package main

// End-to-end coverage of the project's domain file, its DomainRecording,
// against the compiled binary: rules.arclint.yaml names it under
// domain.file (domain.arclint.yaml when it names none), and every
// command, hook, finding and report that names the domain file names that
// one. Each case runs the same sequence against a ruleset that names a
// nested file and against one that names nothing, and proves the other
// file is never read or written.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	cmlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/cml"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

type domainFileCase struct {
	name string
	// rules is the repository ruleset.
	rules string
	// file is the project's domain file, and other the file the project
	// does not use.
	file, other string
}

func domainFileCases() []domainFileCase {
	return []domainFileCase{
		{
			name:  "a ruleset that names the file",
			rules: "runtime: [go]\ndomain:\n  file: docs/model.arclint.yaml\nzones:\n  src:\n    paths: [\"src/**\"]\n",
			file:  "docs/model.arclint.yaml", other: "domain.arclint.yaml",
		},
		{
			name:  "a ruleset that names a .yml file, with a current directory in the path",
			rules: "runtime: [go]\ndomain:\n  file: ./model/domain.yml\nzones:\n  src:\n    paths: [\"src/**\"]\n",
			file:  "model/domain.yml", other: "domain.arclint.yaml",
		},
		{
			name:  "a ruleset that names a CML file",
			rules: "runtime: [go]\ndomain:\n  file: docs/model.cml\nzones:\n  src:\n    paths: [\"src/**\"]\n",
			file:  "docs/model.cml", other: "domain.arclint.yaml",
		},
		{
			name:  "a ruleset that names no file",
			rules: minimalDomainRules,
			file:  "domain.arclint.yaml", other: "docs/model.arclint.yaml",
		},
	}
}

// cml reports a case whose domain file is written in CML.
func (c domainFileCase) cml() bool { return strings.HasSuffix(c.file, ".cml") }

// orderingDomain is orderingDomain written in the case's format: the
// YAML text itself, or the CML the adapter renders for the same language.
func (c domainFileCase) orderingDomain(t *testing.T) string {
	t.Helper()
	if !c.cml() {
		return orderingDomain
	}
	recording, err := vocab.NewDomainRecording(c.file)
	if err != nil {
		t.Fatal(err)
	}
	lang, err := yamlvocab.Parse([]byte(orderingDomain), mustDefaultRecording(t))
	if err != nil {
		t.Fatal(err)
	}
	text, err := cmlvocab.Render(lang)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cmlvocab.Parse(text, recording); err != nil {
		t.Fatal(err)
	}
	return string(text)
}

// projectMarker is the text that names the project in the case's format.
func (c domainFileCase) projectMarker(project string) string {
	if c.cml() {
		return "Domain " + project
	}
	return "project: " + project + "\n"
}

// aggregateMarker is the text that records an aggregate in the case's
// format.
func (c domainFileCase) aggregateMarker(name string) string {
	if c.cml() {
		return "Aggregate " + name + " {"
	}
	return "      " + name + ":\n"
}

// contextMarker is the text that opens a bounded context in the case's
// format.
func (c domainFileCase) contextMarker(name string) string {
	if c.cml() {
		return "BoundedContext " + name + " {"
	}
	return "  " + name + ":\n"
}

func (c domainFileCase) fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", c.rules)
	write(t, root, "src/ok.go", "package src\n")
	return root
}

func (c domainFileCase) requireOnlyTheProjectsFile(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(c.other))); !os.IsNotExist(err) {
		t.Errorf("%s was read or written although the project records in %s (stat: %v)", c.other, c.file, err)
	}
}

func TestDomainFileIsTheOneTheRulesetNames(t *testing.T) {
	for _, c := range domainFileCases() {
		t.Run(c.name, func(t *testing.T) {
			t.Run("commands", c.testCommands)
			t.Run("guided", c.testGuided)
			t.Run("check", c.testCheck)
			t.Run("context", c.testContext)
			t.Run("agents", c.testAgents)
			t.Run("hooks", c.testHooks)
		})
	}
}

func (c domainFileCase) testCommands(t *testing.T) {
	root := c.fixture(t)

	// Nothing is recorded yet: every command that looks says where.
	stdout := mustRunDomain(t, root, "domain", "overview")
	if !strings.Contains(stdout, "No recorded Ubiquitous Language found at "+c.file+".\n") {
		t.Fatalf("overview of a missing file:\n%s", stdout)
	}
	stdout, _, code := runBin(t, root, os.Environ(), "domain", "overview", "--format", "json")
	var missing struct {
		Found  bool   `json:"found"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal([]byte(stdout), &missing); err != nil || code != 0 || missing.Found || missing.Source != c.file {
		t.Fatalf("overview json of a missing file: %d %v %s", code, err, stdout)
	}
	_, stderr, code := runBin(t, root, os.Environ(), "domain", "show", "aggregate", "Order")
	if code == 0 || !strings.Contains(stderr, c.file+" is missing") {
		t.Fatalf("show of a missing file: %d %s", code, stderr)
	}

	// init creates the project's file, creating its directories, and says so.
	stdout = mustRunDomain(t, root, "domain", "init", "--project", "shop")
	if !strings.Contains(stdout, "Initialized "+c.file+" for project shop.") {
		t.Fatalf("init: %s", stdout)
	}
	created, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
	if err != nil || !strings.Contains(string(created), c.projectMarker("shop")) {
		t.Fatalf("init did not create %s: %v\n%s", c.file, err, created)
	}
	stdout = mustRunDomain(t, root, "domain", "init")
	if !strings.Contains(stdout, c.file+" already exists (project shop); left unchanged.") {
		t.Fatalf("second init: %s", stdout)
	}

	// define writes into it, and overview, list and show read from it.
	recordOrdering(t, root)
	written, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file)))
	if err != nil || string(written) == string(created) || !strings.Contains(string(written), c.aggregateMarker("Order")) {
		t.Fatalf("define did not record into %s: %v\n%s", c.file, err, written)
	}
	if stdout := mustRunDomain(t, root, "domain", "overview"); !strings.Contains(stdout, "Source: "+c.file+"\n") || !strings.Contains(stdout, "Order") {
		t.Fatalf("overview:\n%s", stdout)
	}
	stdout, _, code = runBin(t, root, os.Environ(), "domain", "overview", "--format", "json")
	var overview struct {
		Found  bool   `json:"found"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal([]byte(stdout), &overview); err != nil || code != 0 || !overview.Found || overview.Source != c.file {
		t.Fatalf("overview json: %d %v %s", code, err, stdout)
	}
	if stdout := mustRunDomain(t, root, "domain", "list", "aggregates"); !strings.Contains(stdout, "Order") {
		t.Fatalf("list:\n%s", stdout)
	}
	if stdout := mustRunDomain(t, root, "domain", "show", "aggregate", "Order"); !strings.Contains(stdout, "OrderID") {
		t.Fatalf("show:\n%s", stdout)
	}
	if stdout := mustRunDomain(t, root, "domain", "remove", "domain_event", "OrderCancelled"); !strings.Contains(stdout, "OrderCancelled") {
		t.Fatalf("remove: %s", stdout)
	}
	if after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file))); err != nil || strings.Contains(string(after), "OrderCancelled") {
		t.Fatalf("remove did not edit %s: %v\n%s", c.file, err, after)
	}

	// A hand-written file in the project's place is read as it stands.
	write(t, root, c.file, c.orderingDomain(t))
	if stdout := mustRunDomain(t, root, "domain", "show", "value_object", "Money"); !strings.Contains(stdout, "A monetary amount") {
		t.Fatalf("show of a hand-written file:\n%s", stdout)
	}
	c.requireOnlyTheProjectsFile(t, root)
}

func (c domainFileCase) testGuided(t *testing.T) {
	root := c.fixture(t)
	stdin := strings.Join([]string{"Bounded Context", "ordering", "Taking and fulfilling customer orders.", "y", ""}, "\n")
	stdout, stderr, code := runBinStdin(t, root, os.Environ(), stdin, "domain", "define", "--guided")
	if code != 0 {
		t.Fatalf("guided: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Write this definition to "+c.file+"? [y/N]\n") {
		t.Fatalf("guided prompt does not name the project's domain file:\n%s", stdout)
	}
	if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.file))); err != nil || !strings.Contains(string(data), c.contextMarker("ordering")) {
		t.Fatalf("guided write missing from %s: %v\n%s", c.file, err, data)
	}
	c.requireOnlyTheProjectsFile(t, root)
}

func (c domainFileCase) testCheck(t *testing.T) {
	root := c.fixture(t)
	write(t, root, c.file, c.orderingDomain(t))

	stdout, stderr, code := runBin(t, root, os.Environ(), "check", "--format", "json")
	if code != 1 {
		t.Fatalf("check with no code implementing the model: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	var diagnostics []diagnosticDoc
	if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil {
		t.Fatalf("check json: %v\n%s", err, stdout)
	}
	violations, coverage := 0, 0
	for _, d := range diagnostics {
		switch d.Kind {
		case "violation":
			violations++
			if d.Path != c.file || d.Line == 0 {
				t.Errorf("a finding about a recorded term anchors at the project's domain file and the term's line, got %+v", d)
			}
		case "coverage":
			if d.Path != "" {
				coverage++
				if d.Path != c.file || d.Line == 0 {
					t.Errorf("a contract's coverage anchors at the project's domain file and the contract's line, got %+v", d)
				}
			}
		}
	}
	if violations == 0 || coverage == 0 {
		t.Fatalf("check reported %d violations and %d contract coverages; want both:\n%s", violations, coverage, stdout)
	}

	// The plain report prints the anchor and the remedy with the same file.
	plain, _, code := runBin(t, root, os.Environ(), "check")
	if code != 1 || !strings.Contains(plain, c.file+":") {
		t.Fatalf("plain check: exit %d\n%s", code, plain)
	}
	if strings.Contains(plain, c.other) {
		t.Fatalf("plain check names %s although the project records in %s:\n%s", c.other, c.file, plain)
	}
	c.requireOnlyTheProjectsFile(t, root)
}

func (c domainFileCase) testContext(t *testing.T) {
	root := c.fixture(t)
	stdout := mustRunDomain(t, root, "context")
	if strings.Contains(stdout, "project domain") {
		t.Fatalf("context names a domain although none is recorded:\n%s", stdout)
	}
	write(t, root, c.file, c.orderingDomain(t))
	stdout = mustRunDomain(t, root, "context")
	if !strings.Contains(stdout, "project domain ("+c.file+"): 1 context") {
		t.Fatalf("context does not name the project's domain file:\n%s", stdout)
	}
	if strings.Contains(stdout, c.other) {
		t.Fatalf("context names %s:\n%s", c.other, stdout)
	}
	stdout, _, code := runBin(t, root, os.Environ(), "context", "--format", "json")
	var doc struct {
		Domain struct {
			Source string `json:"source"`
		} `json:"domain"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil || code != 0 || doc.Domain.Source != c.file {
		t.Fatalf("context json: %d %v %s", code, err, stdout)
	}
}

func (c domainFileCase) testAgents(t *testing.T) {
	root := c.fixture(t)
	write(t, root, c.file, c.orderingDomain(t))
	stdout := mustRunDomain(t, root, "agents", "md")
	if !strings.Contains(stdout, "record it in the project's domain recording through `arclint domain define`") {
		t.Fatalf("the workflow does not teach validated recording:\n%s", stdout)
	}
	if strings.Contains(stdout, c.file) || strings.Contains(stdout, c.other) {
		t.Fatalf("the block names a domain file:\n%s", stdout)
	}
	mustRunDomain(t, root, "agents", "md", "--write")
	block, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil || string(block) != stdout {
		t.Fatalf("AGENTS.md: %v\n%s", err, block)
	}

	home := t.TempDir()
	env := append(os.Environ(), "WSL_DISTRO_NAME=", "HOME="+home, "CLAUDE_CONFIG_DIR=", "CODEX_HOME=")
	installed, stderr, code := runBin(t, root, env, "agents", "hooks", "install", "--format", "json")
	var report struct{ Orientation string }
	if err := json.Unmarshal([]byte(installed), &report); err != nil || code != 0 || stderr != "" {
		t.Fatalf("hooks install: %d %v %s %s", code, err, installed, stderr)
	}
	if !strings.Contains(report.Orientation, "through `arclint domain define`") || strings.Contains(report.Orientation, c.file) {
		t.Fatalf("the install orientation depends on the domain file:\n%s", report.Orientation)
	}
}

func (c domainFileCase) testHooks(t *testing.T) {
	root := c.fixture(t)
	write(t, root, "src/other.go", "package src\n")
	write(t, root, "src/third.go", "package src\n")
	home := t.TempDir()
	env := append(os.Environ(), "WSL_DISTRO_NAME=", "HOME="+home, "CLAUDE_CONFIG_DIR=", "CODEX_HOME=")
	event := func(session string, fields map[string]any) string {
		t.Helper()
		fields["session_id"] = session
		fields["cwd"] = root
		input, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runBinStdin(t, root, env, string(input), "agents", "hooks", "event")
		if code != 0 || stderr != "" {
			t.Fatalf("event: %d %s %s", code, stdout, stderr)
		}
		return strings.TrimSpace(stdout)
	}
	start := func(session string) {
		t.Helper()
		event(session, map[string]any{"hook_event_name": "SessionStart", "source": "startup"})
	}
	edit := func(session, name string) string {
		return event(session, map[string]any{
			"hook_event_name": "PostToolUse", "tool_name": "Edit",
			"tool_input": map[string]any{"file_path": filepath.Join(root, filepath.FromSlash(name))},
		})
	}
	const contextAdvice = "changed before `arclint context` showed every Zone that owns it"
	domainAdvice := "changed before the project's domain recording changed in this session"

	// An agent that edits the project's domain file first has recorded the
	// meaning before the code: only the missing context is advised.
	start("edits-the-recording")
	if out := edit("edits-the-recording", c.file); out != "{}" {
		t.Fatalf("an edit of the project's domain file was advised: %s", out)
	}
	out := edit("edits-the-recording", "src/ok.go")
	if !strings.Contains(out, contextAdvice) || strings.Contains(out, domainAdvice) {
		t.Fatalf("code after an edit of %s: %s", c.file, out)
	}

	// The file the project does not use is only code.
	start("edits-the-other-file")
	if out := edit("edits-the-other-file", c.other); !strings.Contains(out, "ArcLint workflow: "+c.other+" "+domainAdvice) {
		t.Fatalf("an edit of %s was credited as a domain change: %s", c.other, out)
	}

	// domain define reports a changed recording to the session.
	start("defines-through-the-cli")
	mustRunDomain(t, root, "domain", "define", "bounded_context", "ordering", "--definition", "Taking and fulfilling customer orders.")
	out = edit("defines-through-the-cli", "src/other.go")
	if !strings.Contains(out, contextAdvice) || strings.Contains(out, domainAdvice) {
		t.Fatalf("code after `domain define`: %s", out)
	}

	// Without either, the advice names the project's file.
	start("edits-code-only")
	out = edit("edits-code-only", "src/third.go")
	if !strings.Contains(out, domainAdvice) {
		t.Fatalf("the advice does not name the project's domain file %s: %s", c.file, out)
	}
	if strings.Contains(out, c.other) {
		t.Fatalf("the advice names %s although the project records in %s: %s", c.other, c.file, out)
	}
}

// A ruleset that names a domain file it cannot use fails at once, naming
// the invariant, and so does one that gives a CML file this build cannot
// read yet.
func TestDomainFileRefusals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		file  string
		wants string
	}{
		{"an unknown extension", "docs/model.json", "format-named-by-extension"},
		{"an absolute path", "/etc/model.yaml", "inside-the-repository"},
		{"a path that leaves the repository", "../model.yaml", "inside-the-repository"},
		{"a Windows drive", "C:/model.yaml", "inside-the-repository"},
	} {
		root := t.TempDir()
		write(t, root, "rules.arclint.yaml", "runtime: [go]\ndomain:\n  file: "+tc.file+"\nzones:\n  src:\n    paths: [\"src/**\"]\n")
		for _, args := range [][]string{{"check"}, {"domain", "overview"}, {"context"}, {"agents", "md"}} {
			stdout, stderr, code := runBin(t, root, os.Environ(), args...)
			if code == 0 || !strings.Contains(stderr, tc.wants) || !strings.Contains(stderr, "domain.file: line 3") {
				t.Errorf("%s: %v: exit %d, want a refusal naming %s and line 3\nstdout: %s\nstderr: %s", tc.name, args, code, tc.wants, stdout, stderr)
			}
		}
	}
	// A broken domain file does not take down the commands that need no
	// ruleset, least of all the workflow hooks, which never block.
	broken := t.TempDir()
	write(t, broken, "rules.arclint.yaml", "runtime: [go]\ndomain:\n  file: notes.txt\nzones:\n  src:\n    paths: [\"src/**\"]\n")
	for _, args := range [][]string{{"--version"}, {"--help"}, {"help", "domain"}, {"__complete", "domain", ""}} {
		if stdout, stderr, code := runBin(t, broken, os.Environ(), args...); code != 0 {
			t.Errorf("%v with a broken domain file: exit %d\nstdout: %s\nstderr: %s", args, code, stdout, stderr)
		}
	}
	start, err := json.Marshal(map[string]any{"hook_event_name": "SessionStart", "session_id": "broken", "cwd": broken, "source": "startup"})
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	hookEnv := append(os.Environ(), "WSL_DISTRO_NAME=", "HOME="+home, "CLAUDE_CONFIG_DIR=", "CODEX_HOME=")
	if stdout, stderr, code := runBinStdin(t, broken, hookEnv, string(start), "agents", "hooks", "event"); code != 0 || stderr != "" {
		t.Errorf("a hook event with a broken domain file: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}

	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "runtime: [go]\ndomain:\n  file: docs/model.cml\nzones:\n  src:\n    paths: [\"src/**\"]\n")
	write(t, root, "docs/model.cml", "Domain shop\nBoundedContext {\n")
	_, stderr, code := runBin(t, root, os.Environ(), "domain", "overview")
	if code == 0 || !strings.Contains(stderr, "docs/model.cml:2:") {
		t.Errorf("a CML recording that does not parse: exit %d\nstderr: %s", code, stderr)
	}
}

// init drafts the domain key only for a file other than the default, and
// the draft is read back by the commands that follow.
func TestInitDomainFlag(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, code := runBin(t, root, os.Environ(), "init", "--domain", "./docs/model.arclint.yaml")
	if code != 0 {
		t.Fatalf("init --domain: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	ruleset, err := os.ReadFile(filepath.Join(root, "rules.arclint.yaml"))
	if err != nil || !strings.Contains(string(ruleset), "domain:\n") || !strings.Contains(string(ruleset), "\n  file: docs/model.arclint.yaml\n") {
		t.Fatalf("the drafted ruleset does not name the domain file: %v\n%s", err, ruleset)
	}
	if out := mustRunDomain(t, root, "domain", "init"); !strings.Contains(out, "Initialized docs/model.arclint.yaml") {
		t.Fatalf("domain init after init --domain: %s", out)
	}

	plain := t.TempDir()
	if _, stderr, code := runBin(t, plain, os.Environ(), "init", "--domain", "domain.arclint.yaml"); code != 0 {
		t.Fatalf("init --domain with the default file: exit %d\n%s", code, stderr)
	}
	ruleset, err = os.ReadFile(filepath.Join(plain, "rules.arclint.yaml"))
	if err != nil || !strings.Contains(string(ruleset), "domain:\n") || !strings.Contains(string(ruleset), "  file: domain.arclint.yaml\n") {
		t.Fatalf("an explicitly named default file is not written under domain.file: %v\n%s", err, ruleset)
	}

	refused := t.TempDir()
	_, stderr, code = runBin(t, refused, os.Environ(), "init", "--domain", "notes.txt")
	if code == 0 || !strings.Contains(stderr, "format-named-by-extension") {
		t.Fatalf("init --domain notes.txt: exit %d\n%s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(refused, "rules.arclint.yaml")); !os.IsNotExist(err) {
		t.Fatalf("a ruleset was drafted despite the refusal: %v", err)
	}
	if out, _, code := runBin(t, t.TempDir(), os.Environ(), "init", "--help"); code != 0 || !strings.Contains(out, "--domain") {
		t.Fatalf("init --help does not list --domain: %d\n%s", code, out)
	}
}

func mustDefaultRecording(t *testing.T) vocab.DomainRecording {
	t.Helper()
	recording, err := vocab.DefaultDomainRecording()
	if err != nil {
		t.Fatal(err)
	}
	return recording
}
