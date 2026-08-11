package engine_test

// Acceptance tests for the three driving rules of the design proposal,
// written before the engine (M1 gate 1). Each fixture under
// testdata/fixtures is a self-contained repo root with its own rules.yaml.

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/config"
	"github.com/wixregiga/arclint/internal/engine"
	"github.com/wixregiga/arclint/internal/report"
)

type wantViolation struct {
	ruleID   string
	contract report.Contract
	blame    report.Blame
	path     string
	line     int // 0 means the violation must not be line-anchored
}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func runFixture(t *testing.T, dir string) *engine.Result {
	t.Helper()
	rs, err := config.Load(filepath.Join(fixtureRoot(t), dir, "rules.yaml"))
	if err != nil {
		t.Fatalf("load %s: %v", dir, err)
	}
	res, err := engine.Check(rs)
	if err != nil {
		t.Fatalf("check %s: %v", dir, err)
	}
	return res
}

func TestFixtures(t *testing.T) {
	cases := []struct {
		dir  string
		want []wantViolation
	}{
		{
			// Driving rule 1: third-party import inside internal/entities/
			// is a consumes break, blamed on the consumer, anchored to the
			// import line.
			dir: "external-import-violation",
			want: []wantViolation{{
				ruleID:   "entities.consumes.external",
				contract: report.ContractConsumes,
				blame:    report.BlameConsumer,
				path:     "internal/entities/user.go",
				line:     6,
			}},
		},
		{dir: "external-import-clean"},
		{
			// Driving rule 2: a feature directory with no
			// RegistryFactory.Register("<feature>") match in the registry
			// module is a provides break, blamed on the provider.
			dir: "registration-violation",
			want: []wantViolation{{
				ruleID:   "features.provides.registration[0]",
				contract: report.ContractProvides,
				blame:    report.BlameProvider,
				path:     "internal/features/beta",
			}},
		},
		{dir: "registration-clean"},
		{
			// Driving rule 3: an entity substrate file with no counterpart
			// substrate under internal/setup/** is a provides break, blamed
			// on the provider.
			dir: "correspondence-violation",
			want: []wantViolation{{
				ruleID:   "entities.provides.correspondence[0]",
				contract: report.ContractProvides,
				blame:    report.BlameProvider,
				path:     "internal/entities/user_redis.go",
			}},
		},
		{dir: "correspondence-clean"},
	}

	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			res := runFixture(t, tc.dir)
			if len(res.Violations) != len(tc.want) {
				t.Fatalf("got %d violations, want %d: %+v",
					len(res.Violations), len(tc.want), res.Violations)
			}
			for i, want := range tc.want {
				got := res.Violations[i]
				if got.RuleID != want.ruleID {
					t.Errorf("[%d] ruleId = %q, want %q", i, got.RuleID, want.ruleID)
				}
				if got.Contract != want.contract {
					t.Errorf("[%d] contract = %q, want %q", i, got.Contract, want.contract)
				}
				if got.Blame != want.blame {
					t.Errorf("[%d] blame = %q, want %q", i, got.Blame, want.blame)
				}
				if got.Severity != report.SeverityError {
					t.Errorf("[%d] severity = %q, want error", i, got.Severity)
				}
				if got.Path != want.path {
					t.Errorf("[%d] path = %q, want %q", i, got.Path, want.path)
				}
				if want.line == 0 && got.Line != nil {
					t.Errorf("[%d] line = %d, want none", i, *got.Line)
				}
				if want.line != 0 && (got.Line == nil || *got.Line != want.line) {
					t.Errorf("[%d] line = %v, want %d", i, got.Line, want.line)
				}
				if got.Message == "" {
					t.Errorf("[%d] empty message", i)
				}
				if got.FixHint == "" {
					t.Errorf("[%d] empty fixHint", i)
				}
			}
		})
	}
}

// TestDeterminism requires byte-identical results across repeated runs.
func TestDeterminism(t *testing.T) {
	dirs := []string{"external-import-violation", "registration-violation", "correspondence-violation"}
	for _, dir := range dirs {
		first := runFixture(t, dir)
		for i := 0; i < 2; i++ {
			again := runFixture(t, dir)
			if !reflect.DeepEqual(first.Violations, again.Violations) {
				t.Fatalf("%s: run %d differs from first run", dir, i+2)
			}
		}
	}
}
