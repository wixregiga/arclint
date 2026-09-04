package vocab_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// On failure of any litmus drift test below: regenerate the committed
// skill artifacts via arclint agents skill, or fix the generator;
// never edit fixtures by hand.

func TestSkillMarkdownMatchesLitmus(t *testing.T) {
	wantPath := filepath.Join(repoRoot(t), ".agents", "skills", "domain-librarian", "SKILL.md")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read litmus SKILL.md: %v", err)
	}
	got := []byte(vocab.SkillMarkdown())
	if !bytes.Equal(got, want) {
		t.Fatalf("vocab.SkillMarkdown() drifted from litmus SKILL.md; regenerate the committed skill artifacts via arclint agents skill, or fix the generator; never edit fixtures by hand\n--- first diff hint ---\n%s", firstDiff(got, want))
	}
}

func TestVocabularyYAMLMatchesLitmus(t *testing.T) {
	wantPath := filepath.Join(repoRoot(t), ".agents", "skills", "domain-librarian", "VOCAB.yaml")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read litmus VOCAB.yaml: %v", err)
	}
	got := []byte(vocab.VocabularyYAML())
	if !bytes.Equal(got, want) {
		t.Fatalf("vocab.VocabularyYAML() drifted from litmus VOCAB.yaml; regenerate the committed skill artifacts via arclint agents skill, or fix the generator; never edit fixtures by hand\n--- first diff hint ---\n%s", firstDiff(got, want))
	}
}

func firstDiff(got, want []byte) string {
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	for i := range n {
		if got[i] != want[i] {
			start := i - 40
			if start < 0 {
				start = 0
			}
			endG := i + 80
			if endG > len(got) {
				endG = len(got)
			}
			endW := i + 80
			if endW > len(want) {
				endW = len(want)
			}
			return "byte " + strconv.Itoa(i) + "\ngot:  " + stringify(got[start:endG]) + "\nwant: " + stringify(want[start:endW])
		}
	}
	if len(got) != len(want) {
		return "lengths differ: got " + strconv.Itoa(len(got)) + " want " + strconv.Itoa(len(want))
	}
	return "no difference"
}

func stringify(b []byte) string {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		switch c {
		case '\n':
			out = append(out, '\\', 'n')
		case '\t':
			out = append(out, '\\', 't')
		default:
			out = append(out, c)
		}
	}
	return string(out)
}
