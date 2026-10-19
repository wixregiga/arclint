package main

import "testing"

func TestRepositoryPathIsTheCheckOrMapServeArgument(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, "."},
		{[]string{"check"}, "."},
		{[]string{"check", "docs/examples/go"}, "docs/examples/go"},
		{[]string{"check", "--only", "x"}, "."},
		{[]string{"map", "serve"}, "."},
		{[]string{"map", "serve", "docs/examples/go"}, "docs/examples/go"},
		{[]string{"map", "serve", "--addr", "127.0.0.1:8080"}, "."},
		{[]string{"map", "serve", "--addr", "127.0.0.1:8080", "docs/examples/go"}, "docs/examples/go"},
		{[]string{"map", "serve", "--addr=127.0.0.1:8080", "docs/examples/go"}, "docs/examples/go"},
		{[]string{"map", "serve", "docs/examples/go", "--addr", "127.0.0.1:8080"}, "docs/examples/go"},
		{[]string{"check", "--only", "x", "docs/examples/go"}, "docs/examples/go"},
		{[]string{"check", "--no-baseline", "docs/examples/go"}, "docs/examples/go"},
		{[]string{"check", "--", "-dir"}, "-dir"},
		{[]string{"map", "serve", "--addr", "127.0.0.1:8080", "--"}, "."},
		{[]string{"domain", "overview", "docs/examples/go"}, "."},
	}
	for _, c := range cases {
		if got := repositoryPath(c.args); got != c.want {
			t.Errorf("repositoryPath(%q) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestMapServeNeedsTheRuleset(t *testing.T) {
	if runsWithoutRuleset([]string{"map", "serve"}) {
		t.Fatal("map serve composes against the project's ruleset like every domain command")
	}
}
