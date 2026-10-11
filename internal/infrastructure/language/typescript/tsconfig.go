package typescript

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// The compiler options that change where a non-relative import points
// are compilerOptions.paths and compilerOptions.baseUrl; which
// configuration owns a file is decided in ownership.go. Sources: TSConfig
// Reference (https://www.typescriptlang.org/tsconfig: extends, files,
// include, exclude, references, baseUrl, paths,
// disableSolutionSearching) and the Modules Reference
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

// project is the effective configuration of one tsconfig or jsconfig
// file after its extends chain, with every path repo-relative.
type project struct {
	file       string
	dir        string
	baseURL    string
	hasBaseURL bool
	paths      []pathAlias
	// listed, include and exclude select the files the program starts
	// from: the files option, then include patterns split into
	// segments, minus exclude patterns split the same way.
	listed     map[string]bool
	include    []globPattern
	exclude    []globPattern
	references []string // repo-relative config files
	// disableSolutionSearching stops the ownership search at this
	// configuration instead of climbing to the ones above it.
	disableSolutionSearching bool
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
	baseURL                  setting[string]
	paths                    setting[map[string][]string]
	files                    setting[[]string]
	include                  setting[[]string]
	exclude                  setting[[]string]
	disableSolutionSearching setting[bool]
}

type rawConfig struct {
	Extends         json.RawMessage `json:"extends"`
	CompilerOptions struct {
		BaseURL                  *string              `json:"baseUrl"`
		Paths                    *map[string][]string `json:"paths"`
		DisableSolutionSearching *bool                `json:"disableSolutionSearching"`
	} `json:"compilerOptions"`
	Files      *[]string `json:"files"`
	Include    *[]string `json:"include"`
	Exclude    *[]string `json:"exclude"`
	References []struct {
		Path string `json:"path"`
	} `json:"references"`
}

// configLoader reads observed configuration files once, whatever their
// name, and memoizes their merged options and effective configuration;
// an unreadable or invalid config is skipped rather than fatal, like an
// invalid package.json.
type configLoader struct {
	root     string
	files    map[string]bool
	raw      map[string]*rawConfig
	merged   map[string]*merged
	projects map[string]*project
	loading  map[string]bool
}

func newConfigLoader(root string, files map[string]bool) *configLoader {
	return &configLoader{root: root, files: files, raw: map[string]*rawConfig{}, merged: map[string]*merged{}, projects: map[string]*project{}, loading: map[string]bool{}}
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
	if c.CompilerOptions.DisableSolutionSearching != nil {
		m.disableSolutionSearching = setting[bool]{*c.CompilerOptions.DisableSolutionSearching, dir, true}
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
	if b.disableSolutionSearching.set {
		m.disableSolutionSearching = b.disableSolutionSearching
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

// project returns the effective configuration of one config file, nil
// when it is not observed or does not parse.
func (l *configLoader) project(file string) *project {
	if p, ok := l.projects[file]; ok {
		return p
	}
	p := l.build(file)
	l.projects[file] = p
	return p
}

func (l *configLoader) build(file string) *project {
	m := l.merge(file)
	if m == nil {
		return nil
	}
	c := l.read(file)
	dir := path.Dir(file)
	p := &project{file: file, dir: dir, disableSolutionSearching: m.disableSolutionSearching.value}
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
	p.listed = map[string]bool{}
	for _, f := range m.files.value {
		p.listed[resolveFrom(f, m.files.dir, dir)] = true
	}
	// Without include, files means only those files and neither means
	// every file under the config's directory.
	switch {
	case m.include.set:
		for _, f := range m.include.value {
			if g, ok := includePattern(resolveFrom(f, m.include.dir, dir)); ok {
				p.include = append(p.include, g)
			}
		}
	case !m.files.set:
		g, _ := includePattern(path.Join(dir, "**/*"))
		p.include = []globPattern{g}
	}
	excludes := []string{path.Join(dir, "node_modules"), path.Join(dir, "bower_components"), path.Join(dir, "jspm_packages")}
	if m.exclude.set {
		excludes = nil
		for _, f := range m.exclude.value {
			excludes = append(excludes, resolveFrom(f, m.exclude.dir, dir))
		}
	}
	for _, e := range excludes {
		p.exclude = append(p.exclude, newGlobPattern(e))
	}
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

// includes reports whether the configuration selects file: a listed
// file, or an include match not removed by exclude.
func (p *project) includes(file string) bool {
	if p.listed[file] {
		return true
	}
	var segs []string
	for _, g := range p.include {
		if !strings.HasPrefix(file, g.prefix) {
			continue
		}
		if segs == nil {
			segs = strings.Split(file, "/")
		}
		if globMatch(g.segs, segs, true) {
			return !p.excludes(file, segs)
		}
	}
	return false
}

// excludes matches every exclude pattern against the file or any
// directory containing it, since excluding a directory excludes its
// contents.
func (p *project) excludes(file string, segs []string) bool {
	for _, g := range p.exclude {
		if !strings.HasPrefix(file, g.prefix) {
			continue
		}
		for n := len(segs); n > 0; n-- {
			if globMatch(g.segs, segs[:n], false) {
				return true
			}
		}
	}
	return false
}

// globPattern is one include or exclude pattern split into segments,
// with the literal text every path it matches starts with.
type globPattern struct {
	prefix string
	segs   []string
}

func newGlobPattern(pattern string) globPattern {
	g := globPattern{segs: strings.Split(pattern, "/"), prefix: pattern}
	for i, seg := range g.segs {
		if strings.ContainsAny(seg, "*?") {
			g.prefix = strings.Join(g.segs[:i], "/")
			if i > 0 {
				g.prefix += "/"
			}
			break
		}
	}
	return g
}

// includePattern reads an include pattern the way TypeScript does
// (getSubPatternFromSpec with the files matcher): a final segment with
// no wildcard and no extension names a directory and includes
// everything under it, and a pattern ending in "**" includes nothing.
func includePattern(pattern string) (globPattern, bool) {
	last := path.Base(pattern)
	if last == "**" {
		return globPattern{}, false
	}
	if !strings.ContainsAny(last, "*?") && path.Ext(last) == "" {
		pattern = path.Join(pattern, "**/*")
	}
	return newGlobPattern(pattern), true
}

// packageFolders are the folders an include wildcard never enters.
var packageFolders = map[string]bool{"node_modules": true, "bower_components": true, "jspm_packages": true}

// globMatch matches path segments: "**" spans any number of segments,
// "*" and "?" stay within one segment. In an include pattern a wildcard
// never matches a package folder, "**" never enters a directory whose
// name starts with a dot, and a segment pattern that starts with a
// wildcard never matches a name that does.
func globMatch(pattern, segs []string, include bool) bool {
	if len(pattern) == 0 {
		return len(segs) == 0
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(segs); i++ {
			if globMatch(pattern[1:], segs[i:], include) {
				return true
			}
			if i < len(segs) && include && (strings.HasPrefix(segs[i], ".") || packageFolders[segs[i]]) {
				return false
			}
		}
		return false
	}
	if len(segs) == 0 {
		return false
	}
	if include && strings.ContainsAny(pattern[0], "*?") {
		if packageFolders[segs[0]] || (strings.ContainsAny(pattern[0][:1], "*?") && strings.HasPrefix(segs[0], ".")) {
			return false
		}
	}
	if !segmentMatch(pattern[0], segs[0]) {
		return false
	}
	return globMatch(pattern[1:], segs[1:], include)
}

// segmentMatch matches one path segment against a pattern in which only
// "*" (any run of characters) and "?" (one character) are wildcards;
// every other character, "[" included, stands for itself, as in
// TypeScript's include and exclude patterns.
func segmentMatch(pattern, name string) bool {
	pat, str := []rune(pattern), []rune(name)
	p, n, star, mark := 0, 0, -1, 0
	for n < len(str) {
		switch {
		case p < len(pat) && pat[p] == '*':
			star, mark = p, n
			p++
		case p < len(pat) && (pat[p] == '?' || pat[p] == str[n]):
			p++
			n++
		case star >= 0:
			p = star + 1
			mark++
			n = mark
		default:
			return false
		}
	}
	for p < len(pat) && pat[p] == '*' {
		p++
	}
	return p == len(pat)
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
