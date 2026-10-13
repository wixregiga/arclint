package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitRejectsRulesetAsDomainTarget(t *testing.T) {
	for _, target := range []string{"rules.arclint.yaml", "./docs/../rules.arclint.yaml"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			_, stderr, code := runBin(t, root, os.Environ(), "init", "--domain", target)
			if code == 0 || !strings.Contains(stderr, "never-the-ruleset") {
				t.Fatalf("init accepted the ruleset as the domain: exit %d: %s", code, stderr)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("rejected init wrote files: %v, %v", entries, err)
			}
		})
	}
	t.Run("symlink alias of existing ruleset", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "rules.arclint.yaml", minimalDomainRules)
		if err := os.Symlink("rules.arclint.yaml", filepath.Join(root, "model.yaml")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, stderr, code := runBin(t, root, os.Environ(), "init", "--force", "--domain", "model.yaml")
		if code == 0 || !strings.Contains(stderr, "never-the-ruleset") {
			t.Fatalf("init accepted a ruleset alias: exit %d: %s", code, stderr)
		}
		data, err := os.ReadFile(filepath.Join(root, "rules.arclint.yaml"))
		if err != nil || string(data) != minimalDomainRules {
			t.Fatalf("rejected init overwrote the ruleset: %v\n%s", err, data)
		}
	})
}

func TestRulesetCannotNameItselfAsDomain(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "domain:\n  file: ./rules.arclint.yaml\n"+minimalDomainRules)
	_, stderr, code := runBin(t, root, os.Environ(), "domain", "overview")
	if code == 0 || !strings.Contains(stderr, "never-the-ruleset") {
		t.Fatalf("ruleset names itself: exit %d: %s", code, stderr)
	}
}

// A hard link gives the ruleset's own file a second name; naming that
// name as the domain file is refused like the ruleset's own name.
func TestRulesetCannotNameAHardLinkToItselfAsDomain(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "domain:\n  file: model.yaml\n"+minimalDomainRules)
	if err := os.Link(filepath.Join(root, "rules.arclint.yaml"), filepath.Join(root, "model.yaml")); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	_, stderr, code := runBin(t, root, os.Environ(), "domain", "overview")
	if code == 0 || !strings.Contains(stderr, "never-the-ruleset") {
		t.Fatalf("a hard link to the ruleset as the domain file: exit %d: %s", code, stderr)
	}
}

func TestDomainTranslationRejectsRulesetTarget(t *testing.T) {
	for _, action := range []string{"export", "convert"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "rules.arclint.yaml", minimalDomainRules)
			// Invalid source proves target validation happens before reading it.
			write(t, root, "domain.arclint.yaml", "not a domain document")
			_, stderr, code := runBin(t, root, os.Environ(), "domain", action, "./rules.arclint.yaml")
			if code != 2 || !strings.Contains(stderr, "never-the-ruleset") {
				t.Fatalf("%s target: want usage error, got exit %d: %s", action, code, stderr)
			}
			data, err := os.ReadFile(filepath.Join(root, "rules.arclint.yaml"))
			if err != nil || string(data) != minimalDomainRules {
				t.Fatalf("ruleset changed: %v\n%s", err, data)
			}
		})
	}
	for _, action := range []string{"export", "convert"} {
		t.Run("ruleset alias/"+action, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "rules.arclint.yaml", minimalDomainRules)
			write(t, root, "domain.arclint.yaml", "invalid source proves validation comes first")
			if err := os.Symlink("rules.arclint.yaml", filepath.Join(root, "model.yaml")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			_, stderr, code := runBin(t, root, os.Environ(), "domain", action, "model.yaml")
			if code != 2 || !strings.Contains(stderr, "never-the-ruleset") {
				t.Fatalf("%s accepted ruleset alias: exit %d: %s", action, code, stderr)
			}
		})
	}
}

// A default-named file that is a link to the ruleset is never taken for
// the project's domain file, so no domain command reads or writes the
// ruleset as a language.
func TestDomainDiscoveryRefusesALinkToTheRuleset(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", minimalDomainRules)
	if err := os.Symlink("rules.arclint.yaml", filepath.Join(root, "domain.arclint.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, args := range [][]string{
		{"domain", "overview"},
		{"domain", "define", "bounded_context", "payments", "--definition", "Taking payments."},
	} {
		_, stderr, code := runBin(t, root, os.Environ(), args...)
		if code == 0 || !strings.Contains(stderr, "never-the-ruleset") {
			t.Errorf("%v used the ruleset as the domain file: exit %d: %s", args, code, stderr)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "rules.arclint.yaml"))
	if err != nil || string(data) != minimalDomainRules {
		t.Fatalf("the ruleset changed: %v\n%s", err, data)
	}
}

func TestDomainRejectsDirectorySymlinkOutsideRepository(t *testing.T) {
	for _, extension := range []string{"yaml", "cml"} {
		t.Run(extension, func(t *testing.T) {
			root, external := t.TempDir(), t.TempDir()
			if err := os.Symlink(external, filepath.Join(root, "model")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			c := domainFileCase{file: "model/config." + extension}
			content := c.orderingDomain(t)
			write(t, external, "config."+extension, content)
			write(t, root, "rules.arclint.yaml", "domain:\n  file: "+c.file+"\n"+minimalDomainRules)
			for _, args := range [][]string{
				{"domain", "overview"},
				{"domain", "define", "bounded_context", "payments", "--definition", "Taking payments."},
			} {
				_, stderr, code := runBin(t, root, os.Environ(), args...)
				if code == 0 || !strings.Contains(stderr, "inside-the-repository") || !strings.Contains(stderr, external) {
					t.Errorf("%v accessed the external domain: exit %d: %s", args, code, stderr)
				}
			}
			data, err := os.ReadFile(filepath.Join(external, "config."+extension))
			if err != nil || string(data) != content {
				t.Fatalf("external recording changed: %v\n%s", err, data)
			}
		})
	}
}

func TestDomainTranslationRejectsExternalSymlinkTarget(t *testing.T) {
	for _, extension := range []string{"yaml", "cml"} {
		for _, action := range []string{"export", "convert"} {
			for _, missing := range []bool{false, true} {
				t.Run(extension+"/"+action+"/"+map[bool]string{false: "existing", true: "missing"}[missing], func(t *testing.T) {
					root, external := t.TempDir(), t.TempDir()
					write(t, root, "rules.arclint.yaml", minimalDomainRules)
					write(t, root, "domain.arclint.yaml", orderingDomain)
					if err := os.Symlink(external, filepath.Join(root, "model")); err != nil {
						t.Skipf("symlinks unavailable: %v", err)
					}
					c := domainFileCase{file: "model/domain." + extension}
					content := c.orderingDomain(t)
					if !missing {
						write(t, external, "domain."+extension, content)
					}
					_, stderr, code := runBin(t, root, os.Environ(), "domain", action, c.file)
					if code == 0 || !strings.Contains(stderr, "inside-the-repository") || !strings.Contains(stderr, external) {
						t.Fatalf("external %s: exit %d: %s", action, code, stderr)
					}
					for path, want := range map[string]string{"rules.arclint.yaml": minimalDomainRules, "domain.arclint.yaml": orderingDomain} {
						data, err := os.ReadFile(filepath.Join(root, path))
						if err != nil || string(data) != want {
							t.Fatalf("%s changed: %v\n%s", path, err, data)
						}
					}
					entries, err := os.ReadDir(external)
					if err != nil {
						t.Fatal(err)
					}
					if missing && len(entries) != 0 {
						t.Fatalf("external files created: %v", entries)
					}
					if !missing {
						data, err := os.ReadFile(filepath.Join(external, "domain."+extension))
						if err != nil || string(data) != content || len(entries) != 1 {
							t.Fatalf("external content changed or temp leaked: %v\n%s", err, data)
						}
					}
				})
			}
		}
	}
}

func TestDomainDiscoveryRejectsExternalFileSymlink(t *testing.T) {
	for _, path := range []string{"domain.arclint.yaml", "domain.arclint.yml", "domain.arclint.cml"} {
		t.Run(path, func(t *testing.T) {
			root, external := t.TempDir(), t.TempDir()
			write(t, root, "rules.arclint.yaml", minimalDomainRules)
			write(t, external, path, "external content")
			if err := os.Symlink(filepath.Join(external, path), filepath.Join(root, path)); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			_, stderr, code := runBin(t, root, os.Environ(), "domain", "overview")
			if code == 0 || !strings.Contains(stderr, "inside-the-repository") || !strings.Contains(stderr, external) {
				t.Fatalf("discovery accessed external file: exit %d: %s", code, stderr)
			}
		})
	}
}

func TestDomainConvertFromInternalFileSymlink(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", minimalDomainRules)
	write(t, root, "actual/model.yaml", orderingDomain)
	if err := os.Symlink("actual/model.yaml", filepath.Join(root, "domain.arclint.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	mustRunDomain(t, root, "domain", "convert", "domain.arclint.cml")
	if _, err := os.Lstat(filepath.Join(root, "domain.arclint.yaml")); !os.IsNotExist(err) {
		t.Fatalf("old recording's directory entry remains: %v", err)
	}
	if stdout := mustRunDomain(t, root, "domain", "overview"); !strings.Contains(stdout, "Source: domain.arclint.cml") {
		t.Fatalf("converted domain cannot be discovered: %s", stdout)
	}
}

// A target that is the project's domain file under another name is
// refused before anything is written: a symbolic link, or a hard link,
// which stands in here for a name that differs only in case on a
// case-insensitive file system, because both leave two names for one
// file.
func TestDomainTranslationRejectsSourceAlias(t *testing.T) {
	aliases := map[string]func(oldname, newname string) error{
		"symlink":   func(oldname, newname string) error { return os.Symlink(filepath.Base(oldname), newname) },
		"hard link": os.Link,
	}
	for name, alias := range aliases {
		for _, action := range []string{"export", "convert"} {
			t.Run(name+"/"+action, func(t *testing.T) {
				testSourceAliasRefused(t, alias, action)
			})
		}
	}
}

func testSourceAliasRefused(t *testing.T, alias func(oldname, newname string) error, action string) {
	t.Helper()
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", minimalDomainRules)
	write(t, root, "domain.arclint.yaml", orderingDomain)
	if err := alias(filepath.Join(root, "domain.arclint.yaml"), filepath.Join(root, "model.yaml")); err != nil {
		t.Skipf("links unavailable: %v", err)
	}
	_, stderr, code := runBin(t, root, os.Environ(), "domain", action, "model.yaml")
	if code != 2 || !strings.Contains(stderr, "resolves to the project's domain file") {
		t.Fatalf("source alias was accepted: exit %d: %s", code, stderr)
	}
	data, err := os.ReadFile(filepath.Join(root, "domain.arclint.yaml"))
	if err != nil || string(data) != orderingDomain {
		t.Fatalf("source changed: %v\n%s", err, data)
	}
}
