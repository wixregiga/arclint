package cmlvocab

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const corpusRoot = "syntax/testdata/corpus"

type corpusOutcome struct {
	File     string        `json:"file"`
	Counts   *vocab.Counts `json:"counts,omitempty"`
	Problems []string      `json:"problems,omitempty"`
}

func TestCorpusProjection(t *testing.T) {
	t.Parallel()
	var outcomes []corpusOutcome
	projected, rejected := 0, 0
	err := filepath.WalkDir(corpusRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".cml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(corpusRoot, path)
		r, err := vocab.NewDomainRecording(filepath.ToSlash(relative))
		if err != nil {
			return err
		}
		lang, err := Parse(data, r)
		outcome := corpusOutcome{File: filepath.ToSlash(relative)}
		if err == nil {
			counts := lang.Counts()
			outcome.Counts = &counts
			projected++
			assertCorpusRecordUntouched(t, path, data, lang, false)
		} else {
			problems := strings.Split(err.Error(), "\n")
			if len(problems) > 4 {
				problems = problems[:4]
			}
			outcome.Problems = problems
			rejected++
			// These inputs cannot represent a no-op language: refuse a Record
			// without replacing the authored bytes with a partial projection.
			if len(bytes.TrimSpace(data)) != 0 {
				assertCorpusRecordUntouched(t, path, data, vocab.UbiquitousLanguage{Project: "UnprojectableCorpus"}, true)
			}
		}
		outcomes = append(outcomes, outcome)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(outcomes, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	manifest := "testdata/corpus_manifest.json"
	if os.Getenv("UPDATE_CML_GOLDENS") == "1" {
		if err := os.WriteFile(manifest, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	expected, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(expected) {
		t.Fatal("corpus projection differs from committed manifest")
	}
	t.Logf("%d CML files: %d projected, %d rejected with problems", len(outcomes), projected, rejected)
}

func assertCorpusRecordUntouched(t *testing.T, path string, data []byte, lang vocab.UbiquitousLanguage, rejected bool) {
	t.Helper()
	repo, err := NewRepository(t.TempDir(), recording(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(repo.Path(), data, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	err = repo.Record(lang)
	if (err != nil) != rejected {
		t.Errorf("%s: Record rejection=%v, want %v: %v", path, err != nil, rejected, err)
	}
	after, err := os.Stat(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	unchanged, err := os.ReadFile(repo.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, unchanged) || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Errorf("%s: no-op or refused Record changed the file", path)
	}
}
