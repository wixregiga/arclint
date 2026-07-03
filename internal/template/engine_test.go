package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree writes files (slash-relative path -> content) under root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiscoverAndLoad(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		".arclint/templates/svc/template.yaml":  "version: 1\ndestination: \"s/{{ name }}\"\nvariables: [{name: name, description: d, type: string}]",
		".arclint/templates/svc/files/a.txt":    "hi",
		".arclint/templates/notpl/readme.md":    "no manifest here",
		".arclint/templates/stray.txt":          "not a dir",
		".arclint/templates/docs/template.yaml": "version: 1\ndestination: \"d/{{ name }}\"\nvariables: [{name: name, description: d, type: string}]",
	})
	things, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(things) != 2 || things[0] != "docs" || things[1] != "svc" {
		t.Fatalf("Discover = %v, want [docs svc]", things)
	}
	if _, err := Load(root, "svc"); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, "../evil"); err == nil {
		t.Error("Load must reject path-y names")
	}

	empty := t.TempDir()
	things, err = Discover(empty)
	if err != nil || len(things) != 0 {
		t.Fatalf("Discover on repo without templates = %v, %v", things, err)
	}
}

func TestRenderUnitPathsContentAndBinary(t *testing.T) {
	root := t.TempDir()
	binary := "PNG\x00binary {{ name }} not interpolated"
	writeTree(t, root, map[string]string{
		".arclint/templates/svc/template.yaml":                        "version: 1\ndestination: \"services/{{ name | kebab }}\"\nvariables: [{name: name, description: d, type: string}]",
		".arclint/templates/svc/files/cmd/{{ name | kebab }}/main.go": "package main // {{ name | pascal }} in {{ repo_name }}\n",
		".arclint/templates/svc/files/assets/{{ name | kebab }}.bin":  binary,
		".arclint/templates/svc/files/migrations/.gitkeep":            "",
	})
	tpl, err := Load(root, "svc")
	if err != nil {
		t.Fatal(err)
	}
	vars := map[string]string{"name": "pay gw", "repo_name": "myrepo", "year": "2026", "arclint_version": "0"}

	dest, err := tpl.Destination(vars)
	if err != nil {
		t.Fatal(err)
	}
	if dest != "services/pay-gw" {
		t.Fatalf("Destination = %q", dest)
	}

	files, err := tpl.RenderUnit(vars)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(files["cmd/pay-gw/main.go"]); got != "package main // PayGw in myrepo\n" {
		t.Errorf("content = %q", got)
	}
	if got := string(files["assets/pay-gw.bin"]); got != binary {
		t.Errorf("binary must be verbatim, got %q", got)
	}
	if _, ok := files["migrations/.gitkeep"]; !ok {
		t.Error(".gitkeep must be carried through")
	}
}

func TestRenderUnitDupAndTraversal(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		".arclint/templates/dup/template.yaml":       "version: 1\ndestination: \"x\"\nvariables: [{name: a, description: d, type: string}]",
		".arclint/templates/dup/files/{{ a }}.txt":   "one",
		".arclint/templates/dup/files/fixed.txt":     "two",
		".arclint/templates/esc/template.yaml":       "version: 1\ndestination: \"x\"\nvariables: [{name: a, description: d, type: string}]",
		".arclint/templates/esc/files/{{ a }}/f.txt": "content",
	})
	vars := map[string]string{"a": "fixed"}
	dup, err := Load(root, "dup")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dup.RenderUnit(vars); err == nil || !strings.Contains(err.Error(), "same destination path") {
		t.Errorf("want duplicate-path error, got %v", err)
	}

	esc, err := Load(root, "esc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := esc.RenderUnit(map[string]string{"a": "../../evil"}); err == nil || !strings.Contains(err.Error(), "escapes the destination root") {
		t.Errorf("want traversal error, got %v", err)
	}
	if _, err := esc.Destination(map[string]string{"a": "x"}); err != nil {
		t.Errorf("clean destination rejected: %v", err)
	}
}

// TestRenderUnitRejectsSymlink pins the blocker-3 fix: a symlink under files/
// must be rejected, not followed (which would emit the target's content and
// escape the template dir).
func TestRenderUnitRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		".arclint/templates/sl/template.yaml": "version: 1\ndestination: \"x\"\nvariables: [{name: a, description: d, type: string}]",
		".arclint/templates/sl/files/real.txt": "ok",
	})
	filesDir := filepath.Join(root, ".arclint/templates/sl/files")
	secret := filepath.Join(root, "secret.txt")
	if err := os.WriteFile(secret, []byte("private"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(filesDir, "leak.txt")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
	tpl, err := Load(root, "sl")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tpl.RenderUnit(map[string]string{"a": "x"}); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("want symlink rejection, got %v", err)
	}
}

// TestValidateRelPathDriveAbsolute pins the low-5 fix: a Windows drive-absolute
// path (C:/foo) must be rejected even on Linux where filepath.IsAbs is false.
func TestValidateRelPathDriveAbsolute(t *testing.T) {
	for _, bad := range []string{"C:/foo", "c:bar", "Z:/x/y"} {
		if err := ValidateRelPath(bad); err == nil {
			t.Errorf("ValidateRelPath(%q) = nil, want rejection", bad)
		}
	}
	for _, ok := range []string{"foo/bar", "..cache/x", "a/b.txt"} {
		if err := ValidateRelPath(ok); err != nil {
			t.Errorf("ValidateRelPath(%q) = %v, want nil", ok, err)
		}
	}
}

func TestBuiltins(t *testing.T) {
	b := Builtins("/home/u/myrepo")
	if b["repo_name"] != "myrepo" {
		t.Errorf("repo_name = %q", b["repo_name"])
	}
	for _, k := range []string{"year", "arclint_version"} {
		if b[k] == "" {
			t.Errorf("builtin %s empty", k)
		}
	}
}

func TestUnifiedDiff(t *testing.T) {
	a := []byte("one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	b := []byte("one\ntwo\nthree\nfour\nFIVE\nsix\nseven\neight\nnine\nten\n")
	d := UnifiedDiff("f.txt", a, b)
	for _, part := range []string{"--- a/f.txt", "+++ b/f.txt", "-five", "+FIVE", "@@ -2,7 +2,7 @@"} {
		if !strings.Contains(d, part) {
			t.Errorf("diff missing %q:\n%s", part, d)
		}
	}
	if strings.Contains(d, " one") {
		t.Errorf("line outside context window leaked into hunk:\n%s", d)
	}
	if UnifiedDiff("f.txt", a, a) != "" {
		t.Error("equal content must produce empty diff")
	}
	if d := UnifiedDiff("f.txt", nil, []byte("new\n")); !strings.Contains(d, "+new") {
		t.Errorf("added-file diff wrong:\n%s", d)
	}
}
