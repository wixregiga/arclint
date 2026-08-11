package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load parses, schema-validates, and semantically validates a rules.yaml.
func Load(path string) (*RuleSet, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("cannot read rules file: %w", err)
	}
	return Parse(data, abs)
}

// Parse validates rules.yaml bytes through all three layers: YAML syntax,
// JSON Schema, and semantics (module references, regex compilation, expr
// type-checking).
func Parse(data []byte, absPath string) (*RuleSet, error) {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: invalid YAML: %w", absPath, err)
	}
	if raw == nil {
		return nil, fmt.Errorf("%s: empty rules file", absPath)
	}
	if err := validateAgainstSchema(raw); err != nil {
		return nil, fmt.Errorf("%s: %w", absPath, err)
	}
	rs, err := decodeStrict(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", absPath, err)
	}
	if err := rs.validateSemantics(); err != nil {
		return nil, fmt.Errorf("%s: %w", absPath, err)
	}
	finishRuleSet(rs, data, absPath)
	return rs, nil
}

// ParseTrusted decodes rules.yaml bytes that a fingerprint-matched cache
// entry proves were fully validated before. It skips schema and semantic
// validation; YAML syntax is still enforced by the decode itself.
func ParseTrusted(data []byte, absPath string) (*RuleSet, error) {
	rs, err := decodeStrict(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", absPath, err)
	}
	finishRuleSet(rs, data, absPath)
	return rs, nil
}

func decodeStrict(data []byte) (*RuleSet, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var rs RuleSet
	if err := dec.Decode(&rs); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	return &rs, nil
}

func finishRuleSet(rs *RuleSet, data []byte, absPath string) {
	sum := sha256.Sum256(data)
	rs.SHA256 = hex.EncodeToString(sum[:])
	rs.Path = absPath
	rs.Root = filepath.Dir(absPath)
}
