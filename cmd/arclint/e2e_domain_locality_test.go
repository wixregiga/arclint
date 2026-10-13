package main

// End-to-end coverage of how `arclint domain define` and `arclint domain remove`
// edit a CML DomainRecording, against the compiled binary. The inputs are the
// two real recordings the repository keeps in CML (arclint's own and the
// boxoffice proving ground). For every concept the command line supports the
// tests assert, with a real line diff, that the edit touches only the lines of
// the element it names, that defining a new element and removing it again gives
// back the original bytes, and that defining the same element twice reports
// the second as unchanged and leaves the file alone.
//
// Each case runs in a fresh copy of the recording in its own project, whose
// rules.arclint.yaml names the recording under domain.file. The line diff is a
// longest-common-subsequence diff over lines keyed by whether the line lies
// in the edited element's declaration. A plain line diff cannot tell which of
// several identical closing braces moved; a keyed one cannot match a line
// outside the element with a line inside it, so any rewritten line outside it
// shows as touched.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	cmlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/cml"
)

const cliRecordingRules = `runtime: [go]
domain:
  file: domain.arclint.cml
zones:
  src:
    paths: ["src/**"]
`

var cliRecordings = []struct{ name, path string }{
	{"arclint", filepath.Join("..", "..", "domain.arclint.cml")},
	{"boxoffice", filepath.Join("..", "..", "testing", "boxoffice", "domain.arclint.cml")},
}

func cliRecording(t *testing.T) vocab.DomainRecording {
	t.Helper()
	recording, err := vocab.NewDomainRecording("domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	return recording
}

// cliProject copies a recording into a fresh project.
func cliProject(t *testing.T, recording []byte) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", cliRecordingRules)
	write(t, root, "src/ok.go", "package src\n")
	write(t, root, "domain.arclint.cml", string(recording))
	return root
}

func recordedLanguage(t *testing.T, data []byte) vocab.UbiquitousLanguage {
	t.Helper()
	lang, err := cmlvocab.Parse(data, cliRecording(t))
	if err != nil {
		t.Fatal(err)
	}
	return lang
}

// cliElement names the entry a command edits: its concept as the command line
// spells it, and where it is recorded.
type cliElement struct {
	concept, context, owner, name, on string
}

// keyedLine is a non-blank line and whether it lies in the edited element.
type keyedLine struct {
	number int
	inside bool
	text   string
}

// lineDiff returns the lines of before that no longest common subsequence
// keeps and the lines of after that it does not contain. Lines match only when
// both their text and their side of the element boundary match.
func lineDiff(before, after []keyedLine) (deleted, inserted []keyedLine) {
	same := func(a, b keyedLine) bool { return a.inside == b.inside && a.text == b.text }
	n, m := len(before), len(after)
	table := make([][]int32, n+1)
	for i := range table {
		table[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			switch {
			case same(before[i], after[j]):
				table[i][j] = table[i+1][j+1] + 1
			case table[i+1][j] >= table[i][j+1]:
				table[i][j] = table[i+1][j]
			default:
				table[i][j] = table[i][j+1]
			}
		}
	}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case same(before[i], after[j]):
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			deleted = append(deleted, before[i])
			i++
		default:
			inserted = append(inserted, after[j])
			j++
		}
	}
	return append(deleted, before[i:]...), append(inserted, after[j:]...)
}

func indentation(line string) int { return len(line) - len(strings.TrimLeft(line, " \t")) }

// closingLine finds the line that closes the body the line at index opens, by
// the canonical layout's rule that a closing brace stands at the indentation
// of the declaration it closes. It returns the index of the opening line when
// the line opens no body.
func closingLine(lines []string, index int) int {
	if !strings.HasSuffix(strings.TrimRight(lines[index], " \t\r"), "{") {
		return index
	}
	for j := index + 1; j < len(lines); j++ {
		if indentation(lines[j]) == indentation(lines[index]) && strings.TrimSpace(lines[j]) == "}" {
			return j
		}
	}
	return index
}

// elementStart finds the line an element starts on, as the adapter reads it
// from the file: the line of its AST node.
func elementStart(lang vocab.UbiquitousLanguage, e cliElement) (int, bool) {
	switch e.concept {
	case "bounded_context":
		if c, ok := lang.Context(e.name); ok {
			return c.Line, true
		}
		return 0, false
	}
	c, ok := lang.Context(e.context)
	if !ok {
		return 0, false
	}
	switch e.concept {
	case "aggregate":
		if a, ok := c.Aggregate(e.name); ok {
			return a.Line, true
		}
	case "entity":
		if m, _, ok := c.Entity(e.name); ok {
			return m.Line, true
		}
	case "value_object":
		if v, ok := c.ValueObject(e.name); ok {
			return v.Line, true
		}
	case "domain_event":
		if v, ok := c.Event(e.name); ok {
			return v.Line, true
		}
	case "domain_service":
		if v, ok := c.Service(e.name); ok {
			return v.Line, true
		}
	case "specification":
		if v, ok := c.Specification(e.name); ok {
			return v.Line, true
		}
	case "question":
		if v, ok := c.Question(e.name); ok {
			return v.Line, true
		}
	case "invariant":
		if a, ok := c.Aggregate(e.owner); ok {
			if v, ok := a.Invariant(e.name); ok {
				return v.Line, true
			}
		}
		if o, ok := c.ValueObject(e.owner); ok {
			if v, ok := o.Invariant(e.name); ok {
				return v.Line, true
			}
		}
	case "assertion":
		if a, ok := c.Aggregate(e.owner); ok {
			if v, ok := a.Assertion(e.name); ok {
				return v.Line, true
			}
		}
		if s, ok := c.Service(e.owner); ok {
			if v, ok := s.Assertion(e.name); ok {
				return v.Line, true
			}
		}
	}
	return 0, false
}

// declaredLines returns the lines of the edited element in one recording: its
// leading comments and directives, its doc string, its declaration line and
// its body, together with the lines the edit may change around it: the
// opening and closing line of a value object or service that gains or loses
// its body with the element, the context map's contains clause when a context
// comes or goes, the operation declaration an assertion names, and the Module
// that a context's first value object, specification or event brings with it.
// other is the recording on the other side of the edit.
func declaredLines(t *testing.T, data, other []byte, e cliElement) map[int]bool {
	t.Helper()
	lines := strings.Split(string(data), "\n")
	inside := map[int]bool{}
	mark := func(from, to int) {
		for i := from; i <= to; i++ {
			inside[i+1] = true
		}
	}
	lang := recordedLanguage(t, data)
	if start, ok := elementStart(lang, e); ok {
		index := start - 1
		first := index
		if e.concept != "question" {
			for first > 0 && strings.HasPrefix(strings.TrimSpace(lines[first-1]), "//") && indentation(lines[first-1]) == indentation(lines[index]) {
				first--
			}
		}
		header := index
		for header < len(lines)-1 && strings.HasPrefix(strings.TrimSpace(lines[header]), `"`) {
			header++
		}
		last := closingLine(lines, header)
		if e.concept == "question" {
			header, last = index, index
		}
		if e.concept == "aggregate" {
			// The container opens above the root, and above the factory service a fresh
			// aggregate writes ahead of it.
			for i := index - 1; i >= 0; i-- {
				if strings.TrimSpace(lines[i]) != "" && indentation(lines[i]) < indentation(lines[index]) {
					if strings.HasPrefix(strings.TrimSpace(lines[i]), "Aggregate ") {
						first, last = i, closingLine(lines, i)
					}
					break
				}
			}
		}
		mark(first, last)
		switch e.concept {
		case "value_object", "specification", "domain_event":
			// A Module the edit created holds nothing else.
			if first > 0 && last+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[first-1]), "Module ") &&
				strings.TrimSpace(lines[last+1]) == "}" && !strings.Contains(string(other), strings.TrimSpace(lines[first-1])) {
				mark(first-1, last+1)
			}
		}
	}
	switch e.concept {
	case "bounded_context":
		// The context map lists the context, and removing a context removes the
		// relations that name it, with the description written above each.
		naming := regexp.MustCompile(`\b` + regexp.QuoteMeta(e.name) + `\b`)
		for i, line := range lines {
			text := strings.TrimSpace(line)
			if strings.HasPrefix(text, "contains ") {
				mark(i, i)
			}
			if indentation(line) == 2 && !strings.HasPrefix(text, "//") && !strings.HasPrefix(text, "contains ") && naming.MatchString(text) && strings.ContainsAny(text, "-<>") {
				first := i
				for first > 0 && strings.HasPrefix(strings.TrimSpace(lines[first-1]), "//") {
					first--
				}
				mark(first, closingLine(lines, i))
			}
		}
	case "invariant", "assertion":
		if owner := ownerStart(lang, e); owner > 0 {
			header := owner - 1
			for header < len(lines)-1 && strings.HasPrefix(strings.TrimSpace(lines[header]), `"`) {
				header++
			}
			if !strings.HasPrefix(strings.TrimSpace(lines[header]), "Entity ") {
				mark(header, header)
				mark(closingLine(lines, header), closingLine(lines, header))
			}
			if e.on != "" {
				declaration := regexp.MustCompile(`^\s*(def )?void ` + regexp.QuoteMeta(e.on) + `\(\);$`)
				for i := header; i <= closingLine(lines, header); i++ {
					if declaration.MatchString(lines[i]) {
						mark(i, i)
					}
				}
			}
		}
	}
	return inside
}

// ownerStart is the line of the value object or service an invariant or
// assertion is recorded under, or of its aggregate.
func ownerStart(lang vocab.UbiquitousLanguage, e cliElement) int {
	c, ok := lang.Context(e.context)
	if !ok {
		return 0
	}
	if v, ok := c.ValueObject(e.owner); ok {
		return v.Line
	}
	if s, ok := c.Service(e.owner); ok {
		return s.Line
	}
	if a, ok := c.Aggregate(e.owner); ok {
		return a.Line
	}
	return 0
}

func keyedLines(data []byte, inside map[int]bool) []keyedLine {
	var out []keyedLine
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, keyedLine{number: i + 1, inside: inside[i+1], text: line})
		}
	}
	return out
}

// requireOnlyElement fails unless every line the edit touched lies in the edited
// element. It returns what it touched.
func requireOnlyElement(t *testing.T, before, after []byte, e cliElement, doing string) (deleted, inserted []keyedLine) {
	t.Helper()
	deleted, inserted = lineDiff(
		keyedLines(before, declaredLines(t, before, after, e)),
		keyedLines(after, declaredLines(t, after, before, e)),
	)
	var outside []string
	for _, line := range deleted {
		if !line.inside {
			outside = append(outside, fmt.Sprintf("-%d %s", line.number, strings.TrimSpace(line.text)))
		}
	}
	for _, line := range inserted {
		if !line.inside {
			outside = append(outside, fmt.Sprintf("+%d %s", line.number, strings.TrimSpace(line.text)))
		}
	}
	if len(outside) > 0 {
		t.Fatalf("%s touched lines outside %s %q:\n  %s", doing, e.concept, e.name, strings.Join(outside, "\n  "))
	}
	return deleted, inserted
}

// exportOracleCopy writes an edited recording into the directory the
// CML_ORACLE_OUTPUT environment variable names, so that the real Context
// Mapper can check it; without the variable it writes nothing.
func exportOracleCopy(t *testing.T, name string, data []byte) {
	t.Helper()
	dir := os.Getenv("CML_ORACLE_OUTPUT")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	name = regexp.MustCompile(`[^A-Za-z0-9]+`).ReplaceAllString(name, "-")
	if err := os.WriteFile(filepath.Join(dir, "cli-"+strings.Trim(name, "-")+".cml"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readDomainFile(t *testing.T, root string) []byte {
	t.Helper()
	return []byte(readFile(t, root, "domain.arclint.cml"))
}

// cliCase is one `domain define` of a new element, with the commands that
// prepare the project and the command that removes the element again.
type cliCase struct {
	name    string
	setup   [][]string
	define  []string
	remove  []string
	element cliElement
	// leavesOperation says the removal keeps the operation declaration the
	// definition wrote for the assertion's On name: Record keeps every
	// operation, because it may be authored CML.
	leavesOperation bool
}

// cliCases builds the cases for a recording from the elements it holds.
func cliCases(base vocab.UbiquitousLanguage) []cliCase {
	var context, aggregate string
	for _, c := range base.Contexts {
		if len(c.Aggregates) > 0 {
			context, aggregate = c.Name, c.Aggregates[0].Name
			break
		}
	}
	scoped := func(args ...string) []string {
		return append(append([]string{"domain"}, args...), "--context", context)
	}
	removal := func(concept, name string, owner string) []string {
		args := []string{"domain", "remove", concept, name, "--context", context}
		if owner != "" {
			args = append(args, "--owner", owner)
		}
		return args
	}
	defineAggregateOperation := scoped("define", "assertion", "cli-first-promise", "--owner", aggregate, "--on", "CliOperation", "--statement", "The first promise on the operation.")
	return []cliCase{
		{
			name:    "bounded context",
			define:  []string{"domain", "define", "bounded_context", "cli_added", "--definition", "A context the command line added."},
			remove:  []string{"domain", "remove", "bounded_context", "cli_added"},
			element: cliElement{concept: "bounded_context", name: "cli_added"},
		},
		{
			name:    "aggregate with every property",
			define:  scoped("define", "aggregate", "CliAggregate", "--definition", "An aggregate the command line added.", "--identity", "CliAggregateID", "--alias", "Cli aggregate alias", "--repository", "CliAggregateRepository", "--factory", "CliAggregateFactory"),
			remove:  removal("aggregate", "CliAggregate", ""),
			element: cliElement{concept: "aggregate", context: context, name: "CliAggregate"},
		},
		{
			name:    "entity",
			define:  scoped("define", "entity", "CliEntity", "--owner", aggregate, "--definition", "An entity the command line added.", "--alias", "Cli entity alias"),
			remove:  removal("entity", "CliEntity", aggregate),
			element: cliElement{concept: "entity", context: context, owner: aggregate, name: "CliEntity"},
		},
		{
			name:    "value object",
			define:  scoped("define", "value_object", "CliValue", "--definition", "A value object the command line added.", "--alias", "Cli value alias"),
			remove:  removal("value_object", "CliValue", ""),
			element: cliElement{concept: "value_object", context: context, name: "CliValue"},
		},
		{
			name:    "value object in a context without a module",
			setup:   [][]string{{"domain", "define", "bounded_context", "cli_bare", "--definition", "A context with nothing in it."}},
			define:  []string{"domain", "define", "value_object", "CliLonely", "--context", "cli_bare", "--definition", "The first value object of its context."},
			remove:  []string{"domain", "remove", "value_object", "CliLonely", "--context", "cli_bare"},
			element: cliElement{concept: "value_object", context: "cli_bare", name: "CliLonely"},
		},
		{
			name:    "invariant of an aggregate",
			define:  scoped("define", "invariant", "cli-promise", "--owner", aggregate, "--statement", "An invariant the command line added."),
			remove:  removal("invariant", "cli-promise", aggregate),
			element: cliElement{concept: "invariant", context: context, owner: aggregate, name: "cli-promise"},
		},
		{
			name:    "invariant of a value object",
			setup:   [][]string{scoped("define", "value_object", "CliBare", "--definition", "A value object with no invariant yet.")},
			define:  scoped("define", "invariant", "cli-value-promise", "--owner", "CliBare", "--statement", "An invariant of a value object."),
			remove:  removal("invariant", "cli-value-promise", "CliBare"),
			element: cliElement{concept: "invariant", context: context, owner: "CliBare", name: "cli-value-promise"},
		},
		{
			name:    "assertion on an operation the root declares",
			setup:   [][]string{defineAggregateOperation},
			define:  scoped("define", "assertion", "cli-second-promise", "--owner", aggregate, "--on", "CliOperation", "--statement", "The second promise on the operation."),
			remove:  removal("assertion", "cli-second-promise", aggregate),
			element: cliElement{concept: "assertion", context: context, owner: aggregate, name: "cli-second-promise", on: "CliOperation"},
		},
		{
			name:            "assertion on a new operation",
			define:          scoped("define", "assertion", "cli-promise-on-new", "--owner", aggregate, "--on", "CliNewOperation", "--statement", "A promise on an operation the root does not declare yet."),
			remove:          removal("assertion", "cli-promise-on-new", aggregate),
			element:         cliElement{concept: "assertion", context: context, owner: aggregate, name: "cli-promise-on-new", on: "CliNewOperation"},
			leavesOperation: true,
		},
		{
			name:            "assertion of a domain service",
			setup:           [][]string{scoped("define", "domain_service", "CliService", "--definition", "A service with no assertion yet.")},
			define:          scoped("define", "assertion", "cli-service-promise", "--owner", "CliService", "--on", "Advise", "--statement", "A promise of a service."),
			remove:          removal("assertion", "cli-service-promise", "CliService"),
			element:         cliElement{concept: "assertion", context: context, owner: "CliService", name: "cli-service-promise", on: "Advise"},
			leavesOperation: true,
		},
		{
			name:    "business rule that is an invariant",
			define:  scoped("define", "business_rule", "cli-rule-invariant", "--owner", aggregate, "--statement", "A rule without an operation is an invariant."),
			remove:  removal("business_rule", "cli-rule-invariant", aggregate),
			element: cliElement{concept: "invariant", context: context, owner: aggregate, name: "cli-rule-invariant"},
		},
		{
			name:            "business rule that is an assertion",
			define:          scoped("define", "business_rule", "cli-rule-assertion", "--owner", aggregate, "--on", "CliRuleOperation", "--statement", "A rule on an operation is an assertion."),
			remove:          removal("business_rule", "cli-rule-assertion", aggregate),
			element:         cliElement{concept: "assertion", context: context, owner: aggregate, name: "cli-rule-assertion", on: "CliRuleOperation"},
			leavesOperation: true,
		},
		{
			name:    "domain event",
			define:  scoped("define", "domain_event", "CliEvent", "--definition", "An event the command line added.", "--raised-by", aggregate),
			remove:  removal("domain_event", "CliEvent", ""),
			element: cliElement{concept: "domain_event", context: context, name: "CliEvent"},
		},
		{
			name:    "domain service",
			define:  scoped("define", "domain_service", "CliService", "--definition", "A service the command line added."),
			remove:  removal("domain_service", "CliService", ""),
			element: cliElement{concept: "domain_service", context: context, name: "CliService"},
		},
		{
			name:    "specification",
			define:  scoped("define", "specification", "CliSpecification", "--definition", "A specification the command line added."),
			remove:  removal("specification", "CliSpecification", ""),
			element: cliElement{concept: "specification", context: context, name: "CliSpecification"},
		},
		{
			name:    "question",
			define:  scoped("define", "question", "cli-question", "--text", "A question the command line added?"),
			remove:  removal("question", "cli-question", ""),
			element: cliElement{concept: "question", context: context, name: "cli-question"},
		},
	}
}

func runDefinitions(t *testing.T, root string, commands ...[]string) {
	t.Helper()
	for _, args := range commands {
		mustRunDomain(t, root, args...)
	}
}

// backdated gives the project's recording a modification time long past and
// returns its state, so that any later write shows.
func backdated(t *testing.T, root string) (data []byte, info os.FileInfo) {
	t.Helper()
	path := filepath.Join(root, "domain.arclint.cml")
	past := time.Date(2001, time.February, 3, 4, 5, 6, 0, time.UTC)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return readDomainFile(t, root), info
}

func requireUntouched(t *testing.T, root string, data []byte, info os.FileInfo, doing string) {
	t.Helper()
	path := filepath.Join(root, "domain.arclint.cml")
	now, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(readDomainFile(t, root)) != string(data) || !now.ModTime().Equal(info.ModTime()) || !os.SameFile(info, now) {
		t.Fatalf("%s rewrote the recording (mtime %v -> %v, same file %v)", doing, info.ModTime(), now.ModTime(), os.SameFile(info, now))
	}
}

func TestDomainDefineAndRemoveTouchOnlyTheirElement(t *testing.T) {
	for _, source := range cliRecordings {
		committed, err := os.ReadFile(source.path)
		if err != nil {
			t.Fatal(err)
		}
		base := recordedLanguage(t, committed)
		for _, c := range cliCases(base) {
			t.Run(source.name+"/"+c.name, func(t *testing.T) {
				root := cliProject(t, committed)
				runDefinitions(t, root, c.setup...)
				before := readDomainFile(t, root)

				stdout := mustRunDomain(t, root, c.define...)
				if !strings.HasPrefix(stdout, "Defined ") {
					t.Fatalf("%v said %q", c.define, stdout)
				}
				defined := readDomainFile(t, root)
				if string(defined) == string(before) {
					t.Fatalf("%v changed nothing", c.define)
				}
				if _, inserted := requireOnlyElement(t, before, defined, c.element, "defining"); len(inserted) == 0 {
					t.Fatalf("defining inserted no line of the element")
				}
				exportOracleCopy(t, source.name+" "+c.name+" defined", defined)

				// The same command again records nothing and touches nothing.
				settled, info := backdated(t, root)
				again := mustRunDomain(t, root, c.define...)
				if !strings.HasPrefix(again, "Unchanged ") {
					t.Fatalf("the second %v said %q, want it unchanged", c.define, again)
				}
				requireUntouched(t, root, settled, info, "the second define")

				mustRunDomain(t, root, c.remove...)
				removed := readDomainFile(t, root)
				exportOracleCopy(t, source.name+" "+c.name+" removed", removed)
				requireOnlyElement(t, defined, removed, c.element, "removing")
				if !c.leavesOperation {
					if string(removed) != string(before) {
						t.Fatalf("defining and removing %q did not give the original bytes back:\n%s", c.element.name, firstLineDifference(before, removed))
					}
					return
				}
				// Record keeps an operation declaration when the assertion that named it goes:
				// the declaration may be authored CML. The removal leaves that, and the braces
				// that now hold it, and nothing else.
				gone, left := lineDiff(keyedLines(before, nil), keyedLines(removed, nil))
				operation := regexp.MustCompile(`^\s*(def )?void ` + regexp.QuoteMeta(c.element.on) + `\(\);$`)
				sawOperation := false
				for _, line := range gone {
					if !strings.HasPrefix(strings.TrimSpace(line.text), "Service ") {
						t.Fatalf("removing the assertion lost %q", line.text)
					}
				}
				for _, line := range left {
					switch {
					case operation.MatchString(line.text):
						sawOperation = true
					case strings.HasPrefix(strings.TrimSpace(line.text), "Service ") || strings.TrimSpace(line.text) == "}":
					default:
						t.Fatalf("removing the assertion left %q behind", line.text)
					}
				}
				if !sawOperation {
					t.Fatalf("removing the assertion removed the operation declaration %q too", c.element.on)
				}
			})
		}
	}
}

func firstLineDifference(a, b []byte) string {
	x, y := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	for i := 0; i < len(x) && i < len(y); i++ {
		if x[i] != y[i] {
			return fmt.Sprintf("line %d: %q against %q", i+1, x[i], y[i])
		}
	}
	return fmt.Sprintf("%d lines against %d", len(x), len(y))
}

// cliTarget is an element the recording already holds, with the commands that
// change and remove it. Each update and each removal runs in a fresh copy.
type cliTarget struct {
	element cliElement
	// definition rewrites the text the element is defined by; it must replace
	// exactly one line.
	definition []string
	// others change the element's other properties.
	others [][]string
	remove []string
}

const changedText = "A text the command line wrote."

// cliTargets takes the first element of each concept from the recording.
func cliTargets(base vocab.UbiquitousLanguage) []cliTarget {
	var targets []cliTarget
	seen := map[string]bool{}
	first := func(concept string) bool {
		if seen[concept] {
			return false
		}
		seen[concept] = true
		return true
	}
	remove := func(concept, context, owner, name string) []string {
		args := []string{"domain", "remove", concept, name}
		if context != "" {
			args = append(args, "--context", context)
		}
		if owner != "" {
			args = append(args, "--owner", owner)
		}
		return args
	}
	for _, c := range base.Contexts {
		if first("bounded_context") {
			targets = append(targets, cliTarget{
				element:    cliElement{concept: "bounded_context", name: c.Name},
				definition: []string{"domain", "define", "bounded_context", c.Name, "--definition", changedText},
				remove:     remove("bounded_context", "", "", c.Name),
			})
		}
		for _, a := range c.Aggregates {
			if first("aggregate") {
				at := []string{"--context", c.Name}
				targets = append(targets, cliTarget{
					element:    cliElement{concept: "aggregate", context: c.Name, name: a.Name},
					definition: append([]string{"domain", "define", "aggregate", a.Name, "--definition", changedText}, at...),
					others: [][]string{
						append([]string{"domain", "define", "aggregate", a.Name, "--alias", "Cli alias"}, at...),
						append([]string{"domain", "define", "aggregate", a.Name, "--repository", "CliStore"}, at...),
						append([]string{"domain", "define", "aggregate", a.Name, "--factory", "CliFactory"}, at...),
						append([]string{"domain", "define", "aggregate", a.Name, "--identity", "CliChangedID"}, at...),
					},
					remove: remove("aggregate", c.Name, "", a.Name),
				})
			}
			for _, e := range a.Entities {
				if first("entity") {
					targets = append(targets, cliTarget{
						element:    cliElement{concept: "entity", context: c.Name, owner: a.Name, name: e.Name},
						definition: []string{"domain", "define", "entity", e.Name, "--context", c.Name, "--definition", changedText},
						others:     [][]string{{"domain", "define", "entity", e.Name, "--context", c.Name, "--alias", "Cli entity alias"}},
						remove:     remove("entity", c.Name, a.Name, e.Name),
					})
				}
			}
			for _, i := range a.Invariants {
				if first("invariant") {
					targets = append(targets, cliTarget{
						element:    cliElement{concept: "invariant", context: c.Name, owner: a.Name, name: i.Key},
						definition: []string{"domain", "define", "invariant", i.Key, "--context", c.Name, "--owner", a.Name, "--statement", changedText},
						remove:     remove("invariant", c.Name, a.Name, i.Key),
					})
				}
			}
			for _, as := range a.Assertions {
				if first("assertion") {
					targets = append(targets, cliTarget{
						element:    cliElement{concept: "assertion", context: c.Name, owner: a.Name, name: as.Key, on: as.On},
						definition: []string{"domain", "define", "assertion", as.Key, "--context", c.Name, "--owner", a.Name, "--statement", changedText},
						others:     [][]string{{"domain", "define", "assertion", as.Key, "--context", c.Name, "--owner", a.Name, "--on", as.On}},
						remove:     remove("assertion", c.Name, a.Name, as.Key),
					})
				}
			}
		}
		for _, v := range c.ValueObjects {
			if first("value_object") {
				targets = append(targets, cliTarget{
					element:    cliElement{concept: "value_object", context: c.Name, name: v.Name},
					definition: []string{"domain", "define", "value_object", v.Name, "--context", c.Name, "--definition", changedText},
					others:     [][]string{{"domain", "define", "value_object", v.Name, "--context", c.Name, "--alias", "Cli value alias"}},
					remove:     remove("value_object", c.Name, "", v.Name),
				})
			}
			if len(v.Invariants) > 0 && first("value object invariant") {
				i := v.Invariants[0]
				targets = append(targets, cliTarget{
					element:    cliElement{concept: "invariant", context: c.Name, owner: v.Name, name: i.Key},
					definition: []string{"domain", "define", "invariant", i.Key, "--context", c.Name, "--owner", v.Name, "--statement", changedText},
					remove:     remove("invariant", c.Name, v.Name, i.Key),
				})
			}
		}
		for _, e := range c.Events {
			if first("domain_event") {
				others := [][]string{}
				if len(c.Aggregates) > 0 {
					others = append(others, []string{"domain", "define", "domain_event", e.Name, "--context", c.Name, "--raised-by", c.Aggregates[0].Name})
				}
				others = append(others, []string{"domain", "define", "domain_event", e.Name, "--context", c.Name, "--raised-by", ""})
				targets = append(targets, cliTarget{
					element:    cliElement{concept: "domain_event", context: c.Name, name: e.Name},
					definition: []string{"domain", "define", "domain_event", e.Name, "--context", c.Name, "--definition", changedText},
					others:     others,
					remove:     remove("domain_event", c.Name, "", e.Name),
				})
			}
		}
		for _, s := range c.Services {
			if first("domain_service") {
				targets = append(targets, cliTarget{
					element:    cliElement{concept: "domain_service", context: c.Name, name: s.Name},
					definition: []string{"domain", "define", "domain_service", s.Name, "--context", c.Name, "--definition", changedText},
					remove:     remove("domain_service", c.Name, "", s.Name),
				})
			}
			for _, as := range s.Assertions {
				if first("service assertion") {
					targets = append(targets, cliTarget{
						element:    cliElement{concept: "assertion", context: c.Name, owner: s.Name, name: as.Key, on: as.On},
						definition: []string{"domain", "define", "assertion", as.Key, "--context", c.Name, "--owner", s.Name, "--statement", changedText},
						remove:     remove("assertion", c.Name, s.Name, as.Key),
					})
				}
			}
		}
		for _, s := range c.Specifications {
			if first("specification") {
				targets = append(targets, cliTarget{
					element:    cliElement{concept: "specification", context: c.Name, name: s.Name},
					definition: []string{"domain", "define", "specification", s.Name, "--context", c.Name, "--definition", changedText},
					remove:     remove("specification", c.Name, "", s.Name),
				})
			}
		}
		for _, q := range c.Questions {
			if first("question") {
				targets = append(targets, cliTarget{
					element:    cliElement{concept: "question", context: c.Name, name: q.Key},
					definition: []string{"domain", "define", "question", q.Key, "--context", c.Name, "--text", changedText},
					remove:     remove("question", c.Name, "", q.Key),
				})
			}
		}
	}
	return targets
}

func TestDomainDefineOfAnExistingElementAndRemoveTouchOnlyIt(t *testing.T) {
	for _, source := range cliRecordings {
		committed, err := os.ReadFile(source.path)
		if err != nil {
			t.Fatal(err)
		}
		targets := cliTargets(recordedLanguage(t, committed))
		for _, target := range targets {
			label := source.name + "/" + target.element.concept + " " + target.element.name
			t.Run(label+"/definition", func(t *testing.T) {
				root := cliProject(t, committed)
				before := readDomainFile(t, root)
				mustRunDomain(t, root, target.definition...)
				after := readDomainFile(t, root)
				exportOracleCopy(t, label+" definition", after)
				deleted, inserted := requireOnlyElement(t, before, after, target.element, "changing the definition")
				if len(deleted) != 1 || len(inserted) != 1 || !strings.Contains(inserted[0].text, changedText) {
					t.Fatalf("changing the text of %s %q replaced %d lines with %d, want one line with the new text", target.element.concept, target.element.name, len(deleted), len(inserted))
				}
			})
			for _, other := range target.others {
				t.Run(label+"/"+strings.Join(other[3:], " "), func(t *testing.T) {
					root := cliProject(t, committed)
					before := readDomainFile(t, root)
					mustRunDomain(t, root, other...)
					after := readDomainFile(t, root)
					exportOracleCopy(t, label+" "+strings.Join(other[3:], " "), after)
					requireOnlyElement(t, before, after, target.element, "changing a property")
					settled, info := backdated(t, root)
					if again := mustRunDomain(t, root, other...); !strings.HasPrefix(again, "Unchanged ") {
						t.Fatalf("the second %v said %q", other, again)
					}
					requireUntouched(t, root, settled, info, "the second define")
				})
			}
			t.Run(label+"/remove", func(t *testing.T) {
				root := cliProject(t, committed)
				before := readDomainFile(t, root)
				mustRunDomain(t, root, target.remove...)
				after := readDomainFile(t, root)
				exportOracleCopy(t, label+" remove", after)
				deleted, _ := requireOnlyElement(t, before, after, target.element, "removing")
				if len(deleted) == 0 {
					t.Fatalf("removing %s %q deleted no line", target.element.concept, target.element.name)
				}
			})
		}
		if len(targets) < 6 {
			t.Fatalf("%s holds too few elements to test: %d", source.name, len(targets))
		}
	}
}
