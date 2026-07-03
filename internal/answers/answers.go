// Package answers implements the sharded saved-answers store: one YAML file
// per generated unit at .arclint/answers/<unit-path-slug>.yaml
// (docs/design/templating.md §4). The files are committed to the repo; they
// are the durable link between generated code and the template that made it.
package answers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// CurrentVersion is the answers file schema version written by this build.
const CurrentVersion = 1

// Unit is one generated unit's record. Answers are stored post-resolution
// (defaults and when applied); variables skipped by when are absent, not
// empty. Files maps each rendered file (relative to Destination, slash
// separated) to the sha256 hex of its content at generation/apply time — the
// baseline that lets `arclint make` tell user edits from template changes.
type Unit struct {
	Version         int
	Template        string
	TemplateVersion int
	Destination     string // slash-separated, relative to repo root
	GeneratedAt     string // RFC3339 UTC
	Answers         map[string]string
	Files           map[string]string
}

// diskUnit is the on-disk YAML shape. Answers values are `any` so
// hand-written scalars (true, 8080) load without type errors.
type diskUnit struct {
	Version         int               `yaml:"version"`
	Template        string            `yaml:"template"`
	TemplateVersion int               `yaml:"template_version"`
	Destination     string            `yaml:"destination"`
	GeneratedAt     string            `yaml:"generated_at,omitempty"`
	Answers         map[string]any    `yaml:"answers"`
	Files           map[string]string `yaml:"files,omitempty"`
}

// Slug converts a unit destination path to its shard file stem. The readable
// part is the destination with "/" replaced by "-", but that mapping is
// lossy: "services/a-b" and "services/a/b" both flatten to
// "services-a-b" and would clobber each other's shard (silent answer loss).
// So an 8-char sha256 prefix of the *exact* destination is appended, making
// every destination map to a unique stem:
// "services/payment-gateway" -> "services-payment-gateway-1a2b3c4d".
// The suffix is disambiguation only; the destination inside the shard is the
// source of truth (make finds shards via List, never by reversing the slug).
func Slug(destination string) string {
	clean := strings.Trim(filepath.ToSlash(destination), "/")
	readable := strings.ReplaceAll(clean, "/", "-")
	sum := sha256.Sum256([]byte(clean))
	return readable + "-" + hex.EncodeToString(sum[:])[:8]
}

// Dir returns the answers directory under the repo root.
func Dir(repoRoot string) string {
	return filepath.Join(repoRoot, ".arclint", "answers")
}

// Path returns the shard path for a destination.
func Path(repoRoot, destination string) string {
	return filepath.Join(Dir(repoRoot), Slug(destination)+".yaml")
}

// Save writes (or overwrites) the unit's shard.
func Save(repoRoot string, u *Unit) error {
	if u.Template == "" || u.Destination == "" {
		return fmt.Errorf("answers unit needs template and destination — refusing to write an unusable shard")
	}
	d := diskUnit{
		Version:         u.Version,
		Template:        u.Template,
		TemplateVersion: u.TemplateVersion,
		Destination:     u.Destination,
		GeneratedAt:     u.GeneratedAt,
		Answers:         map[string]any{},
		Files:           u.Files,
	}
	if d.Version == 0 {
		d.Version = CurrentVersion
	}
	for k, v := range u.Answers {
		d.Answers[k] = v
	}
	data, err := yaml.Marshal(d)
	if err != nil {
		return fmt.Errorf("cannot encode answers for %s — %w", u.Destination, err)
	}
	if err := os.MkdirAll(Dir(repoRoot), 0o755); err != nil {
		return fmt.Errorf("cannot create %s — %w", Dir(repoRoot), err)
	}
	path := Path(repoRoot, u.Destination)
	// Refuse to clobber a shard that already holds a different unit. With the
	// sha-suffixed slug this only fires on a genuine sha256 prefix collision,
	// but it turns silent answer loss into a loud, recoverable error.
	if existing, err := Load(path); err == nil {
		if want, got := strings.Trim(filepath.ToSlash(u.Destination), "/"), strings.Trim(filepath.ToSlash(existing.Destination), "/"); want != got {
			return fmt.Errorf("shard %s already records destination %q — refusing to overwrite it with %q; delete the stale shard first", path, got, want)
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("cannot write %s — %w", path, err)
	}
	return nil
}

// Load reads one shard. Scalar answer values of any YAML type are normalized
// to strings (true -> "true", 8080 -> "8080").
func Load(path string) (*Unit, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s — %w", path, err)
	}
	var d diskUnit
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("corrupt answers file %s — %s", path, yaml.FormatError(err, false, false))
	}
	if d.Template == "" || d.Destination == "" {
		return nil, fmt.Errorf("corrupt answers file %s — template and destination are required; fix or delete the file", path)
	}
	u := &Unit{
		Version:         d.Version,
		Template:        d.Template,
		TemplateVersion: d.TemplateVersion,
		Destination:     strings.Trim(filepath.ToSlash(d.Destination), "/"),
		GeneratedAt:     d.GeneratedAt,
		Answers:         map[string]string{},
		Files:           d.Files,
	}
	for k, v := range d.Answers {
		u.Answers[k] = fmt.Sprint(v)
	}
	return u, nil
}

// List loads every shard under .arclint/answers/, sorted by destination.
// A missing directory is an empty list, not an error.
func List(repoRoot string) ([]*Unit, error) {
	entries, err := os.ReadDir(Dir(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot list %s — %w", Dir(repoRoot), err)
	}
	var units []*Unit
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		u, err := Load(filepath.Join(Dir(repoRoot), e.Name()))
		if err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	sort.Slice(units, func(i, j int) bool { return units[i].Destination < units[j].Destination })
	return units, nil
}
