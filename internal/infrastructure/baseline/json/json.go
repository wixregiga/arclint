// Package jsonbaseline loads and writes immutable Baseline snapshots
// as JSON. The file is committed and reviewable: every entry carries
// the finding it covers, not just a hash, and output is deterministic
// so regeneration diffs only when findings change. During the
// transition the target engine keeps its own file beside the legacy
// baseline.
package jsonbaseline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wixregiga/arclint/internal/domain/baseline"
)

// Path is the committed baseline location, relative to the repository
// root.
const Path = ".arclint/baseline.v2.json"

const version = 1

type fileDoc struct {
	Version       int          `json:"version"`
	Identity      string       `json:"identity,omitempty"`
	CapturedRules []string     `json:"capturedRules"`
	Findings      []findingDoc `json:"findings"`
}

type findingDoc struct {
	RuleID  string `json:"ruleId"`
	Subject string `json:"subject"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// Store implements the application's BaselineSource and BaselineOutput
// ports over one repository root.
type Store struct {
	root string
}

// NewStore binds the store to a repository root.
func NewStore(root string) (Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Store{}, fmt.Errorf("baseline store root: %w", err)
	}
	return Store{root: abs}, nil
}

func (s Store) filePath() string {
	return filepath.Join(s.root, filepath.FromSlash(Path))
}

// Load reads the committed Baseline. No file means no Baseline; a
// malformed or wrong-version file is a configuration error; a
// corrupted policy file must not silently disable itself.
func (s Store) Load() (baseline.Snapshot, bool, error) {
	data, err := os.ReadFile(s.filePath())
	if os.IsNotExist(err) {
		return baseline.Snapshot{}, false, nil
	}
	if err != nil {
		return baseline.Snapshot{}, false, fmt.Errorf("read %s: %w", Path, err)
	}
	var doc fileDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return baseline.Snapshot{}, false, fmt.Errorf("%s: %v (recapture the baseline)", Path, err)
	}
	if doc.Version != version {
		return baseline.Snapshot{}, false, fmt.Errorf("%s: version %d, this arclint expects %d (recapture the baseline)",
			Path, doc.Version, version)
	}
	entries := make([]baseline.Entry, 0, len(doc.Findings))
	for _, f := range doc.Findings {
		e, err := baseline.NewEntry(f.RuleID, f.Subject, f.Message, f.Count)
		if err != nil {
			return baseline.Snapshot{}, false, fmt.Errorf("%s: %v (recapture the baseline)", Path, err)
		}
		entries = append(entries, e)
	}
	snapshot, err := baseline.New(doc.CapturedRules, entries, doc.Identity)
	if err != nil {
		return baseline.Snapshot{}, false, fmt.Errorf("%s: %v (recapture the baseline)", Path, err)
	}
	return snapshot, true, nil
}

// Write persists a complete immutable Baseline deterministically:
// identical snapshots produce identical bytes.
func (s Store) Write(snapshot baseline.Snapshot) error {
	entries := snapshot.Entries()
	findings := make([]findingDoc, 0, len(entries))
	for _, e := range entries {
		findings = append(findings, findingDoc{
			RuleID:  e.RuleID(),
			Subject: e.Subject(),
			Message: e.Message(),
			Count:   e.Count(),
		})
	}
	doc := fileDoc{
		Version:       version,
		Identity:      snapshot.Identity(),
		CapturedRules: snapshot.CapturedRules(),
		Findings:      findings,
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode baseline: %w", err)
	}
	target := s.filePath()
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}
