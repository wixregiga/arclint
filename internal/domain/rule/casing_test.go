package rule_test

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// TestCaseTermTruthTable is the committed truth table for term casing:
// the one producing implementation the yaml expansion and the SDK rely
// on. Extend the table, never fork the transform.
func TestCaseTermTruthTable(t *testing.T) {
	cases := []struct {
		term string
		want map[string]string
	}{
		{"Rule", map[string]string{
			"flatcase":   "rule",
			"snake_case": "rule",
			"kebab-case": "rule",
			"camelCase":  "rule",
			"PascalCase": "Rule",
		}},
		{"Order Line", map[string]string{
			"flatcase":   "orderline",
			"snake_case": "order_line",
			"kebab-case": "order-line",
			"camelCase":  "orderLine",
			"PascalCase": "OrderLine",
		}},
		{"OrderLine", map[string]string{
			"flatcase":   "orderline",
			"snake_case": "order_line",
			"kebab-case": "order-line",
			"camelCase":  "orderLine",
			"PascalCase": "OrderLine",
		}},
		{"order-line", map[string]string{
			"flatcase":   "orderline",
			"snake_case": "order_line",
			"kebab-case": "order-line",
			"camelCase":  "orderLine",
			"PascalCase": "OrderLine",
		}},
		{"HTTP Server", map[string]string{
			"flatcase":   "httpserver",
			"snake_case": "http_server",
			"kebab-case": "http-server",
			"camelCase":  "httpServer",
			"PascalCase": "HttpServer",
		}},
		{"HTTPServer", map[string]string{
			"flatcase":   "httpserver",
			"snake_case": "http_server",
			"kebab-case": "http-server",
			"camelCase":  "httpServer",
			"PascalCase": "HttpServer",
		}},
		{"invoice_v2", map[string]string{
			"flatcase":   "invoicev2",
			"snake_case": "invoice_v2",
			"kebab-case": "invoice-v2",
			"camelCase":  "invoiceV2",
			"PascalCase": "InvoiceV2",
		}},
	}
	for _, c := range cases {
		for caseName, want := range c.want {
			got, err := rule.CaseTerm(c.term, caseName)
			if err != nil {
				t.Errorf("CaseTerm(%q, %s): %v", c.term, caseName, err)
				continue
			}
			if got != want {
				t.Errorf("CaseTerm(%q, %s) = %q, want %q", c.term, caseName, got, want)
			}
		}
	}
}

func TestCaseTermRejectsUnknownCaseAndEmptyTerms(t *testing.T) {
	if _, err := rule.CaseTerm("Order", "SCREAMING_SNAKE"); err == nil {
		t.Errorf("unknown case accepted")
	}
	if _, err := rule.CaseTerm("Order", "regex:.*"); err == nil {
		t.Errorf("regex accepted as a producing case")
	}
	for _, term := range []string{"", "  ", "---", "!!"} {
		if _, err := rule.CaseTerm(term, "flatcase"); err == nil {
			t.Errorf("term %q: wordless term accepted", term)
		}
	}
}
