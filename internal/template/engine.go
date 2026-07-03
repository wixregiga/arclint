package template

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jofyi/arclint/internal/version"
)

// Template is a loaded thing type: a directory under .arclint/templates/
// with a validated manifest.
type Template struct {
	Name     string
	Dir      string // absolute path to the template directory
	Manifest *Manifest
}

// TemplatesDir returns the templates root under the repo root.
func TemplatesDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".arclint", "templates")
}

// Discover lists thing types by listing .arclint/templates/ — a directory
// containing a template.yaml is a thing type, nothing else registers one
// (docs/design/templating.md §1). Sorted, possibly empty.
func Discover(repoRoot string) ([]string, error) {
	entries, err := os.ReadDir(TemplatesDir(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot list %s — %w", TemplatesDir(repoRoot), err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(TemplatesDir(repoRoot), e.Name(), "template.yaml")); err == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Load loads and validates one template by thing name.
func Load(repoRoot, name string) (*Template, error) {
	if strings.ContainsAny(name, `/\`) || name == ".." || name == "." {
		return nil, fmt.Errorf("invalid template name %q — thing names are plain directory names", name)
	}
	dir := filepath.Join(TemplatesDir(repoRoot), name)
	m, err := LoadManifest(filepath.Join(dir, "template.yaml"))
	if err != nil {
		return nil, err
	}
	return &Template{Name: name, Dir: dir, Manifest: m}, nil
}

// Builtins returns the variables arclint injects itself
// (docs/design/templating.md §3). Manifest variables shadow these.
func Builtins(repoRoot string) map[string]string {
	return map[string]string{
		"repo_name":       filepath.Base(repoRoot),
		"year":            strconv.Itoa(time.Now().Year()),
		"arclint_version": version.Version,
	}
}

// Destination interpolates the manifest's destination path and validates it
// as a clean, relative, traversal-free slash path.
func (t *Template) Destination(vars map[string]string) (string, error) {
	out, err := NewRenderer(vars).Render([]byte(t.Manifest.Destination), t.Name+"/template.yaml:destination")
	if err != nil {
		return "", err
	}
	dest := path.Clean(filepath.ToSlash(string(out)))
	if err := validateRelPath(dest); err != nil {
		return "", fmt.Errorf("destination %q: %w", dest, err)
	}
	return dest, nil
}

// RenderUnit renders the whole files/ tree to memory: map of rendered
// relative path (slash-separated, under the destination) to rendered content.
// Nothing touches disk; a render error anywhere means no files at all.
func (t *Template) RenderUnit(vars map[string]string) (map[string][]byte, error) {
	filesRoot := filepath.Join(t.Dir, "files")
	if info, err := os.Stat(filesRoot); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("template %q has no files/ directory — create %s and put the template files under it", t.Name, filesRoot)
	}
	r := NewRenderer(vars)
	out := map[string][]byte{}
	walkErr := filepath.WalkDir(filesRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Reject symlinks: os.ReadFile would follow the link and emit the
		// target's content, letting a template escape its own files/ tree.
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("template file %q is a symlink — symlinks are not allowed in files/; replace it with a regular file", filepath.ToSlash(p))
		}
		rel, err := filepath.Rel(filesRoot, p)
		if err != nil {
			return err
		}
		srcRel := filepath.ToSlash(rel)
		rendered, err := r.Render([]byte(srcRel), t.Name+"/files/"+srcRel)
		if err != nil {
			return err
		}
		dstRel := path.Clean(filepath.ToSlash(string(rendered)))
		if err := validateRelPath(dstRel); err != nil {
			return fmt.Errorf("file %q renders to path %q: %w", srcRel, dstRel, err)
		}
		if _, dup := out[dstRel]; dup {
			return fmt.Errorf("two template files render to the same destination path %q — rename one of them", dstRel)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if isBinary(data) {
			out[dstRel] = data // copied verbatim; the name was still interpolated
			return nil
		}
		content, err := r.Render(data, t.Name+"/files/"+srcRel)
		if err != nil {
			return err
		}
		out[dstRel] = content
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return out, nil
}

// isBinary is the Git heuristic: a NUL byte in the first 8000 bytes.
func isBinary(data []byte) bool {
	n := min(len(data), 8000)
	return bytes.IndexByte(data[:n], 0) >= 0
}

// ValidateRelPath is the exported guard for callers outside this package
// (e.g. the CLI's --out flag) so path validation stays in one place.
func ValidateRelPath(p string) error { return validateRelPath(p) }

// validateRelPath rejects absolute paths, empty paths, and any .. segment —
// the path traversal guard (docs/design/templating.md §3, paths).
func validateRelPath(p string) error {
	if p == "" || p == "." {
		return fmt.Errorf("path is empty")
	}
	if strings.HasPrefix(p, "/") || filepath.IsAbs(filepath.FromSlash(p)) {
		return fmt.Errorf("path must be relative")
	}
	// filepath.IsAbs("C:/foo") is false on Linux, so a Windows drive-absolute
	// path would slip through the check above. Reject a leading drive letter
	// (e.g. "C:/foo", "c:bar") explicitly, on every OS.
	if len(p) >= 2 && p[1] == ':' &&
		((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z')) {
		return fmt.Errorf("path must be relative")
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return fmt.Errorf("path escapes the destination root")
		}
	}
	return nil
}
