package vocabfs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	vocabfs "github.com/wixregiga/arclint/internal/infrastructure/vocab"
	cmlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/cml"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

func symlink(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}

func recording(t *testing.T, path string) vocab.DomainRecording {
	t.Helper()
	r, err := vocab.NewDomainRecording(path)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestResolveRecordingSymlinks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		linkFile bool
		external bool
		missing  bool
	}{
		{"external directory existing file", false, true, false},
		{"external directory missing file", false, true, true},
		{"external file", true, true, false},
		{"dangling external file", true, true, true},
		{"internal directory existing file", false, false, false},
		{"internal directory missing file", false, false, true},
		{"internal file", true, false, false},
		{"dangling internal file", true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, "actual")
			if tc.external {
				directory = filepath.Join(t.TempDir(), "actual")
			}
			if err := os.Mkdir(directory, 0o750); err != nil {
				t.Fatal(err)
			}
			var err error
			directory, err = filepath.EvalSymlinks(directory)
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(directory, "domain.yaml")
			if !tc.missing {
				if err := os.WriteFile(target, []byte("project: shop\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			path := "model/domain.yaml"
			if tc.linkFile {
				path = "domain.yaml"
				symlink(t, target, filepath.Join(root, path))
			} else {
				symlink(t, directory, filepath.Join(root, "model"))
			}
			got, err := vocabfs.ResolveRecording(root, recording(t, path))
			if tc.external || (tc.linkFile && tc.missing) {
				if err == nil || !strings.Contains(err.Error(), "inside-the-repository") || !strings.Contains(err.Error(), path) {
					t.Fatalf("unsafe location = %s, %v", got, err)
				}
				return
			}
			if err != nil || got != target {
				t.Fatalf("resolved = %s, %v; want %s", got, err, target)
			}
		})
	}
}

func TestResolveRecordingRootThroughSymlink(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "repository")
	symlink(t, root, link)
	got, err := vocabfs.ResolveRecording(link, recording(t, "missing/domain.cml"))
	resolvedRoot, rootErr := filepath.EvalSymlinks(root)
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	want := filepath.Join(resolvedRoot, "missing", "domain.cml")
	if err != nil || got != want {
		t.Fatalf("resolved = %s, %v; want %s", got, err, want)
	}
}

func repositories(t *testing.T, root, path string) vocab.Repository {
	t.Helper()
	r := recording(t, path)
	if r.Format() == vocab.DomainRecordingCML {
		repo, err := cmlvocab.NewRepository(root, r)
		if err != nil {
			t.Fatal(err)
		}
		return repo
	}
	repo, err := yamlvocab.NewRepository(root, r)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestRepositoriesRejectSymlinkChangedAfterConstruction(t *testing.T) {
	for _, extension := range []string{"yaml", "cml"} {
		for _, kind := range []string{"directory", "file", "missing", "dangling"} {
			t.Run(extension+"/"+kind, func(t *testing.T) {
				root, outside := t.TempDir(), t.TempDir()
				path := "model/domain." + extension
				if kind == "file" || kind == "dangling" {
					path = "domain." + extension
				}
				repo := repositories(t, root, path)
				target := filepath.Join(outside, "domain."+extension)
				const content = "external content must survive"
				if kind != "missing" && kind != "dangling" {
					if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "file" || kind == "dangling" {
					symlink(t, target, filepath.Join(root, path))
				} else {
					symlink(t, outside, filepath.Join(root, "model"))
				}
				_, _, readErr := repo.RecordedLanguage()
				writeErr := repo.Record(vocab.UbiquitousLanguage{Project: "shop"})
				for _, err := range []error{readErr, writeErr} {
					if err == nil || !strings.Contains(err.Error(), "inside-the-repository") {
						t.Errorf("unsafe access succeeded: %v", err)
					}
				}
				entries, err := os.ReadDir(outside)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "missing" || kind == "dangling" {
					if len(entries) != 0 {
						t.Fatalf("outside files created: %v", entries)
					}
				} else {
					data, err := os.ReadFile(target)
					if err != nil || string(data) != content || len(entries) != 1 {
						t.Fatalf("external recording changed or temp leaked: %v, %v, %s", err, entries, data)
					}
				}
			})
		}
	}
}

func TestRepositoriesAllowInternalSymlinksAndSymlinkedRoot(t *testing.T) {
	for _, extension := range []string{"yaml", "cml"} {
		for _, kind := range []string{"directory", "file", "root"} {
			t.Run(extension+"/"+kind, func(t *testing.T) {
				root := t.TempDir()
				path := "actual/domain." + extension
				actual := repositories(t, root, path)
				lang := vocab.UbiquitousLanguage{Project: "shop"}
				if err := actual.Record(lang); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "directory":
					symlink(t, filepath.Join(root, "actual"), filepath.Join(root, "model"))
					path = "model/domain." + extension
				case "file":
					symlink(t, filepath.Join(root, filepath.FromSlash(path)), filepath.Join(root, "domain."+extension))
					path = "domain." + extension
				case "root":
					link := filepath.Join(t.TempDir(), "repository")
					symlink(t, root, link)
					root = link
				}
				repo := repositories(t, root, path)
				read, found, err := repo.RecordedLanguage()
				if err != nil || !found || read.Project != "shop" {
					t.Fatalf("read inside symlink: %v, %v, %+v", found, err, read)
				}
				lang.Project = "updated"
				if err := repo.Record(lang); err != nil {
					t.Fatal(err)
				}
				read, _, err = actual.RecordedLanguage()
				if err != nil || read.Project != "updated" {
					t.Fatalf("internal target was not updated: %v, %+v", err, read)
				}
			})
		}
	}
}
