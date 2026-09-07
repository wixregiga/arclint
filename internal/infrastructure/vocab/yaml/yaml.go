// Package yamlvocab loads and stores the project's recorded Ubiquitous
// Language from the project's domain file. Reading is strict: every
// key is held to what its building block records, and content that
// cannot become a valid vocab.UbiquitousLanguage is an error, never a
// partial value. Record preserves the human authoring of untouched
// entries (comments, ordering, scalar styles), fills folded prose to
// lineWidth, and writes atomically.
package yamlvocab

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// Repository implements the domain-owned vocab.Repository port over
// one domain file (vocab.UbiquitousLanguageFileName) beside the
// resolved ruleset root.
type Repository struct {
	path string
	root string
}

// NewRepository binds the repository to
// filepath.Join(root, UbiquitousLanguageFileName).
func NewRepository(root string) (Repository, error) {
	if root == "" {
		return Repository{}, errors.New("domain file root: empty path")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Repository{}, fmt.Errorf("domain file root: %w", err)
	}
	return Repository{path: filepath.Join(absRoot, vocab.UbiquitousLanguageFileName), root: absRoot}, nil
}

// Path returns the absolute path of the domain file.
func (r Repository) Path() string { return r.path }

// RecordedLanguage returns the language and whether the file exists.
// A missing file is (zero UbiquitousLanguage, false, nil).
func (r Repository) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return vocab.UbiquitousLanguage{}, false, nil
		}
		return vocab.UbiquitousLanguage{}, false, fmt.Errorf("read %s: %w", r.path, err)
	}
	lang, err := parse(data, r.path)
	if err != nil {
		return vocab.UbiquitousLanguage{}, true, err
	}
	return lang, true, nil
}

// Parse turns authored domain file content into the recorded language,
// applying the same strict reading and domain invariants as
// RecordedLanguage. Callers that hold fixture bytes rather than a
// repository file, such as the rule-test harness feeding ctx.domain(),
// parse through here.
func Parse(data []byte) (vocab.UbiquitousLanguage, error) {
	return parse(data, vocab.UbiquitousLanguageFileName)
}

// Parser adapts Parse to the application's VocabularySource port.
type Parser struct{}

// ParseUbiquitousLanguage implements application.VocabularySource.
func (Parser) ParseUbiquitousLanguage(content []byte) (vocab.UbiquitousLanguage, error) {
	return Parse(content)
}

// Record persists the complete language. An existing file is edited in
// place as a node tree, so the comments, ordering, and scalar styles of
// every entry the language still records survive; a missing or empty
// file is written fresh with the editor schema modeline.
func (r Repository) Record(m vocab.UbiquitousLanguage) error {
	var root yaml.Node
	existing, err := os.ReadFile(r.path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(existing, &root); err != nil {
			return fmt.Errorf("%s: %v", r.path, err)
		}
		body := documentBody(&root)
		switch {
		case body == nil:
			root = *freshDocument(m, r.schemaModeline())
		case body.Kind != yaml.MappingNode:
			return fmt.Errorf("%s: the file is not a mapping; it cannot be edited in place", r.path)
		default:
			syncDocument(body, m)
		}
	case errors.Is(err, fs.ErrNotExist):
		root = *freshDocument(m, r.schemaModeline())
	default:
		return fmt.Errorf("read %s: %w", r.path, err)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		_ = enc.Close()
		return fmt.Errorf("encode %s: %w", r.path, err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("encode %s: %w", r.path, err)
	}
	return atomicWrite(r.path, reflowFolded(buf.Bytes()))
}

// schemaModeline chooses the editor schema hint for a freshly created
// file: the project's schema copy when it exists under the bound root,
// else the published $id.
func (r Repository) schemaModeline() string {
	local := filepath.Join(r.root, filepath.FromSlash(vocab.SchemaPath))
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		return "# yaml-language-server: $schema=" + vocab.SchemaPath
	}
	return "# yaml-language-server: $schema=" + vocab.SchemaID
}

// atomicWrite writes data to path via a same-directory temp file and
// rename, leaving no temp residue on success or failure. Mode 0o600
// matches the baseline JSON store.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".domain-*.yaml")
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
