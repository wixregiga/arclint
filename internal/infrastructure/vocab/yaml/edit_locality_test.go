package yamlvocab_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

// Recording refills folded prose once, the first time a hand-written
// file is recorded. After that, recording an unchanged language changes
// no byte, and one edit changes only the lines of the element it edits.
// The proof runs on this repository's own domain file.
func TestRecordEditsOnlyTheEditedElementOnceNormalized(t *testing.T) {
	source, err := os.ReadFile("../../../../domain.arclint.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		header string
		edit   func(*vocab.UbiquitousLanguage) bool
	}{
		{
			"a value object's definition",
			"      DomainRecording:",
			func(l *vocab.UbiquitousLanguage) bool {
				for ci := range l.Contexts {
					for vi := range l.Contexts[ci].ValueObjects {
						if v := &l.Contexts[ci].ValueObjects[vi]; v.Name == "DomainRecording" {
							v.Definition += " This sentence is the edit."
							return true
						}
					}
				}
				return false
			},
		},
		{
			"an aggregate member's definition",
			"          Pattern:",
			func(l *vocab.UbiquitousLanguage) bool {
				for ci := range l.Contexts {
					for ai := range l.Contexts[ci].Aggregates {
						for ei := range l.Contexts[ci].Aggregates[ai].Entities {
							if e := &l.Contexts[ci].Aggregates[ai].Entities[ei]; e.Name == "Pattern" {
								e.Definition += " This sentence is the edit."
								return true
							}
						}
					}
				}
				return false
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, _ := repository(t, string(source))
			lang, _, err := repo.RecordedLanguage()
			if err != nil {
				t.Fatal(err)
			}
			if err := repo.Record(lang); err != nil {
				t.Fatal(err)
			}
			normalized := readRecording(t, repo)
			if err := repo.Record(lang); err != nil {
				t.Fatal(err)
			}
			if again := readRecording(t, repo); again != normalized {
				t.Fatal("recording an unchanged language after normalization changed the file")
			}

			if !tc.edit(&lang) {
				t.Fatalf("the domain file records no %s", strings.TrimSpace(tc.header))
			}
			if err := repo.Record(lang); err != nil {
				t.Fatal(err)
			}
			edited := readRecording(t, repo)
			before, after := strings.Split(normalized, "\n"), strings.Split(edited, "\n")
			start, end := elementSpan(t, before, tc.header)
			prefix := 0
			for prefix < len(before) && prefix < len(after) && before[prefix] == after[prefix] {
				prefix++
			}
			suffix := 0
			for suffix < len(before)-prefix && suffix < len(after)-prefix && before[len(before)-1-suffix] == after[len(after)-1-suffix] {
				suffix++
			}
			if prefix == len(before) && len(before) == len(after) {
				t.Fatal("the edit changed nothing")
			}
			if prefix < start || len(before)-suffix > end {
				t.Fatalf("the edit changed lines %d to %d, outside %s (lines %d to %d)", prefix+1, len(before)-suffix, strings.TrimSpace(tc.header), start+1, end)
			}

			read, _, err := repo.RecordedLanguage()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(withoutLines(read), withoutLines(lang)) {
				t.Fatal("the edited file does not read back as the edited language")
			}
		})
	}
}

func readRecording(t *testing.T, repo yamlvocab.Repository) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(repo.Path()))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// elementSpan returns the lines of the element whose key line is
// header: from that line up to the next non-blank line indented no
// deeper.
func elementSpan(t *testing.T, lines []string, header string) (int, int) {
	t.Helper()
	depth := len(header) - len(strings.TrimLeft(header, " "))
	for i, line := range lines {
		if line != header {
			continue
		}
		end := i + 1
		for end < len(lines) {
			trimmed := strings.TrimLeft(lines[end], " ")
			if trimmed != "" && len(lines[end])-len(trimmed) <= depth {
				break
			}
			end++
		}
		return i, end
	}
	t.Fatalf("no line %q in the normalized file", header)
	return 0, 0
}
