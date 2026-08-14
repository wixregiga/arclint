package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wixregiga/arclint/internal/report"
)

// The baseline is adopted debt: findings that existed when a repository
// adopted arclint, recorded so check reports only NEW breaks. It is a
// committed, reviewable file — every entry carries the finding it
// covers, not just a hash — and it contains no timestamps, so
// regeneration diffs only when findings change (determinism, M9).
// Entries key on report.Fingerprint (rule, path, message — line moves
// do not reopen findings) and carry a count, so two identical findings
// in one file baseline independently.

// BaselinePath is the committed baseline location, relative to the
// repo root.
const BaselinePath = ".arclint/baseline.json"

const baselineVersion = 1

type baselineEntry struct {
	RuleID  string `json:"ruleId"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type baselineFile struct {
	Version  int                      `json:"version"`
	Findings map[string]baselineEntry `json:"findings"`
}

// loadBaseline reads the committed baseline under root. No file means
// no baseline (nil, nil); a malformed or wrong-version file is a
// configuration error — a corrupted policy file must not silently
// disable itself.
func loadBaseline(root string) (map[string]baselineEntry, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(BaselinePath)))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var f baselineFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %v (regenerate with `arclint baseline`)", BaselinePath, err)
	}
	if f.Version != baselineVersion {
		return nil, fmt.Errorf("%s: version %d, this arclint expects %d (regenerate with `arclint baseline`)",
			BaselinePath, f.Version, baselineVersion)
	}
	if f.Findings == nil {
		f.Findings = map[string]baselineEntry{}
	}
	return f.Findings, nil
}

// applyBaseline splits findings covered by the baseline away from the
// kept set, consuming each entry's count so extra occurrences of an
// identical finding stay reported. stale is the number of adopted
// findings that no longer occur — the signal to refresh the file.
func applyBaseline(entries map[string]baselineEntry, vs []report.Violation) (kept, baselined []report.Violation, stale int) {
	remaining := make(map[string]int, len(entries))
	for fp, e := range entries {
		remaining[fp] = e.Count
	}
	kept = make([]report.Violation, 0, len(vs))
	for _, v := range vs {
		fp := report.Fingerprint(v)
		if remaining[fp] > 0 {
			remaining[fp]--
			v.Baselined = true
			baselined = append(baselined, v)
			continue
		}
		kept = append(kept, v)
	}
	for _, n := range remaining {
		stale += n
	}
	return kept, baselined, stale
}

// WriteBaseline records the given findings as the committed baseline
// and returns the file path. Output is deterministic: map keys marshal
// sorted, and identical inputs produce identical bytes.
func WriteBaseline(root string, vs []report.Violation) (string, error) {
	findings := map[string]baselineEntry{}
	for _, v := range vs {
		fp := report.Fingerprint(v)
		e, ok := findings[fp]
		if !ok {
			e = baselineEntry{RuleID: v.RuleID, Path: v.Path, Message: v.Message}
		}
		e.Count++
		findings[fp] = e
	}
	data, err := json.MarshalIndent(baselineFile{Version: baselineVersion, Findings: findings}, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(BaselinePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
