package agentfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/wixregiga/arclint/internal/application"
)

// Status describes installed artifacts; only the host can report current activation and trust.
func (w Writer) Status() (application.AgentInstallation, error) {
	root, err := filepath.Abs(w.Root)
	if err != nil {
		return application.AgentInstallation{}, fmt.Errorf("inspect agent installation path: %w", err)
	}
	configPath := ".arclint/domain-guard.json"
	if err := safePath(root, filepath.Join(root, configPath)); err != nil {
		return application.AgentInstallation{}, fmt.Errorf("inspect agent installation path: %w", err)
	}
	content, err := os.ReadFile(filepath.Join(root, configPath))
	if err != nil {
		return application.AgentInstallation{}, fmt.Errorf("agent setup not readable: %w", err)
	}
	var config struct{ DomainFiles, DomainSourcePatterns, SourcePatterns []string }
	if err := json.Unmarshal(content, &config); err != nil {
		return application.AgentInstallation{}, fmt.Errorf("agent status: %w", err)
	}
	status := application.AgentInstallation{Project: root, DomainFiles: config.DomainFiles, DomainSourcePatterns: config.DomainSourcePatterns, SourcePatterns: config.SourcePatterns, InstalledHosts: []string{}, ChangedAssets: []string{}, Problems: []string{}, Intact: true}
	if _, err := os.Stat(filepath.Join(root, "rules.arclint.yaml")); os.IsNotExist(err) {
		if _, err := os.Stat(filepath.Join(root, ".arclint/rules.yaml")); err == nil {
			status.LegacyRules = true
		}
	}
	receiptPath := ".arclint/agent-assets.json"
	receipt := map[string]string{}
	changed := map[string]bool{}
	mark := func(path string) { changed[path] = true; status.Intact = false }
	if err := safePath(root, filepath.Join(root, receiptPath)); err != nil {
		mark(receiptPath)
		status.Problems = append(status.Problems, "Asset receipt is unsafe; integrity unverified: "+err.Error())
	} else if data, err := os.ReadFile(filepath.Join(root, receiptPath)); err != nil {
		mark(receiptPath)
		status.Problems = append(status.Problems, "Asset receipt unavailable; integrity unverified: "+err.Error())
	} else if err := json.Unmarshal(data, &receipt); err != nil {
		mark(receiptPath)
		status.Problems = append(status.Problems, "Asset receipt invalid; integrity unverified: "+err.Error())
	} else if len(receipt) == 0 {
		mark(receiptPath)
		status.Problems = append(status.Problems, "Asset receipt empty; integrity unverified")
	}
	verify := func(path string) {
		target := filepath.Join(root, path)
		if err := safePath(root, target); err != nil {
			mark(path)
			return
		}
		data, err := os.ReadFile(target)
		if err != nil || receipt[path] == "" || digest(data) != receipt[path] {
			mark(path)
			return
		}
	}
	verify(configPath)
	for path := range receipt {
		verify(path)
	}
	if len(w.HostInspectors) == 0 {
		status.Intact = false
		status.Problems = append(status.Problems, "Host registration inspection unavailable; integrity unverified")
	}
	foundHost := false
	for _, inspector := range w.HostInspectors {
		host, err := inspector.InspectHostInstallation()
		if err != nil {
			status.Intact = false
			status.Problems = append(status.Problems, err.Error())
			continue
		}
		applies := host.Present || host.Registered
		for _, path := range host.Paths {
			if _, found := receipt[path]; found {
				applies = true
			}
		}
		if !applies {
			continue
		}
		foundHost = true
		status.Problems = append(status.Problems, host.Problems...)
		if len(host.Problems) > 0 {
			status.Intact = false
		}
		present := host.Registered
		for _, path := range host.Paths {
			verify(path)
			target := filepath.Join(root, path)
			if err := safePath(root, target); err != nil {
				present = false
			} else if _, err := os.ReadFile(target); err != nil {
				present = false
			}
		}
		if !host.Registered {
			status.Intact = false
		}
		if present {
			status.InstalledHosts = append(status.InstalledHosts, host.Name)
		}
	}
	if !foundHost {
		status.Intact = false
		status.Problems = append(status.Problems, "No complete ArcLint host registration was identified; integrity unverified")
	}
	for path := range changed {
		status.ChangedAssets = append(status.ChangedAssets, path)
	}
	sort.Strings(status.InstalledHosts)
	sort.Strings(status.ChangedAssets)
	sort.Strings(status.Problems)
	return status, nil
}
