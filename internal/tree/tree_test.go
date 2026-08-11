package tree_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/tree"
)

func write(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, p := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func paths(tr *tree.Tree) []string {
	var out []string
	for _, f := range tr.Files {
		out = append(out, f.Path)
	}
	return out
}

func TestWalkExclusions(t *testing.T) {
	root := t.TempDir()
	write(t, root,
		"a.go",
		"src/b.go",
		".git/objects/x",
		".arclint/cache.json",
		"vendor/dep/dep.go",
		"node_modules/pkg/index.js",
		"testdata/fixture.go",
		"src/testdata/f.go",
	)
	tr, err := tree.Walk(root, tree.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.go", "src/b.go"}
	if !reflect.DeepEqual(paths(tr), want) {
		t.Errorf("got %v, want %v", paths(tr), want)
	}

	tr, err = tree.Walk(root, tree.Options{IncludeTestdata: true})
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"a.go", "src/b.go", "src/testdata/f.go", "testdata/fixture.go"}
	if !reflect.DeepEqual(paths(tr), want) {
		t.Errorf("with testdata: got %v, want %v", paths(tr), want)
	}
}

func TestWalkCustomExcludes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "keep.go", "gen/out.go", "deep/gen/out.go", "note.md")
	tr, err := tree.Walk(root, tree.Options{Exclude: []string{"**/gen", "*.md"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"keep.go"}
	if !reflect.DeepEqual(paths(tr), want) {
		t.Errorf("got %v, want %v", paths(tr), want)
	}
}

func TestWalkSymlinksSkipped(t *testing.T) {
	root := t.TempDir()
	write(t, root, "real/target.go")
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	tr, err := tree.Walk(root, tree.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"real/target.go"}
	if !reflect.DeepEqual(paths(tr), want) {
		t.Errorf("got %v, want %v", paths(tr), want)
	}
}

func TestWalkDeterministic(t *testing.T) {
	root := t.TempDir()
	write(t, root, "z/x.go", "a/b.go", "a/c.go", "m/n/o.go", "top.go")
	first, err := tree.Walk(root, tree.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		again, err := tree.Walk(root, tree.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(paths(first), paths(again)) {
			t.Fatalf("walk %d differs: %v vs %v", i, paths(first), paths(again))
		}
	}
}

func TestInvalidExcludePattern(t *testing.T) {
	root := t.TempDir()
	if _, err := tree.Walk(root, tree.Options{Exclude: []string{"[unclosed"}}); err == nil {
		t.Error("invalid exclude pattern accepted")
	}
}
