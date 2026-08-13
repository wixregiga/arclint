package engine_test

// M9: uniform except clauses. One fixture tree fires rules from three
// clause families (consumes, invariant, graph); except entries then
// suppress each finding by anchor path while the rules keep firing for
// the other file.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/engine"
)

const exceptBaseline = `runtime: [go]
modules:
  entities: ["internal/entities/**"]
  shared: ["internal/shared/**"]

contracts:
  entities:
    consumes:
      internal: []
    invariants:
      - id: no-todo
        kind: content
        files: "internal/entities/**"
        must_not: ['TODO']

dependencies:
  - id: shared-protected
    kind: protected
    module: shared
    allow: []
`

const exceptApplied = `runtime: [go]
modules:
  entities: ["internal/entities/**"]
  shared: ["internal/shared/**"]

contracts:
  entities:
    consumes:
      internal: []
      except:
        - paths: ["internal/entities/legacy.go"]
          reason: "grandfathered shared access; remove with the reports rewrite"
    invariants:
      - id: no-todo
        kind: content
        files: "internal/entities/**"
        must_not: ['TODO']
        except:
          - paths: ["internal/entities/legacy.go"]
            reason: "tracked in the rewrite ticket"

dependencies:
  - id: shared-protected
    kind: protected
    module: shared
    allow: []
    except:
      - paths: ["internal/entities/legacy.go"]
        reason: "grandfathered shared access"
`

// writeExceptRepo materializes the fixture: two entity files import
// shared (consumes + protected findings) and carry TODOs (content
// findings).
func writeExceptRepo(t *testing.T, rules string) *config.RuleSet {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                      "module test.local/case\n\ngo 1.24\n",
		"internal/shared/db.go":       "package shared\n",
		"internal/entities/user.go":   "package entities\n\nimport _ \"test.local/case/internal/shared\"\n\n// TODO fix\n",
		"internal/entities/legacy.go": "package entities\n\nimport _ \"test.local/case/internal/shared\"\n\n// TODO fix\n",
		"rules.yaml":                  rules,
	}
	for rel, content := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := config.Load(filepath.Join(root, "rules.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func countByID(res *engine.Result) map[string]int {
	out := map[string]int{}
	for _, v := range res.Violations {
		out[v.RuleID]++
	}
	return out
}

func TestExceptSuppressesByAnchorAcrossClauseKinds(t *testing.T) {
	res, err := engine.Check(writeExceptRepo(t, exceptBaseline))
	if err != nil {
		t.Fatal(err)
	}
	base := countByID(res)
	if base["entities.consumes.internal"] != 2 || base["no-todo"] != 2 || base["shared-protected"] != 2 {
		t.Fatalf("baseline: %v", base)
	}
	if len(res.Suppressed) != 0 {
		t.Fatalf("baseline suppressed = %d", len(res.Suppressed))
	}

	res, err = engine.Check(writeExceptRepo(t, exceptApplied))
	if err != nil {
		t.Fatal(err)
	}
	got := countByID(res)
	if got["entities.consumes.internal"] != 1 || got["no-todo"] != 1 || got["shared-protected"] != 1 {
		t.Fatalf("after except: %v", got)
	}
	for _, v := range res.Violations {
		if v.Path == "internal/entities/legacy.go" {
			t.Errorf("legacy.go finding survived: %+v", v)
		}
		if v.Path == "internal/entities/user.go" && v.Capability == "" {
			t.Errorf("kept finding lost its capability: %+v", v)
		}
	}
	if len(res.Suppressed) != 3 {
		t.Fatalf("suppressed = %d, want 3", len(res.Suppressed))
	}
	for _, v := range res.Suppressed {
		if !v.Suppressed || v.SuppressedReason == "" {
			t.Errorf("suppressed finding not marked with its reason: %+v", v)
		}
		if v.Path != "internal/entities/legacy.go" {
			t.Errorf("wrong anchor suppressed: %+v", v)
		}
	}
}

func TestExceptValidation(t *testing.T) {
	cases := []struct {
		name, except string
	}{
		{"empty reason", "      except:\n        - paths: [\"a/**\"]\n          reason: \"\"\n"},
		{"no paths", "      except:\n        - paths: []\n          reason: \"why\"\n"},
		{"bad glob", "      except:\n        - paths: [\"[\"]\n          reason: \"why\"\n"},
	}
	for _, tc := range cases {
		rules := "runtime: [go]\nmodules:\n  a: [\"a/**\"]\ncontracts:\n  a:\n    consumes:\n      internal: []\n" + tc.except
		if _, err := config.Parse([]byte(rules), "/test/rules.yaml"); err == nil {
			t.Errorf("%s: accepted", tc.name)
		}
	}
}
