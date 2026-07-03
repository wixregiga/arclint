package prompt

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"users-api":       "users-api",
		"payment gateway": "'payment gateway'",
		"a'b":             `'a'\''b'`,
		"":                "''",
		"has$dollar":      "'has$dollar'",
		"plain_snake":     "plain_snake",
	}
	for in, want := range cases {
		if got := ShellQuote(in); got != want {
			t.Errorf("ShellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTip(t *testing.T) {
	var sb strings.Builder
	Tip(&sb, "arclint new service users-api --var port=8080")
	out := sb.String()
	if !strings.Contains(out, "tip: next time, skip the prompts with:") ||
		!strings.Contains(out, "arclint new service users-api --var port=8080") {
		t.Errorf("Tip output wrong: %q", out)
	}
}

func TestInteractiveFalseInTests(t *testing.T) {
	// go test runs without a TTY on stdin/stdout, so prompts must not fire.
	if Interactive() {
		t.Skip("running under a real TTY; nothing to assert")
	}
}

func TestUseAccessibleMode(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"no env set", map[string]string{}, false},
		{"TERM=dumb", map[string]string{"TERM": "dumb"}, true},
		{"ARCLINT_ACCESSIBLE=1", map[string]string{"ARCLINT_ACCESSIBLE": "1"}, true},
		{"ARCLINT_ACCESSIBLE=0 does not opt in", map[string]string{"ARCLINT_ACCESSIBLE": "0"}, false},
		{"TERM=xterm-256color", map[string]string{"TERM": "xterm-256color"}, false},
		{"both set", map[string]string{"TERM": "dumb", "ARCLINT_ACCESSIBLE": "1"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := func(k string) string { return c.env[k] }
			if got := useAccessibleMode(env); got != c.want {
				t.Errorf("useAccessibleMode() = %v, want %v", got, c.want)
			}
		})
	}
}
