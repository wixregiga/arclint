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

	"github.com/wixregiga/arclint/internal/application"
)

// HostInspector supplies host-owned registration evidence to the asset inspector.
// It is an infrastructure seam; the application receives the resulting status.
type HostInspector interface {
	InspectHostInstallation() (application.AgentHostInstallation, error)
}

// Writer publishes generated setup artifacts under one project root.
type Writer struct {
	HostInspectors []HostInspector
	Root           string
	PreserveSkills bool
	CheckOnly      bool
}

func digest(content []byte) string { sum := sha256.Sum256(content); return hex.EncodeToString(sum[:]) }

// Original draft installations predate the asset receipt.
var legacy = map[string]string{
	".codex/hooks/arclint-domain-guard/guard.py":     "acbdbd762f84d56c7d4a8dcc72d818e00e9c991f7469d6b8db66d3da29a8ac47",
	".omp/extensions/arclint-domain-guard/guard.mjs": "1de998554c584f0dcf701f8326bf396025fe4008cd25937bf344c2e3ccd123be",
	".agents/skills/domain-librarian/SKILL.md":       "48ccddde71e85a854c8965b3be99be2d796893bca14d020a745d8c1f33776490",
}

// Write implements the generated artifact port using the same ownership checks as hooks.
func (w Writer) Write(dir, name string, content []byte) (bool, string, error) {
	target := filepath.Join(w.Root, dir, name)
	if err := safePath(w.Root, target); err != nil {
		return false, target, err
	}
	before, readErr := os.ReadFile(target)
	if w.PreserveSkills && readErr == nil && (name == "SKILL.md" || name == "VOCAB.yaml") && !bytes.Equal(before, content) {
		rel, _ := filepath.Rel(w.Root, target)
		var receipt map[string]string
		data, _ := os.ReadFile(filepath.Join(w.Root, ".arclint/agent-assets.json"))
		_ = json.Unmarshal(data, &receipt)
		if receipt[filepath.ToSlash(rel)] != digest(before) && legacy[filepath.ToSlash(rel)] != digest(before) {
			return false, target, nil
		}
	}
	_, err := install(w.Root, map[string][]byte{target: content}, w.CheckOnly)
	return !bytes.Equal(before, content), target, err
}

// Install preflights every path, preserves changed assets and records installed hashes.
// Merge paths must have been validated and merged by their format-specific adapter.
func Install(root string, files map[string][]byte, mergePaths ...string) ([]string, error) {
	return install(root, files, false, mergePaths...)
}

// Preflight checks the same paths and ownership as Install without writes.
func Preflight(root string, files map[string][]byte, mergePaths ...string) error {
	_, err := install(root, files, true, mergePaths...)
	return err
}

func install(root string, files map[string][]byte, checkOnly bool, mergePaths ...string) ([]string, error) {
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
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("agent setup: %w", err)
	}
	merge := map[string]bool{}
	for _, p := range mergePaths {
		merge[p] = true
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
		if err == nil && !bytes.Equal(before, content) && !merge[target] && receipt[rel] != digest(before) && legacy[rel] != digest(before) {
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
	if checkOnly {
		return targets, nil
	}
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

// CheckPath rejects escaping and symlinked managed output paths.
func CheckPath(root, target string) error { return safePath(root, target) }

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
