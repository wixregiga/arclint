package main

// End-to-end coverage of how a project's domain file, its
// DomainRecording, is found when rules.arclint.yaml names none: the first
// of domain.arclint.yaml, domain.arclint.yml and domain.arclint.cml that
// exists, domain.arclint.yaml when none does. A default-named file that
// is not the domain file is ignored, and every command says so on stderr
// without changing its stdout or its exit code.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// brokenCML is a CML document no reader accepts: a command that read it
// would fail naming it, so a file holding it proves it was never read.
const brokenCML = "Domain shop\nBoundedContext {\n"

func ignoredWarning(ignored, chosen string) string {
	return "arclint: " + ignored + " is ignored; the project's domain file is " + chosen +
		". Name the one to use under domain.file in rules.arclint.yaml, or remove the other.\n"
}

func TestDomainFileIsFoundAmongTheDefaultNames(t *testing.T) {
	for _, file := range []string{"domain.arclint.yml", "domain.arclint.cml"} {
		t.Run(file, func(t *testing.T) {
			c := domainFileCase{name: file, rules: minimalDomainRules, file: file, other: "domain.arclint.yaml"}
			root := c.fixture(t)
			write(t, root, file, c.orderingDomain(t))

			stdout, stderr, code := runBin(t, root, os.Environ(), "domain", "overview")
			if code != 0 || stderr != "" || !strings.Contains(stdout, "Source: "+file+"\n") || !strings.Contains(stdout, "Order") {
				t.Fatalf("overview: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
			}
			stdout, stderr, code = runBin(t, root, os.Environ(), "domain", "overview", "--format", "json")
			var overview struct {
				Found  bool   `json:"found"`
				Source string `json:"source"`
			}
			if err := json.Unmarshal([]byte(stdout), &overview); err != nil || code != 0 || stderr != "" || !overview.Found || overview.Source != file {
				t.Fatalf("overview json: exit %d %v\nstdout: %s\nstderr: %s", code, err, stdout, stderr)
			}

			stdout, stderr, code = runBin(t, root, os.Environ(), "check", "--format", "json")
			var diagnostics []diagnosticDoc
			if err := json.Unmarshal([]byte(stdout), &diagnostics); err != nil || code != 1 || stderr != "" {
				t.Fatalf("check: exit %d %v\nstdout: %s\nstderr: %s", code, err, stdout, stderr)
			}
			anchored := 0
			for _, d := range diagnostics {
				if d.Kind == "violation" && d.Path == file && d.Line > 0 {
					anchored++
				}
			}
			if anchored == 0 {
				t.Fatalf("check reported no finding anchored at %s:\n%s", file, stdout)
			}

			stdout, stderr, code = runBin(t, root, os.Environ(), "context")
			if code != 0 || stderr != "" || !strings.Contains(stdout, "project domain ("+file+"): 1 context") {
				t.Fatalf("context: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
			}

			before := readFile(t, root, file)
			stdout, stderr, code = runBin(t, root, os.Environ(), "domain", "define", "bounded_context", "billing", "--definition", "Charging customers.")
			if code != 0 || stderr != "" || !strings.HasPrefix(stdout, "Defined ") {
				t.Fatalf("define: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
			}
			if after := readFile(t, root, file); after == before || !strings.Contains(after, c.contextMarker("billing")) {
				t.Fatalf("define did not record into %s:\n%s", file, after)
			}
			for _, name := range []string{"domain.arclint.yaml", "domain.arclint.yml", "domain.arclint.cml"} {
				if name != file {
					requireAbsent(t, root, name)
				}
			}
		})
	}
}

func TestDomainFileDefaultsToYAMLWhenNoDefaultNameExists(t *testing.T) {
	root := domainFixture(t)
	stdout, stderr, code := runBin(t, root, os.Environ(), "domain", "overview")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "No recorded Ubiquitous Language found at domain.arclint.yaml.\n") {
		t.Fatalf("overview: exit %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if out := mustRunDomain(t, root, "domain", "init", "--project", "shop"); !strings.Contains(out, "Initialized domain.arclint.yaml for project shop.") {
		t.Fatalf("init: %s", out)
	}
}

func TestIgnoredDefaultNamedFilesAreReportedOnStderrOnly(t *testing.T) {
	const yaml, yml, cml = "domain.arclint.yaml", "domain.arclint.yml", "domain.arclint.cml"
	for _, tc := range []struct {
		name    string
		rules   string
		files   []string
		read    string
		ignored []string
	}{
		{"YAML and CML", minimalDomainRules, []string{yaml, cml}, yaml, []string{cml}},
		{"YAML and .yml", minimalDomainRules, []string{yaml, yml}, yaml, []string{yml}},
		{".yml and CML", minimalDomainRules, []string{yml, cml}, yml, []string{cml}},
		{"all three", minimalDomainRules, []string{yaml, yml, cml}, yaml, []string{yml, cml}},
		{
			"a named file beside the YAML file",
			"runtime: [go]\ndomain:\n  file: docs/model.cml\nzones:\n  src:\n    paths: [\"src/**\"]\n",
			[]string{"docs/model.cml", yaml}, "docs/model.cml", []string{yaml},
		},
		{
			"a named file beside all three",
			"runtime: [go]\ndomain:\n  file: docs/model.cml\nzones:\n  src:\n    paths: [\"src/**\"]\n",
			[]string{"docs/model.cml", yaml, yml, cml}, "docs/model.cml", []string{yaml, yml, cml},
		},
		{
			"a default name that the ruleset names beside another",
			"runtime: [go]\ndomain:\n  file: domain.arclint.cml\nzones:\n  src:\n    paths: [\"src/**\"]\n",
			[]string{yaml, cml}, cml, []string{yaml},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := domainFileCase{rules: tc.rules, file: tc.read, other: "unused"}
			root := c.fixture(t)
			for _, file := range tc.files {
				switch {
				case file == tc.read:
					write(t, root, file, domainFileCase{file: file}.orderingDomain(t))
				case strings.HasSuffix(file, ".cml"):
					write(t, root, file, brokenCML)
				default:
					write(t, root, file, "version: 99\nnot a domain: [\n")
				}
			}
			// Only discovery passes files over and reports them; a file
			// the ruleset names is the project's own choice, and the files
			// beside it are ignored without a word.
			var want strings.Builder
			if tc.rules == minimalDomainRules {
				for _, ignored := range tc.ignored {
					want.WriteString(ignoredWarning(ignored, tc.read))
				}
			}

			// The domain file is the one read, and stdout stays valid JSON.
			stdout, stderr, code := runBin(t, root, os.Environ(), "domain", "overview", "--format", "json")
			var overview struct {
				Found  bool   `json:"found"`
				Source string `json:"source"`
			}
			if err := json.Unmarshal([]byte(stdout), &overview); err != nil || code != 0 || !overview.Found || overview.Source != tc.read {
				t.Fatalf("overview json: exit %d %v\nstdout: %s\nstderr: %s", code, err, stdout, stderr)
			}
			if stderr != want.String() {
				t.Fatalf("overview stderr:\n%q\nwant:\n%q", stderr, want.String())
			}

			// The exit code is the one the same project gives without the
			// ignored files, and stdout carries no warning.
			clean := c.fixture(t)
			write(t, clean, tc.read, domainFileCase{file: tc.read}.orderingDomain(t))
			cleanOut, cleanErr, cleanCode := runBin(t, clean, os.Environ(), "check", "--format", "json")
			stdout, stderr, code = runBin(t, root, os.Environ(), "check", "--format", "json")
			if cleanErr != "" || code != cleanCode || stdout != cleanOut || !json.Valid([]byte(stdout)) {
				t.Fatalf("check with ignored files: exit %d (clean %d)\nstdout: %s\nwant stdout: %s\nclean stderr: %s", code, cleanCode, stdout, cleanOut, cleanErr)
			}
			if stderr != want.String() {
				t.Fatalf("check stderr:\n%q\nwant:\n%q", stderr, want.String())
			}

			// Plain text output carries the warning on stderr alone as well.
			stdout, stderr, code = runBin(t, root, os.Environ(), "domain", "overview")
			if code != 0 || strings.Contains(stdout, "is ignored") || stderr != want.String() {
				t.Fatalf("plain overview: exit %d\nstdout: %s\nstderr: %q", code, stdout, stderr)
			}

			// A command that writes says so before it writes, and records
			// into the domain file.
			stdout, stderr, code = runBin(t, root, os.Environ(), "domain", "define", "bounded_context", "billing", "--definition", "Charging customers.")
			if code != 0 || !strings.HasPrefix(stdout, "Defined ") || stderr != want.String() {
				t.Fatalf("define: exit %d\nstdout: %s\nstderr: %q", code, stdout, stderr)
			}
			if !strings.Contains(readFile(t, root, tc.read), "billing") {
				t.Fatalf("define did not record into %s", tc.read)
			}
			for _, file := range tc.ignored {
				if strings.Contains(readFile(t, root, file), "billing") {
					t.Fatalf("define wrote into the ignored %s", file)
				}
			}
		})
	}
}

// The workflow hooks never block: with a default-named file ignored they
// still answer in JSON on stdout and exit 0, the warning on stderr.
func TestIgnoredDefaultNamedFileDoesNotBreakTheHooks(t *testing.T) {
	root := domainFixture(t)
	write(t, root, "domain.arclint.yaml", orderingDomain)
	write(t, root, "domain.arclint.cml", brokenCML)
	home := t.TempDir()
	env := append(os.Environ(), "WSL_DISTRO_NAME=", "HOME="+home, "CLAUDE_CONFIG_DIR=", "CODEX_HOME=")
	input, err := json.Marshal(map[string]any{"hook_event_name": "SessionStart", "session_id": "ignored", "cwd": root, "source": "startup"})
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runBinStdin(t, root, env, string(input), "agents", "hooks", "event")
	if code != 0 || !json.Valid([]byte(strings.TrimSpace(stdout))) || stderr != "" {
		t.Fatalf("hook event: exit %d\nstdout: %s\nstderr: %q", code, stdout, stderr)
	}
}

// init composes before any ruleset exists, so it has no domain file to
// settle and says nothing about the default-named files in its directory.
func TestInitSaysNothingAboutDefaultNamedFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, "domain.arclint.yaml", orderingDomain)
	write(t, root, "domain.arclint.cml", brokenCML)
	_, stderr, code := runBin(t, root, os.Environ(), "init")
	if code != 0 || strings.Contains(stderr, "is ignored") {
		t.Fatalf("init: exit %d\nstderr: %s", code, stderr)
	}
}

func TestConvertSettlesDomainFileByWhatDiscoveryFinds(t *testing.T) {
	t.Run("to a default name from a YAML project leaves no domain key", func(t *testing.T) {
		root := domainFixture(t)
		recordOrdering(t, root)
		before := mustRunDomain(t, root, "domain", "overview", "--format", "json")

		stdout := mustRunDomain(t, root, "domain", "convert", "domain.arclint.cml")
		if !strings.Contains(stdout, "rules.arclint.yaml names no domain file: domain.arclint.cml is found by its name.\n") {
			t.Fatalf("convert said:\n%s", stdout)
		}
		if got := readFile(t, root, "rules.arclint.yaml"); got != minimalDomainRules {
			t.Fatalf("rules.arclint.yaml changed:\n%s", got)
		}
		requireAbsent(t, root, "domain.arclint.yaml")
		stdout, stderr, code := runBin(t, root, os.Environ(), "domain", "overview", "--format", "json")
		if code != 0 || stderr != "" || strings.ReplaceAll(stdout, "domain.arclint.cml", "domain.arclint.yaml") != before {
			t.Fatalf("overview after convert: exit %d\nstdout: %s\nstderr: %s\nwant (as YAML): %s", code, stdout, stderr, before)
		}
	})

	t.Run("to a file outside the default names writes the key, and back removes it", func(t *testing.T) {
		root := domainFixture(t)
		recordOrdering(t, root)
		before := mustRunDomain(t, root, "domain", "overview", "--format", "json")

		stdout := mustRunDomain(t, root, "domain", "convert", "docs/model.cml")
		if !strings.Contains(stdout, "rules.arclint.yaml now names docs/model.cml under domain.file.\n") {
			t.Fatalf("convert said:\n%s", stdout)
		}
		if !strings.Contains(readFile(t, root, "rules.arclint.yaml"), "domain:\n  file: \"docs/model.cml\"\n") {
			t.Fatalf("rules.arclint.yaml does not name docs/model.cml:\n%s", readFile(t, root, "rules.arclint.yaml"))
		}
		if got := mustRunDomain(t, root, "domain", "overview", "--format", "json"); strings.ReplaceAll(got, "docs/model.cml", "domain.arclint.yaml") != before {
			t.Fatalf("overview after convert:\n%s\nwant (as YAML):\n%s", got, before)
		}

		stdout = mustRunDomain(t, root, "domain", "convert", "domain.arclint.cml")
		if !strings.Contains(stdout, "rules.arclint.yaml no longer names a domain file: domain.arclint.cml is found by its name.\n") {
			t.Fatalf("convert back said:\n%s", stdout)
		}
		if got := readFile(t, root, "rules.arclint.yaml"); got != minimalDomainRules {
			t.Fatalf("the key stayed:\n%s", got)
		}
		requireAbsent(t, root, "docs/model.cml")
		if got := mustRunDomain(t, root, "domain", "overview", "--format", "json"); strings.ReplaceAll(got, "domain.arclint.cml", "domain.arclint.yaml") != before {
			t.Fatalf("overview after convert back:\n%s\nwant (as YAML):\n%s", got, before)
		}
	})

	t.Run("beside a stale file that discovery would pick writes the key", func(t *testing.T) {
		root := domainFixture(t)
		recordOrdering(t, root)
		write(t, root, "domain.arclint.yml", "version: 1\nproject: stale\n")
		// The YAML file is the domain file and the .yml beside it is ignored.
		if _, stderr, code := runBin(t, root, os.Environ(), "domain", "overview"); code != 0 || stderr != ignoredWarning("domain.arclint.yml", "domain.arclint.yaml") {
			t.Fatalf("before convert: exit %d\nstderr: %q", code, stderr)
		}

		stdout := mustRunDomain(t, root, "domain", "convert", "domain.arclint.cml")
		if !strings.Contains(stdout, "rules.arclint.yaml now names domain.arclint.cml under domain.file.\n") {
			t.Fatalf("convert said:\n%s", stdout)
		}
		if !strings.Contains(readFile(t, root, "rules.arclint.yaml"), "domain:\n  file: \"domain.arclint.cml\"\n") {
			t.Fatalf("rules.arclint.yaml does not name domain.arclint.cml:\n%s", readFile(t, root, "rules.arclint.yaml"))
		}
		requireAbsent(t, root, "domain.arclint.yaml")
		// The named file is read; the stale .yml stays beside it, ignored
		// without a word because the ruleset names the project's choice.
		stdout, stderr, code := runBin(t, root, os.Environ(), "domain", "overview")
		if code != 0 || !strings.Contains(stdout, "Source: domain.arclint.cml\n") || stderr != "" {
			t.Fatalf("overview after convert: exit %d\nstdout: %s\nstderr: %q", code, stdout, stderr)
		}
	})

	t.Run("from a ruleset that names the default file removes the key", func(t *testing.T) {
		root := domainFixture(t)
		write(t, root, "rules.arclint.yaml", "runtime: [go]\ndomain:\n  file: domain.arclint.yaml\nzones:\n  src:\n    paths: [\"src/**\"]\n")
		recordOrdering(t, root)
		mustRunDomain(t, root, "domain", "convert", "domain.arclint.cml")
		if got := readFile(t, root, "rules.arclint.yaml"); got != minimalDomainRules {
			t.Fatalf("the key stayed:\n%s", got)
		}
		if out := mustRunDomain(t, root, "domain", "overview"); !strings.Contains(out, "Source: domain.arclint.cml\n") {
			t.Fatalf("overview:\n%s", out)
		}
	})
}

// init writes the key for every file but domain.arclint.yaml: at init
// time no domain file exists yet, so discovery would choose the YAML one.
func TestInitDomainFlagNamesAFileDiscoveryWouldNotChoose(t *testing.T) {
	for _, file := range []string{"domain.arclint.cml", "domain.arclint.yml"} {
		root := t.TempDir()
		if _, stderr, code := runBin(t, root, os.Environ(), "init", "--domain", file); code != 0 {
			t.Fatalf("init --domain %s: exit %d\n%s", file, code, stderr)
		}
		if got := readFile(t, root, "rules.arclint.yaml"); !strings.Contains(got, "domain:\n") || !strings.Contains(got, "\n  file: "+file+"\n") {
			t.Fatalf("init --domain %s did not name the file:\n%s", file, got)
		}
		if out := mustRunDomain(t, root, "domain", "init"); !strings.Contains(out, "Initialized "+file) {
			t.Fatalf("domain init after init --domain %s: %s", file, out)
		}
	}
}
