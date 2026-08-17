package rule_test

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

func mustGlob(t *testing.T, pattern string) rule.Glob {
	t.Helper()
	g, err := rule.NewGlob(pattern)
	if err != nil {
		t.Fatalf("NewGlob(%q): %v", pattern, err)
	}
	return g
}

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"internal/domain/**", "internal/domain/rule/root.go", true},
		{"internal/domain/**", "internal/domain", true}, // ** matches zero segments
		{"internal/domain/**", "internal/domainx/root.go", false},
		{"**/generated/**", "x/generated/y.go", true},
		{"**/generated/**", "generated/y.go", true},
		{"**/generated/**", "x/gen/y.go", false},
		{"**/mock_*.go", "a/b/mock_x.go", true},
		{"**/mock_*.go", "mock_x.go", true},
		{"**/mock_*.go", "a/b/mockx.go", false},
		{"internal/**/*.go", "internal/config/x.go", true},
		{"internal/**/*.go", "internal/x.go", true},
		{"internal/**/*.go", "internal/config/x.txt", false},
		{"cmd/**", "cmd/arclint/main.go", true},
		{"*.go", "b.go", true},
		{"*.go", "a/b.go", false}, // * never crosses /
		{"?at.go", "cat.go", true},
		{"?at.go", "chat.go", false},
		{"[a-c]at.go", "bat.go", true},
		{"[a-c]at.go", "dat.go", false},
		{"[!a-c]at.go", "dat.go", true},
		{"[!a-c]at.go", "bat.go", false},
		{"[^a-c]at.go", "dat.go", true},
		{`a\*.go`, "a*.go", true},
		{`a\*.go`, "ab.go", false},
		{"a/**/b", "a/b", true},
		{"a/**/b", "a/x/y/b", true},
		{"a**b.go", "axxb.go", true}, // mid-segment ** collapses to *
	}
	for _, c := range cases {
		if got := mustGlob(t, c.pattern).Match(c.path); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestGlobMatchesSubtree(t *testing.T) {
	g := mustGlob(t, "internal/features/*")
	if !g.MatchesSubtree("internal/features/auth/handler.go") {
		t.Errorf("subtree membership: a glob naming a directory must claim its subtree")
	}
	if g.Match("internal/features/auth/handler.go") {
		t.Errorf("plain Match must not cross into the subtree")
	}
	if g.MatchesSubtree("internal/other/auth.go") {
		t.Errorf("subtree membership must not leak outside the pattern")
	}
}

func TestGlobConstructionRejectsInvalid(t *testing.T) {
	for _, pattern := range []string{"", "{a,b}/x", "a//b", "a[", `a\`, "a/[]b"} {
		if _, err := rule.NewGlob(pattern); err == nil {
			t.Errorf("NewGlob(%q): expected construction error", pattern)
		}
	}
}
