package codex

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/agent"
	agentfiles "github.com/wixregiga/arclint/internal/infrastructure/agents/files"
)

//go:embed assets/arclint-domain-reviewer.toml
var reviewerDefinition []byte

const (
	reviewerPath         = ".codex/agents/arclint-domain-reviewer.toml"
	reviewerMetadataPath = ".arclint/reviewer.json"
)

// Reviewer delivers the independent native agent from embedded authored instructions.
type Reviewer struct{ root, version string }

type reviewerMetadata struct {
	Name           string `json:"name"`
	Host           string `json:"host"`
	ArcLintVersion string `json:"arclintVersion"`
}

// NewReviewer binds delivery to a project and the shipping ArcLint release.
func NewReviewer(root, version string) *Reviewer {
	return &Reviewer{root: root, version: strings.TrimSpace(version)}
}

func shippedReviewer() (agent.Agent, error) {
	var definition struct {
		Name         string `toml:"name"`
		Description  string `toml:"description"`
		Instructions string `toml:"developer_instructions"`
		Sandbox      string `toml:"sandbox_mode"`
	}
	if err := toml.Unmarshal(reviewerDefinition, &definition); err != nil {
		return agent.Agent{}, fmt.Errorf("reviewer definition: %w", err)
	}
	if definition.Name != "arclint-domain-reviewer" || definition.Sandbox != "read-only" {
		return agent.Agent{}, fmt.Errorf("reviewer definition must name arclint-domain-reviewer with read-only sandbox")
	}
	value, err := agent.New(definition.Name, definition.Description, definition.Instructions)
	if err != nil {
		return agent.Agent{}, fmt.Errorf("reviewer definition: %w", err)
	}
	return value, nil
}

// InstallReviewer preserves owner edits and changes only reviewer-owned assets.
func (i *Reviewer) InstallReviewer(host agent.Host) ([]string, error) {
	if err := host.Validate(); err != nil {
		return nil, fmt.Errorf("reviewer host: %w", err)
	}
	if i.version == "" {
		return nil, fmt.Errorf("reviewer installation requires an ArcLint release version")
	}
	definition, err := shippedReviewer()
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(i.root)
	if err != nil {
		return nil, fmt.Errorf("reviewer project: %w", err)
	}
	metadata, err := json.MarshalIndent(reviewerMetadata{Name: definition.Name(), Host: host.String(), ArcLintVersion: i.version}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("reviewer metadata: %w", err)
	}
	paths, err := agentfiles.Install(root, map[string][]byte{
		filepath.Join(root, reviewerPath):         reviewerDefinition,
		filepath.Join(root, reviewerMetadataPath): append(metadata, '\n'),
	})
	if err != nil {
		return nil, fmt.Errorf("install reviewer assets: %w", err)
	}
	return paths, nil
}

// ReviewerStatus checks the installed release and asset hashes without claiming discovery.
func (i *Reviewer) ReviewerStatus(host agent.Host) (application.ReviewerInstallation, error) {
	root, err := filepath.Abs(i.root)
	if err != nil {
		return application.ReviewerInstallation{}, fmt.Errorf("reviewer project: %w", err)
	}
	status := application.ReviewerInstallation{Project: root, Path: filepath.Join(root, reviewerPath), Name: "arclint-domain-reviewer", Host: host.String(), AvailableVersion: i.version}
	if err := host.Validate(); err != nil {
		return status, fmt.Errorf("reviewer host: %w", err)
	}
	definition, err := readReviewerAsset(root, reviewerPath)
	if errors.Is(err, os.ErrNotExist) {
		status.Problems = append(status.Problems, "Native agent file is missing")
		return status, nil
	}
	if err != nil {
		return status, fmt.Errorf("read reviewer: %w", err)
	}
	status.Installed = true
	metadataBytes, err := readReviewerAsset(root, reviewerMetadataPath)
	var metadata reviewerMetadata
	if err != nil || json.Unmarshal(metadataBytes, &metadata) != nil || metadata.Name != status.Name || metadata.Host != status.Host || metadata.ArcLintVersion == "" {
		status.Problems = append(status.Problems, "Reviewer release metadata is missing or invalid")
	} else {
		status.InstalledVersion = metadata.ArcLintVersion
	}
	receiptBytes, err := readReviewerAsset(root, ".arclint/agent-assets.json")
	var receipt map[string]string
	if err != nil || json.Unmarshal(receiptBytes, &receipt) != nil {
		status.Problems = append(status.Problems, "Asset receipt is missing or invalid; integrity is unverified")
	} else {
		for _, asset := range []struct {
			path    string
			content []byte
		}{{reviewerPath, definition}, {reviewerMetadataPath, metadataBytes}} {
			sum := sha256.Sum256(asset.content)
			if receipt[asset.path] != hex.EncodeToString(sum[:]) {
				status.Problems = append(status.Problems, "Changed or unrecorded installed asset: "+asset.path)
			}
		}
	}
	status.Intact = len(status.Problems) == 0
	return status, nil
}

func readReviewerAsset(root, relative string) ([]byte, error) {
	target := filepath.Join(root, relative)
	for path := target; path != root; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect reviewer asset: %w", err)
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("reviewer integrity cannot be verified through symlink: %s", path)
		}
	}
	content, err := os.ReadFile(target)
	if err != nil {
		return nil, fmt.Errorf("read reviewer asset: %w", err)
	}
	return content, nil
}
