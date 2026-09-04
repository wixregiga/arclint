package application_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
)

// repoRoot locates the repository root from this source file.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller: no source location")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// On failure: regenerate the committed skill artifacts via arclint
// agents skill, or fix the generator; never edit fixtures by hand.

func TestPublishSkillProtocolRenderMatchesLitmus(t *testing.T) {
	uc, err := application.NewPublishSkillProtocol(stubArtifactWriter{})
	if err != nil {
		t.Fatalf("NewPublishSkillProtocol: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(repoRoot(t), ".agents", "skills", "domain-librarian", "SKILL.md"))
	if err != nil {
		t.Fatalf("read litmus SKILL.md: %v", err)
	}
	got := []byte(uc.Render())
	if !bytes.Equal(got, want) {
		t.Fatalf("PublishSkillProtocol.Render drifted from litmus SKILL.md; regenerate the committed skill artifacts via arclint agents skill, or fix the generator; never edit fixtures by hand")
	}
}

func TestPublishSkillVocabularyRenderMatchesLitmus(t *testing.T) {
	uc, err := application.NewPublishSkillVocabulary(stubArtifactWriter{})
	if err != nil {
		t.Fatalf("NewPublishSkillVocabulary: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(repoRoot(t), ".agents", "skills", "domain-librarian", "VOCAB.yaml"))
	if err != nil {
		t.Fatalf("read litmus VOCAB.yaml: %v", err)
	}
	got := []byte(uc.Render())
	if !bytes.Equal(got, want) {
		t.Fatalf("PublishSkillVocabulary.Render drifted from litmus VOCAB.yaml; regenerate the committed skill artifacts via arclint agents skill, or fix the generator; never edit fixtures by hand")
	}
}

type stubArtifactWriter struct{}

func (stubArtifactWriter) Write(dir, filename string, content []byte) (bool, string, error) {
	return false, filepath.Join(dir, filename), nil
}
