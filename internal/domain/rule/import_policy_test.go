package rule_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// TestImportPathListIsNeverEmpty enforces ImportPolicy's recorded
// invariant: a list holds at least one path, and forbid says none.
func TestImportPathListIsNeverEmpty(t *testing.T) {
	for _, paths := range [][]string{nil, {}, {""}, {" effect"}, {"effect", "effect"}} {
		if _, err := rule.NewImportPathList(paths); err == nil {
			t.Errorf("NewImportPathList(%q) accepted", paths)
		}
	}
	if _, err := rule.NewImportPathList([]string{"effect", "github.com/pkg/errors", "sqlalchemy.orm"}); err != nil {
		t.Errorf("a list of real import paths: %v", err)
	}
}

// TestImportPathListMatchesThePathAsWritten proves a listed path
// permits only an import spelled exactly that way, in each language's
// own separator, while allow and forbid keep their meaning.
func TestImportPathListMatchesThePathAsWritten(t *testing.T) {
	only, err := rule.NewImportPathList([]string{"effect", "sqlalchemy.orm", "github.com/pkg/errors"})
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{
		"effect": true, "effect/http": false, "effect/Effect": false, "effects": false,
		"sqlalchemy.orm": true, "sqlalchemy": false, "sqlalchemy.orm.session": false,
		"github.com/pkg/errors": true, "github.com/pkg/errors/v2": false,
	} {
		if got := only.Permits(path); got != want {
			t.Errorf("Permits(%q) = %v, want %v", path, got, want)
		}
	}
	if !rule.ImportAllow.Permits("anything") || rule.ImportForbid.Permits("effect") {
		t.Errorf("allow must permit every path and forbid none")
	}
	if !only.Lists() || !only.Restricts() || only.Forbids() {
		t.Errorf("a list restricts without forbidding the whole class")
	}
	if only.String() != "[effect, sqlalchemy.orm, github.com/pkg/errors]" {
		t.Errorf("String() = %q, want the paths as written", only.String())
	}
}

// TestConsumesConstraintTakesAListForExternalOnly checks the
// Constraint around the policy: a list alone is a restriction, its
// proposition names the paths, and the standard library takes no list.
func TestConsumesConstraintTakesAListForExternalOnly(t *testing.T) {
	only, err := rule.NewImportPathList([]string{"effect"})
	if err != nil {
		t.Fatal(err)
	}
	c := rule.ConsumesConstraint{External: only}
	if err := c.Validate(); err != nil {
		t.Errorf("external list alone: %v", err)
	}
	if got := c.Proposition(); !strings.Contains(got, "uses only the external imports [effect]") {
		t.Errorf("Proposition() = %q", got)
	}
	if err := (rule.ConsumesConstraint{Stdlib: only}).Validate(); err == nil {
		t.Errorf("a stdlib list must be rejected")
	}
}
