package template

import "testing"

func TestSplitWords(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"payment gateway", []string{"payment", "gateway"}},
		{"payment-gateway", []string{"payment", "gateway"}},
		{"payment_gateway", []string{"payment", "gateway"}},
		{"paymentGateway", []string{"payment", "Gateway"}},
		{"PaymentGateway", []string{"Payment", "Gateway"}},
		{"HTTPServer", []string{"HTTP", "Server"}},
		{"a  b", []string{"a", "b"}},
		{"", nil},
	}
	for _, c := range cases {
		got := splitWords(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitWords(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitWords(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestFilterOutputs(t *testing.T) {
	cases := []struct {
		filter string
		in     string
		want   string
	}{
		{"pascal", "payment gateway", "PaymentGateway"},
		{"camel", "payment gateway", "paymentGateway"},
		{"snake", "payment gateway", "payment_gateway"},
		{"kebab", "payment gateway", "payment-gateway"},
		{"upper", "payment gateway", "PAYMENT GATEWAY"},
		{"lower", "Payment Gateway", "payment gateway"},
		{"kebab", "PaymentGateway", "payment-gateway"},
		{"plural", "payment gateway", "payment gateways"},
		{"plural", "bus", "buses"},
		{"plural", "box", "boxes"},
		{"plural", "quiz", "quizes"},
		{"plural", "match", "matches"},
		{"plural", "dish", "dishes"},
		{"plural", "city", "cities"},
		{"plural", "day", "days"},
		{"plural", "person", "people"},
		{"plural", "child", "children"},
		{"plural", "service mouse", "service mice"},
	}
	for _, c := range cases {
		fn, ok := filters[c.filter]
		if !ok {
			t.Fatalf("filter %q missing from table", c.filter)
		}
		if got := fn(c.in); got != c.want {
			t.Errorf("%s(%q) = %q, want %q", c.filter, c.in, got, c.want)
		}
	}
}

func TestKebabPascalRoundTrip(t *testing.T) {
	// kebab(pascal(x)) == kebab(x) for word-clean inputs (design §5).
	for _, x := range []string{"payment gateway", "users-api", "big_data_thing"} {
		if got, want := toKebab(toPascal(x)), toKebab(x); got != want {
			t.Errorf("kebab(pascal(%q)) = %q, want %q", x, got, want)
		}
	}
}
