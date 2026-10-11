package typescript

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// The compiler options that change where a non-relative import points
// are compilerOptions.paths and compilerOptions.baseUrl. Which tsconfig
// governs a file follows the TypeScript language service: the nearest
// tsconfig.json up the tree when it includes the file, else the first
// of its project references that does; a file no configuration
// includes gets no aliases. Sources: TSConfig Reference
// (https://www.typescriptlang.org/tsconfig: extends, files, include,
// exclude, references, baseUrl, paths) and the Modules Reference
// (https://www.typescriptlang.org/docs/handbook/modules/reference.html:
// "paths", "baseUrl").

// configDirTemplate is TypeScript 5.5's placeholder for the directory
// of the configuration being built, so a shared base config can name
// paths of the config that extends it.
const configDirTemplate = "${configDir}"

// pathAlias is one compilerOptions.paths entry: a pattern with at most
// one "*", and its substitutions in the order TypeScript tries them.
type pathAlias struct {
	pattern string
	targets []string // repo-relative, may carry one "*"
}

// project is the effective configuration of one tsconfig file after
// its extends chain, with every path repo-relative.
type project struct {
	file       string
	dir        string
	baseURL    string
	hasBaseURL bool
	paths      []pathAlias
	files      []string
	include    []string
	exclude    []string
	hasFiles   bool
	hasInclude bool
	hasExclude bool
	references []string // repo-relative config files
}

// setting is one option value with the directory of the config that
// declared it, since tsconfig paths resolve from their declaring file.
type setting[T any] struct {
	value T
	dir   string
	set   bool
}

// merged is a configuration's options after extends, before paths are
// resolved against the leaf config's directory.
type merged struct {
	baseURL setting[string]
	paths   setting[map[string][]string]
	files   setting[[]string]
	include setting[[]string]
	exclude setting[[]string]
}

type rawConfig struct {
	Extends         json.RawMessage `json:"extends"`
	CompilerOptions struct {
		BaseURL *string              `json:"baseUrl"`
		Paths   *map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
	Files      *[]string `json:"files"`
	Include    *[]string `json:"include"`
	Exclude    *[]string `json:"exclude"`
	References []struct {
		Path string `json:"path"`
	} `json:"references"`
}

// isConfigName reports the file names TypeScript tooling treats as
// project configurations: tsconfig.json and tsconfig.<name>.json.
func isConfigName(base string) bool {
	return base == "tsconfig.json" || (strings.HasPrefix(base, "tsconfig.") && strings.HasSuffix(base, ".json"))
}

// configLoader reads observed configuration files once and memoizes
// their merged options; an unreadable or invalid config is skipped
// rather than fatal, like an invalid package.json.
type configLoader struct {
	root    string
	files   map[string]bool
	raw     map[string]*rawConfig
	merged  map[string]*merged
	loading map[string]bool
}

func (l *configLoader) read(file string) *rawConfig {
	if c, ok := l.raw[file]; ok {
		return c
	}
	l.raw[file] = nil
	if !l.files[file] {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(l.root, filepath.FromSlash(file)))
	if err != nil {
		return nil
	}
	var c rawConfig
	if err := json.Unmarshal(stripJSONC(data), &c); err != nil {
		return nil
	}
	l.raw[file] = &c
	return &c
}

// extendsTargets resolves the extends field, a string or (TypeScript
// 5.0) an array applied in order. A package specifier names a file
// under node_modules, which the walk never observes, so it is skipped.
func (l *configLoader) extendsTargets(dir string, raw json.RawMessage) []string {
	var specs []string
	if len(raw) == 0 {
		return nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		specs = []string{one}
	} else if err := json.Unmarshal(raw, &specs); err != nil {
		return nil
	}
	var out []string
	for _, spec := range specs {
		if !strings.HasPrefix(spec, "./") && !strings.HasPrefix(spec, "../") {
			continue
		}
		target := path.Join(dir, spec)
		if !l.files[target] && !strings.HasSuffix(target, ".json") && l.files[target+".json"] {
			target += ".json"
		}
		if l.files[target] {
			out = append(out, target)
		}
	}
	return out
}

// merge applies a config over its extends chain: every base in order,
// then the config's own options, each overriding the whole option.
func (l *configLoader) merge(file string) *merged {
	if m, ok := l.merged[file]; ok {
		return m
	}
	if l.loading[file] {
		return &merged{} // an extends cycle contributes nothing further
	}
	c := l.read(file)
	if c == nil {
		l.merged[file] = nil
		return nil
	}
	l.loading[file] = true
	dir := path.Dir(file)
	m := &merged{}
	for _, base := range l.extendsTargets(dir, c.Extends) {
		if b := l.merge(base); b != nil {
			m.overlay(b)
		}
	}
	if c.CompilerOptions.BaseURL != nil {
		m.baseURL = setting[string]{*c.CompilerOptions.BaseURL, dir, true}
	}
	if c.CompilerOptions.Paths != nil {
		m.paths = setting[map[string][]string]{*c.CompilerOptions.Paths, dir, true}
	}
	if c.Files != nil {
		m.files = setting[[]string]{*c.Files, dir, true}
	}
	if c.Include != nil {
		m.include = setting[[]string]{*c.Include, dir, true}
	}
	if c.Exclude != nil {
		m.exclude = setting[[]string]{*c.Exclude, dir, true}
	}
	delete(l.loading, file)
	l.merged[file] = m
	return m
}

func (m *merged) overlay(b *merged) {
	if b.baseURL.set {
		m.baseURL = b.baseURL
	}
	if b.paths.set {
		m.paths = b.paths
	}
	if b.files.set {
		m.files = b.files
	}
	if b.include.set {
		m.include = b.include
	}
	if b.exclude.set {
		m.exclude = b.exclude
	}
}

// resolveFrom turns one config value into a repo-relative path: from
// the leaf config's directory for ${configDir}, else from the
// directory of the config that declared it.
func resolveFrom(value, declaringDir, leafDir string) string {
	if rest, ok := strings.CutPrefix(value, configDirTemplate); ok {
		return path.Join(leafDir, rest)
	}
	return path.Join(declaringDir, value)
}

// project finalizes the effective configuration of one config file.
func (l *configLoader) project(file string) *project {
	m := l.merge(file)
	if m == nil {
		return nil
	}
	c := l.read(file)
	dir := path.Dir(file)
	p := &project{file: file, dir: dir}
	if m.baseURL.set {
		p.baseURL, p.hasBaseURL = resolveFrom(m.baseURL.value, m.baseURL.dir, dir), true
	}
	if m.paths.set {
		// Substitutions resolve from baseUrl when set, else from the
		// config that declared paths (TypeScript 4.1 and later).
		from := m.paths.dir
		if p.hasBaseURL {
			from = p.baseURL
		}
		p.paths = aliasesFrom(m.paths.value, from, dir)
	}
	for _, f := range m.files.value {
		p.files = append(p.files, resolveFrom(f, m.files.dir, dir))
	}
	for _, f := range m.include.value {
		p.include = append(p.include, resolveFrom(f, m.include.dir, dir))
	}
	for _, f := range m.exclude.value {
		p.exclude = append(p.exclude, resolveFrom(f, m.exclude.dir, dir))
	}
	p.hasFiles, p.hasInclude, p.hasExclude = m.files.set, m.include.set, m.exclude.set
	for _, ref := range c.References {
		target := path.Join(dir, ref.Path)
		if !strings.HasSuffix(target, ".json") {
			target = path.Join(target, "tsconfig.json")
		}
		p.references = append(p.references, target)
	}
	return p
}

// aliasesFrom resolves a paths map: each substitution from the given
// directory, or from the leaf config for ${configDir}. TypeScript
// rejects a pattern or substitution with two wildcards, so they are
// dropped.
func aliasesFrom(paths map[string][]string, from, leafDir string) []pathAlias {
	var out []pathAlias
	for pattern, targets := range paths {
		if strings.Count(pattern, "*") > 1 {
			continue
		}
		a := pathAlias{pattern: pattern}
		for _, t := range targets {
			if strings.Count(t, "*") <= 1 {
				a.targets = append(a.targets, resolveFrom(t, from, leafDir))
			}
		}
		out = append(out, a)
	}
	return out
}

// loadProjects builds the effective configuration of every observed
// tsconfig file, keyed by its repo-relative path.
func loadProjects(root string, files map[string]bool) map[string]*project {
	l := &configLoader{root: root, files: files, raw: map[string]*rawConfig{}, merged: map[string]*merged{}, loading: map[string]bool{}}
	out := map[string]*project{}
	for f := range files {
		if isConfigName(path.Base(f)) {
			if p := l.project(f); p != nil {
				out[f] = p
			}
		}
	}
	return out
}

// includes reports whether the configuration's program contains file:
// a listed file, or an include match not removed by exclude. Without
// include, files means only those files and neither means every file
// under the config's directory.
func (p *project) includes(file string) bool {
	for _, f := range p.files {
		if f == file {
			return true
		}
	}
	patterns := p.include
	if !p.hasInclude {
		if p.hasFiles {
			return false
		}
		patterns = []string{path.Join(p.dir, "**/*")}
	}
	matched := false
	for _, pattern := range patterns {
		if includeMatches(pattern, file) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	excludes := p.exclude
	if !p.hasExclude {
		excludes = []string{path.Join(p.dir, "node_modules"), path.Join(p.dir, "bower_components"), path.Join(p.dir, "jspm_packages")}
	}
	for _, pattern := range excludes {
		if excludeMatches(pattern, file) {
			return false
		}
	}
	return true
}

// includeMatches matches an include pattern; a final segment with no
// wildcard and no extension names a directory and includes everything
// under it.
func includeMatches(pattern, file string) bool {
	last := path.Base(pattern)
	if !strings.ContainsAny(last, "*?") && path.Ext(last) == "" {
		pattern = path.Join(pattern, "**/*")
	}
	return globMatch(strings.Split(pattern, "/"), strings.Split(file, "/"))
}

// excludeMatches matches an exclude pattern against the file or any
// directory containing it, since excluding a directory excludes its
// contents.
func excludeMatches(pattern, file string) bool {
	parts := strings.Split(pattern, "/")
	segs := strings.Split(file, "/")
	for n := len(segs); n > 0; n-- {
		if globMatch(parts, segs[:n]) {
			return true
		}
	}
	return false
}

// globMatch matches path segments: "**" spans any number of segments,
// "*" and "?" stay within one segment.
func globMatch(pattern, segs []string) bool {
	if len(pattern) == 0 {
		return len(segs) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if globMatch(pattern[1:], segs[i:]) {
				return true
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	if ok, err := path.Match(pattern[0], segs[0]); err != nil || !ok {
		return false
	}
	return globMatch(pattern[1:], segs[1:])
}

// projectFor returns the configuration that governs file, or nil.
func (r *resolver) projectFor(file string) *project {
	dir := path.Dir(file)
	for {
		cfg := path.Join(dir, "tsconfig.json")
		if p, ok := r.projects[cfg]; ok {
			if p.includes(file) {
				return p
			}
			for _, ref := range p.references {
				if rp, ok := r.projects[ref]; ok && rp.includes(file) {
					return rp
				}
			}
			return nil
		}
		if dir == "." {
			return nil
		}
		dir = path.Dir(dir)
	}
}

// resolveAlias resolves a non-relative specifier through paths, then
// baseUrl, returning the file it reaches and the directory it names
// ("" when none). A pattern that matches but whose substitutions reach
// no file falls through, as in TypeScript.
func (r *resolver) resolveAlias(p *project, spec string) (targetFile, targetDir string) {
	if a, star, ok := bestAlias(p.paths, spec); ok {
		for _, t := range a.targets {
			if f, d := r.resolveFile(strings.Replace(t, "*", star, 1)); f != "" {
				return f, d
			}
		}
	}
	if p.hasBaseURL {
		if f, d := r.resolveFile(path.Join(p.baseURL, spec)); f != "" {
			return f, d
		}
	}
	return "", ""
}

// bestAlias picks the paths entry for spec the way TypeScript does: an
// exact pattern first, else the wildcard pattern with the longest
// prefix. It returns the text the "*" stands for.
func bestAlias(aliases []pathAlias, spec string) (pathAlias, string, bool) {
	var best pathAlias
	bestPrefix := -1
	var bestStar string
	for _, a := range aliases {
		prefix, suffix, wild := strings.Cut(a.pattern, "*")
		if !wild {
			if a.pattern == spec {
				return a, "", true
			}
			continue
		}
		if len(spec) >= len(prefix)+len(suffix) && strings.HasPrefix(spec, prefix) && strings.HasSuffix(spec, suffix) && len(prefix) > bestPrefix {
			best, bestPrefix = a, len(prefix)
			bestStar = spec[len(prefix) : len(spec)-len(suffix)]
		}
	}
	return best, bestStar, bestPrefix >= 0
}

// stripJSONC turns tsconfig's JSON with comments into JSON: it drops a
// leading byte order mark, then line and block comments, then trailing
// commas, leaving string contents untouched.
func stripJSONC(data []byte) []byte {
	data = []byte(strings.TrimPrefix(string(data), "\ufeff"))
	return stripTrailingCommas(stripComments(data))
}

// scanJSON calls emit for every byte outside a string literal and copies
// string literals through unchanged; emit returns how many extra bytes
// it consumed.
func scanJSON(data []byte, emit func(out []byte, i int) ([]byte, int)) []byte {
	out := make([]byte, 0, len(data))
	inString, escaped := false, false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		var skip int
		out, skip = emit(out, i)
		i += skip
	}
	return out
}

func stripComments(data []byte) []byte {
	return scanJSON(data, func(out []byte, i int) ([]byte, int) {
		switch {
		case data[i] == '/' && i+1 < len(data) && data[i+1] == '/':
			j := i
			for j < len(data) && data[j] != '\n' {
				j++
			}
			if j < len(data) {
				out = append(out, '\n')
			}
			return out, j - i
		case data[i] == '/' && i+1 < len(data) && data[i+1] == '*':
			j := i + 2
			for j+1 < len(data) && (data[j] != '*' || data[j+1] != '/') {
				j++
			}
			return append(out, ' '), j + 1 - i
		}
		return append(out, data[i]), 0
	})
}

func stripTrailingCommas(data []byte) []byte {
	return scanJSON(data, func(out []byte, i int) ([]byte, int) {
		if data[i] == ',' {
			j := i + 1
			for j < len(data) && strings.ContainsRune(" \t\r\n", rune(data[j])) {
				j++
			}
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				return out, 0
			}
		}
		return append(out, data[i]), 0
	})
}
