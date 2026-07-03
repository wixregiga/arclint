package template

import (
	"strings"
	"testing"
)

func render(t *testing.T, src string, vars map[string]string) (string, error) {
	t.Helper()
	out, err := NewRenderer(vars).Render([]byte(src), "test.txt")
	return string(out), err
}

func TestRendererTable(t *testing.T) {
	vars := map[string]string{
		"name":  "payment gateway",
		"x":     "UserAccount",
		"empty": "",
	}
	cases := []struct {
		src  string
		want string
	}{
		{"hello", "hello"},
		{"{{ name }}", "payment gateway"},
		{"{{name}}", "payment gateway"},
		{"{{ name|kebab }}", "payment-gateway"},
		{"{{ name | pascal }}", "PaymentGateway"},
		{"{{ name | camel }}", "paymentGateway"},
		{"{{ name | snake }}", "payment_gateway"},
		{"{{ name | upper }}", "PAYMENT GATEWAY"},
		{"{{ name | lower }}", "payment gateway"},
		{"{{ name | plural }}", "payment gateways"},
		{"{{ x | snake | upper }}", "USER_ACCOUNT"}, // chaining, left to right
		{"a {{ name | kebab }} b {{ name | pascal }} c", "a payment-gateway b PaymentGateway c"},
		{"{{{{ not a tag }}}}", "{{ not a tag }}"}, // escapes
		{"literal {{{{", "literal {{"},
		{"${{{{ github }}}}", "${{ github }}"},
		{"{{ empty }}", ""},
	}
	for _, c := range cases {
		got, err := render(t, c.src, vars)
		if err != nil {
			t.Errorf("Render(%q) error: %v", c.src, err)
			continue
		}
		if got != c.want {
			t.Errorf("Render(%q) = %q, want %q", c.src, got, c.want)
		}
	}
}

func TestRendererErrors(t *testing.T) {
	vars := map[string]string{"name": "x"}
	cases := []struct {
		src     string
		errPart string
	}{
		{"{{ nope }}", `unknown variable "nope"`},
		{"{{ name | shout }}", `unknown filter "shout"`},
		{"{{ name | }}", "empty filter"},
		{"{{ }}", "empty {{ }} tag"},
		{"{{ Name }}", "invalid variable name"},
		{"open {{ name", "unterminated {{ tag"},
	}
	for _, c := range cases {
		_, err := render(t, c.src, vars)
		if err == nil {
			t.Errorf("Render(%q): want error containing %q, got nil", c.src, c.errPart)
			continue
		}
		if !strings.Contains(err.Error(), c.errPart) {
			t.Errorf("Render(%q) error = %q, want substring %q", c.src, err, c.errPart)
		}
		if !strings.Contains(err.Error(), "test.txt:") {
			t.Errorf("Render(%q) error %q lacks origin position", c.src, err)
		}
	}
}

func TestRendererErrorPosition(t *testing.T) {
	_, err := render(t, "line one\nab {{ bad }}", map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "test.txt:2:4:") {
		t.Fatalf("want position test.txt:2:4:, got %v", err)
	}
}
