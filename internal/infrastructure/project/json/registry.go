// Package jsonprojects remembers the projects ArcLint Studio has served,
// created or opened, for one user, in a JSON file under the user's
// configuration directory. Every change reads the file, changes it and
// replaces it whole, so a reader never sees a half-written list.
package jsonprojects

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// fileVersion is the shape of the file this package writes.
const fileVersion = 1

// Registry implements application.ProjectRegistry over one file.
type Registry struct {
	path        string
	unavailable error
}

// NewRegistry remembers projects in the file at path.
func NewRegistry(path string) Registry {
	return Registry{path: path}
}

// UserRegistry remembers projects in arclint/projects.json under the
// user's configuration directory. Without one, every call says why.
func UserRegistry() Registry {
	dir, err := os.UserConfigDir()
	if err != nil {
		return Registry{unavailable: fmt.Errorf("no user configuration directory to remember projects in: %w", err)}
	}
	return Registry{path: filepath.Join(dir, "arclint", "projects.json")}
}

// Path is the file the projects are remembered in.
func (r Registry) Path() string { return r.path }

type document struct {
	Version  int      `json:"version"`
	Projects []string `json:"projects"`
}

// Directories lists the remembered projects, the most recently used
// first. A file that does not exist yet remembers none.
func (r Registry) Directories() ([]string, error) {
	doc, err := r.read()
	if err != nil {
		return nil, err
	}
	return doc.Projects, nil
}

// Remember puts dir first, once.
func (r Registry) Remember(dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("remember %q: not an absolute path", dir)
	}
	doc, err := r.read()
	if err != nil {
		return err
	}
	dir = filepath.Clean(dir)
	doc.Projects = append([]string{dir}, slices.DeleteFunc(doc.Projects, func(p string) bool { return p == dir })...)
	return r.write(doc)
}

// Forget removes dir; a directory not remembered is no error.
func (r Registry) Forget(dir string) error {
	doc, err := r.read()
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(doc.Projects), func(p string) bool { return p == dir })
	if len(kept) == len(doc.Projects) {
		return nil
	}
	doc.Projects = kept
	return r.write(doc)
}

func (r Registry) read() (document, error) {
	if r.unavailable != nil {
		return document{}, r.unavailable
	}
	data, err := os.ReadFile(r.path)
	if errors.Is(err, fs.ErrNotExist) {
		return document{Version: fileVersion}, nil
	}
	if err != nil {
		return document{}, fmt.Errorf("read %s: %w", r.path, err)
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return document{}, fmt.Errorf("read %s: %w", r.path, err)
	}
	if doc.Version != fileVersion {
		return document{}, fmt.Errorf("read %s: version %d is not %d", r.path, doc.Version, fileVersion)
	}
	for _, p := range doc.Projects {
		if !filepath.IsAbs(p) {
			return document{}, fmt.Errorf("read %s: %q is not an absolute path", r.path, p)
		}
	}
	return doc, nil
}

// write replaces the file whole: the list goes to a temporary file
// beside it, which is then renamed over it.
func (r Registry) write(doc document) error {
	doc.Version = fileVersion
	if doc.Projects == nil {
		doc.Projects = []string{}
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("write %s: %w", r.path, err)
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		return fmt.Errorf("write %s: %w", r.path, err)
	}
	temp, err := os.CreateTemp(filepath.Dir(r.path), ".projects-*.json")
	if err != nil {
		return fmt.Errorf("write %s: %w", r.path, err)
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	if _, err := temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write %s: %w", r.path, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", r.path, err)
	}
	if err := os.Rename(temp.Name(), r.path); err != nil {
		return fmt.Errorf("write %s: %w", r.path, err)
	}
	return nil
}
