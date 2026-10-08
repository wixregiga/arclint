// Package agentfiles writes ArcLint-owned integration assets without replacing user edits.
package agentfiles

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func digest(content []byte) string { sum := sha256.Sum256(content); return hex.EncodeToString(sum[:]) }

// Install preflights every path, preserves changed assets and records installed hashes.
func Install(root string, files map[string][]byte) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	receiptPath := filepath.Join(root, ".arclint/agent-assets.json")
	if err := safePath(root, receiptPath); err != nil {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	receipt := map[string]string{}
	receiptBefore, err := os.ReadFile(receiptPath)
	if err == nil {
		if err := json.Unmarshal(receiptBefore, &receipt); err != nil {
			return nil, fmt.Errorf("invalid agent asset receipt: %w", err)
		}
		if receipt == nil {
			return nil, fmt.Errorf("invalid agent asset receipt: expected an object, got null")
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	previous := map[string][]byte{receiptPath: receiptBefore}
	targets := make([]string, 0, len(files))
	for target, content := range files {
		if err := safePath(root, target); err != nil {
			return nil, fmt.Errorf("agent setup: %w", err)
		}
		rel, _ := filepath.Rel(root, target)
		rel = filepath.ToSlash(rel)
		before, err := os.ReadFile(target)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("agent setup: %w", err)
		}
		previous[target] = before
		if err == nil && !bytes.Equal(before, content) && receipt[rel] != digest(before) {
			return nil, fmt.Errorf("preserving existing different file: %s; review local edits before updating", target)
		}
		receipt[rel] = digest(content)
		targets = append(targets, target)
	}
	sort.Strings(targets)
	receiptContent, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	// Use a private map; the caller's planned artifacts stay unchanged.
	writes := make(map[string][]byte, len(files)+1)
	for target, content := range files {
		writes[target] = content
	}
	writes[receiptPath] = append(receiptContent, '\n')
	for _, target := range append(append([]string{}, targets...), receiptPath) {
		before, err := os.ReadFile(target)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("agent setup: %w", err)
		}
		if !bytes.Equal(before, previous[target]) {
			return nil, fmt.Errorf("install target changed concurrently: %s", target)
		}
		if bytes.Equal(before, writes[target]) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return nil, fmt.Errorf("agent setup: %w", err)
		}
		f, err := os.CreateTemp(filepath.Dir(target), ".arclint-install-*")
		if err != nil {
			return nil, fmt.Errorf("agent setup: %w", err)
		}
		name := f.Name()
		_, writeErr := f.Write(writes[target])
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			_ = os.Remove(name)
			return nil, fmt.Errorf("write %s: %v %v", target, writeErr, closeErr)
		}
		if err := os.Rename(name, target); err != nil {
			_ = os.Remove(name)
			return nil, fmt.Errorf("agent setup: %w", err)
		}
	}
	return targets, nil
}

// safePath rejects escaping and symlinked managed output paths.
func safePath(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("install path escapes project: %s", target)
	}
	for p := target; p != root; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlinked install path: %s", p)
		}
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect install path: %w", err)
		}
	}
	return nil
}
