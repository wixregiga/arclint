package sobekextension

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// Evaluator implements the domain's ExtensionEvaluator port: discover
// and register the repository's extensions once, validate parameters
// host-side against the extension's published schema, and lend the
// sandboxed Host scoped to exactly the Rule's selected subjects.
// Per-finding severity, contract, and blame from the legacy wire shape
// are ignored: in the target model Severity belongs to the Rule.
type Evaluator struct {
	root      string
	opts      Options
	suppliers []ExtensionSupplier

	once     sync.Once
	registry *Registry
	loadErr  error
}

// ExtensionSupplier hands the host the Extension sources the
// repository's extended Patterns distribute. It runs once, on first
// use, so it may load the ruleset lazily.
type ExtensionSupplier func() ([]rule.ConfiguredExtension, error)

// NewEvaluator binds the evaluator to a repository root. On first use
// it registers the sources every supplier hands over, then the
// repository's own extensions under <root>/.arclint/extensions.
func NewEvaluator(root string, suppliers ...ExtensionSupplier) (*Evaluator, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("extensions root: %w", err)
	}
	for _, s := range suppliers {
		if s == nil {
			return nil, fmt.Errorf("extensions root %s: nil extension supplier", abs)
		}
	}
	return &Evaluator{
		root:      abs,
		opts:      Options{CacheDir: filepath.Join(abs, ".arclint", "cache")},
		suppliers: suppliers,
	}, nil
}

func (e *Evaluator) load() {
	e.once.Do(func() {
		var supplied []SuppliedSource
		for _, supplier := range e.suppliers {
			exts, err := supplier()
			if err != nil {
				e.loadErr = fmt.Errorf("pattern extensions: %w", err)
				return
			}
			for _, ext := range exts {
				supplied = append(supplied, SuppliedSource{
					Name:   SuppliedSourceName(ext),
					Source: ext.Extension.Source(),
				})
			}
		}
		e.registry, e.loadErr = Load(e.root, supplied, e.opts)
	})
}

// SuppliedSourceName is the attribution a Pattern-distributed
// extension carries in diagnostics and inventories: the Pattern
// reference followed by the file inside its extensions directory.
func SuppliedSourceName(ext rule.ConfiguredExtension) string {
	return ext.Pattern.String() + "/extensions/" + ext.Extension.FileName()
}

// Evaluate runs one extension rule over the selected subjects.
func (e *Evaluator) Evaluate(extension string, params map[string]any, subjects []string,
	zones []rule.Zone, obs conformance.Observations, knowledge vocab.UbiquitousLanguage,
) ([]conformance.ExtensionFinding, error) {
	e.load()
	if e.loadErr != nil {
		return nil, e.loadErr
	}
	ruleType := e.registry.Get(extension)
	if ruleType == nil {
		return nil, fmt.Errorf("no extension registers rule %q (looked in %s and the extended patterns)",
			extension, filepath.Join(e.root, filepath.FromSlash(ExtensionsDir)))
	}
	validated, err := ruleType.ValidateParams(params)
	if err != nil {
		return nil, err
	}
	reported, err := ruleType.Check(e.host(subjects, zones, obs, knowledge), validated)
	if err != nil {
		return nil, err
	}
	findings := make([]conformance.ExtensionFinding, 0, len(reported))
	for _, v := range reported {
		findings = append(findings, conformance.ExtensionFinding{
			Path:        v.Path,
			Line:        v.Line,
			Message:     v.Message,
			Remediation: v.FixHint,
		})
	}
	return findings, nil
}

// RegisteredExtensionRules implements the application's
// ExtensionInventory port: every rule definition the repository's
// extensions register, with its source file, in registration order.
func (e *Evaluator) RegisteredExtensionRules() ([]application.RegisteredExtensionRule, error) {
	e.load()
	if e.loadErr != nil {
		return nil, e.loadErr
	}
	types := e.registry.Types()
	out := make([]application.RegisteredExtensionRule, 0, len(types))
	for _, t := range types {
		out = append(out, application.RegisteredExtensionRule{Name: t.Name, Source: t.SourcePath})
	}
	return out, nil
}

// host lends the read-only capability surface, scoped to the selected
// subjects: files outside the Rule's Scope are invisible and
// unreadable, so exclusions hold mechanically.
func (e *Evaluator) host(subjects []string, zones []rule.Zone, obs conformance.Observations, knowledge vocab.UbiquitousLanguage) Host {
	inScope := make(map[string]bool, len(subjects))
	for _, s := range subjects {
		inScope[s] = true
	}
	domain := domainInfoFrom(knowledge)
	return Host{
		Files: func(glob string) ([]FileInfo, error) {
			var matcher *rule.Glob
			if glob != "" {
				g, err := rule.NewGlob(glob)
				if err != nil {
					return nil, fmt.Errorf("host files: %w", err)
				}
				matcher = &g
			}
			out := []FileInfo{}
			for _, f := range obs.Files() {
				if !inScope[f.Path] {
					continue
				}
				if matcher != nil && !matcher.Match(f.Path) {
					continue
				}
				base := filepath.Base(f.Path)
				ext := filepath.Ext(f.Path)
				out = append(out, FileInfo{
					Path: f.Path,
					Name: base,
					Stem: strings.TrimSuffix(base, ext),
					Ext:  ext,
					Dir:  filepath.ToSlash(filepath.Dir(f.Path)),
					Size: int(f.Size),
				})
			}
			return out, nil
		},
		Read: func(path string) (string, error) {
			if !inScope[path] {
				return "", fmt.Errorf("%s is outside this rule's scope", path)
			}
			content := obs.Content()
			if content == nil {
				return "", fmt.Errorf("read %s: no content capability on observations", path)
			}
			data, err := content.Read(path)
			if err != nil {
				return "", fmt.Errorf("read %s: %w", path, err)
			}
			return data, nil
		},
		Imports: func(path string) []ImportInfo {
			if !inScope[path] {
				return nil
			}
			facts, ok := obs.FactsFor(path)
			if !ok || !facts.Supports(rule.FactImports) {
				return nil
			}
			out := make([]ImportInfo, 0, len(facts.Imports))
			for _, imp := range facts.Imports {
				out = append(out, ImportInfo{
					Path:       imp.Path,
					Line:       imp.Line,
					Class:      string(imp.Class),
					TargetDir:  imp.TargetDir,
					TargetFile: imp.TargetFile,
				})
			}
			return out
		},
		Facts: func(path string) *FactsInfo {
			if !inScope[path] {
				return nil
			}
			facts, ok := obs.FactsFor(path)
			if !ok || !facts.Supports(rule.FactDeclarations) {
				return nil
			}
			info := &FactsInfo{Path: path, Package: facts.Package, Decls: []DeclInfo{}}
			for _, d := range facts.Declarations {
				decl := DeclInfo{
					Kind: d.Kind, Name: d.Name, Owner: d.Owner,
					Exported: d.Exported, StartLine: d.StartLine, EndLine: d.EndLine,
					Results: d.Results,
				}
				for _, param := range d.Params {
					decl.Params = append(decl.Params, ParamInfo{
						Name: param.Name, Type: param.Type,
						Optional: param.Optional, Variadic: param.Variadic,
					})
				}
				info.Decls = append(info.Decls, decl)
			}
			return info
		},
		Zones: func() map[string][]string {
			out := map[string][]string{}
			for _, m := range zones {
				members := []string{}
				for _, f := range subjects {
					if m.Contains(f) {
						members = append(members, f)
					}
				}
				out[string(m.Name())] = members
			}
			return out
		},
		ZoneOf: func(path string) []string {
			if !inScope[path] {
				return nil
			}
			var out []string
			for _, m := range zones {
				if m.Contains(path) {
					out = append(out, string(m.Name()))
				}
			}
			sort.Strings(out)
			return out
		},
		Domain:   func() DomainInfo { return domain },
		CaseTerm: rule.CaseTerm,
	}
}

// emptyDomainInfo returns a DomainInfo whose collections are non-nil
// empty slices so JavaScript sees arrays rather than null.
func emptyDomainInfo() DomainInfo {
	return DomainInfo{
		Source:    vocab.UbiquitousLanguageFileName,
		Contexts:  []DomainContextInfo{},
		Relations: []DomainRelationInfo{},
	}
}

// domainInfoFrom translates the recorded Language into the SDK wire
// shape, guaranteeing non-nil slices (never null in JS).
func domainInfoFrom(lang vocab.UbiquitousLanguage) DomainInfo {
	info := emptyDomainInfo()
	info.Project = lang.Project
	if len(lang.Contexts) > 0 {
		info.Contexts = make([]DomainContextInfo, len(lang.Contexts))
		for i, c := range lang.Contexts {
			info.Contexts[i] = DomainContextInfo{
				Name:           c.Name,
				Definition:     c.Definition,
				Aggregates:     aggregateInfos(c.Aggregates),
				ValueObjects:   valueObjectInfos(c.ValueObjects),
				Events:         eventInfos(c.Events),
				Services:       serviceInfos(c.Services),
				Specifications: specificationInfos(c.Specifications),
				Questions:      questionInfos(c.Questions),
				Line:           c.Line,
			}
		}
	}
	if len(lang.Relations) > 0 {
		info.Relations = make([]DomainRelationInfo, len(lang.Relations))
		for i, r := range lang.Relations {
			info.Relations[i] = DomainRelationInfo{
				From:        r.From,
				To:          r.To,
				Kind:        string(r.Kind),
				Description: r.Description,
				Line:        r.Line,
			}
		}
	}
	return info
}

func aggregateInfos(aggregates []vocab.Aggregate) []DomainAggregateInfo {
	out := make([]DomainAggregateInfo, len(aggregates))
	for i, a := range aggregates {
		out[i] = DomainAggregateInfo{
			Name:       a.Name,
			Definition: a.Definition,
			Identity:   a.Identity,
			Aliases:    a.Aliases,
			Entities:   entityInfos(a.Entities),
			Invariants: invariantInfos(a.Invariants),
			Assertions: assertionInfos(a.Assertions),
			Repository: a.Repository,
			Factory:    a.Factory,
			Line:       a.Line,
		}
	}
	return out
}

func entityInfos(entities []vocab.Entity) []DomainEntityInfo {
	out := make([]DomainEntityInfo, len(entities))
	for i, e := range entities {
		out[i] = DomainEntityInfo{
			Name:       e.Name,
			Definition: e.Definition,
			Identity:   e.Identity,
			Aliases:    e.Aliases,
			Line:       e.Line,
		}
	}
	return out
}

func valueObjectInfos(values []vocab.ValueObject) []DomainValueObjectInfo {
	out := make([]DomainValueObjectInfo, len(values))
	for i, v := range values {
		out[i] = DomainValueObjectInfo{
			Name:       v.Name,
			Definition: v.Definition,
			Aliases:    v.Aliases,
			Invariants: invariantInfos(v.Invariants),
			Line:       v.Line,
		}
	}
	return out
}

func invariantInfos(invs []vocab.Invariant) []DomainInvariantInfo {
	out := make([]DomainInvariantInfo, len(invs))
	for i, inv := range invs {
		out[i] = DomainInvariantInfo{
			Key:       inv.Key,
			Statement: inv.Statement,
			Line:      inv.Line,
		}
	}
	return out
}

func assertionInfos(assertions []vocab.Assertion) []DomainAssertionInfo {
	out := make([]DomainAssertionInfo, len(assertions))
	for i, a := range assertions {
		out[i] = DomainAssertionInfo{
			Key:       a.Key,
			On:        a.On,
			Statement: a.Statement,
			Line:      a.Line,
		}
	}
	return out
}

func eventInfos(events []vocab.DomainEvent) []DomainEventInfo {
	out := make([]DomainEventInfo, len(events))
	for i, e := range events {
		out[i] = DomainEventInfo{
			Name:       e.Name,
			Definition: e.Definition,
			RaisedBy:   e.RaisedBy,
			Line:       e.Line,
		}
	}
	return out
}

func serviceInfos(services []vocab.DomainService) []DomainTermInfo {
	out := make([]DomainTermInfo, len(services))
	for i, s := range services {
		out[i] = DomainTermInfo{Name: s.Name, Definition: s.Definition, Line: s.Line}
	}
	return out
}

func specificationInfos(specs []vocab.Specification) []DomainTermInfo {
	out := make([]DomainTermInfo, len(specs))
	for i, s := range specs {
		out[i] = DomainTermInfo{Name: s.Name, Definition: s.Definition, Line: s.Line}
	}
	return out
}

func questionInfos(questions []vocab.Question) []DomainQuestionInfo {
	out := make([]DomainQuestionInfo, len(questions))
	for i, q := range questions {
		out[i] = DomainQuestionInfo{Key: q.Key, Text: q.Text, Line: q.Line}
	}
	return out
}

// SDKWriter implements the application's SDKScaffold port: write the
// editor-facing SDK declarations beside the repository's extensions.
type SDKWriter struct {
	root string
}

// NewSDKWriter binds the writer to a repository root.
func NewSDKWriter(root string) (SDKWriter, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return SDKWriter{}, fmt.Errorf("sdk root: %w", err)
	}
	return SDKWriter{root: abs}, nil
}

// Write installs arclint.d.ts and tsconfig.json under
// .arclint/extensions.
func (w SDKWriter) Write() ([]string, error) {
	return SDKInit(w.root)
}
