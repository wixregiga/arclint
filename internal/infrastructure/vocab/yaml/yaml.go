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
	vocabfs "github.com/wixregiga/arclint/internal/infrastructure/vocab"
)

// Repository implements the domain-owned vocab.Repository port over
// the project's DomainRecording, a YAML file inside the resolved ruleset root.
type Repository struct {
	path      string
	root      string
	recording vocab.DomainRecording
}

// NewRepository binds the repository to the DomainDomainRecording's file under
// root. A DomainRecording written in another format is refused: this adapter
// reads and writes YAML only.
func NewRepository(root string, recording vocab.DomainRecording) (Repository, error) {
	if root == "" {
		return Repository{}, errors.New("domain file root: empty path")
	}
	if recording.Format() != vocab.DomainRecordingYAML {
		return Repository{}, fmt.Errorf("domain file %q: the YAML adapter does not read a %q recording", recording.Path(), recording.Format())
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Repository{}, fmt.Errorf("domain file root: %w", err)
	}
	return Repository{
		path:      filepath.Join(absRoot, filepath.FromSlash(recording.Path())),
		root:      absRoot,
		recording: recording,
	}, nil
}

// Path returns the absolute path of the domain file.
func (r Repository) Path() string { return r.path }

// Recording returns where this repository writes the language down.
func (r Repository) Recording() vocab.DomainRecording { return r.recording }

// RecordedLanguage returns the language and whether the file exists.
// The language carries the DomainRecording it was read from. A missing file
// is an empty UbiquitousLanguage that carries the DomainRecording it would be
// written to, false, and no error.
func (r Repository) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	path, err := vocabfs.ResolveRecording(r.root, r.recording)
	if err != nil {
		return vocab.UbiquitousLanguage{}, false, fmt.Errorf("resolve domain recording: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return vocab.UbiquitousLanguage{Recording: r.recording}, false, nil
		}
		return vocab.UbiquitousLanguage{}, false, fmt.Errorf("read %s: %w", r.path, err)
	}
	lang, err := parse(data, r.path, r.recording)
	if err != nil {
		return vocab.UbiquitousLanguage{}, true, err
	}
	return lang, true, nil
}

// Parse turns authored domain file content into the recorded language,
// applying the same strict reading and domain invariants as
// RecordedLanguage. The language carries the given DomainRecording, and
// messages name its file. Callers that hold fixture bytes rather than
// a repository file, such as the rule-test harness feeding
// ctx.domain(), parse through here.
func Parse(data []byte, recording vocab.DomainRecording) (vocab.UbiquitousLanguage, error) {
	if recording.Format() != vocab.DomainRecordingYAML {
		return vocab.UbiquitousLanguage{}, fmt.Errorf("domain file %q: the YAML adapter does not read a %q recording", recording.Path(), recording.Format())
	}
	return parse(data, recording.Path(), recording)
}

// Parser adapts Parse to the application's VocabularySource port.
type Parser struct{}

// ParseUbiquitousLanguage implements application.VocabularySource.
func (Parser) ParseUbiquitousLanguage(content []byte, recording vocab.DomainRecording) (vocab.UbiquitousLanguage, error) {
	return Parse(content, recording)
}

// Record persists the complete language. An existing file is edited in
// place as a node tree, so the comments, ordering, and scalar styles of
// every entry the language still records survive; a missing or empty
// file is written fresh with the editor schema modeline.
func (r Repository) Record(m vocab.UbiquitousLanguage) error {
	data, err := r.Draft(m)
	if err != nil {
		return err
	}
	return r.atomicWrite(data)
}

// Draft works out, in memory, the file Record would write for m: the
// existing file edited in place, or a fresh one. It writes nothing.
func (r Repository) Draft(m vocab.UbiquitousLanguage) ([]byte, error) {
	path, err := vocabfs.ResolveRecording(r.root, r.recording)
	if err != nil {
		return nil, fmt.Errorf("resolve domain recording: %w", err)
	}
	var root yaml.Node
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(existing, &root); err != nil {
			return nil, fmt.Errorf("%s: %v", r.path, err)
		}
		body := documentBody(&root)
		switch {
		case body == nil:
			root = *freshDocument(m, r.schemaModeline())
		case body.Kind != yaml.MappingNode:
			return nil, fmt.Errorf("%s: the file is not a mapping; it cannot be edited in place", r.path)
		default:
			syncDocument(body, m)
		}
	case errors.Is(err, fs.ErrNotExist):
		root = *freshDocument(m, r.schemaModeline())
	default:
		return nil, fmt.Errorf("read %s: %w", r.path, err)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		_ = enc.Close()
		return nil, fmt.Errorf("encode %s: %w", r.path, err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("encode %s: %w", r.path, err)
	}
	return reflowFolded(buf.Bytes()), nil
}

// schemaModeline chooses the editor schema hint for a freshly created
// file: the project's schema copy when it exists under the bound root,
// named by its path from the directory the file is in, else the
// published $id.
func (r Repository) schemaModeline() string {
	local := filepath.Join(r.root, filepath.FromSlash(vocab.SchemaPath))
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		if relative, err := filepath.Rel(filepath.Dir(r.path), local); err == nil {
			return "# yaml-language-server: $schema=" + filepath.ToSlash(relative)
		}
	}
	return "# yaml-language-server: $schema=" + vocab.SchemaID
}

// atomicWrite writes data to path via a same-directory temp file and
// rename, leaving no temp residue on success or failure. The file keeps
// its mode; a new file is written 0o600, as the ruleset writer does.
func (r Repository) atomicWrite(data []byte) error {
	path, err := vocabfs.ResolveRecording(r.root, r.recording)
	if err != nil {
		return fmt.Errorf("resolve domain recording: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	path, err = vocabfs.ResolveRecording(r.root, r.recording)
	if err != nil {
		return fmt.Errorf("resolve domain recording: %w", err)
	}
	dir = filepath.Dir(path)
	mode := fs.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
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
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp for %s: %w", path, err)
	}
	target, err := vocabfs.ResolveRecording(r.root, r.recording)
	if err != nil {
		return fmt.Errorf("resolve domain recording: %w", err)
	}
	if target != path {
		return fmt.Errorf("domain recording location changed from %s to %s during write", path, target)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("rename temp onto %s: %w", path, err)
	}
	cleanup = false
	return nil
}
