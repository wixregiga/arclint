package yamlrule_test

import (
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

const timeLimitBound = `
runtime: [go]
extends:
  - pattern: acme/hexagonal@1.0.0
    bind:
      core: internal/core/**
      ports: internal/ports/**
      adapters: internal/adapters/**
`

func limitOf(t *testing.T, r rule.Rule) time.Duration {
	t.Helper()
	params, ok := r.Params().(rule.ExtensionParams)
	if !ok {
		t.Fatalf("rule %s is not an extension rule", r.ID())
	}
	return params.Limit().Duration()
}

// A uses Rule sets its own time_limit, an Override sets one for a
// distributed uses Rule, and a Rule without either keeps the default.
func TestLoadTimeLimit(t *testing.T) {
	p := loadSamplePattern(t)
	cfg := mustLoad(t, timeLimitBound+`rules:
  acme/hexagonal:core/checked:
    time_limit: 30s
  core/slow:
    on: core
    uses: acme/check
    time_limit: 1m30s
  core/default:
    on: core
    uses: acme/check
`, p)
	if got := limitOf(t, ruleByID(t, cfg, "acme/hexagonal:core/checked")); got != 30*time.Second {
		t.Errorf("override time_limit = %s, want 30s", got)
	}
	if got := limitOf(t, ruleByID(t, cfg, "core/slow")); got != 90*time.Second {
		t.Errorf("rule time_limit = %s, want 1m30s", got)
	}
	if got := limitOf(t, ruleByID(t, cfg, "core/default")); got != rule.DefaultTimeLimit.Duration() {
		t.Errorf("default time_limit = %s, want %s", got, rule.DefaultTimeLimit)
	}
}

func TestLoadRejectsMisplacedOrInvalidTimeLimit(t *testing.T) {
	p := loadSamplePattern(t)
	for name, tc := range map[string]struct{ rules, want string }{
		"on a native rule": {`  core/snake:
    on: core
    naming: snake_case
    time_limit: 10s
`, "time_limit applies only to an extension (uses) rule"},
		"override of a native rule": {`  acme/hexagonal:core/stdlib-only:
    time_limit: 10s
`, "time_limit applies only to an extension (uses) rule"},
		"not a duration": {`  core/slow:
    on: core
    uses: acme/check
    time_limit: ten seconds
`, `time_limit: time limit "ten seconds": not a duration`},
		"bare number": {`  core/slow:
    on: core
    uses: acme/check
    time_limit: 10
`, "time_limit: expected a string"},
		"empty on a uses rule": {`  core/slow:
    on: core
    uses: acme/check
    time_limit: ""
`, `time_limit: time limit "": not a duration`},
		"empty on a native rule": {`  core/snake:
    on: core
    naming: snake_case
    time_limit: ""
`, `time_limit: time limit "": not a duration`},
		"empty on an override": {`  acme/hexagonal:core/checked:
    time_limit: ""
`, `time_limit: time limit "": not a duration`},
		"zero": {`  acme/hexagonal:core/checked:
    time_limit: 0s
`, "must be a positive duration"},
		"negative": {`  core/slow:
    on: core
    uses: acme/check
    time_limit: -5s
`, "must be a positive duration"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadString(t, timeLimitBound+"rules:\n"+tc.rules, p)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}
