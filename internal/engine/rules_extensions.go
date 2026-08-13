package engine

import (
	"fmt"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/wixregiga/arclint/internal/ext"
	"github.com/wixregiga/arclint/internal/report"
)

// checkExtensions validates every rules.yaml extension instance against
// its provider schema and runs the evaluation phase. Rule instances whose
// type no extension registered, or whose params fail schema validation,
// are configuration errors.
func checkExtensions(ctx *Context, reg *ext.Registry) ([]report.Violation, error) {
	if len(ctx.RS.Rules) == 0 {
		return nil, nil
	}
	host := ctx.extensionHost()
	var vs []report.Violation
	for i, inst := range ctx.RS.Rules {
		rt := reg.Get(inst.Type)
		if rt == nil {
			return nil, fmt.Errorf("rules[%d]: no extension registers rule type %q (looked in %s)",
				i, inst.Type, ext.ExtensionsDir)
		}
		params, err := rt.ValidateParams(inst.Params)
		if err != nil {
			return nil, fmt.Errorf("rules[%d]: %w", i, err)
		}
		id := inst.ID
		if id == "" {
			id = fmt.Sprintf("rules.%s[%d]", inst.Type, i)
		}
		instSev := severityOf(inst.Severity)

		reported, err := rt.Check(host, params)
		if err != nil {
			// A crashing or timed-out rule must fail CI visibly, but it is
			// an execution failure, not silence.
			vs = append(vs, report.Violation{
				RuleID:   id,
				Contract: report.Contract(rt.Contract),
				Blame:    report.Blame(rt.Blame),
				Severity: report.SeverityError,
				Path:     rt.SourcePath,
				Message:  fmt.Sprintf("extension rule failed: %v", err),
				FixHint:  "fix the extension or remove the rule instance",
			})
			continue
		}
		for _, r := range reported {
			sev := r.Severity
			if sev == "" {
				sev = instSev
			}
			contract := r.Contract
			if contract == "" {
				contract = rt.Contract
			}
			blame := r.Blame
			if blame == "" {
				blame = rt.Blame
			}
			v := report.Violation{
				RuleID:     id,
				Contract:   report.Contract(contract),
				Blame:      report.Blame(blame),
				Severity:   report.Severity(sev),
				Capability: rt.Capability,
				Path:       filepath.ToSlash(r.Path),
				Message:    r.Message,
				FixHint:    r.FixHint,
			}
			if r.Line > 0 {
				v.Line = report.IntPtr(r.Line)
			}
			vs = append(vs, v)
		}
	}
	return vs, nil
}

// extensionHost lends the read-only host surface to extension rules: the
// walked tree, cached file contents, the import analysis, and module
// membership. Nothing else is reachable from a rule.
func (c *Context) extensionHost() ext.Host {
	return ext.Host{
		Files: func(glob string) ([]ext.FileInfo, error) {
			if glob != "" && !doublestar.ValidatePattern(glob) {
				return nil, fmt.Errorf("invalid glob %q", glob)
			}
			var out []ext.FileInfo
			for _, f := range c.Tree.Files {
				if glob != "" {
					if ok, _ := doublestar.Match(glob, f.Path); !ok {
						continue
					}
				}
				out = append(out, ext.FileInfo{
					Path: f.Path, Name: f.Name(), Stem: f.Stem(), Ext: f.Ext(),
					Dir: f.Dir(), Size: int(f.Size),
				})
			}
			return out, nil
		},
		Read: func(path string) (string, error) {
			f := c.fileByPath(path)
			if f == nil {
				return "", fmt.Errorf("no such file in the tree: %s", path)
			}
			return string(c.Content(f)), nil
		},
		Imports: func(path string) []ext.ImportInfo {
			imports := c.Imports[path]
			out := make([]ext.ImportInfo, 0, len(imports))
			for _, imp := range imports {
				out = append(out, ext.ImportInfo{
					Path: imp.Path, Line: imp.Line,
					Class: string(imp.Class), TargetDir: imp.TargetDir,
					TargetFile: imp.TargetFile,
				})
			}
			return out
		},
		Modules: func() map[string][]string {
			out := map[string][]string{}
			for name, files := range c.ModuleFiles {
				paths := make([]string, 0, len(files))
				for _, f := range files {
					paths = append(paths, f.Path)
				}
				out[name] = paths
			}
			return out
		},
	}
}
