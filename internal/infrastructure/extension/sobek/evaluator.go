package sobekextension

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// Evaluator implements the domain's ExtensionEvaluator port: discover
// and register the repository's extensions once, validate parameters
// host-side against the extension's published schema, and lend the
// sandboxed Host scoped to exactly the Rule's selected subjects.
// Per-finding severity, contract, and blame from the legacy wire shape
// are ignored: in the target model Severity belongs to the Rule.
type Evaluator struct {
	root string
	opts Options

	once     sync.Once
	registry *Registry
	loadErr  error
}

// NewEvaluator binds the evaluator to a repository root; extensions
// load lazily from <root>/.arclint/extensions on first use.
func NewEvaluator(root string) (*Evaluator, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("extensions root: %w", err)
	}
	return &Evaluator{
		root: abs,
		opts: Options{CacheDir: filepath.Join(abs, ".arclint", "cache")},
	}, nil
}

// Evaluate runs one extension rule over the selected subjects.
func (e *Evaluator) Evaluate(extension string, params map[string]any, subjects []string,
	modules []rule.Module, obs conformance.Observations,
) ([]conformance.ExtensionFinding, error) {
	e.once.Do(func() { e.registry, e.loadErr = LoadDir(e.root, e.opts) })
	if e.loadErr != nil {
		return nil, e.loadErr
	}
	ruleType := e.registry.Get(extension)
	if ruleType == nil {
		return nil, fmt.Errorf("no extension registers rule %q (looked in %s)",
			extension, filepath.Join(e.root, filepath.FromSlash(ExtensionsDir)))
	}
	validated, err := ruleType.ValidateParams(params)
	if err != nil {
		return nil, err
	}
	reported, err := ruleType.Check(e.host(subjects, modules, obs), validated)
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

// host lends the read-only capability surface, scoped to the selected
// subjects: files outside the Rule's Applicability are invisible and
// unreadable, so exclusions hold mechanically.
func (e *Evaluator) host(subjects []string, modules []rule.Module, obs conformance.Observations) Host {
	inScope := make(map[string]bool, len(subjects))
	for _, s := range subjects {
		inScope[s] = true
	}
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
				return "", fmt.Errorf("%s is outside this rule's applicability", path)
			}
			data, err := os.ReadFile(filepath.Join(e.root, filepath.FromSlash(path)))
			if err != nil {
				return "", fmt.Errorf("read %s: %w", path, err)
			}
			return string(data), nil
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
		Modules: func() map[string][]string {
			out := map[string][]string{}
			for _, m := range modules {
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
		ModuleOf: func(path string) []string {
			if !inScope[path] {
				return nil
			}
			var out []string
			for _, m := range modules {
				if m.Contains(path) {
					out = append(out, string(m.Name()))
				}
			}
			sort.Strings(out)
			return out
		},
	}
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
