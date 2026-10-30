package studio

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
)

func TestDraftLifecycle(t *testing.T) {
	for _, recording := range []struct{ source, name string }{
		{"docs/examples/go/domain.arclint.yaml", "domain.arclint.yaml"},
		{"testing/boxoffice/domain.arclint.cml", "domain.arclint.cml"},
	} {
		t.Run(recording.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, recording.name)
			original, err := os.ReadFile(filepath.Join("..", "..", "..", recording.source))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			s := FeatureFiles{Project: root}
			list, err := s.List()
			if err != nil || len(list) != 0 {
				t.Fatal(list, err)
			}
			d, err := s.Save(application.FeatureDraft{Title: "Draft", Body: "世界 😀", References: []application.FeatureReference{{ID: "r", From: 900, To: 1000, Quote: "old", Reading: "names", Term: &application.FeatureTerm{Concept: "value_object", Context: "gone", Name: "Missing"}}}})
			if err != nil {
				t.Fatal(err)
			}
			old := d
			list, err = s.List()
			if err != nil || len(list) != 1 {
				t.Fatal(list, err)
			}
			d.Body += " revised"
			d, err = s.Save(d)
			if err != nil {
				t.Fatal(err)
			}
			reopened, err := (FeatureFiles{Project: root}).Get(d.ID)
			if err != nil || reopened.Body != d.Body || reopened.References[0].From != 900 {
				t.Fatal(reopened, err)
			}
			if _, err = s.Save(old); !errors.Is(err, application.ErrFeatureConflict) {
				t.Fatal(err)
			}
			if err = s.Delete(old.ID, old.Revision); !errors.Is(err, application.ErrFeatureConflict) {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			outcomes := make(chan error, 2)
			for range 2 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					copy := d
					copy.Body += " winner"
					_, err := (FeatureFiles{Project: root}).Save(copy)
					outcomes <- err
				}()
			}
			wg.Wait()
			close(outcomes)
			successes := 0
			conflicts := 0
			for err := range outcomes {
				if err == nil {
					successes++
				} else if errors.Is(err, application.ErrFeatureConflict) {
					conflicts++
				} else {
					t.Fatal(err)
				}
			}
			if successes != 1 || conflicts != 1 {
				t.Fatal(successes, conflicts)
			}
			d, err = s.Get(d.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Delete(d.ID, d.Revision); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Get(d.ID); !errors.Is(err, application.ErrFeatureMissing) {
				t.Fatal(err)
			}
			bytes, err := os.ReadFile(path)
			if err != nil || string(bytes) != string(original) {
				t.Fatal(string(bytes), err)
			}
		})
	}
}
func TestDraftRefusals(t *testing.T) {
	root := t.TempDir()
	s := FeatureFiles{Project: root}
	if _, err := s.Get("../domain"); !errors.Is(err, application.ErrFeatureInput) {
		t.Fatal(err)
	}
	d, err := s.Save(application.FeatureDraft{References: []application.FeatureReference{}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".arclint", "studio", "features", d.ID+".json")
	if err = os.WriteFile(path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(d.ID); err == nil {
		t.Fatal("corruption hidden")
	}
	if _, err = s.List(); err == nil {
		t.Fatal("corruption hidden")
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err = os.WriteFile(outside, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(d.ID); err == nil {
		t.Fatal("link followed")
	}
	if _, err = s.Save(d); err == nil {
		t.Fatal("link overwritten")
	}
	if err = s.Delete(d.ID, d.Revision); err == nil {
		t.Fatal("link removed")
	}
	root2 := t.TempDir()
	if err = os.Symlink(root, filepath.Join(root2, ".arclint")); err != nil {
		t.Fatal(err)
	}
	if _, err = (FeatureFiles{Project: root2}).Save(application.FeatureDraft{References: []application.FeatureReference{}}); err == nil {
		t.Fatal("directory link followed")
	}
}

func TestDraftRangeRefusalsPreserveStoredText(t *testing.T) {
	store := FeatureFiles{Project: t.TempDir()}
	draft, err := store.Save(application.FeatureDraft{Body: "Keep this", References: []application.FeatureReference{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, span := range [][2]int{{-1, 2}, {3, 2}} {
		invalid := draft
		invalid.Body = "Must not replace it"
		invalid.References = []application.FeatureReference{{ID: "r", From: span[0], To: span[1], Reading: "plain"}}
		if _, err := store.Save(invalid); !errors.Is(err, application.ErrFeatureInput) {
			t.Fatalf("accepted impossible range %v: %v", span, err)
		}
		got, err := store.Get(draft.ID)
		if err != nil || got.Body != draft.Body || got.Revision != draft.Revision {
			t.Fatalf("refusal changed stored text: %+v %v", got, err)
		}
	}
	draft.References = []application.FeatureReference{{ID: "collapsed", From: 500, To: 500, Quote: "removed", Reading: "plain"}}
	if _, err := store.Save(draft); err != nil {
		t.Fatalf("lost a collapsed stale annotation: %v", err)
	}
}

func TestDraftMetadataIgnorePreservesAdopterFile(t *testing.T) {
	for _, existing := range []string{"", "# owned by the adopting project\nprivate-*\n"} {
		t.Run(map[bool]string{true: "existing", false: "new"}[existing != ""], func(t *testing.T) {
			project := t.TempDir()
			directory := filepath.Join(project, ".arclint", "studio", "features")
			if existing != "" {
				if err := os.MkdirAll(directory, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, ".gitignore"), []byte(existing), 0600); err != nil {
					t.Fatal(err)
				}
			}
			store := FeatureFiles{Project: project}
			draft, err := store.Save(application.FeatureDraft{References: []application.FeatureReference{}})
			if err != nil {
				t.Fatal(err)
			}
			ignore, err := os.ReadFile(filepath.Join(directory, ".gitignore"))
			want := existing
			if want == "" {
				want = ".lock\n.write-*\n"
			}
			if err != nil || string(ignore) != want {
				t.Fatalf("ignore content: %q %v", ignore, err)
			}
			if existing != "" {
				return
			}
			if err := os.WriteFile(filepath.Join(directory, ".write-crashed"), []byte("incomplete"), 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"init", "-q", project}, {"-C", project, "ls-files", "--others", "--exclude-standard"}} {
				output, err := exec.Command("git", args...).CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %s %v", args, output, err)
				}
				if len(args) > 3 && (!strings.Contains(string(output), draft.ID+".json") || strings.Contains(string(output), ".lock") || strings.Contains(string(output), ".write-")) {
					t.Fatalf("version-control candidates include local metadata or omit the draft: %s", output)
				}
			}
		})
	}
}

func TestDraftFilesRemainConfinedAndReadable(t *testing.T) {
	project := t.TempDir()
	store := FeatureFiles{Project: project}
	draft, err := store.Save(application.FeatureDraft{Body: "In the project", References: []application.FeatureReference{}})
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(project, ".arclint", "studio", "features")
	if err := os.WriteFile(filepath.Join(directory, ".gitignore"), []byte(".lock\n.write-*\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if entries, err := store.List(); err != nil || len(entries) != 1 {
		t.Fatalf("non-document metadata: %v %v", entries, err)
	}
	outside := t.TempDir()
	original, err := os.ReadFile(filepath.Join(directory, draft.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, draft.ID+".json"), original, 0600); err != nil {
		t.Fatal(err)
	}
	held := directory + "-held"
	if err := os.Rename(directory, held); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, directory); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(draft.ID); err == nil {
		t.Fatal("opened a replacement directory outside storage")
	}
	draft.Body = "Must not escape"
	if _, err := store.Save(draft); err == nil {
		t.Fatal("saved through a replacement directory outside storage")
	}
	if err := store.Delete(draft.ID, draft.Revision); err == nil {
		t.Fatal("deleted through a replacement directory outside storage")
	}
	after, err := os.ReadFile(filepath.Join(outside, draft.ID+".json"))
	if err != nil || string(after) != string(original) {
		t.Fatalf("outside document changed: %v", err)
	}
	if err := os.Remove(directory); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(held, directory); err != nil {
		t.Fatal(err)
	}
	invalid := []byte(`{"version":1,"feature":{"id":"` + draft.ID + `","revision":"","references":[]}}`)
	if err := os.WriteFile(filepath.Join(directory, draft.ID+".json"), invalid, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(draft.ID); err == nil {
		t.Fatal("accepted a stored document with missing text fields")
	}
}
