// Package studio stores Studio application artifacts independently of the domain recording.
package studio

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/wixregiga/arclint/internal/application"
)

const documentLimit = 1 << 20

var fileID = regexp.MustCompile("^[a-f0-9]{32}$")

// Serialize across fresh adapters in the same process as well as requests.
var filesMu sync.Mutex

type (
	// FeatureFiles stores versioned draft documents.
	FeatureFiles  struct{ Project string }
	storedFeature struct {
		Version int                      `json:"version"`
		Feature application.FeatureDraft `json:"feature"`
	}
)

func validDraft(d application.FeatureDraft) error {
	if d.References == nil {
		return application.ErrFeatureInput
	}
	for _, r := range d.References {
		if r.ID == "" {
			return application.ErrFeatureInput
		}
		switch r.Reading {
		case "plain":
			if r.Term != nil {
				return application.ErrFeatureInput
			}
		case "names", "value", "rule":
			if r.Term == nil || r.Term.Concept == "" || r.Term.Name == "" {
				return application.ErrFeatureInput
			}
		default:
			return application.ErrFeatureInput
		}
	}
	return nil
}

// directory refuses links at every storage component, including the final file.
func (s FeatureFiles) directory(create bool) (string, error) {
	p := s.Project
	for _, part := range []string{".arclint", "studio", "features"} {
		p = filepath.Join(p, part)
		info, err := os.Lstat(p)
		if errors.Is(err, os.ErrNotExist) && create {
			if err = os.Mkdir(p, 0o700); err != nil && !os.IsExist(err) {
				return "", fmt.Errorf("feature storage: %w", err)
			}
			info, err = os.Lstat(p)
		}
		if err != nil {
			return "", fmt.Errorf("feature storage: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("unsafe feature directory %s", p)
		}
	}
	return p, nil
}

func regular(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("feature storage: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe feature file %s", path)
	}
	return nil
}

func readFeature(dir, id string) (application.FeatureDraft, error) {
	var stored storedFeature
	path := filepath.Join(dir, id+".json")
	if err := regular(path); err != nil {
		return stored.Feature, fmt.Errorf("feature storage: %w", err)
	}
	f, err := os.Open(path)
	if err != nil {
		return stored.Feature, fmt.Errorf("feature storage: %w", err)
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, documentLimit+1))
	if err != nil {
		return stored.Feature, fmt.Errorf("feature storage: %w", err)
	}
	if len(b) > documentLimit {
		return stored.Feature, errors.New("feature file too large")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&stored); err != nil {
		return stored.Feature, fmt.Errorf("feature storage: %w", err)
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return stored.Feature, errors.New("trailing feature data")
	}
	if stored.Version != 1 || stored.Feature.ID != id || stored.Feature.Revision != "" {
		return stored.Feature, errors.New("invalid stored feature")
	}
	if err = validDraft(stored.Feature); err != nil {
		return stored.Feature, fmt.Errorf("feature storage: %w", err)
	}
	sum := sha256.Sum256(b)
	stored.Feature.Revision = hex.EncodeToString(sum[:])
	return stored.Feature, nil
}

// List accesses persisted feature drafts.
func (s FeatureFiles) List() ([]application.FeatureSummary, error) {
	filesMu.Lock()
	defer filesMu.Unlock()
	result := []application.FeatureSummary{}
	dir, err := s.directory(false)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("feature storage: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("feature storage: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			return nil, fmt.Errorf("unexpected feature file %s", name)
		}
		id := name[:len(name)-5]
		if !fileID.MatchString(id) {
			return nil, errors.New("invalid stored feature id")
		}
		d, err := readFeature(dir, id)
		if err != nil {
			return nil, fmt.Errorf("feature storage: %w", err)
		}
		result = append(result, application.FeatureSummary{ID: d.ID, Title: d.Title, Revision: d.Revision})
	}
	return result, nil
}

// Get accesses persisted feature drafts.
func (s FeatureFiles) Get(id string) (application.FeatureDraft, error) {
	filesMu.Lock()
	defer filesMu.Unlock()
	return s.get(id)
}

func (s FeatureFiles) get(id string) (application.FeatureDraft, error) {
	if !fileID.MatchString(id) {
		return application.FeatureDraft{}, application.ErrFeatureInput
	}
	dir, err := s.directory(false)
	if err == nil {
		var d application.FeatureDraft
		d, err = readFeature(dir, id)
		if err == nil {
			return d, nil
		}
	}
	if errors.Is(err, os.ErrNotExist) {
		err = application.ErrFeatureMissing
	}
	return application.FeatureDraft{}, err
}

// Save accesses persisted feature drafts.
func (s FeatureFiles) Save(d application.FeatureDraft) (application.FeatureDraft, error) {
	filesMu.Lock()
	defer filesMu.Unlock()
	if err := validDraft(d); err != nil {
		return d, fmt.Errorf("feature storage: %w", err)
	}
	var err error
	d, err = s.address(d)
	if err != nil {
		return d, err
	}
	d.Revision = ""
	b, err := json.Marshal(storedFeature{Version: 1, Feature: d})
	if err != nil {
		return d, fmt.Errorf("feature storage: %w", err)
	}
	if len(b) > documentLimit {
		return d, application.ErrFeatureInput
	}
	dir, err := s.directory(true)
	if err != nil {
		return d, fmt.Errorf("feature storage: %w", err)
	}
	f, err := os.CreateTemp(dir, ".write-")
	if err != nil {
		return d, fmt.Errorf("feature storage: %w", err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err = f.Write(b); err != nil {
		_ = f.Close()
		return d, fmt.Errorf("feature storage: %w", err)
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return d, fmt.Errorf("feature storage: %w", err)
	}
	if err = f.Close(); err != nil {
		return d, fmt.Errorf("feature storage: %w", err)
	}
	if err = os.Rename(f.Name(), filepath.Join(dir, d.ID+".json")); err != nil {
		return d, fmt.Errorf("feature storage: %w", err)
	}
	sum := sha256.Sum256(b)
	d.Revision = hex.EncodeToString(sum[:])
	return d, nil
}

// Delete accesses persisted feature drafts.
func (s FeatureFiles) Delete(id, revision string) error {
	filesMu.Lock()
	defer filesMu.Unlock()
	old, err := s.get(id)
	if err != nil {
		return fmt.Errorf("feature storage: %w", err)
	}
	if old.Revision != revision {
		return application.ErrFeatureConflict
	}
	dir, err := s.directory(false)
	if err != nil {
		return fmt.Errorf("feature storage: %w", err)
	}
	if err := os.Remove(filepath.Join(dir, id+".json")); err != nil {
		return fmt.Errorf("delete feature: %w", err)
	}
	return nil
}

func (s FeatureFiles) address(d application.FeatureDraft) (application.FeatureDraft, error) {
	if d.ID != "" {
		old, err := s.get(d.ID)
		if err != nil {
			return d, fmt.Errorf("feature storage: %w", err)
		}
		if old.Revision != d.Revision {
			return d, application.ErrFeatureConflict
		}
	}
	if d.ID != "" {
		return d, nil
	}
	{
		if d.Revision != "" {
			return d, application.ErrFeatureInput
		}
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			return d, fmt.Errorf("feature storage: %w", err)
		}
		d.ID = hex.EncodeToString(id[:])
	}
	return d, nil
}
