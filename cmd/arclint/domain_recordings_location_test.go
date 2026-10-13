package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

func TestDomainRecordingsExistsAndRemoveRejectExternalSymlinks(t *testing.T) {
	for _, kind := range []string{"directory", "file", "missing", "dangling"} {
		t.Run(kind, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			path, link, target := "model/domain.yaml", filepath.Join(root, "model"), outside
			if kind == "file" || kind == "dangling" {
				path = "domain.yaml"
				link, target = filepath.Join(root, path), filepath.Join(outside, "domain.yaml")
			}
			if kind != "missing" && kind != "dangling" {
				write(t, outside, "domain.yaml", orderingDomain)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			r, err := vocab.NewDomainRecording(path)
			if err != nil {
				t.Fatal(err)
			}
			d := domainRecordings{root: root}
			_, existsErr := d.Exists(r)
			_, removeErr := d.PlanRemoval(r)
			for _, err := range []error{existsErr, removeErr} {
				if err == nil || !strings.Contains(err.Error(), "inside-the-repository") {
					t.Errorf("external access allowed: %v", err)
				}
			}
			if kind != "missing" && kind != "dangling" {
				data, err := os.ReadFile(filepath.Join(outside, "domain.yaml"))
				if err != nil || string(data) != orderingDomain {
					t.Fatalf("external recording removed or changed: %v, %s", err, data)
				}
			}
		})
	}
}

func TestDomainRecordingsExistsAndRemoveAllowInternalSymlinks(t *testing.T) {
	root := t.TempDir()
	write(t, root, "actual/domain.yaml", orderingDomain)
	if err := os.Symlink(filepath.Join(root, "actual"), filepath.Join(root, "model")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	r, err := vocab.NewDomainRecording("model/domain.yaml")
	if err != nil {
		t.Fatal(err)
	}
	d := domainRecordings{root: root}
	if found, err := d.Exists(r); err != nil || !found {
		t.Fatalf("internal recording does not exist: %v, %v", found, err)
	}
	removal, err := d.PlanRemoval(r)
	if err != nil {
		t.Fatal(err)
	}
	if found, err := d.Exists(r); err != nil || !found {
		t.Fatalf("planning the removal removed the recording: %v, %v", found, err)
	}
	if err := removal.Apply(); err != nil {
		t.Fatal(err)
	}
	if found, err := d.Exists(r); err != nil || found {
		t.Fatalf("removed recording exists: %v, %v", found, err)
	}
	if err := removal.Revert(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "actual", "domain.yaml"))
	if err != nil || string(data) != orderingDomain {
		t.Fatalf("putting the removal back did not restore the recording: %v, %s", err, data)
	}
}

// A domain file the ruleset names that cannot be opened stays the
// project's DomainRecording, so the workflow hooks, which run without
// the project's domain, still recognize an edit to it; reading the
// language reports why the file could not be opened.
func TestComposeKnowledgeKeepsANamedRecordingThatCannotBeOpened(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	write(t, root, "rules.arclint.yaml", "runtime: [go]\ndomain:\n  file: docs/model.yaml\nzones:\n  src:\n    paths: [\"src/**\"]\n")
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	recording, _, knowledge, err := composeKnowledge(root, filepath.Join(root, "rules.arclint.yaml"))
	if err == nil || !strings.Contains(err.Error(), "inside-the-repository") {
		t.Fatalf("err = %v, want the location refused", err)
	}
	if knowledge != nil || recording.Path() != "docs/model.yaml" {
		t.Fatalf("composeKnowledge = %q, %v; want the named recording kept", recording.Path(), knowledge)
	}
	unopened := unopenedRecording{recording: recording, err: err}
	if unopened.Recording() != recording {
		t.Fatal("the stand-in names another file")
	}
	if _, _, readErr := unopened.RecordedLanguage(); readErr == nil || !strings.Contains(readErr.Error(), "inside-the-repository") {
		t.Fatalf("reading the language: %v, want why the file could not be opened", readErr)
	}
}
