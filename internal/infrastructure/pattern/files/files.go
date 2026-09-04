// Package patternfiles is the file form of a distributed Pattern shared
// by every PatternSource: which files in a Pattern directory ship,
// how a validated Pattern is loaded from them, and the JSON documents
// a Manifest and a Registry index travel as.
package patternfiles

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

// Collect gathers the shipped files of one Pattern directory: the
// pattern.yaml at its root and every regular file below extensions/,
// in path order. Hidden entries and the Manifest itself are not
// Pattern files. A directory without pattern.yaml is not a Pattern.
func Collect(fsys fs.FS, dir string) ([]distribution.PatternFile, error) {
	dir = path.Clean(dir)
	read := func(rel string) (distribution.PatternFile, error) {
		full := rel
		if dir != "." {
			full = dir + "/" + rel
		}
		data, err := fs.ReadFile(fsys, full)
		if err != nil {
			return distribution.PatternFile{}, fmt.Errorf("read %s: %w", full, err)
		}
		return distribution.NewPatternFile(rel, data)
	}
	doc, err := read(distribution.PatternFileName)
	if err != nil {
		return nil, err
	}
	files := []distribution.PatternFile{doc}
	extDir := distribution.ExtensionsDir
	if dir != "." {
		extDir = dir + "/" + extDir
	}
	err = fs.WalkDir(fsys, extDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == extDir {
				return fs.SkipDir
			}
			return err
		}
		if strings.HasPrefix(d.Name(), ".") && p != extDir {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel := p
		if dir != "." {
			rel = strings.TrimPrefix(p, dir+"/")
		}
		f, err := read(rel)
		if err != nil {
			return err
		}
		files = append(files, f)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("extensions of %s: %w", dir, err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path() < files[j].Path() })
	return files, nil
}

// Load validates the Pattern the files describe. Source names where
// the files came from, for messages; the Extensions are the
// installable files directly under extensions/.
func Load(files []distribution.PatternFile, source string) (rule.Pattern, error) {
	var doc []byte
	found := false
	var exts []rule.PatternExtension
	for _, f := range files {
		switch {
		case f.Path() == distribution.PatternFileName:
			doc = f.Data()
			found = true
		case path.Dir(f.Path()) == distribution.ExtensionsDir && rule.InstallableExtensionFileName(path.Base(f.Path())):
			ext, err := rule.NewPatternExtension(path.Base(f.Path()), string(f.Data()))
			if err != nil {
				return rule.Pattern{}, fmt.Errorf("%s: %v", source, err)
			}
			exts = append(exts, ext)
		}
	}
	if !found {
		return rule.Pattern{}, fmt.Errorf("%s: missing %s", source, distribution.PatternFileName)
	}
	p, err := yamlrule.LoadPattern(doc, source+"/"+distribution.PatternFileName, exts)
	if err != nil {
		return rule.Pattern{}, fmt.Errorf("load pattern: %w", err)
	}
	return p, nil
}

// manifestDoc is the JSON form of a Manifest.
type manifestDoc struct {
	Pattern string            `json:"pattern"`
	Digest  string            `json:"digest"`
	Files   []manifestFileDoc `json:"files"`
}

type manifestFileDoc struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// EncodeManifest renders the Manifest as indented JSON with a trailing
// newline, entries in path order.
func EncodeManifest(m distribution.Manifest) ([]byte, error) {
	if m.IsZero() {
		return nil, fmt.Errorf("encode manifest: unconstructed manifest")
	}
	doc := manifestDoc{Pattern: m.Reference().String(), Digest: m.Digest().String()}
	for _, e := range m.Entries() {
		doc.Files = append(doc.Files, manifestFileDoc{Path: e.Path(), Digest: e.Digest().String()})
	}
	return marshal(doc)
}

// DecodeManifest parses and validates a Manifest document; source
// names it for messages.
func DecodeManifest(data []byte, source string) (distribution.Manifest, error) {
	var doc manifestDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return distribution.Manifest{}, fmt.Errorf("%s: %v", source, err)
	}
	ref, err := rule.ParsePatternReference(doc.Pattern)
	if err != nil {
		return distribution.Manifest{}, fmt.Errorf("%s: pattern: %v", source, err)
	}
	digest, err := distribution.ParseDigest(doc.Digest)
	if err != nil {
		return distribution.Manifest{}, fmt.Errorf("%s: digest: %v", source, err)
	}
	entries := make([]distribution.ManifestEntry, 0, len(doc.Files))
	for _, f := range doc.Files {
		d, err := distribution.ParseDigest(f.Digest)
		if err != nil {
			return distribution.Manifest{}, fmt.Errorf("%s: file %q: digest: %v", source, f.Path, err)
		}
		e, err := distribution.NewManifestEntry(f.Path, d)
		if err != nil {
			return distribution.Manifest{}, fmt.Errorf("%s: %v", source, err)
		}
		entries = append(entries, e)
	}
	m, err := distribution.NewManifest(ref, digest, entries)
	if err != nil {
		return distribution.Manifest{}, fmt.Errorf("%s: %v", source, err)
	}
	return m, nil
}

// indexDoc is the JSON form of a Registry index.
type indexDoc struct {
	Patterns []indexEntryDoc `json:"patterns"`
}

type indexEntryDoc struct {
	Pattern       string   `json:"pattern"`
	Digest        string   `json:"digest"`
	Documentation string   `json:"documentation,omitempty"`
	Coverage      []string `json:"coverage"`
	Rules         int      `json:"rules"`
	Extensions    int      `json:"extensions"`
}

// EncodeIndex renders the index as indented JSON with a trailing
// newline.
func EncodeIndex(x distribution.Index) ([]byte, error) {
	doc := indexDoc{Patterns: []indexEntryDoc{}}
	for _, e := range x.Entries() {
		coverage := make([]string, 0, len(e.Coverage()))
		for _, l := range e.Coverage() {
			coverage = append(coverage, l.RuntimeTarget())
		}
		doc.Patterns = append(doc.Patterns, indexEntryDoc{
			Pattern:       e.Reference().String(),
			Digest:        e.Digest().String(),
			Documentation: e.Documentation(),
			Coverage:      coverage,
			Rules:         e.Rules(),
			Extensions:    e.Extensions(),
		})
	}
	return marshal(doc)
}

// DecodeIndex parses and validates an index document; source names it
// for messages.
func DecodeIndex(data []byte, source string) (distribution.Index, error) {
	var doc indexDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return distribution.Index{}, fmt.Errorf("%s: %v", source, err)
	}
	entries := make([]distribution.IndexEntry, 0, len(doc.Patterns))
	for _, d := range doc.Patterns {
		ref, err := rule.ParsePatternReference(d.Pattern)
		if err != nil {
			return distribution.Index{}, fmt.Errorf("%s: pattern: %v", source, err)
		}
		digest, err := distribution.ParseDigest(d.Digest)
		if err != nil {
			return distribution.Index{}, fmt.Errorf("%s: %s: digest: %v", source, ref, err)
		}
		coverage := make([]rule.Language, 0, len(d.Coverage))
		for _, target := range d.Coverage {
			l, ok := languageOfTarget(target)
			if !ok {
				return distribution.Index{}, fmt.Errorf("%s: %s: coverage %q is not one of go, ts, py", source, ref, target)
			}
			coverage = append(coverage, l)
		}
		e, err := distribution.NewIndexEntry(ref, digest, d.Documentation, coverage, d.Rules, d.Extensions)
		if err != nil {
			return distribution.Index{}, fmt.Errorf("%s: %v", source, err)
		}
		entries = append(entries, e)
	}
	x, err := distribution.NewIndex(entries)
	if err != nil {
		return distribution.Index{}, fmt.Errorf("%s: %v", source, err)
	}
	return x, nil
}

func languageOfTarget(target string) (rule.Language, bool) {
	for _, l := range rule.Languages() {
		if l.RuntimeTarget() == target {
			return l, true
		}
	}
	return "", false
}

func marshal(doc any) ([]byte, error) {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode %T: %w", doc, err)
	}
	return append(data, '\n'), nil
}
