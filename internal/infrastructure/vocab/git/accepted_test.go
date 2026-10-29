package gitvocab_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	gitvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/git"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

type parser struct{}

func (parser) ParseUbiquitousLanguage(content []byte, recording vocab.DomainRecording) (vocab.UbiquitousLanguage, error) {
	return yamlvocab.Parse(content, recording)
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func recording(t *testing.T) vocab.DomainRecording {
	t.Helper()
	r, err := vocab.NewDomainRecording(vocab.DefaultDomainRecordingPath)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRefusesAProjectWithoutARemoteDefaultBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	_, err := gitvocab.NewRepository(root, recording(t), parser{})
	if !errors.Is(err, gitvocab.ErrNoDefaultBranch) {
		t.Fatalf("outside git: err = %v, want ErrNoDefaultBranch", err)
	}
	git(t, root, "init", "-q", "-b", "main")
	_, err = gitvocab.NewRepository(root, recording(t), parser{})
	if !errors.Is(err, gitvocab.ErrNoDefaultBranch) || !strings.Contains(err.Error(), "refs/remotes/origin/HEAD") {
		t.Fatalf("no remote: err = %v, want ErrNoDefaultBranch naming origin/HEAD", err)
	}
}

// TestReadsTheRecordingTheDefaultBranchHoldsFromANestedProject clones a
// repository whose project sits in a subdirectory, so the path git
// shows is the prefix and the recording's path together, and proves the
// branch is read, not the working tree.
func TestReadsTheRecordingTheDefaultBranchHoldsFromANestedProject(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	base := t.TempDir()
	upstream := filepath.Join(base, "upstream", "services", "billing")
	if err := os.MkdirAll(upstream, 0o750); err != nil {
		t.Fatal(err)
	}
	accepted := "version: 1\nproject: billing\ncontexts:\n  billing:\n    definition: Invoices and payments.\n"
	if err := os.WriteFile(filepath.Join(upstream, vocab.DefaultDomainRecordingPath), []byte(accepted), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, filepath.Join(base, "upstream"), "init", "-q", "-b", "main")
	git(t, filepath.Join(base, "upstream"), "add", ".")
	git(t, filepath.Join(base, "upstream"), "commit", "-q", "-m", "accepted")
	git(t, base, "clone", "-q", filepath.Join(base, "upstream"), filepath.Join(base, "clone"))
	root := filepath.Join(base, "clone", "services", "billing")
	proposed := strings.Replace(accepted, "Invoices and payments.", "Invoices, payments and refunds.", 1)
	if err := os.WriteFile(filepath.Join(root, vocab.DefaultDomainRecordingPath), []byte(proposed), 0o600); err != nil {
		t.Fatal(err)
	}

	repo, err := gitvocab.NewRepository(root, recording(t), parser{})
	if err != nil {
		t.Fatal(err)
	}
	if repo.Branch() != "origin/main" {
		t.Fatalf("branch = %q", repo.Branch())
	}
	lang, found, err := repo.RecordedLanguage()
	if err != nil || !found {
		t.Fatalf("found/err = %v/%v", found, err)
	}
	if lang.Contexts[0].Definition != "Invoices and payments." {
		t.Fatalf("read %q, want the branch's recording, not the working tree's", lang.Contexts[0].Definition)
	}
	if lang.Recording != recording(t) {
		t.Fatalf("the language carries %v, want its recording", lang.Recording)
	}
	if err := repo.Record(lang); err == nil || !strings.Contains(err.Error(), "git") {
		t.Fatalf("Record: err = %v, want a refusal naming git", err)
	}

	// The branch without the file is an empty language, not an error.
	git(t, filepath.Join(base, "upstream"), "rm", "-q", filepath.Join("services", "billing", vocab.DefaultDomainRecordingPath))
	git(t, filepath.Join(base, "upstream"), "commit", "-q", "-m", "dropped")
	git(t, filepath.Join(base, "clone"), "fetch", "-q", "origin")
	lang, found, err = repo.RecordedLanguage()
	if err != nil || found || !lang.Empty() {
		t.Fatalf("after the branch drops the file: found/err = %v/%v, language %+v", found, err, lang)
	}

	// A file the branch holds that is not a language is an error, never a partial value.
	if err := os.MkdirAll(upstream, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upstream, vocab.DefaultDomainRecordingPath), []byte("version: 1\nproject: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, filepath.Join(base, "upstream"), "add", ".")
	git(t, filepath.Join(base, "upstream"), "commit", "-q", "-m", "broken")
	git(t, filepath.Join(base, "clone"), "fetch", "-q", "origin")
	if _, _, err := repo.RecordedLanguage(); err == nil || !strings.Contains(err.Error(), "origin/main") {
		t.Fatalf("broken file: err = %v, want an error naming the branch", err)
	}
}

// TestReadsTheRemoteBranchEvenBesideALocalBranchOfTheSameName creates a
// local branch spelled origin/main that records something else; the
// adapter still reads what the remote's default branch records.
func TestReadsTheRemoteBranchEvenBesideALocalBranchOfTheSameName(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	base := t.TempDir()
	upstream := filepath.Join(base, "upstream")
	if err := os.MkdirAll(upstream, 0o750); err != nil {
		t.Fatal(err)
	}
	accepted := "version: 1\nproject: billing\ncontexts:\n  billing:\n    definition: Invoices and payments.\n"
	if err := os.WriteFile(filepath.Join(upstream, vocab.DefaultDomainRecordingPath), []byte(accepted), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, upstream, "init", "-q", "-b", "main")
	git(t, upstream, "add", ".")
	git(t, upstream, "commit", "-q", "-m", "accepted")
	clone := filepath.Join(base, "clone")
	git(t, base, "clone", "-q", upstream, clone)
	local := strings.Replace(accepted, "Invoices and payments.", "Something local.", 1)
	if err := os.WriteFile(filepath.Join(clone, vocab.DefaultDomainRecordingPath), []byte(local), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, clone, "checkout", "-q", "-b", "origin/main")
	git(t, clone, "commit", "-q", "-am", "local")

	repo, err := gitvocab.NewRepository(clone, recording(t), parser{})
	if err != nil {
		t.Fatal(err)
	}
	lang, found, err := repo.RecordedLanguage()
	if err != nil || !found {
		t.Fatalf("found/err = %v/%v", found, err)
	}
	if lang.Contexts[0].Definition != "Invoices and payments." {
		t.Fatalf("read %q, want the remote branch's recording", lang.Contexts[0].Definition)
	}
}
