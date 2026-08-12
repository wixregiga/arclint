package config

import (
	"strings"
	"testing"
)

func TestModuleForms(t *testing.T) {
	rs, err := parse(t, `runtime: [go]
modules:
  terse: ["a/**"]
  documented:
    paths: ["b/**", "c/d.go"]
    description: "What b is for."
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := rs.Modules["terse"]; len(got.Paths) != 1 || got.Paths[0] != "a/**" || got.Description != "" {
		t.Errorf("terse form: %+v", got)
	}
	if got := rs.Modules["documented"]; len(got.Paths) != 2 || got.Description != "What b is for." {
		t.Errorf("documented form: %+v", got)
	}
}

func TestModuleFormRejections(t *testing.T) {
	cases := []struct {
		name, yaml, want string
	}{
		{
			"mapping without paths",
			"runtime: [go]\nmodules:\n  a: {description: \"no globs\"}\n",
			"missing property 'paths'",
		},
		{
			"scalar value",
			"runtime: [go]\nmodules:\n  a: \"a/**\"\n",
			"schema validation failed",
		},
		{
			"unknown key in mapping",
			"runtime: [go]\nmodules:\n  a: {paths: [\"a/**\"], describe: \"typo\"}\n",
			"schema validation failed",
		},
	}
	for _, tc := range cases {
		if _, err := parse(t, tc.yaml); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want containing %q", tc.name, err, tc.want)
		}
	}
}
