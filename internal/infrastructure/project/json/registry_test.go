package jsonprojects_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	jsonprojects "github.com/wixregiga/arclint/internal/infrastructure/project/json"
)

func TestRegistryRemembersMostRecentFirstAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arclint", "projects.json")
	r := jsonprojects.NewRegistry(path)
	if dirs, err := r.Directories(); err != nil || len(dirs) != 0 {
		t.Fatalf("a registry with no file yet: %v, %v", dirs, err)
	}
	for _, dir := range []string{"/work/orders", "/work/billing", "/work/orders/"} {
		if err := r.Remember(dir); err != nil {
			t.Fatal(err)
		}
	}
	// Another instance, as the next Studio run, reads the same file.
	dirs, err := jsonprojects.NewRegistry(path).Directories()
	if err != nil || strings.Join(dirs, ",") != "/work/orders,/work/billing" {
		t.Fatalf("remembered %v, %v", dirs, err)
	}
	if err := r.Forget("/work/orders"); err != nil {
		t.Fatal(err)
	}
	if err := r.Forget("/work/never"); err != nil {
		t.Fatalf("forgetting a project not remembered: %v", err)
	}
	if dirs, _ := r.Directories(); strings.Join(dirs, ",") != "/work/billing" {
		t.Fatalf("after forgetting: %v", dirs)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the file: %v, mode %v", err, info.Mode())
	}
	if err := r.Remember("relative"); err == nil {
		t.Fatal("a relative directory was remembered")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files stayed beside the registry: %v %v", entries, err)
	}
}

func TestRegistryRefusesAFileItCannotRead(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		`{"version":2,"projects":[]}`:            "version 2",
		`{"version":1,"projects":["relative"]}`: "not an absolute path",
		`not json`:                              "invalid character",
	}
	for content, reason := range cases {
		path := filepath.Join(dir, "projects.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		r := jsonprojects.NewRegistry(path)
		if _, err := r.Directories(); err == nil || !strings.Contains(err.Error(), reason) {
			t.Errorf("%s: err = %v, want %q", content, err, reason)
		}
		if err := r.Remember("/work/orders"); err == nil {
			t.Errorf("%s: Remember overwrote a file it cannot read", content)
		}
		if data, _ := os.ReadFile(path); string(data) != content {
			t.Errorf("%s: the file changed to %s", content, data)
		}
	}
}

func TestRegistryNeverLeavesATornFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	r := jsonprojects.NewRegistry(path)
	if err := r.Remember("/work/first"); err != nil {
		t.Fatal(err)
	}
	// One Studio answers one request at a time, so its writes never
	// interleave; this pins that a write never leaves a torn file.
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.Remember(filepath.Join("/work", string(rune('a'+i))))
		}()
	}
	wg.Wait()
	if _, err := r.Directories(); err != nil {
		t.Fatalf("the file is torn: %v", err)
	}
}

func TestUserRegistryLivesUnderTheUserConfigurationDirectory(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("AppData", config)
	r := jsonprojects.UserRegistry()
	want, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if r.Path() != filepath.Join(want, "arclint", "projects.json") {
		t.Fatalf("path = %q", r.Path())
	}
}
