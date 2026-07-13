package rules

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/walk"
)

// The dependencies evaluator (rules.md §5.3) is language-agnostic by
// design: imports are extracted with per-extension regexes (rules.md §9.1),
// resolved to repo-relative paths where possible, and matched against the
// rule's module globs. Unresolvable imports (third-party packages) simply
// belong to no module and cannot violate a contract.

// importRef is one extracted import statement.
type importRef struct {
	raw  string
	line int
}

type extractFunc func(src string) []importRef

// extractors keys import extraction by file extension.
var extractors = map[string]extractFunc{
	".go":  extractGo,
	".js":  extractJS,
	".jsx": extractJS,
	".mjs": extractJS,
	".cjs": extractJS,
	".ts":  extractJS,
	".tsx": extractJS,
	".py":  extractPy,
}

var (
	goImportSingle  = regexp.MustCompile(`^\s*import\s+(?:[\w.]+\s+)?"([^"]+)"`)
	goImportBlock   = regexp.MustCompile(`^\s*import\s*\(`)
	goImportInBlock = regexp.MustCompile(`^\s*(?:[\w.]+\s+)?"([^"]+)"`)

	jsFrom    = regexp.MustCompile(`\bfrom\s+['"]([^'"]+)['"]`)
	jsBare    = regexp.MustCompile(`^\s*import\s+['"]([^'"]+)['"]`)
	jsRequire = regexp.MustCompile(`\brequire\(\s*['"]([^'"]+)['"]\s*\)`)

	pyFrom = regexp.MustCompile(`^\s*from\s+([.\w]+)\s+import\b`)
	// pyImport captures everything after "import" up to a statement
	// separator (";"), a comment ("#"), or end of line, so lines like
	// "import os; import sys" and "import os  # noqa" are tolerated. Each
	// captured segment is then split on "," and "as" downstream.
	pyImport = regexp.MustCompile(`^\s*import\s+([^;#]+)`)
)

func extractGo(src string) []importRef {
	var out []importRef
	inBlock := false
	for i, line := range splitLines(src) {
		switch {
		case inBlock:
			if m := goImportInBlock.FindStringSubmatch(line); m != nil {
				out = append(out, importRef{raw: m[1], line: i + 1})
			} else if strings.TrimSpace(line) == ")" {
				inBlock = false
			}
		case goImportBlock.MatchString(line):
			inBlock = true
		default:
			if m := goImportSingle.FindStringSubmatch(line); m != nil {
				out = append(out, importRef{raw: m[1], line: i + 1})
			}
		}
	}
	return out
}

func extractJS(src string) []importRef {
	var out []importRef
	for i, line := range splitLines(src) {
		for _, m := range jsFrom.FindAllStringSubmatch(line, -1) {
			out = append(out, importRef{raw: m[1], line: i + 1})
		}
		if m := jsBare.FindStringSubmatch(line); m != nil {
			out = append(out, importRef{raw: m[1], line: i + 1})
		}
		for _, m := range jsRequire.FindAllStringSubmatch(line, -1) {
			out = append(out, importRef{raw: m[1], line: i + 1})
		}
	}
	return out
}

func extractPy(src string) []importRef {
	var out []importRef
	for i, line := range splitLines(src) {
		// Python allows multiple ";"-separated statements per line (e.g.
		// "import os; import sys"); a trailing "#" starts a comment.
		// Strip the comment, then walk each statement independently so
		// pyFrom/pyImport (both anchored to the start of a statement)
		// still match.
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		for _, stmt := range strings.Split(line, ";") {
			if m := pyFrom.FindStringSubmatch(stmt); m != nil {
				out = append(out, importRef{raw: m[1], line: i + 1})
				continue
			}
			if m := pyImport.FindStringSubmatch(stmt); m != nil {
				for _, part := range strings.Split(m[1], ",") {
					name := strings.TrimSpace(part)
					// Drop a trailing "as alias": the module identity for
					// resolution purposes is the name before "as".
					if before, _, ok := strings.Cut(name, " as "); ok {
						name = strings.TrimSpace(before)
					}
					fields := strings.Fields(name)
					if len(fields) > 0 {
						out = append(out, importRef{raw: fields[0], line: i + 1})
					}
				}
			}
		}
	}
	return out
}

// goModule lazily reads the module path from go.mod at the repo root, so
// Go import paths can be mapped back to repo-relative paths.
func (c *evalCtx) goModule() string {
	c.goModOnce.Do(func() {
		data, err := os.ReadFile(filepath.Join(c.root, "go.mod"))
		if err != nil {
			return
		}
		for _, line := range splitLines(string(data)) {
			if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
				c.goModPath = strings.TrimSpace(rest)
				return
			}
		}
	})
	return c.goModPath
}

// resolveImport maps a raw import string to a repo-relative slash path, or
// "" when it cannot be resolved (third-party, unknown scheme).
func resolveImport(raw, fileRel, ext string, c *evalCtx) string {
	switch ext {
	case ".go":
		mod := c.goModule()
		if mod == "" {
			return ""
		}
		if raw == mod {
			return "."
		}
		if rest, ok := strings.CutPrefix(raw, mod+"/"); ok {
			return rest
		}
		return ""
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx":
		if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
			return path.Clean(path.Join(path.Dir(fileRel), raw))
		}
		return ""
	case ".py":
		if strings.HasPrefix(raw, ".") {
			dots := len(raw) - len(strings.TrimLeft(raw, "."))
			rest := strings.TrimLeft(raw, ".")
			dir := path.Dir(fileRel)
			for i := 1; i < dots; i++ {
				dir = path.Dir(dir)
			}
			if rest == "" {
				return dir
			}
			return path.Join(dir, strings.ReplaceAll(rest, ".", "/"))
		}
		return strings.ReplaceAll(raw, ".", "/")
	}
	return ""
}

// matchModulePath reports whether a module glob covers a repo-relative
// path. Beyond plain glob matching it treats "prefix/**" as covering the
// prefix directory itself, so an extension-less import target like
// "internal/app" lands in module glob "internal/app/**".
func matchModulePath(glob, rel string) bool {
	if walk.Match(glob, rel) {
		return true
	}
	if prefix, ok := strings.CutSuffix(glob, "/**"); ok {
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
			return true
		}
	}
	return false
}

// modulesOf returns every module (sorted) whose globs cover rel.
func modulesOf(modules map[string][]string, names []string, rel string) []string {
	var out []string
	for _, name := range names {
		for _, g := range modules[name] {
			if matchModulePath(g, rel) {
				out = append(out, name)
				break
			}
		}
	}
	return out
}

// contractViolation returns a non-empty reason when the from→to import
// edge violates the rule's contract.
func contractViolation(p *config.DependenciesParams, from, to string) string {
	switch p.Contract {
	case "layers":
		fi := slices.Index(p.Layers, from)
		ti := slices.Index(p.Layers, to)
		if fi >= 0 && ti >= 0 && ti < fi {
			return fmt.Sprintf("layers contract %v forbids importing the higher layer %q", p.Layers, to)
		}
	case "forbidden":
		for _, e := range p.Forbidden {
			if slices.Contains(e.From, from) && slices.Contains(e.To, to) {
				return fmt.Sprintf("forbidden edge: %q may not import %q", from, to)
			}
		}
	case "independence":
		if slices.Contains(p.Among, from) && slices.Contains(p.Among, to) {
			return fmt.Sprintf("independence contract: %q and %q may not import each other", from, to)
		}
	case "mayDependOn":
		if allowed, ok := p.MayDependOn[from]; ok && !slices.Contains(allowed, to) {
			return fmt.Sprintf("module %q may only depend on %v", from, allowed)
		}
	}
	return ""
}

// compileDependencies builds the dependencies evaluator (rules.md §5.3):
// extract imports from every targeted file that belongs to a module,
// resolve them, and check every from→to module edge against the contract.
// Violations point at the importing file and the import statement's line.
func compileDependencies(id string, r config.Rule) ruleFunc {
	p := r.Dependencies
	names := make([]string, 0, len(p.Modules))
	for name := range p.Modules {
		names = append(names, name)
	}
	sort.Strings(names)

	return func(c *evalCtx) ([]Violation, error) {
		scope := targeted(c.paths, r.Files)
		return forFiles(scope, func(rel string) ([]Violation, error) {
			ext := strings.ToLower(path.Ext(rel))
			extract, ok := extractors[ext]
			if !ok {
				return nil, nil
			}
			fromMods := modulesOf(p.Modules, names, rel)
			if len(fromMods) == 0 {
				return nil, nil
			}
			data, err := c.read(rel)
			if err != nil {
				return nil, fmt.Errorf("rule %q: cannot read %s — %v", id, rel, err)
			}
			var vs []Violation
			for _, imp := range extract(string(data)) {
				target := resolveImport(imp.raw, rel, ext, c)
				if target == "" {
					continue
				}
				toMods := modulesOf(p.Modules, names, target)
				for _, from := range fromMods {
					for _, to := range toMods {
						if from == to {
							continue
						}
						reason := contractViolation(p, from, to)
						if reason == "" {
							continue
						}
						line := imp.line
						vs = append(vs, Violation{
							RuleID:   id,
							Category: r.Type,
							Severity: r.Severity,
							Path:     rel,
							Line:     &line,
							Message:  fmt.Sprintf("imports %s (module %q) — %s", imp.raw, to, reason),
							FixHint:  r.FixHint,
						})
					}
				}
			}
			return vs, nil
		})
	}
}
