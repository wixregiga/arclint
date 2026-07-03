package answers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlug(t *testing.T) {
	// The readable prefix is the destination with "/" -> "-"; a sha suffix
	// keeps it unique. Assert on the prefix and determinism, not the digest.
	prefixes := map[string]string{
		"services/payment-gateway":   "services-payment-gateway-",
		"docs/pages/getting-started": "docs-pages-getting-started-",
		"single":                     "single-",
		"/leading/and/trailing/":     "leading-and-trailing-",
	}
	for in, wantPrefix := range prefixes {
		got := Slug(in)
		if !strings.HasPrefix(got, wantPrefix) {
			t.Errorf("Slug(%q) = %q, want prefix %q", in, got, wantPrefix)
		}
		if got != Slug(in) {
			t.Errorf("Slug(%q) not deterministic", in)
		}
	}
}

// TestSlugNoCollision pins the blocker-1 fix: two distinct destinations that
// flatten to the same "/"->"-" form must not share a shard file.
func TestSlugNoCollision(t *testing.T) {
	a := Slug("services/a-b")
	b := Slug("services/a/b")
	if a == b {
		t.Fatalf("Slug collision: services/a-b and services/a/b both -> %q", a)
	}
}

// TestSaveRefusesForeignShard pins the Save guard: writing a unit whose slug
// path already holds a different destination is refused, not clobbered.
func TestSaveRefusesForeignShard(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	// Hand-plant a shard at the slug path for dest A but recording dest B.
	path := Path(root, "services/a-b")
	raw := "version: 1\ntemplate: svc\ntemplate_version: 1\ndestination: services/other\nanswers: {}\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Save(root, &Unit{Template: "svc", Destination: "services/a-b", Answers: map[string]string{}})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("want refusal, got %v", err)
	}
	// Same destination must still save fine (overwrite is allowed).
	same := "version: 1\ntemplate: svc\ntemplate_version: 1\ndestination: services/a-b\nanswers: {}\n"
	if err := os.WriteFile(path, []byte(same), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(root, &Unit{Template: "svc", Destination: "services/a-b", Answers: map[string]string{}}); err != nil {
		t.Fatalf("same-destination overwrite should succeed, got %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	u := &Unit{
		Version:         CurrentVersion,
		Template:        "service",
		TemplateVersion: 3,
		Destination:     "services/pay-gw",
		GeneratedAt:     "2026-01-01T00:00:00Z",
		Answers: map[string]string{
			"name":    "pay gw",
			"with_db": "true",
			"port":    "8080",
		},
		Files: map[string]string{"main.go": "abc123"},
	}
	if err := Save(root, u); err != nil {
		t.Fatal(err)
	}
	path := Path(root, u.Destination)
	if !strings.HasPrefix(filepath.Base(path), "services-pay-gw-") || !strings.HasSuffix(path, ".yaml") {
		t.Fatalf("shard path = %s", path)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Template != "service" || got.TemplateVersion != 3 || got.Destination != "services/pay-gw" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	for k, want := range u.Answers {
		if got.Answers[k] != want {
			t.Errorf("answer %s = %q, want %q", k, got.Answers[k], want)
		}
	}
	if got.Files["main.go"] != "abc123" {
		t.Errorf("files hash lost: %v", got.Files)
	}
}

func TestLoadHandEditedScalars(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := "version: 1\ntemplate: svc\ntemplate_version: 1\ndestination: s/x\nanswers:\n  with_db: true\n  port: 8080\n"
	path := filepath.Join(Dir(root), "s-x.yaml")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	u, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if u.Answers["with_db"] != "true" || u.Answers["port"] != "8080" {
		t.Fatalf("scalar normalization failed: %v", u.Answers)
	}
}

func TestLoadCorrupt(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(Dir(root), "bad.yaml")
	if err := os.WriteFile(path, []byte("answers: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "corrupt answers file") {
		t.Fatalf("want corrupt error, got %v", err)
	}
}

func TestListSortedAndMissingDir(t *testing.T) {
	root := t.TempDir()
	units, err := List(root)
	if err != nil || len(units) != 0 {
		t.Fatalf("List on missing dir = %v, %v", units, err)
	}
	for _, dest := range []string{"z/last", "a/first", "m/middle"} {
		if err := Save(root, &Unit{Template: "svc", Destination: dest, Answers: map[string]string{}}); err != nil {
			t.Fatal(err)
		}
	}
	units, err = List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 3 || units[0].Destination != "a/first" || units[2].Destination != "z/last" {
		t.Fatalf("List order wrong: %v", units)
	}
}
