package rule

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FieldSchema describes one accepted Rule field: its kind, whether it
// is required, its default and enum where finite, and whether a Rule
// Override may change it.
type FieldSchema struct {
	Name         string
	Kind         string // "string", "enum", "glob", "glob_list", "module_list", "allow_list", "policy", "case", "object"
	Required     bool
	Default      string
	Enum         []string
	Configurable bool
	Doc          string
}

// TypeSchema is the machine-readable description used to configure,
// validate, inspect, and autocomplete a complete Rule of one Rule
// Type: the common Rule fields plus the Type-specific parameters.
type TypeSchema struct {
	Type   Type
	Common []FieldSchema
	Params []FieldSchema
}

// Schema returns the complete schema contribution for this Rule Type.
func (t Type) Schema() TypeSchema {
	common := []FieldSchema{
		{
			Name: "id", Kind: "string", Required: true,
			Doc: "explicit stable Rule ID, optionally namespace-qualified",
		},
		{
			Name: "type", Kind: "enum", Required: true, Enum: typeStrings(),
			Doc: "one published ArcLint Rule Type",
		},
		{
			Name: "claim", Kind: "string",
			Doc: "architectural proposition; derived canonically when absent",
		},
		{
			Name: "severity", Kind: "enum", Default: string(DefaultSeverity),
			Enum:         []string{string(SeverityError), string(SeverityWarning), string(SeverityInfo)},
			Configurable: true,
			Doc:          "gate importance, independent from Assurance and Evidence Method",
		},
	}
	var params []FieldSchema
	switch t {
	case TypeConsumes:
		params = []FieldSchema{
			{
				Name: "internal", Kind: "allow_list",
				Doc: "declared Modules this Module may import; absent = unrestricted, empty = none",
			},
			{
				Name: "external", Kind: "policy", Default: string(ImportAllow),
				Enum: []string{string(ImportAllow), string(ImportForbid)},
				Doc:  "third-party import policy",
			},
			{
				Name: "stdlib", Kind: "policy", Default: string(ImportAllow),
				Enum: []string{string(ImportAllow), string(ImportForbid)},
				Doc:  "standard-library import policy",
			},
		}
	case TypeStructure:
		params = []FieldSchema{
			{Name: "require", Kind: "glob_list", Doc: "each glob must match at least one member file"},
			{Name: "forbid", Kind: "glob_list", Doc: "no member file may match any glob"},
			{
				Name: "each", Kind: "enum", Enum: expansionSourceStrings(),
				Doc: "derives the globs from a recorded vocabulary collection; {name:<case>} placeholders resolve once per recorded term",
			},
		}
	case TypeNaming:
		params = []FieldSchema{
			{Name: "files", Kind: "glob", Doc: "narrows the member files judged; absent = all"},
			{
				Name: "case", Kind: "case", Required: true,
				Doc: "kebab-case | snake_case | camelCase | PascalCase | regex:<pattern>, any-of with |",
			},
		}
	case TypeLayers:
		params = []FieldSchema{
			{
				Name: "layers", Kind: "module_list", Required: true,
				Doc: "Modules ordered highest first; imports may go same or lower only",
			},
		}
	case TypeProtected:
		params = []FieldSchema{
			{Name: "module", Kind: "string", Required: true, Doc: "the protected Module"},
			{Name: "allow", Kind: "module_list", Doc: "Modules permitted to import it"},
		}
	case TypeIndependence:
		params = []FieldSchema{
			{
				Name: "folders", Kind: "glob_list", Required: true,
				Doc: "globs selecting sibling Folders that may not import each other",
			},
		}
	case TypeAcyclic:
		params = []FieldSchema{
			{Name: "modules", Kind: "module_list", Doc: "cycle scope; absent = every declared Module"},
		}
	case TypeExtension:
		params = []FieldSchema{
			{
				Name: "uses", Kind: "string", Required: true,
				Doc: "the extension rule name registered under .arclint/extensions",
			},
			{
				Name: "with", Kind: "object",
				Doc: "parameters validated host-side against the extension's published schema",
			},
		}
	}
	return TypeSchema{Type: t, Common: common, Params: params}
}

// Describe explains the accepted configuration of this Rule Type.
func (s TypeSchema) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "rule type %s\n", s.Type)
	for _, section := range []struct {
		title  string
		fields []FieldSchema
	}{{"common", s.Common}, {"params", s.Params}} {
		for _, f := range section.fields {
			req := ""
			if f.Required {
				req = " (required)"
			}
			cfg := ""
			if f.Configurable {
				cfg = " (configurable)"
			}
			fmt.Fprintf(&b, "  %s.%s: %s%s%s — %s\n", section.title, f.Name, f.Kind, req, cfg, f.Doc)
		}
	}
	return b.String()
}

func typeStrings() []string {
	out := make([]string, 0, len(Types()))
	for _, t := range Types() {
		out = append(out, string(t))
	}
	return out
}

// Schema returns the published Rule Schema: a deterministic, indented
// JSON Schema (draft 2020-12) document describing the complete
// rules.yaml grammar — the document shape, runtime targets, scan
// settings, Module declarations, every Rule Type parameter shape, and
// the Pattern identity header. Runtime validation and this published
// editor schema accept the same values; the committed
// docs/rules.schema.json holds exactly these bytes, and a differential
// test proves both properties against the real loader.
func Schema() ([]byte, error) {
	out, err := json.MarshalIndent(schemaDocument(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal rule schema: %w", err)
	}
	return append(out, '\n'), nil
}

// String patterns for schema formats, each derived from the domain
// constructor that validates the same value at runtime.
const (
	// ruleIDJSONPattern mirrors NewID: LOCAL or NAMESPACE:LOCAL, each
	// part using a-z 0-9 . _ - /, never starting with . / - and never
	// ending with . or /.
	ruleIDJSONPattern = `^([a-z0-9_]([a-z0-9._/-]*[a-z0-9_-])?:)?[a-z0-9_]([a-z0-9._/-]*[a-z0-9_-])?$`
	// moduleNameJSONPattern mirrors NewModuleName: non-empty a-z 0-9 _ -.
	moduleNameJSONPattern = `^[a-z0-9_-]+$`
	// globJSONPattern mirrors the structural part of NewGlob: non-empty
	// slash-separated segments without brace alternation. Escape and
	// character-class validity remain runtime checks.
	globJSONPattern = `^[^/{}]+(/[^/{}]+)*$`
)

// runtimeTargets are the rules.yaml runtime spellings the loader maps
// onto Languages(). The spelling is owned by the ruleset format; the
// Language each value resolves to is owned by this package.
func runtimeTargets() []string { return []string{"go", "ts", "py"} }

// caseSpecPattern derives the naming-case grammar from the published
// named cases: alternatives of a named case or regex:PATTERN, combined
// with "|"; a regex alternative cannot itself contain "|" because the
// specification splits on it first.
func caseSpecPattern() string {
	names := make([]string, 0, len(namedCases))
	for name := range namedCases {
		names = append(names, name)
	}
	sort.Strings(names)
	alt := `(` + strings.Join(names, "|") + `|regex:[^|]*)`
	return `^\s*` + alt + `(\s*\|\s*` + alt + `)*\s*$`
}

// schemaDocument builds the complete rules.yaml document schema. Every
// enum is sourced from the published domain values; map key order is
// irrelevant because encoding/json emits object keys sorted.
func schemaDocument() map[string]any {
	return map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://raw.githubusercontent.com/wixregiga/arclint/main/docs/rules.schema.json",
		"title":                "ArcLint ruleset",
		"description":          "The complete rules.yaml document ArcLint accepts: runtime targets, scan policy, Module declarations, per-Module contracts, repository-scoped Extension invariants, repository-wide dependency Rules, and the Pattern identity header of a distribution file. Unknown keys are rejected everywhere.",
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"pattern": patternHeaderSchema(),
			"runtime": map[string]any{
				"description": "Language targets whose code facts observation produces.",
				"type":        "array",
				"items":       map[string]any{"enum": runtimeTargets()},
			},
			"scan":       scanSchema(),
			"modules":    modulesSchema(),
			"contracts":  contractsSchema(),
			"repository": repositorySchema(),
			"dependencies": map[string]any{
				"description": "Repository-wide dependency Rules over the Module graph.",
				"type":        "array",
				"items":       schemaRef("dependency"),
			},
		},
		"$defs": schemaDefs(),
	}
}

func schemaDefs() map[string]any {
	return map[string]any{
		"ruleID": map[string]any{
			"description": "Explicit stable Rule ID: LOCAL or NAMESPACE:LOCAL, each part using a-z 0-9 . _ - /, never starting with . / - and never ending with . or /.",
			"type":        "string",
			"pattern":     ruleIDJSONPattern,
		},
		"moduleName": map[string]any{
			"description": "Repository-local Module name: a-z 0-9 _ -.",
			"type":        "string",
			"pattern":     moduleNameJSONPattern,
		},
		"severity": map[string]any{
			"description": "Gate importance of a Violation.",
			"enum":        []string{string(SeverityError), string(SeverityWarning), string(SeverityInfo)},
			"default":     string(DefaultSeverity),
		},
		"importPolicy": map[string]any{
			"description": "Policy for one import class.",
			"enum":        []string{string(ImportAllow), string(ImportForbid)},
			"default":     string(ImportAllow),
		},
		"glob": map[string]any{
			"description": "Repo-relative path pattern: * and ? never cross /, a ** segment matches any number of segments, [...] matches one character. Brace alternation is rejected.",
			"type":        "string",
			"pattern":     globJSONPattern,
		},
		"caseSpec": map[string]any{
			"description": "File-name case vocabulary: one or more alternatives of kebab-case, snake_case, camelCase, PascalCase, or regex:PATTERN, combined with | (any-of); applies to the file stem, extension excluded.",
			"type":        "string",
			"pattern":     caseSpecPattern(),
		},
		"expansionSource": map[string]any{
			"description": "Recorded Ubiquitous Language collection an expanded structure Rule derives its globs from.",
			"enum":        expansionSourceStrings(),
		},
		"expandedGlob": map[string]any{
			"description": "Structure glob that may carry {name:<case>} placeholders, each resolving once per recorded term; cases are " + strings.Join(TermCaseNames(), ", ") + ".",
			"type":        "string",
			"pattern":     expandedGlobPattern(),
		},
		"consumes":                   consumesSchema(),
		"invariant":                  oneOfRefs("structureInvariant", "expandedStructureInvariant", "namingInvariant", "extensionInvariant"),
		"structureInvariant":         structureInvariantSchema(),
		"expandedStructureInvariant": expandedStructureInvariantSchema(),
		"namingInvariant":            namingInvariantSchema(),
		"extensionInvariant":         extensionInvariantSchema(),
		"dependency":                 oneOfRefs("layersDependency", "protectedDependency", "independenceDependency", "acyclicDependency"),
		"layersDependency":           layersDependencySchema(),
		"protectedDependency":        protectedDependencySchema(),
		"independenceDependency":     independenceDependencySchema(),
		"acyclicDependency":          acyclicDependencySchema(),
	}
}

func repositorySchema() map[string]any {
	return strictObjectSchema(
		"Repository-scoped Extension invariants that inspect files outside every declared Module.",
		map[string]any{
			"invariants": map[string]any{
				"description": "Extension Rules whose subjects are the whole repository.",
				"type":        "array",
				"items":       schemaRef("extensionInvariant"),
			},
		},
	)
}

func patternHeaderSchema() map[string]any {
	return strictObjectSchema(
		"Pattern identity header, present only in a Pattern distribution file — never in a repository ruleset.",
		map[string]any{
			"namespace": map[string]any{"description": "Pattern namespace qualifying distributed Rule IDs.", "type": "string", "minLength": 1},
			"name":      map[string]any{"description": "Pattern name within its namespace.", "type": "string", "minLength": 1},
			"version":   map[string]any{"description": "Pattern version; never part of Rule identity.", "type": "string", "minLength": 1},
			"coverage": map[string]any{
				"description": "Languages the Pattern declares coverage for.",
				"type":        "array",
				"items":       map[string]any{"enum": languageStrings()},
			},
		},
		"namespace", "name", "version",
	)
}

func scanSchema() map[string]any {
	return strictObjectSchema(
		"Repository observation policy: what the walk excludes and how unknown imports are treated.",
		map[string]any{
			"unknown_imports": map[string]any{
				"description": "Policy for imports that classify neither stdlib, internal, nor external.",
				"enum":        []string{string(UnknownImportsError), string(UnknownImportsWarn), string(UnknownImportsIgnore)},
				"default":     string(UnknownImportsWarn),
			},
			"exclude": map[string]any{
				"description": "Globs the repository walk skips entirely.",
				"type":        "array",
				"items":       schemaRef("glob"),
			},
			"include_testdata": map[string]any{
				"description": "Whether testdata directories are observed.",
				"type":        "boolean",
				"default":     false,
			},
		},
	)
}

func modulesSchema() map[string]any {
	return map[string]any{
		"description":   "Declared Modules: named logical groupings of files selected by membership globs. Modules may overlap.",
		"type":          "object",
		"propertyNames": schemaRef("moduleName"),
		"additionalProperties": strictObjectSchema(
			"One Module declaration.",
			map[string]any{
				"paths": map[string]any{
					"description": "Membership selectors; a glob naming a directory claims its whole subtree.",
					"type":        "array",
					"minItems":    1,
					"items":       schemaRef("glob"),
				},
				"description": map[string]any{"description": "Authoring description of the Module.", "type": "string"},
			},
			"paths",
		),
	}
}

func contractsSchema() map[string]any {
	return map[string]any{
		"description":   "Per-Module contracts. Every key must name a declared Module; the loader enforces the declaration, the schema validates the name shape.",
		"type":          "object",
		"propertyNames": schemaRef("moduleName"),
		"additionalProperties": strictObjectSchema(
			"Rules bound to one declared Module.",
			map[string]any{
				"consumes": schemaRef("consumes"),
				"invariants": map[string]any{
					"description": "Structure, naming, and extension Rules over the Module's member files.",
					"type":        "array",
					"items":       schemaRef("invariant"),
				},
			},
		),
	}
}

func consumesSchema() map[string]any {
	s := strictObjectSchema(
		"A consumes Rule: what the contract's Module may import. At least one restriction must be declared — an internal allow-list, external: forbid, or stdlib: forbid.",
		map[string]any{
			"id": schemaRef("ruleID"),
			"internal": map[string]any{
				"description": "Declared Modules this Module may import; absent means unrestricted, empty means none. The owning Module is always permitted implicitly.",
				"type":        "array",
				"uniqueItems": true,
				"items":       schemaRef("moduleName"),
			},
			"external": schemaRef("importPolicy"),
			"stdlib":   schemaRef("importPolicy"),
			"severity": schemaRef("severity"),
		},
		"id",
	)
	s["anyOf"] = []any{
		map[string]any{"required": []string{"internal"}},
		map[string]any{
			"required":   []string{"external"},
			"properties": map[string]any{"external": map[string]any{"const": string(ImportForbid)}},
		},
		map[string]any{
			"required":   []string{"stdlib"},
			"properties": map[string]any{"stdlib": map[string]any{"const": string(ImportForbid)}},
		},
	}
	return s
}

func structureInvariantSchema() map[string]any {
	s := strictObjectSchema(
		"A structure Rule: requires or forbids member files matching globs. At least one non-empty glob list must be declared.",
		map[string]any{
			"id":       schemaRef("ruleID"),
			"kind":     map[string]any{"const": string(TypeStructure)},
			"severity": schemaRef("severity"),
			"require": map[string]any{
				"description": "Each glob must match at least one member file.",
				"type":        "array",
				"items":       schemaRef("glob"),
			},
			"forbid": map[string]any{
				"description": "No member file may match any glob.",
				"type":        "array",
				"items":       schemaRef("glob"),
			},
		},
		"id", "kind",
	)
	s["anyOf"] = []any{
		map[string]any{
			"required":   []string{"require"},
			"properties": map[string]any{"require": map[string]any{"minItems": 1}},
		},
		map[string]any{
			"required":   []string{"forbid"},
			"properties": map[string]any{"forbid": map[string]any{"minItems": 1}},
		},
	}
	return s
}

// expandedGlobPattern derives the expanded-glob grammar from the
// published term cases: ordinary glob segments interleaved with
// {name:<case>} placeholders.
func expandedGlobPattern() string {
	segment := `([^/{}]|\{name:(` + strings.Join(TermCaseNames(), "|") + `)\})+`
	return `^` + segment + `(/` + segment + `)*$`
}

func expansionSourceStrings() []string {
	out := make([]string, 0, len(ExpansionSources()))
	for _, s := range ExpansionSources() {
		out = append(out, string(s))
	}
	return out
}

func expandedStructureInvariantSchema() map[string]any {
	s := strictObjectSchema(
		"An expanded structure Rule: one universally quantified claim whose require/forbid globs derive from a recorded Ubiquitous Language collection, {name:<case>} placeholders resolving once per recorded term. A project recording nothing derives no obligations; the Rule exists and says so.",
		map[string]any{
			"id":       schemaRef("ruleID"),
			"kind":     map[string]any{"const": string(TypeStructure)},
			"each":     schemaRef("expansionSource"),
			"severity": schemaRef("severity"),
			"require": map[string]any{
				"description": "Each derived glob must match at least one member file.",
				"type":        "array",
				"items":       schemaRef("expandedGlob"),
			},
			"forbid": map[string]any{
				"description": "No member file may match any derived glob.",
				"type":        "array",
				"items":       schemaRef("expandedGlob"),
			},
		},
		"id", "kind", "each",
	)
	s["anyOf"] = []any{
		map[string]any{
			"required":   []string{"require"},
			"properties": map[string]any{"require": map[string]any{"minItems": 1}},
		},
		map[string]any{
			"required":   []string{"forbid"},
			"properties": map[string]any{"forbid": map[string]any{"minItems": 1}},
		},
	}
	return s
}

func namingInvariantSchema() map[string]any {
	return strictObjectSchema(
		"A naming Rule: constrains member file names to a finite case vocabulary.",
		map[string]any{
			"id":       schemaRef("ruleID"),
			"kind":     map[string]any{"const": string(TypeNaming)},
			"severity": schemaRef("severity"),
			"files": map[string]any{
				"description": "Narrows the member files judged; absent means all.",
				"$ref":        "#/$defs/glob",
			},
			"case": schemaRef("caseSpec"),
		},
		"id", "kind", "case",
	)
}

func extensionInvariantSchema() map[string]any {
	return strictObjectSchema(
		"An extension Rule: delegates enforcement to a named Extension through the sandboxed SDK.",
		map[string]any{
			"id":       schemaRef("ruleID"),
			"kind":     map[string]any{"const": string(TypeExtension)},
			"severity": schemaRef("severity"),
			"files": map[string]any{
				"description": "Narrows the member files judged; absent means all.",
				"$ref":        "#/$defs/glob",
			},
			"uses": map[string]any{
				"description": "The extension rule name registered under .arclint/extensions.",
				"type":        "string",
				"pattern":     `\S`,
			},
			"with": map[string]any{
				"description": "Parameters validated host-side against the extension's published schema before any extension code runs.",
				"type":        "object",
			},
		},
		"id", "kind", "uses",
	)
}

func layersDependencySchema() map[string]any {
	return strictObjectSchema(
		"A layers Rule: orders Modules highest first; a Module may import same or lower layers, never higher.",
		map[string]any{
			"id":       schemaRef("ruleID"),
			"kind":     map[string]any{"const": string(TypeLayers)},
			"severity": schemaRef("severity"),
			"layers": map[string]any{
				"description": "Modules ordered highest first; at least two, no duplicates.",
				"type":        "array",
				"minItems":    2,
				"uniqueItems": true,
				"items":       schemaRef("moduleName"),
			},
		},
		"id", "kind", "layers",
	)
}

func protectedDependencySchema() map[string]any {
	return strictObjectSchema(
		"A protected Rule: restricts which Modules may import one Module.",
		map[string]any{
			"id":       schemaRef("ruleID"),
			"kind":     map[string]any{"const": string(TypeProtected)},
			"severity": schemaRef("severity"),
			"module": map[string]any{
				"description": "The protected Module.",
				"$ref":        "#/$defs/moduleName",
			},
			"allow": map[string]any{
				"description": "Modules permitted to import it; absent or empty means none.",
				"type":        "array",
				"uniqueItems": true,
				"items":       schemaRef("moduleName"),
			},
		},
		"id", "kind", "module",
	)
}

func independenceDependencySchema() map[string]any {
	return strictObjectSchema(
		"An independence Rule: sibling Folders selected by the globs may not import each other.",
		map[string]any{
			"id":       schemaRef("ruleID"),
			"kind":     map[string]any{"const": string(TypeIndependence)},
			"severity": schemaRef("severity"),
			"folders": map[string]any{
				"description": "Globs selecting sibling Folders; at least one, no duplicates.",
				"type":        "array",
				"minItems":    1,
				"uniqueItems": true,
				"items":       schemaRef("glob"),
			},
		},
		"id", "kind", "folders",
	)
}

func acyclicDependencySchema() map[string]any {
	return strictObjectSchema(
		"An acyclic Rule: forbids dependency cycles among declared Modules.",
		map[string]any{
			"id":       schemaRef("ruleID"),
			"kind":     map[string]any{"const": string(TypeAcyclic)},
			"severity": schemaRef("severity"),
			"modules": map[string]any{
				"description": "Cycle scope; absent means every declared Module.",
				"type":        "array",
				"uniqueItems": true,
				"items":       schemaRef("moduleName"),
			},
		},
		"id", "kind",
	)
}

// strictObjectSchema builds an object schema that rejects unknown keys,
// mirroring the loader's strict decoding and per-kind field whitelists.
func strictObjectSchema(description string, properties map[string]any, required ...string) map[string]any {
	out := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
	}
	if description != "" {
		out["description"] = description
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

func schemaRef(name string) map[string]any {
	return map[string]any{"$ref": "#/$defs/" + name}
}

func oneOfRefs(names ...string) map[string]any {
	refs := make([]any, 0, len(names))
	for _, name := range names {
		refs = append(refs, schemaRef(name))
	}
	return map[string]any{"oneOf": refs}
}

func languageStrings() []string {
	out := make([]string, 0, len(Languages()))
	for _, l := range Languages() {
		out = append(out, string(l))
	}
	return out
}
