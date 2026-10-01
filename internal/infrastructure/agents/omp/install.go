// Package omp installs the project-local domain guard for OMP.
package omp

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed assets/index.js assets/guard.mjs
var assets embed.FS

// Installer writes the supported OMP integration under a project root.
type Installer struct{ root string }

// NewInstaller binds installation to a project root.
func NewInstaller(root string) *Installer { return &Installer{root: root} }

// Install is idempotent. It refuses to overwrite any different artifact so
// concurrent edits and project configuration remain intact.
func (i *Installer) Install(host string, domains []string) ([]string, error) {
	if host != "omp" {
		return nil, fmt.Errorf("unsupported host %q", host)
	}
	root, err := filepath.Abs(i.root)
	if err != nil {
		return nil, fmt.Errorf("install OMP guard: %w", err)
	}
	seen := map[string]bool{}
	paths := make([]string, 0, len(domains))
	for _, name := range domains {
		clean := filepath.Clean(name)
		if name == "" || filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("domain file must be project-relative: %q", name)
		}
		full := filepath.Join(root, clean)
		resolved, resolveErr := filepath.EvalSymlinks(full)
		if resolveErr != nil {
			return nil, fmt.Errorf("domain file %s: %w", name, resolveErr)
		}
		rel, relErr := filepath.Rel(root, resolved)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("domain file escapes project: %s", name)
		}
		info, statErr := os.Stat(full)
		if statErr != nil {
			return nil, fmt.Errorf("install OMP guard: %w", statErr)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("domain file is not regular: %s", name)
		}
		clean = filepath.ToSlash(clean)
		if !seen[clean] {
			paths = append(paths, clean)
			seen[clean] = true
		}
	}
	sort.Strings(paths)
	config, err := json.MarshalIndent(struct {
		Version     int      `json:"version"`
		DomainFiles []string `json:"domainFiles"`
	}{1, paths}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("install OMP guard: %w", err)
	}
	config = append(config, '\n')
	if existing, readErr := os.ReadFile(filepath.Join(root, ".arclint/domain-guard.json")); readErr == nil {
		var recorded struct {
			Version     int      `json:"version"`
			DomainFiles []string `json:"domainFiles"`
		}
		if json.Unmarshal(existing, &recorded) != nil || recorded.Version != 1 || strings.Join(recorded.DomainFiles, "\x00") != strings.Join(paths, "\x00") {
			return nil, fmt.Errorf("preserving different domain scope")
		}
		config = existing
	} else if !os.IsNotExist(readErr) {
		return nil, fmt.Errorf("read guard scope: %w", readErr)
	}
	files := map[string][]byte{".arclint/domain-guard.json": config}
	for _, name := range []string{"index.js", "guard.mjs"} {
		content, readErr := assets.ReadFile("assets/" + name)
		if readErr != nil {
			return nil, fmt.Errorf("install OMP guard: %w", readErr)
		}
		files[".omp/extensions/arclint-domain-guard/"+name] = content
	}
	targets := make([]string, 0, len(files))
	for name, content := range files {
		target := filepath.Join(root, filepath.FromSlash(name))
		// Reject symlinked output paths, including parent directories.
		for p := target; p != root; p = filepath.Dir(p) {
			info, statErr := os.Lstat(p)
			if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("refusing symlinked install path: %s", p)
			}
			if statErr != nil && !os.IsNotExist(statErr) {
				return nil, fmt.Errorf("install OMP guard: %w", statErr)
			}
		}
		existing, readErr := os.ReadFile(target)
		if readErr == nil && !bytes.Equal(existing, content) {
			return nil, fmt.Errorf("preserving existing different file: %s", target)
		}
		if readErr != nil && !os.IsNotExist(readErr) {
			return nil, fmt.Errorf("install OMP guard: %w", readErr)
		}
		targets = append(targets, target)
	}
	sort.Strings(targets)
	for _, target := range targets {
		rel, relErr := filepath.Rel(root, target)
		if relErr != nil {
			return nil, fmt.Errorf("install OMP guard: %w", relErr)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return nil, fmt.Errorf("install OMP guard: %w", err)
		}
		file, createErr := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if os.IsExist(createErr) {
			existing, readErr := os.ReadFile(target)
			if readErr != nil || !bytes.Equal(existing, files[filepath.ToSlash(rel)]) {
				return nil, fmt.Errorf("install target changed concurrently: %s", target)
			}
			continue
		}
		if createErr != nil {
			return nil, fmt.Errorf("install OMP guard: %w", createErr)
		}
		_, writeErr := file.Write(files[filepath.ToSlash(rel)])
		closeErr := file.Close()
		if writeErr != nil {
			return nil, fmt.Errorf("install OMP guard: %w", writeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("install OMP guard: %w", closeErr)
		}
	}
	return targets, nil
}
