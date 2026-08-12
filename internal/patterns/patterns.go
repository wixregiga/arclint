// Package patterns ships architectural pattern bundles: a complete,
// working rules.yaml plus optional TypeScript extensions. Built-ins are
// embedded in the binary; a repository adds its own under
// .arclint/patterns/<name>/ (nested names like fsd/go are legal). A
// pattern directory contains:
//
//	pattern.yaml       description + compatible runtimes
//	rules.yaml         the template (valid and loadable as-is)
//	extensions/*.ts    optional rule extensions, installed by init
package patterns

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed builtin
var builtinFS embed.FS

// LocalDir is the repository-local pattern root, relative to the repo root.
const LocalDir = ".arclint/patterns"

// Pattern is one architectural pattern bundle.
type Pattern struct {
	// Name is the registry key: the builtin directory name, or the
	// pattern's path relative to .arclint/patterns for local patterns.
	Name        string
	Description string
	// Runtimes lists the language targets the pattern supports.
	Runtimes []string
	// Source is "builtin" or the local pattern directory.
	Source string
	// RulesYAML is the complete rules.yaml template.
	RulesYAML []byte
	// Extensions maps extension file basenames to contents.
	Extensions map[string][]byte
}

type meta struct {
	Description string   `yaml:"description"`
	Runtimes    []string `yaml:"runtimes"`
}

// load reads one pattern rooted at dir inside fsys.
func load(fsys fs.FS, dir, name, source string) (*Pattern, error) {
	metaRaw, err := fs.ReadFile(fsys, path.Join(dir, "pattern.yaml"))
	if err != nil {
		return nil, fmt.Errorf("pattern %s: %w", name, err)
	}
	var m meta
	if err := yaml.Unmarshal(metaRaw, &m); err != nil {
		return nil, fmt.Errorf("pattern %s: pattern.yaml: %w", name, err)
	}
	if m.Description == "" || len(m.Runtimes) == 0 {
		return nil, fmt.Errorf("pattern %s: pattern.yaml needs description and runtimes", name)
	}
	for _, r := range m.Runtimes {
		if r != "go" && r != "ts" && r != "py" {
			return nil, fmt.Errorf("pattern %s: unknown runtime %q", name, r)
		}
	}
	rules, err := fs.ReadFile(fsys, path.Join(dir, "rules.yaml"))
	if err != nil {
		return nil, fmt.Errorf("pattern %s: %w", name, err)
	}
	p := &Pattern{
		Name: name, Description: m.Description, Runtimes: m.Runtimes,
		Source: source, RulesYAML: rules, Extensions: map[string][]byte{},
	}
	entries, err := fs.ReadDir(fsys, path.Join(dir, "extensions"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("pattern %s: %w", name, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".ts") {
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(dir, "extensions", e.Name()))
		if err != nil {
			return nil, fmt.Errorf("pattern %s: %w", name, err)
		}
		p.Extensions[e.Name()] = data
	}
	return p, nil
}

// Builtins returns the embedded patterns, sorted by name.
func Builtins() ([]*Pattern, error) {
	entries, err := fs.ReadDir(builtinFS, "builtin")
	if err != nil {
		return nil, err
	}
	var out []*Pattern
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := load(builtinFS, path.Join("builtin", e.Name()), e.Name(), "builtin")
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Local discovers repository patterns: every directory under
// .arclint/patterns containing a pattern.yaml, named by its relative
// path. A missing directory yields nil.
func Local(root string) ([]*Pattern, error) {
	base := filepath.Join(root, filepath.FromSlash(LocalDir))
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, nil
	}
	fsys := os.DirFS(base)
	var out []*Pattern
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() || p == "." {
			return nil
		}
		if _, err := fs.Stat(fsys, path.Join(p, "pattern.yaml")); err != nil {
			return nil
		}
		pat, err := load(fsys, p, p, filepath.ToSlash(filepath.Join(LocalDir, p)))
		if err != nil {
			return err
		}
		out = append(out, pat)
		return fs.SkipDir // a pattern does not nest inside another
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// All returns builtins plus local patterns for a repo root ("" for
// builtins only). A local pattern with a builtin's name shadows it.
func All(root string) ([]*Pattern, error) {
	builtins, err := Builtins()
	if err != nil {
		return nil, err
	}
	if root == "" {
		return builtins, nil
	}
	local, err := Local(root)
	if err != nil {
		return nil, err
	}
	byName := map[string]*Pattern{}
	for _, p := range builtins {
		byName[p.Name] = p
	}
	for _, p := range local {
		byName[p.Name] = p
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]*Pattern, 0, len(names))
	for _, n := range names {
		out = append(out, byName[n])
	}
	return out, nil
}

// Find resolves a pattern by name over All(root).
func Find(root, name string) (*Pattern, error) {
	all, err := All(root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, p := range all {
		if p.Name == name {
			return p, nil
		}
		names = append(names, p.Name)
	}
	return nil, fmt.Errorf("unknown pattern %q (available: %s)", name, strings.Join(names, ", "))
}

// Supports reports whether the pattern covers every requested runtime.
func (p *Pattern) Supports(runtimes []string) bool {
	for _, r := range runtimes {
		if !slices.Contains(p.Runtimes, r) {
			return false
		}
	}
	return len(runtimes) > 0
}

var runtimeLine = regexp.MustCompile(`(?m)^runtime:.*$`)

// RenderRules returns the template with its runtime line set to the
// requested targets. Every template carries exactly one runtime line.
func (p *Pattern) RenderRules(runtimes []string) ([]byte, error) {
	if !p.Supports(runtimes) {
		return nil, fmt.Errorf("pattern %q supports runtimes %v, not %v", p.Name, p.Runtimes, runtimes)
	}
	locs := runtimeLine.FindAllIndex(p.RulesYAML, -1)
	if len(locs) != 1 {
		return nil, fmt.Errorf("pattern %q: template must contain exactly one runtime line, found %d", p.Name, len(locs))
	}
	line := "runtime: [" + strings.Join(runtimes, ", ") + "]"
	return runtimeLine.ReplaceAll(p.RulesYAML, []byte(line)), nil
}

// Materialize writes the pattern into a repository root: rules.yaml with
// the chosen runtimes, and every extension under .arclint/extensions/.
// Existing files are refused unless force is set. It returns the written
// paths, repo-relative.
func (p *Pattern) Materialize(root string, runtimes []string, force bool) ([]string, error) {
	rules, err := p.RenderRules(runtimes)
	if err != nil {
		return nil, err
	}
	type out struct {
		rel  string
		data []byte
	}
	outs := []out{{"rules.yaml", rules}}
	names := make([]string, 0, len(p.Extensions))
	for name := range p.Extensions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		outs = append(outs, out{path.Join(".arclint", "extensions", name), p.Extensions[name]})
	}
	if !force {
		for _, o := range outs {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(o.rel))); err == nil {
				return nil, fmt.Errorf("%s already exists (use --force to overwrite)", o.rel)
			}
		}
	}
	var written []string
	for _, o := range outs {
		abs := filepath.Join(root, filepath.FromSlash(o.rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(abs, o.data, 0o644); err != nil {
			return nil, err
		}
		written = append(written, o.rel)
	}
	return written, nil
}
