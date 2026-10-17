package main

// The context command lists the recorded entries a worksite uses apart
// from those it declares: in Go, TypeScript, and Python, each used
// entry with the lines that reach it and the contracts that bind them,
// and every reference resolution could not follow, with its reason.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// usesDomain records an Order aggregate, a Money value object, and a
// Pricing service, each with one contract.
const usesDomain = `version: 1
project: ordering
contexts:
  ordering:
    definition: Taking orders.
    aggregates:
      Order:
        definition: A customer's request.
        identity: OrderID
        invariants:
          has-total: An Order carries a total.
    value_objects:
      Money:
        definition: An amount.
        invariants:
          not-negative: Money is never negative.
    services:
      Pricing:
        definition: Prices an order.
        assertions:
          quote-positive:
            on: Quote
            statement: A quote is positive.
`

func TestContextListsUsedEntriesInEveryLanguage(t *testing.T) {
	for _, tc := range []struct {
		name         string
		files        map[string]string
		worksite     string
		used         []string
		undetermined []string
	}{
		{
			name: "go",
			files: map[string]string{
				"app/place_order.go": `package app

import "example.com/shop/ordering"

// PlaceOrder places an order and quotes it.
type PlaceOrder struct {
	pricing ordering.Pricing
	notify  func(*ordering.Order)
}

// Run places and quotes.
func (uc PlaceOrder) Run(cents int) (ordering.Money, error) {
	total, err := ordering.NewMoney(cents)
	if err != nil {
		return ordering.Money{}, err
	}
	order := ordering.NewOrder(total)
	if err := order.Place(); err != nil {
		return ordering.Money{}, err
	}
	uc.notify(order)
	return uc.pricing.Quote(order)
}
`,
				"go.mod": `module example.com/shop

go 1.24
`,
				"ordering/money.go": `package ordering

import "errors"

// Money is an amount in cents.
type Money struct{ cents int }

// NewMoney builds Money.
func NewMoney(cents int) (Money, error) {
	if cents < 0 {
		return Money{}, errors.New("negative")
	}
	return Money{cents: cents}, nil
}
`,
				"ordering/order.go": `package ordering

// Order is a customer's request.
type Order struct{ total Money }

// EnsureHasTotal enforces has-total.
func (o *Order) EnsureHasTotal() error { return nil }
`,
				"ordering/order_commands.go": `package ordering

// Place places the order.
func (o *Order) Place() error { return o.EnsureHasTotal() }

// NewOrder builds an Order.
func NewOrder(total Money) *Order { return &Order{total: total} }
`,
				"ordering/pricing.go": `package ordering

// Pricing prices an order.
type Pricing struct{}

// Quote prices the order.
func (p Pricing) Quote(o *Order) (Money, error) {
	if err := p.AssertQuotePositive(o.total); err != nil {
		return Money{}, err
	}
	return o.total, nil
}

// AssertQuotePositive checks quote-positive.
func (Pricing) AssertQuotePositive(m Money) error { return nil }
`,
				"rules.arclint.yaml": `runtime: [go]
zones:
  ordering:
    paths: [ordering/**]
  app:
    paths: [app/**]
`,
			},
			worksite: "app/place_order.go",
			used: []string{
				"  used here, context ordering:\n",
				"    service Pricing: app/place_order.go:7,22\n",
				"      assertion quote-positive (Pricing, on Quote): A quote is positive. ordering/pricing.go:15\n",
				"    value object Money: app/place_order.go:13\n",
				"      invariant not-negative (Money): Money is never negative. ordering/money.go:9\n",
				"    aggregate Order: app/place_order.go:17,18\n",
				"      invariant has-total (Order): An Order carries a total. ordering/order.go:7\n",
			},
			undetermined: []string{"    app/place_order.go:21: notify is a field of function type; the function it holds is not declared\n"},
		},
		{
			name: "typescript",
			files: map[string]string{
				"rules.arclint.yaml": `runtime: [ts]
zones:
  ordering:
    paths: [src/ordering/**]
  app:
    paths: [src/app/**]
`,
				"src/app/place-order.ts": `import { Money } from "../ordering/money.js";
import { Order, type Pricing } from "../ordering/order.js";

export class PlaceOrder {
  constructor(private readonly pricing: Pricing) {}

  run(cents: number, handler: any): Money {
    const order = new Order(Money.of(cents));
    order.place();
    handler.notify(order);
    return this.pricing.quote(order);
  }
}
`,
				"src/ordering/money.ts": `export class Money {
  private constructor(readonly cents: number) {}

  static of(cents: number): Money {
    if (cents < 0) throw new Error("negative");
    return new Money(cents);
  }
}
`,
				"src/ordering/order.ts": `import { Money } from "./money.js";

export class Order {
  constructor(readonly total: Money) {}

  place(): void {
    this.ensureHasTotal();
  }

  ensureHasTotal(): void {}
}

export class Pricing {
  quote(order: Order): Money {
    return order.total;
  }

  assertQuotePositive(): void {}
}
`,
			},
			worksite: "src/app/place-order.ts",
			used: []string{
				"  used here, context ordering:\n",
				"    service Pricing: src/app/place-order.ts:5,11\n",
				"      assertion quote-positive (Pricing, on Quote): A quote is positive. src/ordering/order.ts:18\n",
				"    value object Money: src/app/place-order.ts:8\n",
				"      invariant not-negative (Money): Money is never negative. src/ordering/money.ts:2\n",
				"    aggregate Order: src/app/place-order.ts:9\n",
				"      invariant has-total (Order): An Order carries a total. src/ordering/order.ts:10\n",
			},
			undetermined: []string{"    src/app/place-order.ts:10: any has no established declaration\n"},
		},
		{
			name: "python",
			files: map[string]string{
				"ordering/__init__.py":     ``,
				"ordering/app/__init__.py": ``,
				"ordering/app/place_order.py": `from ordering.domain.money import Money
from ordering.domain.order import Order, Pricing


class PlaceOrder:
    pricing: Pricing

    def run(self, cents: int, handler) -> Money:
        order = Order(Money(cents))
        order.place()
        handler.notify(order)
        return self.pricing.quote(order)
`,
				"ordering/domain/__init__.py": ``,
				"ordering/domain/money.py": `class Money:
    def __init__(self, cents: int) -> None:
        if cents < 0:
            raise ValueError("negative")
        self.cents = cents
`,
				"ordering/domain/order.py": `from ordering.domain.money import Money


class Order:
    def __init__(self, total: Money) -> None:
        self.total = total

    def place(self) -> None:
        self.ensure_has_total()

    def ensure_has_total(self) -> None:
        pass


class Pricing:
    def quote(self, order: Order) -> Money:
        return order.total

    def assert_quote_positive(self) -> None:
        pass
`,
				"rules.arclint.yaml": `runtime: [py]
zones:
  domain:
    paths: [ordering/domain/**]
  app:
    paths: [ordering/app/**]
`,
			},
			worksite: "ordering/app/place_order.py",
			used: []string{
				"  used here, context ordering:\n",
				"    service Pricing: ordering/app/place_order.py:6,12\n",
				"      assertion quote-positive (Pricing, on Quote): A quote is positive. ordering/domain/order.py:19\n",
				"    aggregate Order: ordering/app/place_order.py:9\n",
				"      invariant has-total (Order): An Order carries a total. ordering/domain/order.py:11\n",
				"    value object Money: ordering/app/place_order.py:9\n",
				"      invariant not-negative (Money): Money is never negative. ordering/domain/money.py:2\n",
			},
			undetermined: []string{
				"    ordering/app/place_order.py:10: order has no declared type\n",
				"    ordering/app/place_order.py:11: handler has no declared type\n",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "domain.arclint.yaml", usesDomain)
			for rel, content := range tc.files {
				write(t, root, rel, content)
			}
			stdout, stderr, code := runBin(t, root, os.Environ(), "context", tc.worksite)
			if code != 0 {
				t.Fatalf("context: exit %d\nstderr: %s", code, stderr)
			}
			if !strings.Contains(stdout, "nothing recorded is declared here; 3 entries used here, ") {
				t.Errorf("headline does not separate declared from used:\n%s", stdout)
			}
			if !strings.Contains(stdout, strings.Join(tc.used, "")) {
				t.Errorf("used block missing:\n%s\nwant\n%s", stdout, strings.Join(tc.used, ""))
			}
			if !strings.Contains(stdout, "  undetermined here: ") || !strings.Contains(stdout, strings.Join(tc.undetermined, "")) {
				t.Errorf("undetermined references missing:\n%s\nwant %q", stdout, tc.undetermined)
			}
			if strings.Contains(stdout, "declared here, context") {
				t.Errorf("a worksite declaring nothing lists no declared context:\n%s", stdout)
			}

			stdout, stderr, code = runBin(t, root, os.Environ(), "context", tc.worksite, "--format", "json")
			if code != 0 {
				t.Fatalf("context json: exit %d\nstderr: %s", code, stderr)
			}
			var doc struct {
				Domain struct {
					Contexts []json.RawMessage `json:"contexts"`
					Used     []struct {
						Name  string `json:"name"`
						Role  string `json:"role"`
						Sites []struct {
							Path string `json:"path"`
							Line int    `json:"line"`
							Way  string `json:"way"`
						} `json:"sites"`
					} `json:"used"`
					Undetermined []struct {
						Path   string `json:"path"`
						Reason string `json:"reason"`
					} `json:"undetermined"`
				} `json:"domain"`
			}
			if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
				t.Fatalf("context json: %v\n%s", err, stdout)
			}
			roles := map[string]string{}
			for _, u := range doc.Domain.Used {
				roles[u.Name] = u.Role
				for _, s := range u.Sites {
					if s.Path != tc.worksite || s.Line < 1 || (s.Way != "call" && s.Way != "field") {
						t.Errorf("site of %s = %+v", u.Name, s)
					}
				}
			}
			if len(doc.Domain.Contexts) != 0 || roles["Order"] != "aggregate" || roles["Money"] != "value_object" || roles["Pricing"] != "domain_service" || len(roles) != 3 {
				t.Errorf("json keeps used apart from declared: contexts %d, used roles %v", len(doc.Domain.Contexts), roles)
			}
			if len(doc.Domain.Undetermined) != len(tc.undetermined) {
				t.Errorf("undetermined = %+v", doc.Domain.Undetermined)
			}
			for _, u := range doc.Domain.Undetermined {
				if u.Path != tc.worksite || u.Reason == "" {
					t.Errorf("undetermined = %+v", u)
				}
			}
		})
	}
}

// A file that declares a used entry lists it as declared here, never as
// used; a worksite spanning the caller and the callee uses nothing.
func TestContextNeverListsADeclaredEntryAsUsed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "domain.arclint.yaml", usesDomain)
	write(t, root, "rules.arclint.yaml", "runtime: [py]\n")
	write(t, root, "ordering/__init__.py", "")
	write(t, root, "ordering/money.py", "class Money:\n    cents: int\n\n    def __init__(self, cents: int) -> None:\n        pass\n")
	write(t, root, "ordering/order.py", "from ordering.money import Money\n\n\nclass Order:\n    total: Money\n")
	stdout, stderr, code := runBin(t, root, os.Environ(), "context", "ordering")
	if code != 0 {
		t.Fatalf("context: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, " declared here; no entry used here; --full shows the whole model\n") || strings.Contains(stdout, "used here, context") || strings.Contains(stdout, "undetermined here") {
		t.Fatalf("an entry declared in the worksite is not used:\n%s", stdout)
	}
}

// An aggregate whose contract method lies in the worksite is declared
// here, even when its type is declared in another file of its package:
// the worksite's calls on it are not listed again as used.
func TestContextKeepsAContractCarrierDeclaredNotUsed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "domain.arclint.yaml", usesDomain)
	write(t, root, "rules.arclint.yaml", "runtime: [go]\n")
	write(t, root, "go.mod", "module example.com/shop\n\ngo 1.24\n")
	write(t, root, "ordering/order.go", "package ordering\n\n// Order is a request.\ntype Order struct{ total int }\n")
	write(t, root, "ordering/commands.go", "package ordering\n\n// EnsureHasTotal enforces has-total.\nfunc (o *Order) EnsureHasTotal() error { return nil }\n\n// Place places the order.\nfunc (o *Order) Place() error { return o.EnsureHasTotal() }\n")
	stdout, stderr, code := runBin(t, root, os.Environ(), "context", "ordering/commands.go")
	if code != 0 {
		t.Fatalf("context: exit %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "  declared here, context ordering:\n    aggregates: Order (OrderID)\n") {
		t.Fatalf("Order is declared here:\n%s", stdout)
	}
	if strings.Contains(stdout, "used here, context") || strings.Count(stdout, "has-total (Order)") != 1 {
		t.Fatalf("a declared aggregate is not listed again as used:\n%s", stdout)
	}
}

// A used entry's missing contract joins the unanchored summary, in the
// human output and in JSON, once.
func TestContextSummarizesTheMissingContractsOfUsedEntries(t *testing.T) {
	root := t.TempDir()
	write(t, root, "domain.arclint.yaml", usesDomain)
	write(t, root, "rules.arclint.yaml", "runtime: [py]\n")
	write(t, root, "ordering/__init__.py", "")
	write(t, root, "ordering/order.py", "class Order:\n    def place(self) -> None:\n        pass\n")
	write(t, root, "ordering/app.py", "from ordering.order import Order\n\n\ndef run(order: Order) -> None:\n    order.place()\n")
	stdout, stderr, code := runBin(t, root, os.Environ(), "context", "ordering/app.py")
	if code != 0 {
		t.Fatalf("context: exit %d\nstderr: %s", code, stderr)
	}
	for _, want := range []string{
		"    aggregate Order: ordering/app.py:5\n      invariant has-total (Order): An Order carries a total. missing\n",
		"  unanchored contracts: 1 missing\n    missing: invariant has-total of Order (context ordering)\n      expected method ensure_has_total on Order\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("context lacks %q:\n%s", want, stdout)
		}
	}
	stdout, stderr, code = runBin(t, root, os.Environ(), "context", "ordering/app.py", "--format", "json")
	if code != 0 {
		t.Fatalf("context json: exit %d\nstderr: %s", code, stderr)
	}
	var doc struct {
		Domain struct {
			Unanchored []struct {
				Owner string `json:"owner"`
				Key   string `json:"key"`
			} `json:"unanchored"`
		} `json:"domain"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("context json: %v\n%s", err, stdout)
	}
	if u := doc.Domain.Unanchored; len(u) != 1 || u[0].Owner != "Order" || u[0].Key != "has-total" {
		t.Fatalf("unanchored = %+v", u)
	}
}
