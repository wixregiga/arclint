// Package yaml loads the source-neutral pattern.yaml distribution format.
// Filesystem and embedded Pattern sources use this one parser; where the
// bytes came from never changes the accepted contract.
//
//nolint:goconst // Manifest field and location names intentionally match the published YAML.
package yaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/pattern"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/ruletest"
)

// FileName is the canonical manifest name at a Pattern package root.
const FileName = "pattern.yaml"

type manifestDoc struct {
	Pattern    identityDoc    `yaml:"pattern"`
	Coverage   []string       `yaml:"coverage"`
	Modules    []moduleDoc    `yaml:"modules"`
	Rules      []ruleDoc      `yaml:"rules"`
	Extensions []extensionDoc `yaml:"extensions"`
	Tests      *testsDoc      `yaml:"tests"`
}

type identityDoc struct {
	Namespace string `yaml:"namespace"`
	Name      string `yaml:"name"`
	Version   string `yaml:"version"`
}

type moduleDoc struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Paths       []string `yaml:"paths"`
}

type extensionDoc struct {
	Name  string `yaml:"name"`
	Entry string `yaml:"entry"`
}

type testsDoc struct {
	Root string `yaml:"root"`
}

// ruleDoc is the union of the eight published Rule representations.
// fields remembers presence so an empty field belonging to another kind
// is still rejected rather than silently ignored.
type ruleDoc struct {
	ID       string         `yaml:"id"`
	Kind     string         `yaml:"kind"`
	Claim    string         `yaml:"claim"`
	Severity string         `yaml:"severity"`
	Module   string         `yaml:"module"`
	Files    string         `yaml:"files"`
	Allow    *[]string      `yaml:"allow"`
	Forbid   []string       `yaml:"forbid"`
	Require  []string       `yaml:"require"`
	Each     string         `yaml:"each"`
	Case     string         `yaml:"case"`
	Layers   []string       `yaml:"layers"`
	Modules  []string       `yaml:"modules"`
	Folders  []string       `yaml:"folders"`
	Uses     string         `yaml:"uses"`
	With     map[string]any `yaml:"with"`
	fields   map[string]bool
}

func (d *ruleDoc) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("must be a mapping")
	}
	known := map[string]bool{
		"id": true, "kind": true, "claim": true, "severity": true,
		"module": true, "files": true, "allow": true, "forbid": true,
		"require": true, "each": true, "case": true, "layers": true,
		"modules": true, "folders": true, "uses": true, "with": true,
	}
	fields := make(map[string]bool, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		name := node.Content[i].Value
		if !known[name] {
			return fmt.Errorf("line %d: field %s not found in pattern Rule", node.Content[i].Line, name)
		}
		if fields[name] {
			return fmt.Errorf("line %d: duplicate field %s", node.Content[i].Line, name)
		}
		fields[name] = true
	}
	type plain ruleDoc
	var decoded plain
	if err := node.Decode(&decoded); err != nil {
		return fmt.Errorf("decode Pattern Rule: %w", err)
	}
	*d = ruleDoc(decoded)
	d.fields = fields
	return nil
}

// Load reads and validates one complete Pattern tree rooted at root in
// fileSystem. root is an fs.ValidPath package directory ("." is valid).
// The returned aggregate owns the exact bytes of every file in the tree.
func Load(fileSystem fs.FS, root string) (pattern.Pattern, error) {
	if fileSystem == nil {
		return pattern.Pattern{}, fmt.Errorf("pattern manifest: missing filesystem")
	}
	if !fs.ValidPath(root) {
		return pattern.Pattern{}, fmt.Errorf("pattern root %q: must be an fs.ValidPath", root)
	}
	packageFS, err := fs.Sub(fileSystem, root)
	if err != nil {
		return pattern.Pattern{}, fmt.Errorf("pattern root %q: %w", root, err)
	}
	manifestPath := path.Join(root, FileName)
	data, err := fs.ReadFile(packageFS, FileName)
	if err != nil {
		return pattern.Pattern{}, fmt.Errorf("%s: %w", manifestPath, err)
	}

	var doc manifestDoc
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return pattern.Pattern{}, fmt.Errorf("%s: %v", manifestPath, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("multiple YAML documents are not accepted")
		}
		return pattern.Pattern{}, fmt.Errorf("%s: %v", manifestPath, err)
	}
	fail := func(location, format string, args ...any) (pattern.Pattern, error) {
		return pattern.Pattern{}, fmt.Errorf("%s: %s: %s", manifestPath, location, fmt.Sprintf(format, args...))
	}

	if strings.IndexFunc(doc.Pattern.Namespace, unicode.IsSpace) >= 0 {
		return fail("pattern.namespace", "must not contain whitespace")
	}
	if strings.IndexFunc(doc.Pattern.Name, unicode.IsSpace) >= 0 {
		return fail("pattern.name", "must not contain whitespace")
	}
	ref, err := pattern.NewReference(doc.Pattern.Namespace, doc.Pattern.Name, doc.Pattern.Version)
	if err != nil {
		location := "pattern"
		switch {
		case doc.Pattern.Namespace == "":
			location = "pattern.namespace"
		case doc.Pattern.Name == "":
			location = "pattern.name"
		case doc.Pattern.Version == "" || strings.Contains(err.Error(), "version"):
			location = "pattern.version"
		}
		return fail(location, "%v", err)
	}

	coverage := make([]rule.Language, 0, len(doc.Coverage))
	seenCoverage := map[rule.Language]bool{}
	for i, spelling := range doc.Coverage {
		language, err := rule.ParseLanguage(spelling)
		if err != nil {
			return fail(fmt.Sprintf("coverage[%d]", i), "%v", err)
		}
		if seenCoverage[language] {
			return fail(fmt.Sprintf("coverage[%d]", i), "duplicate language %q", language)
		}
		seenCoverage[language] = true
		coverage = append(coverage, language)
	}

	if len(doc.Modules) == 0 {
		return fail("modules", "at least one Module is required")
	}
	modules := make([]rule.Module, 0, len(doc.Modules))
	declared := map[rule.ModuleName]bool{}
	for i, declaration := range doc.Modules {
		where := fmt.Sprintf("modules[%d]", i)
		name, err := rule.NewModuleName(declaration.Name)
		if err != nil {
			return fail(where+".name", "%v", err)
		}
		if declared[name] {
			return fail(where+".name", "duplicate module %q", name)
		}
		globs, err := rule.NewGlobs(declaration.Paths)
		if err != nil {
			return fail(where+".paths", "%v", err)
		}
		module, err := rule.NewModule(name, declaration.Description, globs)
		if err != nil {
			return fail(where, "%v", err)
		}
		declared[name] = true
		modules = append(modules, module)
	}

	rules := make([]rule.Rule, 0, len(doc.Rules))
	seenRules := map[string]bool{}
	for i, declaration := range doc.Rules {
		where := fmt.Sprintf("rules[%d]", i)
		built, err := translateRule(declaration, declared)
		if err != nil {
			return fail(where, "%v", err)
		}
		id := built.ID().Qualified()
		if seenRules[id] {
			return fail(where+".id", "duplicate rule id %q", id)
		}
		seenRules[id] = true
		rules = append(rules, built)
	}

	extensions := make([]rule.Extension, 0, len(doc.Extensions))
	seenExtensions := map[string]bool{}
	seenEntries := map[string]bool{}
	for i, declaration := range doc.Extensions {
		where := fmt.Sprintf("extensions[%d]", i)
		if seenExtensions[declaration.Name] {
			return fail(where+".name", "duplicate extension %q", declaration.Name)
		}
		if seenEntries[declaration.Entry] {
			return fail(where+".entry", "duplicate extension entry %q", declaration.Entry)
		}
		if !safeTreePath(declaration.Entry) {
			return fail(where+".entry", "%q is not a canonical relative path", declaration.Entry)
		}
		source, err := fs.ReadFile(packageFS, declaration.Entry)
		if err != nil {
			return fail(where+".entry", "%q: %v", declaration.Entry, err)
		}
		extension, err := rule.NewExtension(declaration.Name, declaration.Entry, source)
		if err != nil {
			return fail(where, "%v", err)
		}
		seenExtensions[declaration.Name] = true
		seenEntries[declaration.Entry] = true
		extensions = append(extensions, extension)
	}
	for i, declaration := range doc.Rules {
		if declaration.Kind == string(rule.TypeExtension) && !seenExtensions[declaration.Uses] {
			return fail(fmt.Sprintf("rules[%d].uses", i),
				"Extension %q is not declared in extensions", declaration.Uses)
		}
	}

	var tests []rule.Test
	if doc.Tests != nil {
		if !safeTreePath(doc.Tests.Root) {
			return fail("tests.root", "%q is not a canonical relative path", doc.Tests.Root)
		}
		tests, err = ruletest.LoadFS(packageFS, doc.Tests.Root, manifestPath+": tests.root")
		if err != nil {
			return fail("tests.root", "%v", err)
		}
		if len(tests) == 0 {
			return fail("tests.root", "%q contains no Rule Test YAML files", doc.Tests.Root)
		}
	}

	files, err := loadTree(packageFS)
	if err != nil {
		return fail("tree", "%v", err)
	}
	loaded, err := pattern.New(pattern.Spec{
		Reference: ref, Modules: modules, Rules: rules, Extensions: extensions,
		Tests: tests, Coverage: coverage, Files: files,
	})
	if err != nil {
		return fail("pattern", "%v", err)
	}
	return loaded, nil
}

func translateRule(doc ruleDoc, declared map[rule.ModuleName]bool) (rule.Rule, error) {
	typeValue, err := rule.ParseType(doc.Kind)
	if err != nil {
		return rule.Rule{}, fmt.Errorf("kind: %v", err)
	}
	allowed := map[rule.Type]map[string]bool{
		rule.TypeConsumes:     fieldSet("id", "kind", "claim", "severity", "module", "allow", "forbid"),
		rule.TypeStructure:    fieldSet("id", "kind", "claim", "severity", "module", "each", "require", "forbid"),
		rule.TypeNaming:       fieldSet("id", "kind", "claim", "severity", "module", "files", "case"),
		rule.TypeLayers:       fieldSet("id", "kind", "claim", "severity", "layers"),
		rule.TypeProtected:    fieldSet("id", "kind", "claim", "severity", "module", "allow"),
		rule.TypeIndependence: fieldSet("id", "kind", "claim", "severity", "folders"),
		rule.TypeAcyclic:      fieldSet("id", "kind", "claim", "severity", "modules"),
		rule.TypeExtension:    fieldSet("id", "kind", "claim", "severity", "module", "files", "uses", "with"),
	}[typeValue]
	for field := range doc.fields {
		if !allowed[field] {
			return rule.Rule{}, fmt.Errorf("%s: field is not accepted by kind %s", field, typeValue)
		}
	}

	spec := rule.Spec{ID: doc.ID, Type: typeValue, Claim: doc.Claim, Severity: doc.Severity}
	repositoryScope := func() error {
		scope, scopeErr := rule.RepositoryApplicability()
		spec.Applicability = scope
		if scopeErr != nil {
			return fmt.Errorf("repository applicability: %w", scopeErr)
		}
		return nil
	}
	moduleScope := func(required bool) error {
		if doc.Module == "" {
			if required {
				return fmt.Errorf("module: required by kind %s", typeValue)
			}
			files, err := optionalFiles(doc.Files)
			if err != nil {
				return err
			}
			spec.Applicability, err = rule.RepositoryApplicability(files...)
			if err != nil {
				return fmt.Errorf("repository applicability: %w", err)
			}
			return nil
		}
		name, err := declaredModule("module", doc.Module, declared)
		if err != nil {
			return err
		}
		files, err := optionalFiles(doc.Files)
		if err != nil {
			return err
		}
		spec.Applicability, err = rule.ModuleApplicability([]rule.ModuleName{name}, files...)
		if err != nil {
			return fmt.Errorf("module applicability: %w", err)
		}
		return nil
	}

	switch typeValue {
	case rule.TypeConsumes:
		if err := moduleScope(true); err != nil {
			return rule.Rule{}, err
		}
		params := rule.ConsumesParams{External: rule.ImportAllow, Stdlib: rule.ImportAllow}
		if doc.Allow != nil {
			names, err := declaredModules("allow", *doc.Allow, declared)
			if err != nil {
				return rule.Rule{}, err
			}
			allow, err := rule.NewAllowList(names...)
			if err != nil {
				return rule.Rule{}, fmt.Errorf("allow: %v", err)
			}
			params.Internal = &allow
		}
		for i, category := range doc.Forbid {
			switch category {
			case "external":
				params.External = rule.ImportForbid
			case "stdlib":
				params.Stdlib = rule.ImportForbid
			default:
				return rule.Rule{}, fmt.Errorf("forbid[%d]: %q is not external or stdlib", i, category)
			}
		}
		if len(doc.Forbid) != len(uniqueStrings(doc.Forbid)) {
			return rule.Rule{}, fmt.Errorf("forbid: duplicate import category")
		}
		spec.Params = params
	case rule.TypeStructure:
		if err := moduleScope(true); err != nil {
			return rule.Rule{}, err
		}
		params, expansion, err := translateStructure(doc)
		if err != nil {
			return rule.Rule{}, err
		}
		spec.Params, spec.Expansion = params, expansion
	case rule.TypeNaming:
		if err := moduleScope(true); err != nil {
			return rule.Rule{}, err
		}
		caseSpec, err := rule.NewCaseSpec(doc.Case)
		if err != nil {
			return rule.Rule{}, fmt.Errorf("case: %v", err)
		}
		spec.Params = rule.NamingParams{Case: caseSpec}
	case rule.TypeLayers:
		if err := repositoryScope(); err != nil {
			return rule.Rule{}, err
		}
		names, err := declaredModules("layers", doc.Layers, declared)
		if err != nil {
			return rule.Rule{}, err
		}
		spec.Params = rule.LayersParams{Layers: names}
	case rule.TypeProtected:
		if err := repositoryScope(); err != nil {
			return rule.Rule{}, err
		}
		module, err := declaredModule("module", doc.Module, declared)
		if err != nil {
			return rule.Rule{}, err
		}
		var values []string
		if doc.Allow != nil {
			values = *doc.Allow
		}
		allow, err := declaredModules("allow", values, declared)
		if err != nil {
			return rule.Rule{}, err
		}
		spec.Params = rule.ProtectedParams{Module: module, Allow: allow}
	case rule.TypeIndependence:
		if err := repositoryScope(); err != nil {
			return rule.Rule{}, err
		}
		folders, err := rule.NewGlobs(doc.Folders)
		if err != nil {
			return rule.Rule{}, fmt.Errorf("folders: %v", err)
		}
		spec.Params = rule.IndependenceParams{Folders: folders}
	case rule.TypeAcyclic:
		if err := repositoryScope(); err != nil {
			return rule.Rule{}, err
		}
		names, err := declaredModules("modules", doc.Modules, declared)
		if err != nil {
			return rule.Rule{}, err
		}
		spec.Params = rule.AcyclicParams{Modules: names}
	case rule.TypeExtension:
		if err := moduleScope(false); err != nil {
			return rule.Rule{}, err
		}
		spec.Params = rule.ExtensionParams{Uses: doc.Uses, With: doc.With}
	}
	built, err := rule.New(spec)
	if err != nil {
		return rule.Rule{}, fmt.Errorf("construct Rule: %w", err)
	}
	return built, nil
}

func translateStructure(doc ruleDoc) (rule.StructureParams, *rule.Expansion, error) {
	if doc.Each == "" {
		require, err := rule.NewGlobs(doc.Require)
		if err != nil {
			return rule.StructureParams{}, nil, fmt.Errorf("require: %w", err)
		}
		forbid, err := rule.NewGlobs(doc.Forbid)
		if err != nil {
			return rule.StructureParams{}, nil, fmt.Errorf("forbid: %w", err)
		}
		return rule.StructureParams{Require: require, Forbid: forbid}, nil, nil
	}
	expansion, err := rule.NewExpansion(doc.Each, doc.Require, doc.Forbid)
	if err != nil {
		return rule.StructureParams{}, nil, fmt.Errorf("each: %w", err)
	}
	params, err := expansion.Resolve(vocab.UbiquitousLanguage{})
	if err != nil {
		return rule.StructureParams{}, nil, fmt.Errorf("each: %w", err)
	}
	return params, &expansion, nil
}

func optionalFiles(spelling string) ([]rule.Glob, error) {
	if spelling == "" {
		return nil, nil
	}
	glob, err := rule.NewGlob(spelling)
	if err != nil {
		return nil, fmt.Errorf("files: %v", err)
	}
	return []rule.Glob{glob}, nil
}

func declaredModule(field, spelling string, declared map[rule.ModuleName]bool) (rule.ModuleName, error) {
	name, err := rule.NewModuleName(spelling)
	if err != nil {
		return "", fmt.Errorf("%s: %v", field, err)
	}
	if !declared[name] {
		return "", fmt.Errorf("%s: module %q is not declared", field, name)
	}
	return name, nil
}

func declaredModules(field string, spellings []string, declared map[rule.ModuleName]bool) ([]rule.ModuleName, error) {
	out := make([]rule.ModuleName, 0, len(spellings))
	for i, spelling := range spellings {
		name, err := declaredModule(fmt.Sprintf("%s[%d]", field, i), spelling, declared)
		if err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, nil
}

func fieldSet(names ...string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, name := range names {
		out[name] = true
	}
	return out
}

func uniqueStrings(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func safeTreePath(name string) bool {
	return name != "" && name != "." && fs.ValidPath(name) && !strings.Contains(name, `\`)
}

func loadTree(fileSystem fs.FS) ([]pattern.File, error) {
	var names []string
	err := fs.WalkDir(fileSystem, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." || entry.IsDir() {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s: symbolic links are not distribution files", name)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s: non-regular distribution file", name)
		}
		names = append(names, name)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk distribution tree: %w", err)
	}
	sort.Strings(names)
	files := make([]pattern.File, 0, len(names))
	for _, name := range names {
		data, err := fs.ReadFile(fileSystem, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		file, err := pattern.NewFile(name, data)
		if err != nil {
			return nil, fmt.Errorf("distribution file %s: %w", name, err)
		}
		files = append(files, file)
	}
	return files, nil
}
