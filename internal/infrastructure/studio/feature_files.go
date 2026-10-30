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
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

const documentLimit = 1 << 20

var fileID = regexp.MustCompile("^[a-f0-9]{32}$")

type (
	// FeatureFiles stores versioned draft documents beneath one project.
	FeatureFiles  struct{ Project string }
	storedFeature struct {
		Version int                      `json:"version"`
		Feature application.FeatureDraft `json:"feature"`
	}
)

// directory pins each directory while descending. Root operations confine even a
// concurrent symlink replacement; no subsequent file access uses an absolute path.
func (s FeatureFiles) directory(create bool) (*os.Root, error) {
	root, err := os.OpenRoot(s.Project)
	if err != nil {
		return nil, fmt.Errorf("feature project: %w", err)
	}
	for _, part := range []string{".arclint", "studio", "features"} {
		if create {
			if err = root.Mkdir(part, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
				_ = root.Close()
				return nil, fmt.Errorf("create feature directory: %w", err)
			}
		}
		info, statErr := root.Lstat(part)
		if statErr != nil || !info.IsDir() {
			_ = root.Close()
			if statErr != nil {
				return nil, fmt.Errorf("feature directory: %w", statErr)
			}
			return nil, fmt.Errorf("unsafe feature directory %s", part)
		}
		next, openErr := root.OpenRoot(part)
		_ = root.Close()
		if openErr != nil {
			return nil, fmt.Errorf("open feature directory: %w", openErr)
		}
		root = next
	}
	return root, nil
}

// withFiles reuses the workflow store's rooted-file and advisory-lock pattern.
// One lock covers revision comparison and replacement across Studio processes.
func (s FeatureFiles) withFiles(create bool, work func(*os.Root) error) (returnErr error) {
	root, err := s.directory(create)
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, root.Close()) }()
	lock, err := root.OpenFile(".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("feature lock: %w", err)
	}
	defer func() { returnErr = errors.Join(returnErr, lock.Close()) }()
	info, err := root.Lstat(".lock")
	if err != nil {
		return fmt.Errorf("feature lock: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("unsafe feature lock")
	}
	unlock, err := lockFeatures(lock)
	if err != nil {
		return err
	}
	defer func() { returnErr = errors.Join(returnErr, unlock()) }()
	return work(root)
}

func readFeature(root *os.Root, id string) (application.FeatureDraft, error) {
	var stored storedFeature
	name := id + ".json"
	info, err := root.Lstat(name)
	if err != nil {
		return stored.Feature, fmt.Errorf("feature file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return stored.Feature, fmt.Errorf("unsafe feature file %s", name)
	}
	f, err := root.Open(name)
	if err != nil {
		return stored.Feature, fmt.Errorf("open feature: %w", err)
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, documentLimit+1))
	if err != nil {
		return stored.Feature, fmt.Errorf("read feature: %w", err)
	}
	if len(b) > documentLimit {
		return stored.Feature, errors.New("feature file too large")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&stored); err != nil {
		return stored.Feature, fmt.Errorf("decode feature: %w", err)
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return stored.Feature, errors.New("trailing feature data")
	}
	var required struct {
		Feature struct {
			Title *string `json:"title"`
			Body  *string `json:"body"`
		} `json:"feature"`
	}
	if err := json.Unmarshal(b, &required); err != nil {
		return stored.Feature, fmt.Errorf("decode feature fields: %w", err)
	}
	if required.Feature.Title == nil || required.Feature.Body == nil {
		return stored.Feature, errors.New("stored feature needs title and body fields")
	}
	if stored.Version != 1 || stored.Feature.ID != id || stored.Feature.Revision != "" {
		return stored.Feature, errors.New("invalid stored feature")
	}
	if err = application.CheckFeatureDraft(stored.Feature); err != nil {
		return stored.Feature, fmt.Errorf("stored feature: %w", err)
	}
	sum := sha256.Sum256(b)
	stored.Feature.Revision = hex.EncodeToString(sum[:])
	return stored.Feature, nil
}

// List reads every saved draft in file-name order; a corrupt document is visible as an error.
func (s FeatureFiles) List() ([]application.FeatureSummary, error) {
	result := []application.FeatureSummary{}
	opened := false
	err := s.withFiles(false, func(root *os.Root) error {
		opened = true
		entries, err := fs.ReadDir(root.FS(), ".")
		if err != nil {
			return fmt.Errorf("list features: %w", err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			if !fileID.MatchString(strings.TrimSuffix(name, ".json")) {
				return fmt.Errorf("unexpected feature file %s", name)
			}
			d, err := readFeature(root, strings.TrimSuffix(name, ".json"))
			if err != nil {
				return err
			}
			result = append(result, application.FeatureSummary{ID: d.ID, Title: d.Title, Revision: d.Revision})
		}
		return nil
	})
	if !opened && errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	return result, err
}

// Get reopens one saved draft and computes its current content revision.
func (s FeatureFiles) Get(id string) (application.FeatureDraft, error) {
	if !fileID.MatchString(id) {
		return application.FeatureDraft{}, application.ErrFeatureInput
	}
	var result application.FeatureDraft
	err := s.withFiles(false, func(root *os.Root) error {
		var err error
		result, err = readFeature(root, id)
		return err
	})
	if errors.Is(err, os.ErrNotExist) {
		err = application.ErrFeatureMissing
	}
	return result, err
}

// Save replaces a complete snapshot only while its last-read revision is current.
func (s FeatureFiles) Save(d application.FeatureDraft) (application.FeatureDraft, error) {
	if err := application.CheckFeatureDraft(d); err != nil {
		return d, fmt.Errorf("check feature: %w", err)
	}
	if (d.ID == "" && d.Revision != "") || (d.ID != "" && !fileID.MatchString(d.ID)) {
		return d, application.ErrFeatureInput
	}
	err := s.withFiles(d.ID == "", func(root *os.Root) error {
		id, err := featureAddress(root, d)
		if err != nil {
			return err
		}
		d.ID = id
		d.Revision = ""
		b, err := json.Marshal(storedFeature{Version: 1, Feature: d})
		if err != nil {
			return fmt.Errorf("encode feature: %w", err)
		}
		if len(b) > documentLimit {
			return application.ErrFeatureInput
		}
		if err := ensureMetadataIgnored(root); err != nil {
			return err
		}
		if err := replaceFeature(root, d.ID+".json", b); err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		d.Revision = hex.EncodeToString(sum[:])
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		err = application.ErrFeatureMissing
	}
	return d, err
}

// ensureMetadataIgnored leaves any adopter-authored ignore file intact.
func ensureMetadataIgnored(root *os.Root) error {
	f, err := root.OpenFile(".gitignore", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create feature ignore file: %w", err)
	}
	if _, err = io.WriteString(f, ".lock\n.write-*\n"); err != nil {
		_ = f.Close()
		_ = root.Remove(".gitignore")
		return fmt.Errorf("write feature ignore file: %w", err)
	}
	if err = f.Close(); err != nil {
		_ = root.Remove(".gitignore")
		return fmt.Errorf("close feature ignore file: %w", err)
	}
	return nil
}

func replaceFeature(root *os.Root, name string, body []byte) error {
	temporary := ".write-" + rand.Text()
	f, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create feature: %w", err)
	}
	defer func() { _ = root.Remove(temporary) }()
	if _, err = f.Write(body); err != nil {
		_ = f.Close()
		return fmt.Errorf("write feature: %w", err)
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync feature: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("close feature: %w", err)
	}
	if err = root.Rename(temporary, name); err != nil {
		return fmt.Errorf("replace feature: %w", err)
	}
	return nil
}

// Delete removes a saved draft only if the caller has its current revision.
func (s FeatureFiles) Delete(id, revision string) error {
	if !fileID.MatchString(id) {
		return application.ErrFeatureInput
	}
	err := s.withFiles(false, func(root *os.Root) error {
		old, err := readFeature(root, id)
		if err != nil {
			return err
		}
		if old.Revision != revision {
			return application.ErrFeatureConflict
		}
		if err := root.Remove(id + ".json"); err != nil {
			return fmt.Errorf("delete feature: %w", err)
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		err = application.ErrFeatureMissing
	}
	return err
}

func featureAddress(root *os.Root, draft application.FeatureDraft) (string, error) {
	if draft.ID != "" {
		old, err := readFeature(root, draft.ID)
		if err != nil {
			return "", err
		}
		if old.Revision != draft.Revision {
			return "", application.ErrFeatureConflict
		}
		return draft.ID, nil
	}
	for {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", fmt.Errorf("feature id: %w", err)
		}
		id := hex.EncodeToString(random[:])
		_, err := root.Lstat(id + ".json")
		if errors.Is(err, os.ErrNotExist) {
			return id, nil
		}
		if err != nil {
			return "", fmt.Errorf("feature id: %w", err)
		}
	}
}
