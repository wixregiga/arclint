// Package artifactfs writes generated artifacts (the domain-librarian
// skill files, the published JSON Schemas) through the application's
// ArtifactWriter port. Writes are atomic and create the target
// directory as needed.
package artifactfs

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// Writer implements application.ArtifactWriter over a repository root.
// Paths passed to Write are joined under root when relative.
type Writer struct {
	root string
}

// NewWriter binds the writer to a repository root.
func NewWriter(root string) (Writer, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Writer{}, fmt.Errorf("artifact writer root: %w", err)
	}
	return Writer{root: abs}, nil
}

// Write implements application.ArtifactWriter. dir may be absolute or
// relative to the bound root; the use case chooses the default, so an
// empty dir is a caller error. Returns changed=false when the file
// already holds identical bytes.
func (w Writer) Write(dir, filename string, content []byte) (bool, string, error) {
	if filename == "" {
		return false, "", fmt.Errorf("artifact write: empty filename")
	}
	if dir == "" {
		return false, "", fmt.Errorf("artifact write %s: empty directory", filename)
	}
	targetDir := dir
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(w.root, dir)
	}
	target := filepath.Join(targetDir, filename)

	existing, err := os.ReadFile(target)
	switch {
	case err == nil:
		if bytes.Equal(existing, content) {
			return false, target, nil
		}
	case !os.IsNotExist(err):
		return false, target, fmt.Errorf("read %s: %w", target, err)
	}

	if err := atomicWrite(target, content); err != nil {
		return false, target, err
	}
	return true, target, nil
}

// atomicWrite writes data to path via a same-directory temp file and
// rename, leaving no temp residue on success or failure. Mode 0o600
// matches the vocab YAML and baseline JSON stores.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".artifact-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp for %s: %w", path, err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp onto %s: %w", path, err)
	}
	cleanup = false
	return nil
}
