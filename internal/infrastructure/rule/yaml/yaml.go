// Package yamlrule loads complete Rule aggregates from YAML
// representations of the target ruleset format. The accepted grammar
// is exactly what rules.yaml uses — runtime, scan, modules,
// contracts (consumes plus structure, naming, and extension invariants),
// and dependencies (layers, protected, acyclic) — plus the pattern
// identity header used by Pattern distribution files. A representation
// that cannot become a valid Rule is an error, never a partial value.
package yamlrule

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Repository implements the domain-owned rule.Repository port over one
// ruleset file.
type Repository struct {
	path string
}

// NewRepository binds the repository to a ruleset file path.
func NewRepository(path string) (Repository, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Repository{}, fmt.Errorf("ruleset path: %w", err)
	}
	return Repository{path: abs}, nil
}

// Path returns the absolute ruleset file path.
func (r Repository) Path() string { return r.path }

// Root returns the directory containing the ruleset file: the
// repository root every repo-relative path is resolved against.
func (r Repository) Root() string { return filepath.Dir(r.path) }

// ConfiguredRules loads, validates, and translates the ruleset into
// complete Rule aggregates.
func (r Repository) ConfiguredRules() (rule.Configured, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		return rule.Configured{}, fmt.Errorf("read ruleset: %w", err)
	}
	doc, err := Load(data, r.path)
	if err != nil {
		return rule.Configured{}, err
	}
	if doc.Pattern != nil {
		return rule.Configured{}, fmt.Errorf("%s: a pattern distribution file is not a repository ruleset", r.path)
	}
	return doc.Configured, nil
}

// PatternIdentity is the identity header of a Pattern distribution
// file.
type PatternIdentity struct {
	Namespace string
	Name      string
	Version   string
	Coverage  []rule.Language
}

// Document is one parsed and translated ruleset file.
type Document struct {
	Configured rule.Configured
	// Pattern is non-nil for Pattern distribution files.
	Pattern *PatternIdentity
}

type documentDoc struct {
	Pattern      *patternDoc            `yaml:"pattern"`
	Runtime      []string               `yaml:"runtime"`
	Scan         scanDoc                `yaml:"scan"`
	Modules      map[string]moduleDoc   `yaml:"modules"`
	Contracts    map[string]contractDoc `yaml:"contracts"`
	Dependencies []dependencyDoc        `yaml:"dependencies"`
}

type patternDoc struct {
	Namespace string   `yaml:"namespace"`
	Name      string   `yaml:"name"`
	Version   string   `yaml:"version"`
	Coverage  []string `yaml:"coverage"`
}

type scanDoc struct {
	UnknownImports  string   `yaml:"unknown_imports"`
	Exclude         []string `yaml:"exclude"`
	IncludeTestdata bool     `yaml:"include_testdata"`
}

type moduleDoc struct {
	Paths       []string `yaml:"paths"`
	Description string   `yaml:"description"`
}

type contractDoc struct {
	Consumes   *consumesDoc   `yaml:"consumes"`
	Invariants []invariantDoc `yaml:"invariants"`
}

type consumesDoc struct {
	ID       string    `yaml:"id"`
	Internal *[]string `yaml:"internal"`
	External string    `yaml:"external"`
	Stdlib   string    `yaml:"stdlib"`
	Severity string    `yaml:"severity"`
}

type invariantDoc struct {
	ID       string         `yaml:"id"`
	Kind     string         `yaml:"kind"`
	Severity string         `yaml:"severity"`
	Files    string         `yaml:"files"`
	Case     string         `yaml:"case"`
	Require  []string       `yaml:"require"`
	Forbid   []string       `yaml:"forbid"`
	Uses     string         `yaml:"uses"`
	With     map[string]any `yaml:"with"`
}

type dependencyDoc struct {
	ID       string   `yaml:"id"`
	Kind     string   `yaml:"kind"`
	Severity string   `yaml:"severity"`
	Layers   []string `yaml:"layers"`
	Module   string   `yaml:"module"`
	Allow    []string `yaml:"allow"`
	Modules  []string `yaml:"modules"`
}

// Load parses one target-format ruleset document strictly: unknown
// keys, unknown kinds, and Rules without explicit IDs are errors.
func Load(data []byte, source string) (Document, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var doc documentDoc
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("%s: %v", source, err)
	}
	fail := func(format string, args ...any) (Document, error) {
		return Document{}, fmt.Errorf("%s: %s", source, fmt.Sprintf(format, args...))
	}

	languages, err := translateRuntime(doc.Runtime)
	if err != nil {
		return fail("%v", err)
	}
	scan, err := translateScan(doc.Scan)
	if err != nil {
		return fail("%v", err)
	}
	modules, err := translateModules(doc.Modules)
	if err != nil {
		return fail("%v", err)
	}

	declared := map[string]bool{}
	for _, m := range modules {
		declared[string(m.Name())] = true
	}

	var rules []rule.Rule
	seen := map[string]bool{}
	appendRule := func(spec rule.Spec, where string) error {
		if spec.ID == "" {
			return fmt.Errorf("%s: missing id (every Rule requires an explicit stable Rule ID)", where)
		}
		r, err := rule.New(spec)
		if err != nil {
			return fmt.Errorf("%s: %v", where, err)
		}
		q := r.ID().Qualified()
		if seen[q] {
			return fmt.Errorf("%s: duplicate rule id %q", where, q)
		}
		seen[q] = true
		rules = append(rules, r)
		return nil
	}

	contractNames := make([]string, 0, len(doc.Contracts))
	for name := range doc.Contracts {
		contractNames = append(contractNames, name)
	}
	sort.Strings(contractNames)
	for _, name := range contractNames {
		if !declared[name] {
			return fail("contracts.%s: module is not declared", name)
		}
		contract := doc.Contracts[name]
		if contract.Consumes != nil {
			spec, err := consumesSpec(name, *contract.Consumes)
			if err != nil {
				return fail("contracts.%s.consumes: %v", name, err)
			}
			if err := appendRule(spec, fmt.Sprintf("contracts.%s.consumes", name)); err != nil {
				return fail("%v", err)
			}
		}
		for i, inv := range contract.Invariants {
			where := fmt.Sprintf("contracts.%s.invariants[%d]", name, i)
			spec, err := invariantSpec(name, inv)
			if err != nil {
				return fail("%s: %v", where, err)
			}
			if err := appendRule(spec, where); err != nil {
				return fail("%v", err)
			}
		}
	}
	for i, dep := range doc.Dependencies {
		where := fmt.Sprintf("dependencies[%d]", i)
		spec, err := dependencySpec(dep)
		if err != nil {
			return fail("%s: %v", where, err)
		}
		if err := appendRule(spec, where); err != nil {
			return fail("%v", err)
		}
	}

	out := Document{Configured: rule.Configured{
		Rules:     rules,
		Modules:   modules,
		Languages: languages,
		Scan:      scan,
	}}
	if doc.Pattern != nil {
		identity, err := translatePattern(*doc.Pattern)
		if err != nil {
			return fail("pattern: %v", err)
		}
		out.Pattern = &identity
	}
	return out, nil
}

func translateRuntime(runtime []string) ([]rule.Language, error) {
	aliases := map[string]rule.Language{
		"go": rule.LanguageGo,
		"ts": rule.LanguageTypeScript,
		"py": rule.LanguagePython,
	}
	out := make([]rule.Language, 0, len(runtime))
	for _, target := range runtime {
		l, ok := aliases[target]
		if !ok {
			return nil, fmt.Errorf("runtime target %q: not one of go, ts, py", target)
		}
		out = append(out, l)
	}
	return out, nil
}

func translateScan(doc scanDoc) (rule.Scan, error) {
	policy, err := rule.ParseUnknownImportPolicy(doc.UnknownImports)
	if err != nil {
		return rule.Scan{}, fmt.Errorf("scan: %v", err)
	}
	exclude, err := rule.NewGlobs(doc.Exclude)
	if err != nil {
		return rule.Scan{}, fmt.Errorf("scan.exclude: %v", err)
	}
	return rule.Scan{Exclude: exclude, IncludeTestdata: doc.IncludeTestdata, UnknownImports: policy}, nil
}

func translateModules(docs map[string]moduleDoc) ([]rule.Module, error) {
	names := make([]string, 0, len(docs))
	for name := range docs {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]rule.Module, 0, len(names))
	for _, name := range names {
		doc := docs[name]
		moduleName, err := rule.NewModuleName(name)
		if err != nil {
			return nil, fmt.Errorf("modules.%s: %v", name, err)
		}
		paths, err := rule.NewGlobs(doc.Paths)
		if err != nil {
			return nil, fmt.Errorf("modules.%s: %v", name, err)
		}
		m, err := rule.NewModule(moduleName, doc.Description, paths)
		if err != nil {
			return nil, fmt.Errorf("modules.%s: %v", name, err)
		}
		out = append(out, m)
	}
	return out, nil
}

func consumesSpec(module string, doc consumesDoc) (rule.Spec, error) {
	moduleName, err := rule.NewModuleName(module)
	if err != nil {
		return rule.Spec{}, fmt.Errorf("applicability: %w", err)
	}
	scope, err := rule.ModuleApplicability([]rule.ModuleName{moduleName})
	if err != nil {
		return rule.Spec{}, fmt.Errorf("module: %w", err)
	}
	params := rule.ConsumesParams{}
	if doc.Internal != nil {
		names := make([]rule.ModuleName, 0, len(*doc.Internal))
		for _, n := range *doc.Internal {
			name, err := rule.NewModuleName(n)
			if err != nil {
				return rule.Spec{}, fmt.Errorf("internal: %v", err)
			}
			names = append(names, name)
		}
		allow, err := rule.NewAllowList(names...)
		if err != nil {
			return rule.Spec{}, fmt.Errorf("internal: %v", err)
		}
		params.Internal = &allow
	}
	if params.External, err = rule.ParseImportPolicy(doc.External); err != nil {
		return rule.Spec{}, fmt.Errorf("external: %v", err)
	}
	if params.Stdlib, err = rule.ParseImportPolicy(doc.Stdlib); err != nil {
		return rule.Spec{}, fmt.Errorf("stdlib: %v", err)
	}
	return rule.Spec{
		ID:            doc.ID,
		Type:          rule.TypeConsumes,
		Severity:      doc.Severity,
		Params:        params,
		Applicability: scope,
	}, nil
}

func invariantSpec(module string, doc invariantDoc) (rule.Spec, error) {
	moduleName, err := rule.NewModuleName(module)
	if err != nil {
		return rule.Spec{}, fmt.Errorf("applicability: %w", err)
	}
	forbidFields := func(kind string, fields map[string]bool) error {
		for name, set := range fields {
			if set {
				return fmt.Errorf("kind %s does not accept %s", kind, name)
			}
		}
		return nil
	}
	var files []rule.Glob
	if doc.Files != "" {
		g, err := rule.NewGlob(doc.Files)
		if err != nil {
			return rule.Spec{}, fmt.Errorf("files: %w", err)
		}
		files = []rule.Glob{g}
	}
	spec := rule.Spec{ID: doc.ID, Severity: doc.Severity}
	switch doc.Kind {
	case "structure":
		if err := forbidFields("structure", map[string]bool{
			"files": doc.Files != "", "case": doc.Case != "",
			"uses": doc.Uses != "", "with": len(doc.With) > 0,
		}); err != nil {
			return rule.Spec{}, err
		}
		require, err := rule.NewGlobs(doc.Require)
		if err != nil {
			return rule.Spec{}, fmt.Errorf("require: %v", err)
		}
		forbid, err := rule.NewGlobs(doc.Forbid)
		if err != nil {
			return rule.Spec{}, fmt.Errorf("forbid: %v", err)
		}
		spec.Type = rule.TypeStructure
		spec.Params = rule.StructureParams{Require: require, Forbid: forbid}
		spec.Applicability, err = rule.ModuleApplicability([]rule.ModuleName{moduleName})
		if err != nil {
			return rule.Spec{}, fmt.Errorf("structure: %w", err)
		}
	case "extension":
		if err := forbidFields("extension", map[string]bool{
			"require": len(doc.Require) > 0, "forbid": len(doc.Forbid) > 0,
			"case": doc.Case != "",
		}); err != nil {
			return rule.Spec{}, err
		}
		spec.Type = rule.TypeExtension
		spec.Params = rule.ExtensionParams{Uses: doc.Uses, With: doc.With}
		spec.Applicability, err = rule.ModuleApplicability([]rule.ModuleName{moduleName}, files...)
		if err != nil {
			return rule.Spec{}, fmt.Errorf("extension: %w", err)
		}
	case "naming":
		if err := forbidFields("naming", map[string]bool{
			"require": len(doc.Require) > 0, "forbid": len(doc.Forbid) > 0,
			"uses": doc.Uses != "", "with": len(doc.With) > 0,
		}); err != nil {
			return rule.Spec{}, err
		}
		caseSpec, err := rule.NewCaseSpec(doc.Case)
		if err != nil {
			return rule.Spec{}, fmt.Errorf("naming: %w", err)
		}
		spec.Type = rule.TypeNaming
		spec.Params = rule.NamingParams{Case: caseSpec}
		spec.Applicability, err = rule.ModuleApplicability([]rule.ModuleName{moduleName}, files...)
		if err != nil {
			return rule.Spec{}, fmt.Errorf("naming: %w", err)
		}
	default:
		return rule.Spec{}, fmt.Errorf("invariant kind %q is not part of the target ruleset format", doc.Kind)
	}
	return spec, nil
}

func dependencySpec(doc dependencyDoc) (rule.Spec, error) {
	scope, err := rule.RepositoryApplicability()
	if err != nil {
		return rule.Spec{}, fmt.Errorf("repository: %w", err)
	}
	names := func(field string, values []string) ([]rule.ModuleName, error) {
		out := make([]rule.ModuleName, 0, len(values))
		for _, v := range values {
			name, err := rule.NewModuleName(v)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", field, err)
			}
			out = append(out, name)
		}
		return out, nil
	}
	spec := rule.Spec{ID: doc.ID, Severity: doc.Severity, Applicability: scope}
	switch doc.Kind {
	case "layers":
		if doc.Module != "" || len(doc.Allow) > 0 || len(doc.Modules) > 0 {
			return rule.Spec{}, fmt.Errorf("kind layers accepts only layers")
		}
		layers, err := names("layers", doc.Layers)
		if err != nil {
			return rule.Spec{}, err
		}
		spec.Type = rule.TypeLayers
		spec.Params = rule.LayersParams{Layers: layers}
	case "protected":
		if len(doc.Layers) > 0 || len(doc.Modules) > 0 {
			return rule.Spec{}, fmt.Errorf("kind protected accepts only module and allow")
		}
		module, err := rule.NewModuleName(doc.Module)
		if err != nil {
			return rule.Spec{}, fmt.Errorf("module: %v", err)
		}
		allow, err := names("allow", doc.Allow)
		if err != nil {
			return rule.Spec{}, err
		}
		spec.Type = rule.TypeProtected
		spec.Params = rule.ProtectedParams{Module: module, Allow: allow}
	case "acyclic":
		if len(doc.Layers) > 0 || doc.Module != "" || len(doc.Allow) > 0 {
			return rule.Spec{}, fmt.Errorf("kind acyclic accepts only modules")
		}
		modules, err := names("modules", doc.Modules)
		if err != nil {
			return rule.Spec{}, err
		}
		spec.Type = rule.TypeAcyclic
		spec.Params = rule.AcyclicParams{Modules: modules}
	default:
		return rule.Spec{}, fmt.Errorf("dependency kind %q is not part of the target ruleset format", doc.Kind)
	}
	return spec, nil
}

func translatePattern(doc patternDoc) (PatternIdentity, error) {
	if doc.Namespace == "" || doc.Name == "" || doc.Version == "" {
		return PatternIdentity{}, fmt.Errorf("namespace, name, and version are required")
	}
	coverage := make([]rule.Language, 0, len(doc.Coverage))
	for _, c := range doc.Coverage {
		l, err := rule.ParseLanguage(c)
		if err != nil {
			return PatternIdentity{}, fmt.Errorf("coverage: %w", err)
		}
		coverage = append(coverage, l)
	}
	return PatternIdentity{
		Namespace: doc.Namespace,
		Name:      doc.Name,
		Version:   doc.Version,
		Coverage:  coverage,
	}, nil
}

// DiscoverPath locates the ruleset file: from a starting directory
// upward to the filesystem root.
func DiscoverPath(start, filename string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("ruleset discovery: %w", err)
	}
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	dir := abs
	for {
		candidate := filepath.Join(dir, filename)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s found in %s or any parent directory", filename, abs)
		}
		dir = parent
	}
}
