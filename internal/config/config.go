// Package config defines the rules.yaml data model. Rules are pure data:
// every rule instance names a kind plus params, validated by a published
// JSON Schema and then semantically (module references, regex compilation,
// expr type-checking) before anything runs.
package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// RuleSet is the parsed, validated content of one rules.yaml.
type RuleSet struct {
	// Runtime lists language targets: "go", "ts", "py".
	Runtime []string `yaml:"runtime" json:"runtime"`
	// Scan tunes tree walking and import-classification policy.
	Scan ScanConfig `yaml:"scan" json:"scan,omitempty"`
	// Modules maps a module name to the glob set that defines membership.
	Modules map[string][]string `yaml:"modules" json:"modules"`
	// Contracts keys module names declared in Modules.
	Contracts map[string]ModuleContract `yaml:"contracts" json:"contracts,omitempty"`
	// Dependencies holds graph-wide consumes clauses spanning modules.
	Dependencies []GraphRule `yaml:"dependencies" json:"dependencies,omitempty"`
	// Rules holds layer-2 extension rule instances: pure data naming a
	// rule type registered by .arclint/extensions plus params that the
	// host validates against the extension's declared JSON Schema before
	// the extension ever runs.
	Rules []ExtensionRule `yaml:"rules" json:"rules,omitempty"`

	// Path is the absolute path of the loaded rules.yaml. Not serialized.
	Path string `yaml:"-" json:"-"`
	// Root is the repo root (directory containing rules.yaml). Not serialized.
	Root string `yaml:"-" json:"-"`
	// SHA256 fingerprints the raw rules.yaml bytes. Not serialized.
	SHA256 string `yaml:"-" json:"-"`
}

// ScanConfig controls the file walk and unknown-import policy.
type ScanConfig struct {
	// Exclude adds repo-relative glob patterns to the built-in exclusions
	// (.git, vendor/ and, unless IncludeTestdata, testdata/).
	Exclude []string `yaml:"exclude" json:"exclude,omitempty"`
	// IncludeTestdata includes testdata/ directories, which Go convention
	// excludes by default.
	IncludeTestdata bool `yaml:"include_testdata" json:"include_testdata,omitempty"`
	// UnknownImports is the policy for imports that classify neither
	// internal, stdlib, nor external: warn (default), error, or ignore.
	UnknownImports string `yaml:"unknown_imports" json:"unknown_imports,omitempty"`
}

// ModuleContract groups the three clause kinds for one module.
type ModuleContract struct {
	Consumes   *Consumes       `yaml:"consumes" json:"consumes,omitempty"`
	Provides   []ProvidesRule  `yaml:"provides" json:"provides,omitempty"`
	Invariants []InvariantRule `yaml:"invariants" json:"invariants,omitempty"`
}

// Consumes states what a module may depend on (preconditions).
type Consumes struct {
	// Internal is nil when unrestricted. A YAML list is an allow-list
	// (empty list = may import no other declared module); a mapping form
	// {allow: [...], deny: [...]} expresses both directions.
	Internal *InternalPolicy `yaml:"internal" json:"internal,omitempty"`
	// External: "allow" (default) or "forbid" third-party imports.
	External string `yaml:"external" json:"external,omitempty"`
	// Stdlib: "allow" (default) or "forbid" standard-library imports.
	Stdlib string `yaml:"stdlib" json:"stdlib,omitempty"`
	// Severity defaults to error.
	Severity string `yaml:"severity" json:"severity,omitempty"`
}

// InternalPolicy is either an allow-list (Restricted true) or a deny-list,
// naming declared modules.
type InternalPolicy struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
	// Restricted is true when an allow-list was given (including the empty
	// list, which forbids importing any other declared module).
	Restricted bool `json:"-"`
}

// UnmarshalYAML accepts either a sequence (allow-list) or a mapping with
// allow/deny keys.
func (p *InternalPolicy) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.SequenceNode:
		var allow []string
		if err := node.Decode(&allow); err != nil {
			return err
		}
		p.Allow = allow
		p.Restricted = true
		return nil
	case yaml.MappingNode:
		var m struct {
			Allow *[]string `yaml:"allow"`
			Deny  []string  `yaml:"deny"`
		}
		if err := node.Decode(&m); err != nil {
			return err
		}
		if m.Allow != nil {
			p.Allow = *m.Allow
			p.Restricted = true
		}
		p.Deny = m.Deny
		return nil
	default:
		return fmt.Errorf("line %d: consumes.internal must be a list or a mapping with allow/deny", node.Line)
	}
}

// CaptureSide derives a named value set for correspondence rules. Files is a
// regex fully matching repo-relative paths; when Content is set, values come
// from content captures of Content applied per matching file, otherwise from
// the path captures of Files. Value is a template over named captures, e.g.
// "{substrate}".
type CaptureSide struct {
	Files   string `yaml:"files" json:"files"`
	Content string `yaml:"content" json:"content,omitempty"`
	Value   string `yaml:"value" json:"value"`
}

// ProvidesRule states an obligation a module owes (postconditions).
// Kind "registration": every capture tuple of Each (a regex over the
// module's file paths) must have a Match hit (a regex template over the
// captures) inside the file set named by In (a declared module).
// Kind "correspondence": the value set of Of must be subset-of/equal-to the
// value set of In.
type ProvidesRule struct {
	ID       string `yaml:"id" json:"id,omitempty"`
	Kind     string `yaml:"kind" json:"kind"`
	Severity string `yaml:"severity" json:"severity,omitempty"`

	// registration params
	Each  string `yaml:"each" json:"each,omitempty"`
	Match string `yaml:"match" json:"match,omitempty"`
	// In: registration takes a module name; correspondence takes a capture
	// side. Split at decode time.
	InModule string       `yaml:"-" json:"in_module,omitempty"`
	InSide   *CaptureSide `yaml:"-" json:"in,omitempty"`

	// correspondence params
	Of       *CaptureSide `yaml:"of" json:"of,omitempty"`
	Relation string       `yaml:"relation" json:"relation,omitempty"`
}

// UnmarshalYAML splits the polymorphic `in` key by shape: a scalar is a
// module name (registration), a mapping is a capture side (correspondence).
func (r *ProvidesRule) UnmarshalYAML(node *yaml.Node) error {
	type plain struct {
		ID       string       `yaml:"id"`
		Kind     string       `yaml:"kind"`
		Severity string       `yaml:"severity"`
		Each     string       `yaml:"each"`
		Match    string       `yaml:"match"`
		Of       *CaptureSide `yaml:"of"`
		Relation string       `yaml:"relation"`
		In       yaml.Node    `yaml:"in"`
	}
	var p plain
	if err := node.Decode(&p); err != nil {
		return err
	}
	r.ID, r.Kind, r.Severity = p.ID, p.Kind, p.Severity
	r.Each, r.Match = p.Each, p.Match
	r.Of, r.Relation = p.Of, p.Relation
	switch p.In.Kind {
	case 0: // absent
	case yaml.ScalarNode:
		if err := p.In.Decode(&r.InModule); err != nil {
			return err
		}
	case yaml.MappingNode:
		r.InSide = &CaptureSide{}
		if err := p.In.Decode(r.InSide); err != nil {
			return err
		}
	default:
		return fmt.Errorf("line %d: provides.in must be a module name or a capture side mapping", p.In.Line)
	}
	return nil
}

// InvariantRule states a property that always holds inside a module.
// Kinds: naming (Files + Case), structure (Require/Forbid globs),
// content (Files + Must/MustNot regexes), expr (Files + Assert).
type InvariantRule struct {
	ID       string `yaml:"id" json:"id,omitempty"`
	Kind     string `yaml:"kind" json:"kind"`
	Severity string `yaml:"severity" json:"severity,omitempty"`

	// Files filters by repo-relative glob within the module (naming,
	// content, expr). Empty means every file of the module.
	Files string `yaml:"files" json:"files,omitempty"`

	// naming: kebab-case | camelCase | PascalCase | snake_case |
	// regex:<pattern>, combinable with "|" for any-of (applies to the
	// file stem, extension excluded).
	Case string `yaml:"case" json:"case,omitempty"`

	// structure
	Require []string `yaml:"require" json:"require,omitempty"`
	Forbid  []string `yaml:"forbid" json:"forbid,omitempty"`

	// content
	Must    []string `yaml:"must" json:"must,omitempty"`
	MustNot []string `yaml:"must_not" json:"must_not,omitempty"`

	// expr: an expr-lang predicate over `file`; a false result is a
	// violation. Message overrides the default violation text.
	Assert  string `yaml:"assert" json:"assert,omitempty"`
	Message string `yaml:"message" json:"message,omitempty"`
}

// ExtensionRule is one instance of an extension-registered rule type.
type ExtensionRule struct {
	ID       string         `yaml:"id" json:"id,omitempty"`
	Type     string         `yaml:"type" json:"type"`
	Severity string         `yaml:"severity" json:"severity,omitempty"`
	Params   map[string]any `yaml:"params" json:"params,omitempty"`
}

// GraphRule is a graph-wide consumes clause over declared modules.
// Kinds mirror import-linter's proven contract set.
type GraphRule struct {
	ID       string `yaml:"id" json:"id,omitempty"`
	Kind     string `yaml:"kind" json:"kind"`
	Severity string `yaml:"severity" json:"severity,omitempty"`

	// layers: ordered highest first; a module may import same or lower
	// layers, never higher.
	Layers []string `yaml:"layers" json:"layers,omitempty"`

	// forbidden: no module in From may import any module in To.
	From []string `yaml:"from" json:"from,omitempty"`
	To   []string `yaml:"to" json:"to,omitempty"`

	// independence: no module in Modules may import another in Modules.
	// acyclic: no import cycles among Modules (empty = all declared).
	Modules []string `yaml:"modules" json:"modules,omitempty"`

	// protected: Module may only be imported by modules in Allow.
	Module string   `yaml:"module" json:"module,omitempty"`
	Allow  []string `yaml:"allow" json:"allow,omitempty"`
}
