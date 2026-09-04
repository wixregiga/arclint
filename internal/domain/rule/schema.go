package rule

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FieldSchema describes one accepted Rule field: its kind, whether it
// is required, its default and enum where finite, and whether an
// Override may change it.
type FieldSchema struct {
	Name         string
	Kind         string // "string", "enum", "glob", "glob_list", "module", "module_list", "allow_list", "policy", "case", "regex", "object", "boolean"
	Required     bool
	Default      string
	Enum         []string
	Configurable bool
	Doc          string
}

// TypeSchema is the machine-readable description used to configure,
// validate, inspect, and autocomplete a complete Rule of one Rule
// Type: the common Rule fields plus the fields under the Type's
// Assertion key.
type TypeSchema struct {
	Type   Type
	Key    string
	Common []FieldSchema
	Params []FieldSchema
}

// Schema returns the complete schema contribution for this Rule Type.
func (t Type) Schema() TypeSchema {
	common := []FieldSchema{
		{
			Name: "description", Kind: "string",
			Doc: "architectural proposition (the Claim); derived canonically when absent",
		},
		{
			Name: "severity", Kind: "enum", Default: string(DefaultSeverity),
			Enum:         []string{string(SeverityError), string(SeverityWarning), string(SeverityInfo)},
			Configurable: true,
			Doc:          "gate importance, independent from Assurance and Evidence Method",
		},
	}
	switch t.Scope() {
	case ScopeModules:
		common = append(common, FieldSchema{
			Name: "on", Kind: "module_list", Required: true,
			Doc: "the declared Module or Modules the Rule judges",
		})
	case ScopeOneModule:
		common = append(common, FieldSchema{
			Name: "on", Kind: "module", Required: true,
			Doc: "the one declared Module the Rule protects",
		})
	case ScopeModulesOrRepository:
		common = append(common, FieldSchema{
			Name: "on", Kind: "module_list",
			Doc: "the declared Module or Modules the Rule judges; absent means the whole repository",
		})
	case ScopeRepository:
		// The assertion itself names the Modules; on is not accepted.
	}
	if t.AcceptsFiles() {
		common = append(common, FieldSchema{
			Name: "files", Kind: "glob",
			Doc: "narrows the files judged; absent means every selected file",
		})
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
			{
				Name: "case", Kind: "case", Required: true,
				Doc: "kebab-case | snake_case | camelCase | PascalCase | regex:<pattern>, any-of with |; a bare string under naming is this field",
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
			{Name: "imported_by", Kind: "module_list", Required: true, Doc: "Modules permitted to import the Module named under on; empty means none"},
		}
	case TypeIndependence:
		params = []FieldSchema{
			{
				Name: "independent", Kind: "glob_list", Required: true,
				Doc: "globs selecting sibling Folders that may not import each other",
			},
		}
	case TypeAcyclic:
		params = []FieldSchema{
			{Name: "acyclic", Kind: "module_list", Required: true, Doc: "cycle scope; an empty mapping means every declared Module (inside a Pattern, every Module the Pattern declares)"},
		}
	case TypeInvariants:
		params = []FieldSchema{
			{
				Name: "closed", Kind: "boolean", Default: "false",
				Doc: "when true, every exported error-returning function in the owner's files must call the cluster method",
			},
		}
	case TypeContent:
		params = []FieldSchema{
			{Name: "forbid", Kind: "regex", Required: true, Doc: "no line of a selected file may match this RE2 pattern"},
		}
	case TypeExtension:
		params = []FieldSchema{
			{
				Name: "uses", Kind: "string", Required: true,
				Doc: "the extension rule name registered under .arclint/extensions or distributed by an extended Pattern",
			},
			{
				Name: "with", Kind: "object",
				Doc: "parameters validated host-side against the extension's published schema",
			},
		}
	}
	return TypeSchema{Type: t, Key: t.AssertionKey(), Common: common, Params: params}
}

// Describe explains the accepted configuration of this Rule Type.
func (s TypeSchema) Describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "rule type %s (assertion key %s)\n", s.Type, s.Key)
	for _, section := range []struct {
		title  string
		fields []FieldSchema
	}{{"rule", s.Common}, {s.Key, s.Params}} {
		for _, f := range section.fields {
			req := ""
			if f.Required {
				req = " (required)"
			}
			cfg := ""
			if f.Configurable {
				cfg = " (configurable)"
			}
			fmt.Fprintf(&b, "  %s.%s: %s%s%s  %s\n", section.title, f.Name, f.Kind, req, cfg, f.Doc)
		}
	}
	return b.String()
}

// The published Rule Schema's file identity. arclint itself publishes
// the schema under docs/schemas; an adopting project writes it under
// its schema directory with `arclint rules schema --write`, and the
// modeline of a ruleset names whichever copy is nearby.
const (
	SchemaFileName = "rules.arclint.schema.json"
	SchemaID       = "https://raw.githubusercontent.com/wixregiga/arclint/main/docs/schemas/" + SchemaFileName
)

// Schema returns the published Rule Schema: a deterministic, indented
// JSON Schema (draft 2020-12) document describing the complete ruleset
// grammar: the document shape, runtime targets, scan settings, extended
// Patterns and their Bindings, Module declarations, the rules map with
// every Assertion shape and the Override shape, and the Pattern
// identity header of a distribution file. Runtime validation and this
// published editor schema accept the same values; the committed
// docs/schemas copy holds exactly these bytes, and a differential test
// proves both properties against the real loader.
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
	// idPartJSONPattern mirrors validateIDPart: a-z 0-9 . _ - /, never
	// starting with . / - and never ending with . or /.
	idPartJSONPattern = `[a-z0-9_]([a-z0-9._/-]*[a-z0-9_-])?`
	// ruleIDJSONPattern mirrors NewID: LOCAL or NAMESPACE/NAME:LOCAL.
	ruleIDJSONPattern = `^(` + patternPartJSONPattern + `/` + patternPartJSONPattern + `:)?` + idPartJSONPattern + `$`
	// patternPartJSONPattern is an id part that also excludes "/", the
	// separator inside a PatternReference.
	patternPartJSONPattern = `[a-z0-9_]([a-z0-9._-]*[a-z0-9_-])?`
	// patternReferenceJSONPattern mirrors ParsePatternReference:
	// namespace/name@version with an exact semantic version.
	patternReferenceJSONPattern = `^` + patternPartJSONPattern + `/` + patternPartJSONPattern + `@\d+\.\d+\.\d+([\-+][0-9A-Za-z.\-+]+)?$`
	// moduleNameJSONPattern mirrors NewModuleName: non-empty a-z 0-9 _ -.
	moduleNameJSONPattern = `^[a-z0-9_-]+$`
	// globJSONPattern mirrors the structural part of NewGlob: non-empty
	// slash-separated segments without brace alternation. Escape and
	// character-class validity remain runtime checks.
	globJSONPattern = `^[^/{}]+(/[^/{}]+)*$`
)

// runtimeTargets are the ruleset runtime spellings the loader maps
// onto Languages(), in Languages() order.
func runtimeTargets() []string {
	out := make([]string, 0, len(Languages()))
	for _, l := range Languages() {
		out = append(out, l.RuntimeTarget())
	}
	return out
}

// runtimeTargetSchema is one runtime target spelling, wherever a list
// of targets appears.
func runtimeTargetSchema() map[string]any {
	return map[string]any{"type": "string", "enum": runtimeTargets()}
}

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

// schemaDocument builds the complete ruleset document schema. Every
// enum is sourced from the published domain values; map key order is
// irrelevant because encoding/json emits object keys sorted.
func schemaDocument() map[string]any {
	return map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  SchemaID,
		"title":                "ArcLint ruleset",
		"description":          "The complete " + RulesetFileName + " document ArcLint accepts. A repository ruleset carries runtime, scan, extends, modules, and rules; a Pattern distribution file carries the pattern header, modules, and rules. Every Rule is keyed by its Rule ID and carries exactly one assertion key; an entry with no assertion key is an Override of a Rule an extended Pattern distributes. Unknown keys are rejected everywhere.",
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"pattern": patternHeaderSchema(),
			"runtime": map[string]any{
				"description": "Language targets whose code facts observation produces. Repository rulesets only.",
				"type":        "array",
				"minItems":    1,
				"uniqueItems": true,
				"items":       runtimeTargetSchema(),
			},
			"scan":    scanSchema(),
			"extends": extendsSchema(),
			"modules": map[string]any{
				"description":          "Declared Modules keyed by name. In a repository ruleset a Module is a glob, a list of globs, or an object with paths and description. In a Pattern file a Module is its description, or an object with description and the paths it suggests for the Binding.",
				"type":                 "object",
				"propertyNames":        schemaRef("moduleName"),
				"additionalProperties": schemaRef("module"),
			},
			"rules": rulesSchema(),
		},
		"if": map[string]any{"required": []string{"pattern"}},
		"then": map[string]any{
			"description": "A Pattern distribution file: no runtime, no scan, no extends, because those are repository policy; Modules carry descriptions and suggested paths only.",
			"properties": map[string]any{
				"runtime": false,
				"scan":    false,
				"extends": false,
				"modules": map[string]any{
					"description":          "Declared Modules keyed by name, each a description or an object with description and suggested paths.",
					"type":                 "object",
					"propertyNames":        schemaRef("moduleName"),
					"additionalProperties": schemaRef("patternModule"),
				},
				"rules": map[string]any{
					"description":          "A Pattern distributes Rules and cannot override: every entry carries exactly one assertion key.",
					"type":                 "object",
					"propertyNames":        schemaRef("ruleID"),
					"additionalProperties": patternRuleSchema(),
				},
			},
		},
		"else": map[string]any{
			"description": "A repository ruleset: every Module carries paths, directly or through a Binding.",
			"properties": map[string]any{
				"modules": map[string]any{
					"description":          "Declared Modules keyed by name, each a glob, a list of globs, or an object with paths and description.",
					"type":                 "object",
					"propertyNames":        schemaRef("moduleName"),
					"additionalProperties": schemaRef("repositoryModule"),
				},
			},
		},
		"$defs": schemaDefs(),
	}
}

// defDescriptions is the single source of every $defs description.
// schemaRef writes the target's description beside each $ref, so an
// editor hover reads the same text wherever a definition is used and
// the published schema never carries a reference its reader must
// resolve to understand. Each contextual use of a shared shape (paths
// a Module claims, files a Rule judges, paths an Exclusion removes)
// is its own definition so the hover names the meaning, not the shape.
func defDescriptions() map[string]string {
	descs := map[string]string{
		"ruleID":                    "Explicit stable Rule ID: LOCAL for a repository Rule, or NAMESPACE/NAME:LOCAL for a Rule an extended Pattern distributes. LOCAL uses a-z 0-9 . _ - /, never starting with . / - and never ending with . or /. Inside a Pattern file the Rule ID is local; the loader qualifies it with the Pattern's namespace/name, so two Patterns may distribute the same local identity.",
		"moduleName":                "Module name: lowercase letters, digits, underscore, and hyphen.",
		"protectedModule":           "The one declared Module the Rule protects.",
		"patternReference":          "Exact reference to one published Pattern version: namespace/name@version.",
		"severity":                  "Gate importance of a Violation.",
		"importPolicy":              "Policy for one import class.",
		"glob":                      "Repo-relative path pattern: * and ? never cross /, a ** segment matches any number of segments, [...] matches one character. Brace alternation is rejected.",
		"caseSpec":                  "File-name case vocabulary: one or more alternatives of kebab-case, snake_case, camelCase, PascalCase, or regex:PATTERN, combined with | (any-of); applies to the file stem, extension excluded.",
		"expansionSource":           "Recorded Ubiquitous Language collection an expanded structure Rule derives its globs from.",
		"expandedGlob":              "Structure glob that may carry {name:<case>} placeholders, each resolving once per recorded term; cases are " + strings.Join(TermCaseNames(), ", ") + ".",
		"reason":                    "The recorded reason for an adoption decision; required so the decision stays inspectable.",
		"modulePaths":               "Membership selectors: one glob or a non-empty list of globs. A glob naming a directory claims its whole subtree.",
		"suggestedPaths":            "Paths the Pattern suggests for the Binding: one glob or a non-empty list of globs. The init command writes them into bind when the Pattern is adopted.",
		"boundPaths":                "Repository paths bound to one Pattern Module: one glob or a non-empty list of globs. The only place a Pattern Module's paths live.",
		"judgedFiles":               "Globs narrowing the files the Rule judges: one glob or a non-empty list. Absent means every selected file.",
		"excludedPaths":             "Files the Rule no longer judges: one glob or a non-empty list of globs.",
		"suppressedPaths":           "Files whose findings are suppressed: one glob or a non-empty list of globs.",
		"judgedModules":             "The declared Module or Modules the Rule judges: one Module name or a non-empty list.",
		"judgedModulesOrRepository": "The declared Module or Modules the Rule judges: one Module name or a non-empty list. Absent means the whole repository.",
		"excludedModules":           "Modules the Rule no longer judges: one Module name or a non-empty list.",
		"module":                    "One Module: a glob, a list of globs, or an object in a repository ruleset; a description or an object in a Pattern file.",
		"repositoryModule":          "One repository Module: its paths as a glob or a list of globs, or an object with paths and an optional description.",
		"patternModule":             "One Pattern Module: its description, or an object with the description and the paths the Pattern suggests for the Binding. A Pattern never owns paths.",
		"rule":                      "One Rule carrying exactly one assertion key (" + strings.Join(AssertionKeys(), ", ") + "), or an Override carrying none.",
		"override":                  "An Override of a Rule an extended Pattern distributes, keyed by that Rule's qualified ID. It carries no assertion and no description: it disables the Rule with a reason, changes its severity, excludes subjects, or suppresses findings. To change what a Pattern Rule asserts, disable it and add a local Rule under a new ID.",
		"exclusion":                 "Removes paths or Modules from what the Rule judges; excluded subjects evaluate not applicable.",
		"suppression":               "Keeps findings at the paths while removing their gate effect; suppressed findings are still reported.",
	}
	for _, t := range Types() {
		descs[assertionDefName(t)] = assertionRuleDescription(t)
	}
	return descs
}

// defDescription is the description of one named definition; an
// unknown name yields the empty string, which the schema tests and the
// schema lint both reject, so a reference can never silently point at
// nothing.
func defDescription(name string) string {
	return defDescriptions()[name]
}

func schemaDefs() map[string]any {
	defs := map[string]any{
		"ruleID": map[string]any{
			"description": defDescription("ruleID"),
			"type":        "string",
			"pattern":     ruleIDJSONPattern,
		},
		"moduleName": map[string]any{
			"description": defDescription("moduleName"),
			"type":        "string",
			"pattern":     moduleNameJSONPattern,
		},
		"protectedModule": map[string]any{
			"description": defDescription("protectedModule"),
			"type":        "string",
			"pattern":     moduleNameJSONPattern,
		},
		"patternReference": map[string]any{
			"description": defDescription("patternReference"),
			"type":        "string",
			"pattern":     patternReferenceJSONPattern,
		},
		"severity": map[string]any{
			"description": defDescription("severity"),
			"type":        "string",
			"enum":        []string{string(SeverityError), string(SeverityWarning), string(SeverityInfo)},
			"default":     string(DefaultSeverity),
		},
		"importPolicy": map[string]any{
			"description": defDescription("importPolicy"),
			"type":        "string",
			"enum":        []string{string(ImportAllow), string(ImportForbid)},
			"default":     string(ImportAllow),
		},
		"glob": map[string]any{
			"description": defDescription("glob"),
			"type":        "string",
			"pattern":     globJSONPattern,
		},
		"caseSpec": map[string]any{
			"description": defDescription("caseSpec"),
			"type":        "string",
			"pattern":     caseSpecPattern(),
		},
		"expansionSource": map[string]any{
			"description": defDescription("expansionSource"),
			"type":        "string",
			"enum":        expansionSourceStrings(),
		},
		"expandedGlob": map[string]any{
			"description": defDescription("expandedGlob"),
			"type":        "string",
			"pattern":     expandedGlobPattern(),
		},
		"reason": map[string]any{
			"description": defDescription("reason"),
			"type":        "string",
			"pattern":     `\S`,
		},
		"modulePaths":               globsSchema("modulePaths"),
		"suggestedPaths":            globsSchema("suggestedPaths"),
		"boundPaths":                globsSchema("boundPaths"),
		"judgedFiles":               globsSchema("judgedFiles"),
		"excludedPaths":             globsSchema("excludedPaths"),
		"suppressedPaths":           globsSchema("suppressedPaths"),
		"judgedModules":             moduleNamesSchema("judgedModules"),
		"judgedModulesOrRepository": moduleNamesSchema("judgedModulesOrRepository"),
		"excludedModules":           moduleNamesSchema("excludedModules"),
		"module":                    moduleSchema(),
		"repositoryModule":          repositoryModuleSchema(),
		"patternModule":             patternModuleSchema(),
		"rule":                      ruleSchema(),
		"override":                  overrideSchema(),
		"exclusion":                 exclusionSchema(),
		"suppression":               suppressionSchema(),
	}
	for _, t := range Types() {
		defs[assertionDefName(t)] = assertionRuleSchema(t)
	}
	return defs
}

func assertionDefName(t Type) string { return string(t) + "Rule" }

// assertionRuleDescription names a Rule of one Type by its assertion
// key and states its meaning.
func assertionRuleDescription(t Type) string {
	key := t.AssertionKey()
	article := "A"
	if strings.ContainsRune("aeiou", rune(key[0])) {
		article = "An"
	}
	return article + " " + key + " Rule: " + t.Meaning() + "."
}

// globsSchema is the one-or-many glob shape under the named
// definition's own meaning.
func globsSchema(name string) map[string]any {
	return map[string]any{
		"description": defDescription(name),
		"oneOf": []any{
			schemaRef("glob"),
			map[string]any{"type": "array", "minItems": 1, "uniqueItems": true, "items": schemaRef("glob")},
		},
	}
}

// moduleNamesSchema is the one-or-many Module name shape under the
// named definition's own meaning.
func moduleNamesSchema(name string) map[string]any {
	return map[string]any{
		"description": defDescription(name),
		"oneOf": []any{
			schemaRef("moduleName"),
			map[string]any{"type": "array", "minItems": 1, "uniqueItems": true, "items": schemaRef("moduleName")},
		},
	}
}

func patternHeaderSchema() map[string]any {
	return strictObjectSchema(
		"Pattern identity header, present only in a Pattern distribution file, never in a repository ruleset.",
		map[string]any{
			"namespace": map[string]any{"description": "Pattern namespace; with the name it qualifies every distributed Rule ID as namespace/name:local.", "type": "string", "pattern": `^` + patternPartJSONPattern + `$`},
			"name":      map[string]any{"description": "Pattern name within its namespace.", "type": "string", "pattern": `^` + patternPartJSONPattern + `$`},
			"version":   map[string]any{"description": "Exact published version; never part of Rule identity.", "type": "string", "pattern": `^\d+\.\d+\.\d+([\-+][0-9A-Za-z.\-+]+)?$`},
			"coverage": map[string]any{
				"description": "Runtime targets the Pattern's Rules were written for, spelled exactly like the repository runtime list.",
				"type":        "array",
				"uniqueItems": true,
				"items":       runtimeTargetSchema(),
			},
			"documentation": map[string]any{
				"description": "Where readers learn what the Pattern enforces and why: a URL or a short text.",
				"type":        "string",
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
				"type":        "string",
				"enum":        []string{string(UnknownImportsError), string(UnknownImportsWarn), string(UnknownImportsIgnore)},
				"default":     string(UnknownImportsWarn),
			},
			"exclude": map[string]any{
				"description": "Globs the repository walk skips entirely.",
				"type":        "array",
				"uniqueItems": true,
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

func extendsSchema() map[string]any {
	return map[string]any{
		"description": "Patterns this repository adopts. Each entry pins one exact version and binds every Module the Pattern lists to repository paths. Repository rulesets only.",
		"type":        "array",
		"uniqueItems": true,
		"items": strictObjectSchema(
			"One adopted Pattern and its Bindings.",
			map[string]any{
				"pattern": schemaRef("patternReference"),
				"bind": map[string]any{
					"description":          "Paths for every Module the Pattern lists, keyed by Module name: one glob or a list of globs.",
					"type":                 "object",
					"propertyNames":        schemaRef("moduleName"),
					"additionalProperties": schemaRef("boundPaths"),
				},
			},
			"pattern",
		),
	}
}

func moduleSchema() map[string]any {
	return map[string]any{
		"description": defDescription("module"),
		"oneOf": []any{
			map[string]any{"description": "A Module's one glob in a repository ruleset, or its description in a Pattern file.", "type": "string", "minLength": 1},
			map[string]any{"description": "A Module's globs in a repository ruleset.", "type": "array", "minItems": 1, "uniqueItems": true, "items": schemaRef("glob")},
			strictObjectSchema("A Module object: paths and description in a repository ruleset; description and suggested paths in a Pattern file.", map[string]any{
				"paths":       schemaRef("modulePaths"),
				"description": map[string]any{"description": "Authoring description of the Module.", "type": "string"},
			}),
		},
	}
}

func repositoryModuleSchema() map[string]any {
	return map[string]any{
		"description": defDescription("repositoryModule"),
		"oneOf": []any{
			schemaRef("glob"),
			map[string]any{"description": "The Module's globs.", "type": "array", "minItems": 1, "uniqueItems": true, "items": schemaRef("glob")},
			strictObjectSchema("The Module's paths and an optional description.", map[string]any{
				"paths":       schemaRef("modulePaths"),
				"description": map[string]any{"description": "Authoring description of the Module.", "type": "string"},
			}, "paths"),
		},
	}
}

func patternModuleSchema() map[string]any {
	return map[string]any{
		"description": defDescription("patternModule"),
		"oneOf": []any{
			map[string]any{"description": "What the Module is for.", "type": "string", "pattern": `\S`},
			strictObjectSchema("The Module's description and the paths the Pattern suggests.", map[string]any{
				"description": map[string]any{"description": "What the Module is for.", "type": "string", "pattern": `\S`},
				"paths":       schemaRef("suggestedPaths"),
			}, "description"),
		},
	}
}

func rulesSchema() map[string]any {
	return map[string]any{
		"description":          "Every Rule keyed by its Rule ID. An entry with one assertion key is a Rule; an entry with none is an Override of a Rule an extended Pattern distributes, keyed by that Rule's qualified ID.",
		"type":                 "object",
		"propertyNames":        schemaRef("ruleID"),
		"additionalProperties": schemaRef("rule"),
	}
}

func ruleSchema() map[string]any {
	names := make([]string, 0, len(Types())+1)
	for _, t := range Types() {
		names = append(names, assertionDefName(t))
	}
	names = append(names, "override")
	s := oneOfRefs(names...)
	s["description"] = defDescription("rule")
	return s
}

// patternRuleSchema is the Rule alternative set without the Override:
// a Pattern file distributes Rules and has nothing to override.
func patternRuleSchema() map[string]any {
	names := make([]string, 0, len(Types()))
	for _, t := range Types() {
		names = append(names, assertionDefName(t))
	}
	s := oneOfRefs(names...)
	s["description"] = "One distributed Rule carrying exactly one assertion key (" + strings.Join(AssertionKeys(), ", ") + ")."
	return s
}

// commonRuleProperties are the keys every Rule entry may carry beside
// its assertion key.
func commonRuleProperties(t Type) (map[string]any, []string) {
	props := map[string]any{
		"description": map[string]any{
			"description": "The Claim: the architectural proposition the Rule states. Derived canonically when absent; Pattern Rules should write one.",
			"type":        "string",
		},
		"severity": schemaRef("severity"),
		"disable":  schemaRef("reason"),
		"exclude":  schemaRef("exclusion"),
		"suppress": schemaRef("suppression"),
	}
	var required []string
	switch t.Scope() {
	case ScopeModules:
		props["on"] = schemaRef("judgedModules")
		required = append(required, "on")
	case ScopeOneModule:
		props["on"] = schemaRef("protectedModule")
		required = append(required, "on")
	case ScopeModulesOrRepository:
		props["on"] = schemaRef("judgedModulesOrRepository")
	case ScopeRepository:
		// The assertion itself names the Modules; on is not accepted.
	}
	if t.AcceptsFiles() {
		props["files"] = schemaRef("judgedFiles")
	}
	return props, required
}

func assertionRuleSchema(t Type) map[string]any {
	props, required := commonRuleProperties(t)
	key := t.AssertionKey()
	required = append(required, key)
	description := assertionRuleDescription(t)
	switch t {
	case TypeConsumes:
		imports := strictObjectSchema(
			"What the Module may import. At least one restriction must be declared: an internal allow-list, external: forbid, or stdlib: forbid.",
			map[string]any{
				"internal": map[string]any{
					"description": "Declared Modules this Module may import; absent means unrestricted, empty means none. The owning Module is always permitted implicitly.",
					"type":        "array",
					"uniqueItems": true,
					"items":       schemaRef("moduleName"),
				},
				"external": schemaRef("importPolicy"),
				"stdlib":   schemaRef("importPolicy"),
			},
		)
		imports["anyOf"] = []any{
			map[string]any{"required": []string{"internal"}},
			map[string]any{
				"type":       "object",
				"required":   []string{"external"},
				"properties": map[string]any{"external": map[string]any{"description": "The external import class is forbidden.", "type": "string", "const": string(ImportForbid)}},
			},
			map[string]any{
				"type":       "object",
				"required":   []string{"stdlib"},
				"properties": map[string]any{"stdlib": map[string]any{"description": "The standard-library import class is forbidden.", "type": "string", "const": string(ImportForbid)}},
			},
		}
		props[key] = imports
	case TypeStructure:
		props[key] = map[string]any{
			"description": "Files the Module must or must not contain. With each, the globs derive from a recorded vocabulary collection and may carry {name:<case>} placeholders.",
			"oneOf": []any{
				structureAssertionSchema(false),
				structureAssertionSchema(true),
			},
		}
	case TypeNaming:
		props[key] = map[string]any{
			"description": "The case vocabulary file stems must match: the case spec itself, or an object with case.",
			"oneOf": []any{
				schemaRef("caseSpec"),
				strictObjectSchema("The case vocabulary under its own key.", map[string]any{"case": schemaRef("caseSpec")}, "case"),
			},
		}
	case TypeLayers:
		props[key] = map[string]any{
			"description": "Modules ordered highest first; at least two, no duplicates.",
			"type":        "array",
			"minItems":    2,
			"uniqueItems": true,
			"items":       schemaRef("moduleName"),
		}
	case TypeProtected:
		props[key] = map[string]any{
			"description": "Modules permitted to import the Module named under on; empty means none.",
			"type":        "array",
			"uniqueItems": true,
			"items":       schemaRef("moduleName"),
		}
	case TypeIndependence:
		props[key] = map[string]any{
			"description": "Globs selecting sibling Folders that may not import each other; at least one, no duplicates.",
			"type":        "array",
			"minItems":    1,
			"uniqueItems": true,
			"items":       schemaRef("glob"),
		}
	case TypeAcyclic:
		props[key] = map[string]any{
			"description": "Cycle scope: a list of declared Modules, or an empty mapping for every declared Module; inside a Pattern the empty mapping means every Module the Pattern declares.",
			"oneOf": []any{
				map[string]any{"description": "The declared Modules in scope; at least two, no duplicates.", "type": "array", "minItems": 2, "uniqueItems": true, "items": schemaRef("moduleName")},
				strictObjectSchema("The empty mapping: every declared Module is in scope.", map[string]any{}),
			},
		}
	case TypeInvariants:
		props[key] = strictObjectSchema(
			"Evaluation posture. closed: false (default): child constructors may return errors. closed: true: every exported error-returning function in the owner's files must call the cluster method.",
			map[string]any{
				"closed": map[string]any{
					"description": "When true, extra exported error-returning functions that do not call the cluster method fail. Default: false.",
					"type":        "boolean",
					"default":     false,
				},
			},
		)
	case TypeContent:
		props[key] = strictObjectSchema(
			"Content no selected file may contain.",
			map[string]any{
				"forbid": map[string]any{
					"description": "RE2 regular expression; every matching line is one Violation.",
					"type":        "string",
					"pattern":     `\S`,
				},
			},
			"forbid",
		)
	case TypeExtension:
		props[key] = map[string]any{
			"description": "The extension rule name registered under .arclint/extensions or distributed by an extended Pattern.",
			"type":        "string",
			"pattern":     `\S`,
		}
		props["with"] = map[string]any{
			"description":          "Parameters validated host-side against the extension's published schema before any extension code runs.",
			"type":                 "object",
			"additionalProperties": true,
		}
	}
	return strictObjectSchema(description, props, required...)
}

func structureAssertionSchema(expanded bool) map[string]any {
	itemRef := schemaRef("glob")
	desc := "Plain structure: at least one non-empty glob list."
	props := map[string]any{}
	if expanded {
		itemRef = schemaRef("expandedGlob")
		desc = "Expanded structure: one universally quantified claim whose globs derive from a recorded Ubiquitous Language collection. A project recording nothing derives no obligations; the Rule exists and says so."
		props["each"] = schemaRef("expansionSource")
	}
	props["require"] = map[string]any{
		"description": "Each glob must match at least one member file.",
		"type":        "array",
		"uniqueItems": true,
		"items":       itemRef,
	}
	props["forbid"] = map[string]any{
		"description": "No member file may match any glob.",
		"type":        "array",
		"uniqueItems": true,
		"items":       itemRef,
	}
	var required []string
	if expanded {
		required = []string{"each"}
	}
	s := strictObjectSchema(desc, props, required...)
	s["anyOf"] = []any{
		nonEmptyListBranch("require", "At least one required glob."),
		nonEmptyListBranch("forbid", "At least one forbidden glob."),
	}
	return s
}

// nonEmptyListBranch is one anyOf alternative demanding that the named
// list is present and non-empty.
func nonEmptyListBranch(key, description string) map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{key},
		"properties": map[string]any{
			key: map[string]any{"description": description, "type": "array", "minItems": 1, "uniqueItems": true},
		},
	}
}

func overrideSchema() map[string]any {
	s := strictObjectSchema(
		defDescription("override"),
		map[string]any{
			"disable":  schemaRef("reason"),
			"severity": schemaRef("severity"),
			"exclude":  schemaRef("exclusion"),
			"suppress": schemaRef("suppression"),
		},
	)
	s["minProperties"] = 1
	return s
}

func exclusionSchema() map[string]any {
	s := strictObjectSchema(
		defDescription("exclusion"),
		map[string]any{
			"paths":   schemaRef("excludedPaths"),
			"modules": schemaRef("excludedModules"),
			"reason":  schemaRef("reason"),
		},
		"reason",
	)
	s["anyOf"] = []any{
		map[string]any{"required": []string{"paths"}},
		map[string]any{"required": []string{"modules"}},
	}
	return s
}

func suppressionSchema() map[string]any {
	return strictObjectSchema(
		defDescription("suppression"),
		map[string]any{
			"paths":  schemaRef("suppressedPaths"),
			"reason": schemaRef("reason"),
		},
		"paths", "reason",
	)
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

// strictObjectSchema builds an object schema that rejects unknown keys,
// mirroring the loader's strict decoding and per-kind field whitelists.
func strictObjectSchema(description string, properties map[string]any, required ...string) map[string]any {
	out := map[string]any{
		"description":          description,
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

// schemaRef references a named definition and repeats its description
// beside the reference, the one place a reader or an editor looks.
func schemaRef(name string) map[string]any {
	return map[string]any{"$ref": "#/$defs/" + name, "description": defDescription(name)}
}

func oneOfRefs(names ...string) map[string]any {
	refs := make([]any, 0, len(names))
	for _, name := range names {
		refs = append(refs, schemaRef(name))
	}
	return map[string]any{"oneOf": refs}
}
