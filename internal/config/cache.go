package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Cache records, per repo root, that a rules.yaml with a given fingerprint
// passed full validation under a given arclint version. `arclint load`
// writes it; later commands skip schema and semantic validation on a
// fingerprint hit. Compiled artifacts are not cached in M1: regex and expr
// compilation is microseconds, and serialized programs would dwarf the
// win. The M2 extension pipeline extends this file with transpile results,
// where caching pays.
type cacheEntry struct {
	Version        int       `json:"version"`
	ArclintVersion string    `json:"arclintVersion"`
	RulesPath      string    `json:"rulesPath"`
	RulesSHA256    string    `json:"rulesSha256"`
	ValidatedAt    time.Time `json:"validatedAt"`
}

const cacheVersion = 1

func cachePath(root string) string {
	return filepath.Join(root, ".arclint", "cache.json")
}

// WriteCache records a successful validation.
func WriteCache(rs *RuleSet, arclintVersion string) error {
	entry := cacheEntry{
		Version:        cacheVersion,
		ArclintVersion: arclintVersion,
		RulesPath:      rs.Path,
		RulesSHA256:    rs.SHA256,
		ValidatedAt:    time.Now().UTC(),
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(cachePath(rs.Root))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(cachePath(rs.Root), append(data, '\n'), 0o644)
}

// LoadCached loads a rules.yaml, skipping re-validation when the cache
// fingerprint matches. The boolean reports a cache hit.
func LoadCached(path, arclintVersion string) (*RuleSet, bool, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, false, err
	}
	root := filepath.Dir(abs)

	var entry cacheEntry
	if raw, err := os.ReadFile(cachePath(root)); err == nil {
		if json.Unmarshal(raw, &entry) == nil &&
			entry.Version == cacheVersion &&
			entry.ArclintVersion == arclintVersion &&
			entry.RulesPath == abs {
			rs, err := ParseTrusted(data, abs)
			if err == nil && rs.SHA256 == entry.RulesSHA256 {
				return rs, true, nil
			}
		}
	}
	rs, err := Parse(data, abs)
	if err != nil {
		return nil, false, err
	}
	return rs, false, nil
}
