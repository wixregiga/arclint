package cmlvocab

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	vocabfs "github.com/wixregiga/arclint/internal/infrastructure/vocab"
	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

// Repository implements vocab.Repository over a CML DomainRecording.
type Repository struct {
	path      string
	root      string
	recording vocab.DomainRecording
}

// NewRepository binds a CML recording beneath the resolved repository root.
func NewRepository(root string, recording vocab.DomainRecording) (Repository, error) {
	if root == "" {
		return Repository{}, errors.New("domain file root: empty path")
	}
	if recording.Format() != vocab.DomainRecordingCML {
		return Repository{}, fmt.Errorf("domain file %q: the CML adapter does not read a %q recording", recording.Path(), recording.Format())
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return Repository{}, fmt.Errorf("domain file root: %w", err)
	}
	return Repository{path: filepath.Join(abs, filepath.FromSlash(recording.Path())), root: abs, recording: recording}, nil
}

// Recording returns the file and format this repository records in.
func (r Repository) Recording() vocab.DomainRecording { return r.recording }

// Path returns the recording's absolute file path.
func (r Repository) Path() string { return r.path }

// RecordedLanguage reads a complete validated language. A missing file returns
// the zero value, false and nil; an existing invalid file returns true and error.
func (r Repository) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	path, err := vocabfs.ResolveRecording(r.root, r.recording)
	if err != nil {
		return vocab.UbiquitousLanguage{}, false, fmt.Errorf("resolve domain recording: %w", err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return vocab.UbiquitousLanguage{}, false, nil
	}
	if err != nil {
		return vocab.UbiquitousLanguage{}, false, fmt.Errorf("read %s: %w", r.path, err)
	}
	lang, err := Parse(data, r.recording)
	return lang, true, err
}

// Record synchronizes modeled content while preserving foreign CML constructs.
// An unchanged language does not format the file or perform any write.
func (r Repository) Record(lang vocab.UbiquitousLanguage) error {
	data, unchanged, err := r.draft(lang)
	if err != nil || unchanged {
		return err
	}
	return r.atomicWrite(data)
}

// Draft works out, in memory, the file Record would write for lang: the
// model edited in place when the file holds one, else a fresh model. It
// writes nothing; an unchanged language drafts the file as it is.
func (r Repository) Draft(lang vocab.UbiquitousLanguage) ([]byte, error) {
	data, _, err := r.draft(lang)
	return data, err
}

func (r Repository) draft(lang vocab.UbiquitousLanguage) ([]byte, bool, error) {
	if _, err := vocab.NewUbiquitousLanguage(lang.Project, lang.Description, lang.Contexts, lang.Relations); err != nil {
		return nil, false, fmt.Errorf("record CML: %w", err)
	}
	path, err := vocabfs.ResolveRecording(r.root, r.recording)
	if err != nil {
		return nil, false, fmt.Errorf("resolve domain recording: %w", err)
	}
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, false, fmt.Errorf("read %s: %w", r.path, err)
	}
	var data []byte
	if len(bytes.TrimSpace(existing)) == 0 {
		data, err = Render(lang)
	} else {
		data, err = r.editedBytes(existing, lang)
	}
	if err != nil {
		return nil, false, fmt.Errorf("record %s: %w", r.path, err)
	}
	if data == nil {
		return existing, true, nil
	}
	return data, false, nil
}

// atomicWrite writes data to the recording through a same-directory temp
// file and rename, leaving no temp residue. The file keeps its mode; a
// new file is written 0o600, as the ruleset writer does.
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
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
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
	if err := os.Rename(name, target); err != nil {
		return fmt.Errorf("rename temp onto %s: %w", path, err)
	}
	return nil
}

func (r Repository) editedBytes(existing []byte, lang vocab.UbiquitousLanguage) ([]byte, error) {
	file, err := syntax.ParseFile(r.recording.Path(), existing)
	if err != nil {
		return nil, fmt.Errorf("parse existing CML: %w", err)
	}
	old, p, err := project(file, r.recording)
	if err != nil {
		return nil, err
	}
	if sameLanguage(old, lang) {
		return nil, nil
	}
	if err := syncFile(p, old, lang); err != nil {
		return nil, err
	}
	data, err := syntax.FormatChecked(file)
	if err != nil {
		return nil, fmt.Errorf("format edited CML: %w", err)
	}
	read, err := Parse(data, r.recording)
	if err != nil {
		return nil, err
	}
	if !sameLanguage(read, lang) {
		return nil, errors.New("edited CML does not preserve the requested language; file was not written")
	}
	return data, nil
}
