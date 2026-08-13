package config

// This file is the single source of human documentation for every builtin
// rule type (M4 ADR, docs/decisions.md). The table drives three surfaces
// that must never drift apart:
//
//   - `arclint explain [kind]` (terminal docs)
//   - description fields patched into the published JSON Schema
//     (editor hovers and completion)
//   - generated reference pages for the docs site
//
// Extension rule types document themselves through defineRule's
// `description` field; `arclint explain` merges both sources.

// RuleDoc documents one builtin rule kind.
type RuleDoc struct {
	// Kind is the identifier used in rules.yaml (`kind:` for provides,
	// invariants, and dependencies; "consumes" and "modules" document the
	// two structural mappings; "scan" documents the walk policy).
	Kind string
	// Where states the YAML location the kind appears at.
	Where string
	// Clause is consumes | provides | invariant | "" for structural docs.
	Clause string
	// Blame is consumer | provider | "" for structural docs.
	Blame string
	// Summary is one sentence, shown in listings.
	Summary string
	// Doc is the full explanation, terminal-width prose.
	Doc string
	// Example is a ready-to-paste YAML snippet.
	Example string
}

// RuleDocs documents every builtin kind, in presentation order.
var RuleDocs = []RuleDoc{
	{
		Kind:    "modules",
		Where:   "top-level `modules:`",
		Summary: "Name the parts of your repository; every rule refers to these names.",
		Doc: `A module is a named set of files, defined by path globs. Modules are
the vocabulary of every other rule: contracts, layers, and protections
all refer to module names, never to raw paths.

A glob matches files directly, and a glob naming a directory owns the
whole subtree ("internal/features/*" covers every file under each
feature directory). Overlapping modules are legal: a file may belong to
several modules at once.

The mapping form adds a description, shown by "arclint module ls" and
"arclint module info <name>".`,
		Example: `modules:
  cmd: ["cmd/**"]
  entities:
    paths: ["internal/entities/**"]
    description: "Domain types and invariants; depends on nothing."`,
	},
	{
		Kind:    "consumes",
		Where:   "contracts.<module>.consumes",
		Clause:  "consumes",
		Blame:   "consumer",
		Summary: "What a module may depend on: other modules, third-party, stdlib.",
		Doc: `The consumes clause is a module's precondition: what its files may
import. Every import is classified first, then judged:

  internal  the import resolves to a file inside this repository
            (another module, or undeclared internal code)
  external  a third-party dependency declared in your manifest
            (go.mod require, package.json dependencies, pyproject)
  stdlib    the language's standard library
  unknown   none of the above (policy: scan.unknown_imports)

"internal:" as a list is an allow-list of module names this module may
import ([] means: no other declared module). The mapping form
{allow: [...], deny: [...]} expresses both directions. "external:
forbid" bans third-party imports; "stdlib: forbid" bans the standard
library. A violation blames the importing file (consumer blame).`,
		Example: `contracts:
  entities:
    consumes:
      internal: []        # may import no other declared module
      external: forbid    # no third-party libraries
      stdlib: allow`,
	},
	{
		Kind:    "registration",
		Where:   "contracts.<module>.provides",
		Clause:  "provides",
		Blame:   "provider",
		Summary: "Every instance of a shape must register itself somewhere.",
		Doc: `A registration obligation: for every capture of "each" (a regex over
the module's file paths), the "match" pattern (a regex template over
those captures) must hit inside the files of the "in" module. It
expresses "every feature wires itself into the registry", with the
feature list derived from the tree, never maintained by hand.

A violation blames the module that failed its promise (provider
blame), anchored at the unregistered capture.`,
		Example: `contracts:
  features:
    provides:
      - kind: registration
        each: 'internal/features/(?P<feature>[^/]+)/'
        in: registry
        match: 'Register\("{feature}"\)'`,
	},
	{
		Kind:    "correspondence",
		Where:   "contracts.<module>.provides",
		Clause:  "provides",
		Blame:   "provider",
		Summary: "A value set derived from one side must exist on the other side.",
		Doc: `A correspondence obligation: derive a named value set from path (or
content) captures on the "of" side, another set on the "in" side, and
assert a relation between them: "subset" (every of-value exists in
in-values) or "equal". It expresses shape symmetry like "every entity
substrate has a setup counterpart" without listing either side.

The "files" regexes are full-path matches; when "content" is set,
values come from content captures per matching file. "value" is a
template over named captures, like "{substrate}".`,
		Example: `contracts:
  entities:
    provides:
      - kind: correspondence
        of: { files: 'internal/entities/[^/]+_(?P<db>[a-z0-9]+)\.go', value: "{db}" }
        in: { files: 'internal/setup/(?:[^/]+/)*(?P<db>[a-z0-9]+)\.go', value: "{db}" }
        relation: subset`,
	},
	{
		Kind:    "naming",
		Where:   "contracts.<module>.invariants",
		Clause:  "invariant",
		Blame:   "provider",
		Summary: "File names follow a case convention or regex.",
		Doc: `Applies a naming convention to the file stem (base name minus the
final extension). Cases: kebab-case, snake_case, camelCase,
PascalCase, or regex:<pattern>; combine alternatives with "|" for
any-of. "files" narrows the rule to a glob inside the module.`,
		Example: `contracts:
  src:
    invariants:
      - kind: naming
        files: "internal/**/*.go"
        case: snake_case`,
	},
	{
		Kind:    "structure",
		Where:   "contracts.<module>.invariants",
		Clause:  "invariant",
		Blame:   "provider",
		Summary: "Paths that must exist (require) or must not (forbid).",
		Doc: `Asserts the shape of the tree itself: "require" globs must match at
least one file; "forbid" globs must match none. Use it to guarantee
entrypoints exist and to ban dumping grounds or legacy directories.`,
		Example: `contracts:
  repo:
    invariants:
      - kind: structure
        require: ["cmd/*/main.go"]
        forbid: ["**/utils.go", "internal/misc/**"]`,
	},
	{
		Kind:    "content",
		Where:   "contracts.<module>.invariants",
		Clause:  "invariant",
		Blame:   "provider",
		Summary: "File contents must (or must not) match regexes.",
		Doc: `Scans the module's files: every "must" regex has to match somewhere
in each file, and no "must_not" regex may match anywhere. "files"
narrows the scope with a glob. Violations anchor at the matching line.`,
		Example: `contracts:
  report:
    invariants:
      - kind: content
        files: "internal/report/**/*.go"
        must_not: ['\bpanic\(']`,
	},
	{
		Kind:    "expr",
		Where:   "contracts.<module>.invariants",
		Clause:  "invariant",
		Blame:   "provider",
		Summary: "An expr predicate over each file; false is a violation.",
		Doc: `Where the closed vocabulary runs out but a full extension is
overkill: "assert" is an expr-lang predicate evaluated per file,
type-checked at load time. The "file" variable exposes path, lines,
and imports. "message" overrides the default violation text.

expr is an existing, documented language (expr-lang.org); arclint
invents no syntax.`,
		Example: `contracts:
  src:
    invariants:
      - kind: expr
        files: "internal/**/*.go"
        assert: "file.lines <= 400"
        message: "keep source files under 400 lines"`,
	},
	{
		Kind:    "layers",
		Where:   "top-level `dependencies:`",
		Clause:  "consumes",
		Blame:   "consumer",
		Summary: "An ordered stack: a module imports only same or lower layers.",
		Doc: `Orders modules highest first. A module may import its own layer or
lower layers, never a higher one. This is the classic layered
architecture contract (import-linter's "layers"), covering the whole
graph in one rule.`,
		Example: `dependencies:
  - kind: layers
    layers: [cmd, app, domain]   # cmd -> app -> domain, never upward`,
	},
	{
		Kind:    "forbidden",
		Where:   "top-level `dependencies:`",
		Clause:  "consumes",
		Blame:   "consumer",
		Summary: "No module in `from` may import any module in `to`.",
		Doc: `A directed ban between module sets. Use it for one-off edges the
layer stack does not express, like "nothing in domain touches the
HTTP adapters".`,
		Example: `dependencies:
  - kind: forbidden
    from: [domain]
    to: [http, database]`,
	},
	{
		Kind:    "independence",
		Where:   "top-level `dependencies:`",
		Clause:  "consumes",
		Blame:   "consumer",
		Summary: "Sibling modules must not import each other.",
		Doc: `Every module in the set is independent of every other: no import in
either direction. This is the feature-slice contract: features
communicate through shared concepts, never directly.`,
		Example: `dependencies:
  - kind: independence
    modules: [borrowbook, returnbook, reservebook]`,
	},
	{
		Kind:    "protected",
		Where:   "top-level `dependencies:`",
		Clause:  "consumes",
		Blame:   "consumer",
		Summary: "A module only importable by an allow-listed set.",
		Doc: `Guards a module from the importer side: only modules in "allow" may
import it. A file is an allowed importer when any of its modules is
in the allow set, so umbrella modules do not create false positives.`,
		Example: `dependencies:
  - kind: protected
    module: database
    allow: [repositories]`,
	},
	{
		Kind:    "acyclic",
		Where:   "top-level `dependencies:`",
		Clause:  "consumes",
		Blame:   "consumer",
		Summary: "No import cycles among the named modules.",
		Doc: `Asserts the module graph has no cycles. An empty module list covers
every declared module. The violation reports one full cycle path.`,
		Example: `dependencies:
  - kind: acyclic
    modules: []   # all declared modules`,
	},
	{
		Kind:    "scan",
		Where:   "top-level `scan:`",
		Summary: "Walk policy: excludes, testdata, unknown-import severity.",
		Doc: `Tunes the tree walk and classification policy. "exclude" adds glob
patterns to the built-in exclusions (.git, vendor, node_modules,
.arclint). "include_testdata: true" scans testdata directories, which
Go convention excludes. "unknown_imports" sets what happens when an
import classifies neither internal, external, nor stdlib: warn
(default), error, or ignore.`,
		Example: `scan:
  exclude: ["gen/**"]
  unknown_imports: error`,
	},
}

// FindRuleDoc returns the doc for a builtin kind, or nil.
func FindRuleDoc(kind string) *RuleDoc {
	for i := range RuleDocs {
		if RuleDocs[i].Kind == kind {
			return &RuleDocs[i]
		}
	}
	return nil
}

// fieldDescriptions maps schema definition and field names to the hover
// text patched into the published JSON Schema. Kept beside RuleDocs so
// every human-facing string lives in this file.
var fieldDescriptions = map[string]map[string]string{
	"": { // root
		"runtime":      "Language targets to analyze: go, ts, py. Activates per-language import extraction and classification.",
		"modules":      "Named file sets (path globs) that every rule refers to. A value is a glob list, or {paths, description} to document the module.",
		"contracts":    "Per-module contracts keyed by module name: consumes (what it may import), provides (what it must supply), invariants (what always holds).",
		"dependencies": "Graph-wide dependency rules spanning modules: layers, forbidden, independence, protected, acyclic.",
		"rules":        "Instances of TypeScript extension rule types from .arclint/extensions/, validated against each extension's declared params schema.",
		"scan":         "Tree walk tuning: exclude globs, testdata inclusion, unknown-import policy.",
	},
	"ModuleDef": {
		"paths":       "Path globs defining membership. A glob naming a directory owns its whole subtree.",
		"description": "What this module is for; shown by arclint module ls/info.",
	},
	"Consumes": {
		"id":       "Optional stable rule id for this clause (default <module>.consumes); a pattern binds it to a namespaced requirement id.",
		"internal": "Which declared modules this module may import. A list is an allow-list ([] = none); {allow, deny} expresses both directions.",
		"external": "Third-party imports (declared in go.mod/package.json/pyproject): allow (default) or forbid.",
		"stdlib":   "Standard-library imports: allow (default) or forbid.",
		"severity": "Violation severity: error (default), warn, or info.",
	},
	"ProvidesRule": {
		"kind":     "registration: every capture of `each` must have a `match` hit in the `in` module. correspondence: the value set of `of` relates to the value set of `in`.",
		"each":     "Regex over the module's file paths; each named-capture tuple creates one obligation (registration).",
		"match":    "Regex template over `each` captures that must hit inside the `in` module's files (registration).",
		"of":       "Capture side deriving the obligated value set (correspondence).",
		"relation": "subset (default): every of-value exists in in-values; equal: both sets match exactly (correspondence).",
	},
	"InvariantRule": {
		"kind":    "naming: file-stem case convention. structure: require/forbid path globs. content: must/must_not regexes. expr: a predicate over each file.",
		"files":   "Glob narrowing the rule to matching files inside the module.",
		"case":    "kebab-case, snake_case, camelCase, PascalCase, or regex:<pattern>; combine alternatives with |.",
		"assert":  "expr-lang predicate over `file` (path, lines, imports); false is a violation.",
		"message": "Overrides the default violation text (expr).",
	},
	"GraphRule": {
		"kind":    "layers: ordered stack. forbidden: from-set must not import to-set. independence: no imports among siblings. protected: importable only by allow. acyclic: no cycles.",
		"layers":  "Module names ordered highest first; imports go same-or-lower only.",
		"modules": "Module set for independence/acyclic (empty acyclic = all declared).",
		"module":  "The protected module.",
		"allow":   "Modules allowed to import the protected module.",
	},
	"ScanConfig": {
		"exclude":          "Glob patterns excluded from the walk, in addition to built-ins (.git, vendor, node_modules, .arclint).",
		"include_testdata": "Scan testdata directories, which Go convention excludes by default.",
		"unknown_imports":  "Policy for imports that classify neither internal, external, nor stdlib: warn (default), error, ignore.",
	},
	"ExtensionRule": {
		"type":     "Extension rule type name, registered by a defineRule() in .arclint/extensions/.",
		"params":   "Rule parameters, validated against the extension's declared JSON Schema before any extension code runs.",
		"severity": "Violation severity: error (default), warn, or info.",
	},
}
