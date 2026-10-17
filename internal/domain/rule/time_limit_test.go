package rule_test

import (
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

func TestTimeLimitIsPositive(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		if l, err := rule.NewTimeLimit(d); err == nil {
			t.Errorf("NewTimeLimit(%s) = %s, want a rejection", d, l)
		}
	}
	for _, s := range []string{"", "10", "ten seconds", "0s", "-1s"} {
		if l, err := rule.ParseTimeLimit(s); err == nil {
			t.Errorf("ParseTimeLimit(%q) = %s, want a rejection", s, l)
		}
	}
	l, err := rule.ParseTimeLimit("1m30s")
	if err != nil || l.Duration() != 90*time.Second || l.String() != "1m30s" {
		t.Errorf("ParseTimeLimit(1m30s) = %s, %v", l, err)
	}
	if rule.DefaultTimeLimit.Duration() != 5*time.Second {
		t.Errorf("DefaultTimeLimit = %s, want 5s", rule.DefaultTimeLimit)
	}
}

// Near is strictly more than half: a run twice as slow would stop.
func TestTimeLimitNear(t *testing.T) {
	l, err := rule.NewTimeLimit(4 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if l.Near(2 * time.Second) {
		t.Error("exactly half is not near")
	}
	if !l.Near(2*time.Second + time.Nanosecond) {
		t.Error("more than half is near")
	}
}

func TestWithTimeLimitOnlyOnExtensionRules(t *testing.T) {
	scope, err := rule.RepositoryScope()
	if err != nil {
		t.Fatal(err)
	}
	ext, err := rule.New(rule.Spec{ID: "x/uses", Type: rule.TypeExtension, Params: rule.ExtensionParams{Uses: "x"}, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	if got := ext.Params().(rule.ExtensionParams).Limit(); got != rule.DefaultTimeLimit {
		t.Errorf("unset limit = %s, want the default", got)
	}
	l, err := rule.NewTimeLimit(20 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	slow, err := ext.WithTimeLimit(l)
	if err != nil {
		t.Fatal(err)
	}
	if got := slow.Params().(rule.ExtensionParams).Limit(); got != l {
		t.Errorf("limit = %s, want %s", got, l)
	}
	if got := ext.Params().(rule.ExtensionParams).Limit(); got != rule.DefaultTimeLimit {
		t.Errorf("WithTimeLimit changed the original Rule to %s", got)
	}
	if _, err := ext.WithTimeLimit(rule.TimeLimit{}); err == nil {
		t.Error("a zero TimeLimit was accepted")
	}
	content, err := rule.New(rule.Spec{ID: "x/content", Type: rule.TypeContent, Params: rule.ContentParams{Forbid: "TODO"}, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := content.WithTimeLimit(l); err == nil || !strings.Contains(err.Error(), "only to an extension") {
		t.Errorf("WithTimeLimit on a content Rule = %v, want a rejection", err)
	}
}
