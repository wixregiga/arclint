package rule_test

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

func TestTermCaseRendersReusableValue(t *testing.T) {
	termCase, err := rule.NewTermCase("snake_case")
	if err != nil {
		t.Fatal(err)
	}
	for term, want := range map[string]string{
		"OrderLine":  "order_line",
		"HTTPServer": "http_server",
		"Invoice V2": "invoice_v2",
	} {
		got, renderErr := termCase.Render(term)
		if renderErr != nil || got != want {
			t.Errorf("Render(%q) = %q, %v; want %q", term, got, renderErr, want)
		}
	}
}

func TestTermCaseRejectsUnknownAndUnconstructedCases(t *testing.T) {
	for _, name := range []string{"", "SCREAMING_SNAKE", "regex:.*"} {
		termCase, err := rule.NewTermCase(name)
		if err == nil {
			t.Errorf("NewTermCase(%q) accepted an unpublished case", name)
		}
		if _, renderErr := termCase.Render("Order"); renderErr == nil {
			t.Errorf("failed NewTermCase(%q) returned a renderable value", name)
		}
	}
	var termCase rule.TermCase
	if _, err := termCase.Render("Order"); err == nil {
		t.Error("unconstructed term case rendered a term")
	}
}

func TestTermCaseNeverRendersEmptySegment(t *testing.T) {
	for _, name := range rule.TermCaseNames() {
		termCase, err := rule.NewTermCase(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, term := range []string{"", "  ", "---", "!!"} {
			if _, renderErr := termCase.Render(term); renderErr == nil {
				t.Errorf("%s.Render(%q) accepted a wordless term", name, term)
			}
		}
	}
}

func TestTermCaseNamesPublishesIndependentSortedList(t *testing.T) {
	want := []string{"PascalCase", "camelCase", "flatcase", "kebab-case", "snake_case"}
	names := rule.TermCaseNames()
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("TermCaseNames() = %v, want %v", names, want)
	}
	names[0] = "changed"
	if got := rule.TermCaseNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("mutating returned names changed published cases: %v", got)
	}
}
