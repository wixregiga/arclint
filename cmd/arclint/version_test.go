package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestFormatVersion(t *testing.T) {
	cases := []struct {
		version, revision, timestamp string
		dirty                        bool
		want                         string
	}{
		{"0.1.0", "", "", false, "0.1.0"},
		{"0.1.0", "eef3f139d61bf4a8", "2026-08-13T21:49:25Z", false, "0.1.0 (eef3f13, 2026-08-13)"},
		{"0.1.0", "eef3f139d61bf4a8", "2026-08-13T21:49:25Z", true, "0.1.0 (eef3f13, dirty, 2026-08-13)"},
		{"0.1.0", "abc1234", "", true, "0.1.0 (abc1234, dirty)"},
	}
	for _, c := range cases {
		got := formatVersion(c.version, c.revision, c.timestamp, c.dirty)
		if got != c.want {
			t.Errorf("formatVersion(%q, %q, %q, %v) = %q, want %q",
				c.version, c.revision, c.timestamp, c.dirty, got, c.want)
		}
	}
}

// TestVersionOutput proves the built binary reports the product version
// plus the stamped commit hash: builds must be distinguishable while
// the product version stays put.
func TestVersionOutput(t *testing.T) {
	stdout, stderr, code := runBin(t, t.TempDir(), os.Environ(), "--version")
	if code != 0 {
		t.Fatalf("--version: exit %d, stderr %s", code, stderr)
	}
	// The e2e binary builds inside the repo, so VCS stamping applies.
	re := regexp.MustCompile(`arclint version \d+\.\d+\.\d+ \([0-9a-f]{7}(, dirty)?, \d{4}-\d{2}-\d{2}\)`)
	if !re.MatchString(stdout) {
		t.Errorf("--version output %q does not match %s", strings.TrimSpace(stdout), re)
	}
}

// TestVersionSingleSource guards the three hand-set version places until
// a git tag becomes the single source: main.go's default, the Makefile
// VERSION, and the docs site config must agree, so a bump cannot go out
// half-applied.
func TestVersionSingleSource(t *testing.T) {
	makefile, err := os.ReadFile("../../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	mkRe := regexp.MustCompile(`(?m)^VERSION \?= (\S+)$`)
	mk := mkRe.FindSubmatch(makefile)
	if mk == nil {
		t.Fatal("Makefile: no `VERSION ?=` line")
	}
	if got := string(mk[1]); got != version {
		t.Errorf("Makefile VERSION = %q, main.go version = %q", got, version)
	}

	docsConfig, err := os.ReadFile("../../docs/site/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	tomlRe := regexp.MustCompile(`(?m)^version = "([^"]+)"$`)
	tm := tomlRe.FindSubmatch(docsConfig)
	if tm == nil {
		t.Fatal("docs/site/config.toml: no version line")
	}
	if got := string(tm[1]); got != version {
		t.Errorf("docs config version = %q, main.go version = %q", got, version)
	}
}
