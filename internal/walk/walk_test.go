package walk

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWalkFilesExcludes(t *testing.T) {
	root := t.TempDir()
	files := []string{
		"a.go",
		"sub/b.txt",
		"node_modules/x.js",   // default exclude (dir basename)
		".git/config",         // default exclude
		"vendor/dep/y.go",     // default exclude
		"dist/bundle.js",      // user exclude via glob
		"pkg/testdata/f.yaml", // user exclude via **/testdata/**
		"keep/testdata.go",    // must NOT be excluded: not a testdata dir
	}
	for _, f := range files {
		path := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := WalkFiles([]string{root}, []string{"dist/**", "**/testdata/**"})
	if err != nil {
		t.Fatalf("WalkFiles: %v", err)
	}

	want := []string{
		filepath.Join(root, "a.go"),
		filepath.Join(root, "keep", "testdata.go"),
		filepath.Join(root, "sub", "b.txt"),
	}
	if !slices.Equal(got, want) {
		t.Errorf("WalkFiles = %v, want %v", got, want)
	}
}

func TestWalkFilesDeduplicatesOverlappingRoots(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := WalkFiles([]string{root, root}, nil)
	if err != nil {
		t.Fatalf("WalkFiles: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("WalkFiles returned %d entries for overlapping roots, want 1: %v", len(got), got)
	}
}

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"**/*.go", "internal/config/config.go", true},
		{"**/utils/**", "pkg/utils/strings.go", true},
		{"dist/**", "dist/js/app.js", true},
		{"dist/**", "src/dist.go", false},
		{"*.go", "sub/a.go", false},
		{"[invalid", "anything", false}, // invalid pattern: no match, no panic
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.path); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
