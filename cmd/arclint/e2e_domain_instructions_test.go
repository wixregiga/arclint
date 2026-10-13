package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	cmlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/cml"
)

func boxofficeLanguage(t *testing.T) vocab.UbiquitousLanguage {
	t.Helper()
	recording, err := vocab.NewDomainRecording("domain.arclint.cml")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := cmlvocab.NewRepository(filepath.Join(repoRoot(t), "testing", "boxoffice"), recording)
	if err != nil {
		t.Fatal(err)
	}
	lang, found, err := repo.RecordedLanguage()
	if err != nil || !found {
		t.Fatalf("boxoffice language: found=%v err=%v", found, err)
	}
	return lang
}

func instructionProject(t *testing.T, lang vocab.UbiquitousLanguage, format string) string {
	t.Helper()
	work := t.TempDir()
	write(t, work, "rules.arclint.yaml", "runtime: [go]\ndomain:\n  file: language."+format+"\nzones:\n  src:\n    paths: [\"src/**\"]\n")
	write(t, work, "src/ok.go", "package src\n")
	repo := definitionRepository(t, work, format)
	if err := repo.Record(lang); err != nil {
		t.Fatal(err)
	}
	return work
}

func generatedInstructions(t *testing.T, work string) map[string]string {
	t.Helper()
	mustRunDomain(t, work, "agents", "skill")
	block := mustRunDomain(t, work, "agents", "md")
	out := map[string]string{"AGENTS.md": block}
	for _, name := range skillArtifactNames {
		content, err := os.ReadFile(filepath.Join(work, skillLitmusDir, name))
		if err != nil {
			t.Fatal(err)
		}
		out[name] = string(content)
	}
	return out
}

func hookOrientation(t *testing.T, work, session string) string {
	t.Helper()
	input, err := json.Marshal(map[string]string{
		"hook_event_name": "SessionStart", "session_id": session, "cwd": work,
	})
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runBinStdin(t, work, os.Environ(), string(input), "agents", "hooks", "event")
	if code != 0 || stderr != "" {
		t.Fatalf("hook start: exit=%d\n%s\n%s", code, stdout, stderr)
	}
	var output struct {
		HookSpecificOutput struct{ AdditionalContext string }
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	return output.HookSpecificOutput.AdditionalContext
}

func TestDomainInstructionsAreIdenticalAcrossFormats(t *testing.T) {
	lang := boxofficeLanguage(t)
	yamlProject := instructionProject(t, lang, "yaml")
	cmlProject := instructionProject(t, lang, "cml")
	yamlInstructions := generatedInstructions(t, yamlProject)
	cmlInstructions := generatedInstructions(t, cmlProject)
	for name, want := range yamlInstructions {
		if got := cmlInstructions[name]; got != want {
			t.Errorf("%s differs between recordings of the same language", name)
		}
		if strings.Contains(want, "language.yaml") || strings.Contains(want, "language.cml") {
			t.Errorf("%s names a recording", name)
		}
	}
	if yamlHook, cmlHook := hookOrientation(t, yamlProject, "yaml"), hookOrientation(t, cmlProject, "cml"); yamlHook == "" || yamlHook != cmlHook {
		t.Fatalf("hook instructions differ:\nYAML: %s\nCML: %s", yamlHook, cmlHook)
	}
	for format, work := range map[string]string{"yaml": yamlProject, "cml": cmlProject} {
		for _, args := range [][]string{{"context"}, {"domain"}} {
			stdout := mustRunDomain(t, work, args...)
			if !strings.Contains(stdout, "language."+format) {
				t.Errorf("%v does not report the recording: %s", args, stdout)
			}
		}
	}
}

func TestDomainConversionLeavesInstructionsCurrent(t *testing.T) {
	lang := boxofficeLanguage(t)
	for _, start := range []string{"yaml", "cml"} {
		t.Run(start, func(t *testing.T) {
			work := instructionProject(t, lang, start)
			want := generatedInstructions(t, work)
			mustRunDomain(t, work, "agents", "md", "--write")
			orientation := hookOrientation(t, work, "before")
			formats := []string{"cml", "yaml"}
			if start == "cml" {
				formats = []string{"yaml", "cml"}
			}
			for _, format := range formats {
				mustRunDomain(t, work, "domain", "convert", "language."+format)
				stdout, stderr, code := runBin(t, work, os.Environ(), "check", ".")
				if code != 1 || stderr != "" || strings.Contains(stdout, "agents:") {
					t.Fatalf("after conversion to %s: exit=%d\n%s\n%s", format, code, stdout, stderr)
				}
				block, err := os.ReadFile(filepath.Join(work, "AGENTS.md"))
				if err != nil || string(block) != want["AGENTS.md"] {
					t.Fatalf("conversion changed AGENTS.md: %v", err)
				}
				if got := mustRunDomain(t, work, "agents", "md"); got != want["AGENTS.md"] {
					t.Fatal("conversion left the installed block stale")
				}
				if got := hookOrientation(t, work, "after-"+format); got != orientation {
					t.Fatalf("conversion changed hook instructions:\n%s", got)
				}
				for _, name := range skillArtifactNames {
					content, err := os.ReadFile(filepath.Join(work, skillLitmusDir, name))
					if err != nil || string(content) != want[name] {
						t.Fatalf("conversion changed %s: %v", name, err)
					}
				}
				// The publisher also considers the untouched skill current.
				if got := mustRunDomain(t, work, "agents", "skill"); strings.Contains(got, "wrote ") {
					t.Fatalf("conversion made the skill stale: %s", got)
				}
				assertRecordedLanguage(t, definitionRepository(t, work, format), lang)
			}
		})
	}
}
