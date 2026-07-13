package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"

	"github.com/wixregiga/arclint/internal/config"
)

// Baseline file format (rules.md §9.2, referenced from the config's
// `baseline:` key, conventionally .arclint/baseline.json or
// .arclint/baseline.yaml). The file is a flat array of entries; JSON is a
// YAML subset, so both spellings parse identically:
//
//	- ruleId: no-utils-dir
//	  path: pkg/utils/strings.go
//	  messageHash: a1b2c3d4e5f60718
//
// messageHash is the first 16 hex characters of the SHA-256 of the
// violation message. Line numbers are deliberately excluded because they
// drift; matching is by {ruleId, path, messageHash}. A violation matching
// an entry is suppressed and never affects the exit code. A configured but
// missing baseline file is not an error — a fresh repo simply has no
// grandfathered violations yet.

// BaselineEntry is one grandfathered violation.
type BaselineEntry struct {
	RuleID      string `yaml:"ruleId" json:"ruleId"`
	Path        string `yaml:"path" json:"path"`
	MessageHash string `yaml:"messageHash" json:"messageHash"`
}

// MessageHash returns the sha256-hex-16 baseline hash of a violation
// message.
func MessageHash(msg string) string {
	sum := sha256.Sum256([]byte(msg))
	return hex.EncodeToString(sum[:])[:16]
}

// baselineSet is the loaded baseline, keyed for O(1) suppression checks.
type baselineSet map[string]struct{}

func baselineKey(ruleID, path, hash string) string {
	return ruleID + "\x00" + path + "\x00" + hash
}

func (s baselineSet) has(v Violation) bool {
	if s == nil {
		return false
	}
	_, ok := s[baselineKey(v.RuleID, v.Path, MessageHash(v.Message))]
	return ok
}

// loadBaseline reads the baseline file named by the config, if any exists.
func loadBaseline(cfg *config.File, root string) (baselineSet, error) {
	if cfg.Baseline == "" {
		return nil, nil
	}
	full := filepath.Join(root, filepath.FromSlash(cfg.Baseline))
	data, err := os.ReadFile(full)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read baseline %s — %v", cfg.Baseline, err)
	}
	var entries []BaselineEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("baseline %s is not a valid entry array — %v", cfg.Baseline, err)
	}
	set := make(baselineSet, len(entries))
	for _, e := range entries {
		set[baselineKey(e.RuleID, e.Path, e.MessageHash)] = struct{}{}
	}
	return set, nil
}
