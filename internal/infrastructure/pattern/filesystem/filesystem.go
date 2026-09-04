// Package filesystempattern supplies the local Pattern packages under
// one directory, laid out by reference as
// <dir>/<namespace>/<name>/pattern.yaml beside an optional extensions
// directory. A package that also carries manifest.json is a vendored
// copy and is verified byte for byte against it on every read; a
// package without one is authored in place, and its Digest is computed
// from the files as they are. The same directory is the PatternStore
// vendoring writes into.
package filesystempattern

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
	patternfiles "github.com/wixregiga/arclint/internal/infrastructure/pattern/files"
)

// Source implements the application's PatternSource and PatternStore
// ports over one directory of Pattern packages.
type Source struct {
	// dir is the absolute directory read and written.
	dir string
	// display is the directory as the owner spelled it, for reports.
	display string
}

// NewSource binds the source to a directory; the directory may not
// exist yet, which simply means no local Patterns.
func NewSource(dir string) (Source, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Source{}, fmt.Errorf("patterns dir: %w", err)
	}
	return Source{dir: abs, display: filepath.Clean(dir)}, nil
}

// Dir returns the absolute directory.
func (s Source) Dir() string { return s.dir }

// Available loads every Pattern package under the directory with its
// exact files, in reference order. An invalid package, a package whose
// directory disagrees with its header, or a vendored copy whose files
// drifted from its Manifest is an error, never a silently skipped
// entry.
func (s Source) Available() ([]distribution.Available, error) {
	namespaces, err := s.subdirectories(s.dir)
	if err != nil {
		return nil, fmt.Errorf("patterns: %w", err)
	}
	var out []distribution.Available
	for _, namespace := range namespaces {
		nsDir := filepath.Join(s.dir, namespace)
		if _, err := os.Stat(filepath.Join(nsDir, distribution.PatternFileName)); err == nil {
			return nil, fmt.Errorf("patterns: %s holds a pattern.yaml directly; a local pattern lives at %s/<namespace>/<name>/pattern.yaml",
				filepath.Join(s.display, namespace), s.display)
		}
		names, err := s.subdirectories(nsDir)
		if err != nil {
			return nil, fmt.Errorf("patterns: %w", err)
		}
		for _, name := range names {
			pkgDir := filepath.Join(nsDir, name)
			if _, err := os.Stat(filepath.Join(pkgDir, distribution.PatternFileName)); errors.Is(err, fs.ErrNotExist) {
				continue
			}
			a, err := s.load(namespace, name)
			if err != nil {
				return nil, err
			}
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Reference().String() < out[j].Reference().String()
	})
	return out, nil
}

// Patterns loads every local Pattern in reference order.
func (s Source) Patterns() ([]rule.Pattern, error) {
	available, err := s.Available()
	if err != nil {
		return nil, err
	}
	out := make([]rule.Pattern, 0, len(available))
	for _, a := range available {
		out = append(out, a.Pattern)
	}
	return out, nil
}

// Write stores a VendoredPattern under <dir>/<namespace>/<name> with
// its Manifest, replacing whatever package of that name was there. The
// package is assembled beside its destination and swapped in with a
// rename, so a failure leaves the previous package untouched.
func (s Source) Write(v distribution.VendoredPattern) (application.StoredPattern, error) {
	if v.IsZero() {
		return application.StoredPattern{}, fmt.Errorf("store pattern: unconstructed vendored pattern")
	}
	ref := v.Reference()
	nsDir := filepath.Join(s.dir, ref.Namespace())
	target := filepath.Join(nsDir, ref.Name())
	display := filepath.Join(s.display, ref.Namespace(), ref.Name())
	replaced, err := s.existingVersion(ref.Namespace(), ref.Name())
	if err != nil {
		return application.StoredPattern{}, fmt.Errorf("store pattern %s: %w", ref, err)
	}
	if err := os.MkdirAll(nsDir, 0o750); err != nil {
		return application.StoredPattern{}, fmt.Errorf("store pattern %s: %w", ref, err)
	}
	staging, err := os.MkdirTemp(nsDir, "."+ref.Name()+".staging-")
	if err != nil {
		return application.StoredPattern{}, fmt.Errorf("store pattern %s: %w", ref, err)
	}
	// After a successful swap the staging path no longer exists; on
	// failure the partial package is discarded and the error already
	// on its way out is the one to report.
	defer func() { _ = os.RemoveAll(staging) }()
	if err := writePackage(staging, v); err != nil {
		return application.StoredPattern{}, fmt.Errorf("store pattern %s: %w", ref, err)
	}
	if err := swapIn(staging, target); err != nil {
		return application.StoredPattern{}, fmt.Errorf("store pattern %s: %w", ref, err)
	}
	return application.StoredPattern{Path: display, Replaced: replaced}, nil
}

// load reads one package directory into an Available Pattern.
func (s Source) load(namespace, name string) (distribution.Available, error) {
	pkgDir := filepath.Join(s.dir, namespace, name)
	display := filepath.Join(s.display, namespace, name)
	files, err := patternfiles.Collect(os.DirFS(pkgDir), ".")
	if err != nil {
		return distribution.Available{}, fmt.Errorf("pattern %s: %w", display, err)
	}
	p, err := patternfiles.Load(files, display)
	if err != nil {
		return distribution.Available{}, fmt.Errorf("pattern %s: %w", display, err)
	}
	ref := p.Reference()
	if ref.Namespace() != namespace || ref.Name() != name {
		return distribution.Available{}, fmt.Errorf("pattern %s: declares %s/%s; the directory must be %s",
			display, ref.Namespace(), ref.Name(), filepath.Join(s.display, ref.Namespace(), ref.Name()))
	}
	manifestPath := filepath.Join(pkgDir, distribution.ManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if errors.Is(err, fs.ErrNotExist) {
		v, err := distribution.Vendor(ref, files)
		if err != nil {
			return distribution.Available{}, fmt.Errorf("pattern %s: %w", display, err)
		}
		return available(display, distribution.SourceLocal, p, v, true)
	}
	if err != nil {
		return distribution.Available{}, fmt.Errorf("pattern %s: %w", display, err)
	}
	m, err := patternfiles.DecodeManifest(data, filepath.Join(display, distribution.ManifestFileName))
	if err != nil {
		return distribution.Available{}, fmt.Errorf("pattern %s: %w", display, err)
	}
	if m.Reference() != ref {
		return distribution.Available{}, fmt.Errorf("pattern %s: manifest.json names %s but pattern.yaml declares %s", display, m.Reference(), ref)
	}
	v, err := distribution.NewVendoredPattern(m, files)
	if err != nil {
		return distribution.Available{}, fmt.Errorf("pattern %s: %w; re-run arclint patterns vendor %s, or delete manifest.json to author it in place", display, err, ref)
	}
	return available(display, distribution.SourceLocal, p, v, false)
}

func available(display string, kind distribution.SourceKind, p rule.Pattern, v distribution.VendoredPattern, authored bool) (distribution.Available, error) {
	a, err := distribution.NewAvailable(kind, p, v, authored)
	if err != nil {
		return distribution.Available{}, fmt.Errorf("pattern %s: %w", display, err)
	}
	return a, nil
}

// existingVersion reports the version of the package already stored
// under namespace/name, "" when there is none or it does not load: a
// package that fails to load is still replaced by a write.
func (s Source) existingVersion(namespace, name string) (string, error) {
	pkgDir := filepath.Join(s.dir, namespace, name)
	info, err := os.Stat(pkgDir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", filepath.Join(s.display, namespace, name), err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", filepath.Join(s.display, namespace, name))
	}
	if _, err := os.Stat(filepath.Join(pkgDir, distribution.PatternFileName)); errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return s.loadedVersion(namespace, name), nil
}

// loadedVersion is the version the stored package declares, "" when
// the package does not load.
func (s Source) loadedVersion(namespace, name string) string {
	a, err := s.load(namespace, name)
	if err != nil {
		return ""
	}
	return a.Reference().Version()
}

// subdirectories lists the visible directories directly under dir in
// name order; a missing dir means none.
func (s Source) subdirectories(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

// writePackage lays the files and the Manifest out under dir. Files
// are written owner-only, as every other file arclint writes into a
// repository is.
func writePackage(dir string, v distribution.VendoredPattern) error {
	for _, f := range v.Files() {
		full := filepath.Join(dir, filepath.FromSlash(f.Path()))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, f.Data(), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", full, err)
		}
	}
	data, err := patternfiles.EncodeManifest(v.Manifest())
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	manifestPath := filepath.Join(dir, distribution.ManifestFileName)
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", manifestPath, err)
	}
	return nil
}

// swapIn moves the staged package to target, retiring any previous
// package and restoring it if the move fails.
func swapIn(staging, target string) error {
	retired := ""
	if _, err := os.Stat(target); err == nil {
		retired = target + ".retired-" + filepath.Base(staging)
		if err := os.Rename(target, retired); err != nil {
			return fmt.Errorf("retire previous package: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", target, err)
	}
	if err := os.Rename(staging, target); err != nil {
		if retired != "" {
			if restoreErr := os.Rename(retired, target); restoreErr != nil {
				return fmt.Errorf("move package into place: %w; restoring the previous package failed too: %v", err, restoreErr)
			}
		}
		return fmt.Errorf("move package into place: %w", err)
	}
	if retired != "" {
		if err := os.RemoveAll(retired); err != nil {
			return fmt.Errorf("remove previous package %s: %w", retired, err)
		}
	}
	return nil
}
