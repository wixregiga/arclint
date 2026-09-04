// Package filesystemobservation produces normalized Observations from
// the repository filesystem: a deterministic file walk plus the
// Language Facts of every configured language, supplied by fact
// producers selected at composition time.
package filesystemobservation

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// FactProducer supplies the Language Facts of one Programming
// Language for the files it owns.
type FactProducer interface {
	Language() rule.Language
	// Facts produces the requested fact classes for the files the
	// language owns; a producer supplies only what it can honestly
	// support.
	Facts(root string, files []conformance.ObservedFile, requested []rule.Fact) (map[string]conformance.LanguageFacts, error)
}

// Source implements the application's ObservationSource port over one
// repository root.
type Source struct {
	root      string
	producers []FactProducer
}

// NewSource requires an existing repository root; producers may cover
// any subset of the supported languages.
func NewSource(root string, producers ...FactProducer) (Source, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Source{}, fmt.Errorf("observation root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Source{}, fmt.Errorf("observation root: %w", err)
	}
	if !info.IsDir() {
		return Source{}, fmt.Errorf("observation root %s is not a directory", abs)
	}
	seen := map[rule.Language]bool{}
	for _, p := range producers {
		if seen[p.Language()] {
			return Source{}, fmt.Errorf("duplicate fact producer for language %q", p.Language())
		}
		seen[p.Language()] = true
	}
	return Source{root: abs, producers: producers}, nil
}

// Observe walks the repository under the scan policy and produces the
// requested fact classes for the requested languages. facts is the
// union of what the configured Rules' Enforcement declares, the seam
// Extension enforcement will discriminate through; the builtin fact
// classes (file_tree, imports) are gathered whenever their language is
// requested.
func (s Source) Observe(languages []rule.Language, scan rule.Scan, facts []rule.Fact) (conformance.Observations, error) {
	files, err := walk(s.root, scan)
	if err != nil {
		return conformance.Observations{}, err
	}
	produced := map[string]conformance.LanguageFacts{}
	for _, p := range s.producers {
		if !languageRequested(languages, p.Language()) {
			continue
		}
		supplied, err := p.Facts(s.root, files, facts)
		if err != nil {
			return conformance.Observations{}, fmt.Errorf("%s facts: %w", p.Language(), err)
		}
		for path, f := range supplied {
			if _, dup := produced[path]; dup {
				return conformance.Observations{}, fmt.Errorf("two fact producers claim %s", path)
			}
			produced[path] = f
		}
	}
	obs, err := conformance.NewObservations(files, produced)
	if err != nil {
		return conformance.Observations{}, fmt.Errorf("assemble observations: %w", err)
	}
	// Lazy repository reads for Extension ctx.read: bytes are not
	// eagerly copied into Observations.
	return obs.WithContent(rootContent{root: s.root}), nil
}

// rootContent reads one repo-relative path from the observation root
// on demand.
type rootContent struct{ root string }

func (r rootContent) Read(path string) (string, error) {
	data, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(path)))
	if err != nil {
		return "", fmt.Errorf("read repository content %q: %w", path, err)
	}
	return string(data), nil
}

func languageRequested(languages []rule.Language, l rule.Language) bool {
	for _, known := range languages {
		if known == l {
			return true
		}
	}
	return false
}

// alwaysSkippedDirs are directory base names never walked: VCS
// metadata, ArcLint's own state, vendored third-party sources, and
// package-manager trees.
var alwaysSkippedDirs = map[string]bool{
	".git":         true,
	".hg":          true,
	".svn":         true,
	".arclint":     true,
	"vendor":       true,
	"node_modules": true,
}

// walk lists every regular file under root deterministically. Symbolic
// links are never followed; an unreadable directory is a hard error so
// silence cannot read as conformance.
func walk(root string, scan rule.Scan) ([]conformance.ObservedFile, error) {
	var out []conformance.ObservedFile
	var visit func(abs, rel string) error
	visit = func(abs, rel string) error {
		entries, err := os.ReadDir(abs)
		if err != nil {
			return fmt.Errorf("observation: %v", err)
		}
		for _, e := range entries {
			name := e.Name()
			childAbs := filepath.Join(abs, name)
			childRel := name
			if rel != "" {
				childRel = rel + "/" + name
			}
			switch {
			case e.IsDir():
				if alwaysSkippedDirs[name] {
					continue
				}
				if name == "testdata" && !scan.IncludeTestdata {
					continue
				}
				if excluded(scan.Exclude, childRel) {
					continue
				}
				if err := visit(childAbs, childRel); err != nil {
					return err
				}
			case e.Type()&fs.ModeSymlink != 0:
				continue
			case e.Type().IsRegular():
				if excluded(scan.Exclude, childRel) {
					continue
				}
				info, err := e.Info()
				if err != nil {
					return fmt.Errorf("observation: %s: %v", childRel, err)
				}
				out = append(out, conformance.ObservedFile{Path: childRel, Size: info.Size()})
			}
		}
		return nil
	}
	if err := visit(root, ""); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func excluded(globs []rule.Glob, rel string) bool {
	for _, g := range globs {
		if g.Match(rel) {
			return true
		}
	}
	return false
}
