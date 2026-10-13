package yamlrule_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
)

// A domain file that reaches the ruleset through a link is refused,
// whichever of the two is the link; any other location, and a ruleset
// that does not exist yet, is not.
func TestCheckDomainFileLocationRefusesTheRulesetThroughALink(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	model, err := vocab.NewDomainRecording("model.yaml")
	if err != nil {
		t.Fatal(err)
	}
	rules := filepath.Join(root, "rules.arclint.yaml")
	if err := yamlrule.CheckDomainFileLocation(rules, model, rules); err != nil {
		t.Fatalf("a ruleset that does not exist yet was reached: %v", err)
	}
	if err := os.WriteFile(rules, []byte("zones: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := yamlrule.CheckDomainFileLocation(rules, model, rules); !errors.Is(err, yamlrule.ErrDomainFileIsRuleset) || !strings.Contains(err.Error(), `"model.yaml" reaches rules.arclint.yaml`) {
		t.Fatalf("the ruleset as the domain file: %v", err)
	}
	if err := yamlrule.CheckDomainFileLocation(rules, model, filepath.Join(root, "domain.arclint.yaml")); err != nil {
		t.Fatalf("another file was refused: %v", err)
	}

	linked := t.TempDir()
	actual := filepath.Join(root, "config", "rules.yaml")
	if err := os.MkdirAll(filepath.Dir(actual), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(actual, []byte("zones: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(actual, filepath.Join(linked, "rules.arclint.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := yamlrule.CheckDomainFileLocation(filepath.Join(linked, "rules.arclint.yaml"), model, actual); !errors.Is(err, yamlrule.ErrDomainFileIsRuleset) {
		t.Fatalf("a domain file at the linked ruleset's target: %v", err)
	}
}

// A hard link is a second name for the ruleset's own file, so a domain
// file that is one is refused although no symbolic link joins the names.
func TestCheckDomainFileLocationRefusesAHardLinkToTheRuleset(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	model, err := vocab.NewDomainRecording("model.yaml")
	if err != nil {
		t.Fatal(err)
	}
	rules := filepath.Join(root, "rules.arclint.yaml")
	if err := os.WriteFile(rules, []byte("zones: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "model.yaml")
	if err := os.Link(rules, alias); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	if err := yamlrule.CheckDomainFileLocation(rules, model, alias); !errors.Is(err, yamlrule.ErrDomainFileIsRuleset) {
		t.Fatalf("a hard link to the ruleset as the domain file: %v", err)
	}
	copied := filepath.Join(root, "copy.yaml")
	if err := os.WriteFile(copied, []byte("zones: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := yamlrule.CheckDomainFileLocation(rules, model, copied); err != nil {
		t.Fatalf("a separate file with the same content was refused: %v", err)
	}
}
