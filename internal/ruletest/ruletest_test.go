package ruletest_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/ruletest"
)

// target is a minimal inline ruleset: one module, one content ban.
func target() ruletest.Target {
	rules := []byte(`runtime: [go]
modules:
  all: ["**"]
contracts:
  all:
    invariants:
      - id: "t:no-todo"
        kind: content
        files: "**/*.go"
        must_not: ['TODO']
`)
	return ruletest.Target{
		RulesFor: func([]string) ([]byte, error) { return rules, nil },
	}
}

func TestStrictMatching(t *testing.T) {
	pass := ruletest.Run(ruletest.Case{
		Name:  "violation-expected",
		Files: map[string]string{"a.go": "package a\n\n// TODO fix\n"},
		Expect: []ruletest.Expect{{
			Rule: "t:no-todo", Path: "a.go", Line: 3,
		}},
	}, target())
	if !pass.Pass {
		t.Fatalf("expected pass: %+v", pass)
	}

	missing := ruletest.Run(ruletest.Case{
		Name:   "expectation-unmet",
		Files:  map[string]string{"a.go": "package a\n"},
		Expect: []ruletest.Expect{{Rule: "t:no-todo", Path: "a.go"}},
	}, target())
	if missing.Pass || len(missing.Missing) != 1 {
		t.Fatalf("missing not reported: %+v", missing)
	}

	unexpected := ruletest.Run(ruletest.Case{
		Name:  "unexpected-finding",
		Files: map[string]string{"a.go": "package a\n\n// TODO fix\n"},
	}, target())
	if unexpected.Pass || len(unexpected.Unexpected) != 1 {
		t.Fatalf("unexpected not reported: %+v", unexpected)
	}
}

func TestParseRejectsMalformedCases(t *testing.T) {
	if _, err := ruletest.Parse([]byte("case: x\n"), "inline"); err == nil ||
		!strings.Contains(err.Error(), "files") {
		t.Errorf("missing files accepted: %v", err)
	}
	if _, err := ruletest.Parse([]byte("case: x\nfiles: {\"a.go\": \"package a\"}\nexpect: [{path: p}]\n"), "inline"); err == nil ||
		!strings.Contains(err.Error(), "rule and path") {
		t.Errorf("expectation without rule accepted: %v", err)
	}
}

func TestManifestInjectionRespectsCaseFiles(t *testing.T) {
	// A case-provided go.mod must win over the injected default: the
	// module path differs, so the internal import only resolves if the
	// case's manifest was used.
	res := ruletest.Run(ruletest.Case{
		Name: "own-manifest",
		Files: map[string]string{
			"go.mod":   "module custom.local/mod\n\ngo 1.24\n",
			"a/a.go":   "package a\n\nimport _ \"custom.local/mod/b\"\n",
			"b/b.go":   "package b\n",
			"c/c.go":   "package c\n\n// TODO here\n",
			"d/ok.txt": "not source\n",
		},
		Expect: []ruletest.Expect{{Rule: "t:no-todo", Path: "c/c.go", Line: 3}},
	}, target())
	if !res.Pass {
		t.Fatalf("case failed, so the custom go.mod was not honored: %+v", res)
	}
}
