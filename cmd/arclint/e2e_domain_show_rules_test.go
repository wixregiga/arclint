package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// catalogDomain records, for each language's fixture, one rule the code
// enforces and one it does not under each owner kind, a value object
// recording no rule, and a pricing context whose only code is in a
// language ArcLint does not analyze. Both contexts record an invariant
// keyed well-formed, so the bare key names two entries.
const catalogDomain = `version: 1
project: shop
contexts:
  catalog:
    definition: Listing and reserving products.
    aggregates:
      Product:
        definition: A product offered for sale.
        identity: ProductID
        invariants:
          price-set: A Product always has a price.
          name-set: A Product always has a name.
    value_objects:
      ProductID:
        definition: The stable identity of a Product.
      Sku:
        definition: The stock keeping unit printed on a Product.
        invariants:
          well-formed: A Sku is never empty.
      Colour:
        definition: The colour a Product is offered in.
    services:
      Reservation:
        definition: Reserving a Product for a customer.
        assertions:
          in-stock:
            on: Reserve
            statement: A Reservation reserves only a Product in stock.
          price-known:
            on: Reserve
            statement: A Reservation reserves only a Product with a known price.
  pricing:
    definition: Setting the price of a Product.
    value_objects:
      Discount:
        definition: A reduction taken off a price.
        invariants:
          well-formed: A Discount never exceeds the price.
`

// showRulesFixture is one language's implementation of the catalog
// context: the files and the runtime name the ruleset configures.
type showRulesFixture struct {
	runtime  string
	language string
	files    map[string]string
}

var showRulesFixtures = map[string]showRulesFixture{
	"go": {runtime: "go", language: "go", files: map[string]string{
		"catalog/product.go": `package catalog

import "errors"

// ProductID identifies a Product.
type ProductID string

// Product is the aggregate root.
type Product struct {
	id    ProductID
	name  string
	price int64
}

// NewProduct builds a priced Product.
func NewProduct(id ProductID, name string, price int64) (*Product, error) {
	p := &Product{id: id, name: name, price: price}
	if err := p.EnsurePriceSet(); err != nil {
		return nil, err
	}
	return p, nil
}

// Reprice changes the price.
func (p *Product) Reprice(price int64) error {
	p.price = price
	return p.EnsurePriceSet()
}

// EnsurePriceSet enforces price-set.
func (p *Product) EnsurePriceSet() error {
	if p.price <= 0 {
		return errors.New("a Product always has a price")
	}
	return nil
}

// Sku is the stock keeping unit.
type Sku struct{ code string }

// NewSku constructs a Sku.
func NewSku(code string) (Sku, error) {
	if code == "" {
		return Sku{}, errors.New("a Sku is never empty")
	}
	return Sku{code: code}, nil
}

// Colour is a colour name.
type Colour string
`,
		"catalog/reservation.go": `package catalog

import "errors"

// Reservation reserves Products.
type Reservation struct{ stock map[ProductID]int }

// Reserve reserves one Product.
func (r Reservation) Reserve(id ProductID) error {
	return r.AssertInStock(id)
}

// AssertInStock checks in-stock.
func (r Reservation) AssertInStock(id ProductID) error {
	if r.stock[id] <= 0 {
		return errors.New("out of stock")
	}
	return nil
}

// AssertPriceKnown checks price-known.
func (r Reservation) AssertPriceKnown(id ProductID) error {
	return nil
}
`,
	}},
	"typescript": {runtime: "ts", language: "typescript", files: map[string]string{
		"catalog/product.ts": `export type ProductID = string;

export class Product {
  private constructor(
    private readonly id: ProductID,
    private name: string,
    private price: number,
  ) {
    this.ensurePriceSet();
  }

  static create(id: ProductID, name: string, price: number): Product {
    const p = new Product(id, name, price);
    p.ensurePriceSet();
    return p;
  }

  reprice(price: number): void {
    this.price = price;
    this.ensurePriceSet();
  }

  ensurePriceSet(): void {
    if (this.price <= 0) {
      throw new Error("a Product always has a price");
    }
  }
}

export class Sku {
  private constructor(private readonly code: string) {}

  static create(code: string): Sku {
    if (code === "") {
      throw new Error("a Sku is never empty");
    }
    return new Sku(code);
  }
}

export type Colour = string;
`,
		"catalog/reservation.ts": `import { ProductID } from "./product";

export class Reservation {
  constructor(private readonly stock: Map<ProductID, number>) {}

  reserve(id: ProductID): void {
    this.assertInStock(id);
  }

  assertInStock(id: ProductID): void {
    if ((this.stock.get(id) ?? 0) <= 0) {
      throw new Error("out of stock");
    }
  }

  assertPriceKnown(id: ProductID): void {}
}
`,
	}},
	"python": {runtime: "py", language: "python", files: map[string]string{
		"catalog/product.py": `ProductID = str


class Product:
    def __init__(self, id: ProductID, name: str, price: int) -> None:
        self._id = id
        self._name = name
        self._price = price
        self.ensure_price_set()

    def reprice(self, price: int) -> None:
        self._price = price
        self.ensure_price_set()

    def ensure_price_set(self) -> None:
        if self._price <= 0:
            raise ValueError("a Product always has a price")


class Sku:
    def __init__(self, code: str) -> None:
        if code == "":
            raise ValueError("a Sku is never empty")
        self._code = code


class Colour(str):
    pass
`,
		"catalog/reservation.py": `from catalog.product import ProductID


class Reservation:
    def __init__(self, stock: dict) -> None:
        self._stock = stock

    def reserve(self, id: ProductID) -> None:
        self.assert_in_stock(id)

    def assert_in_stock(self, id: ProductID) -> None:
        if self._stock.get(id, 0) <= 0:
            raise ValueError("out of stock")

    def assert_price_known(self, id: ProductID) -> None:
        pass
`,
	}},
}

// shownRules is the rules document of a show in JSON.
type shownRules struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Rules *struct {
		Checked   bool   `json:"checked"`
		Aggregate string `json:"aggregate"`
		Record    string `json:"record"`
		Checks    []struct {
			Kind     string   `json:"kind"`
			Owner    string   `json:"owner"`
			Key      string   `json:"key"`
			Source   string   `json:"source"`
			Expected string   `json:"expected"`
			Status   string   `json:"status"`
			Reasons  []string `json:"reasons"`
			Findings []struct {
				Rule    string `json:"rule"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"findings"`
		} `json:"checks"`
	} `json:"rules"`
}

func showRulesJSON(t *testing.T, root string, args ...string) shownRules {
	t.Helper()
	stdout := mustRunDomain(t, root, append(append([]string{"domain", "show"}, args...), "--format", "json")...)
	var shown shownRules
	if err := json.Unmarshal([]byte(stdout), &shown); err != nil {
		t.Fatalf("show %v json: %v\n%s", args, err, stdout)
	}
	if shown.Rules == nil {
		t.Fatalf("show %v carries no rules document:\n%s", args, stdout)
	}
	return shown
}

// TestDomainShowReportsRuleEnforcement shows entries by bare name in
// Go, TypeScript, and Python code and reads, for each recorded rule,
// the declaration enforcing it and its status from the current check.
func TestDomainShowReportsRuleEnforcement(t *testing.T) {
	for lang, fixture := range showRulesFixtures {
		t.Run(lang, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "rules.arclint.yaml", "runtime: ["+fixture.runtime+"]\nzones:\n  catalog:\n    paths: [\"catalog/**\"]\n  pricing:\n    paths: [\"pricing/**\"]\n")
			write(t, root, "domain.arclint.yaml", catalogDomain)
			write(t, root, "pricing/discount.rs", "pub struct Discount { pub cents: i64 }\n")
			for path, content := range fixture.files {
				write(t, root, path, content)
			}

			product := showRulesJSON(t, root, "Product")
			if product.Type != "aggregate" || !product.Rules.Checked || len(product.Rules.Checks) != 2 {
				t.Fatalf("Product: %+v", product)
			}
			priceSet, nameSet := product.Rules.Checks[0], product.Rules.Checks[1]
			if priceSet.Key != "price-set" || priceSet.Status != "conforms" || !strings.HasPrefix(priceSet.Source, "catalog/product.") {
				t.Errorf("price-set is enforced at the root's ensure method and conforms: %+v", priceSet)
			}
			if nameSet.Key != "name-set" || nameSet.Status != "finding" || nameSet.Source != "" ||
				!strings.Contains(strings.ToLower(nameSet.Expected), "ensure") || len(nameSet.Findings) == 0 ||
				nameSet.Findings[0].Message == "" || nameSet.Findings[0].Status != "active" {
				t.Errorf("name-set has no ensure method and is a finding with its message: %+v", nameSet)
			}

			sku := showRulesJSON(t, root, "Sku")
			if len(sku.Rules.Checks) != 1 || sku.Rules.Checks[0].Status != "conforms" || sku.Rules.Checks[0].Source == "" {
				t.Errorf("Sku's invariant is enforced at its constructor: %+v", sku.Rules)
			}

			reservation := showRulesJSON(t, root, "Reservation")
			if len(reservation.Rules.Checks) != 2 {
				t.Fatalf("Reservation: %+v", reservation.Rules)
			}
			inStock, priceKnown := reservation.Rules.Checks[0], reservation.Rules.Checks[1]
			if inStock.Status != "conforms" || !strings.HasPrefix(inStock.Source, "catalog/reservation.") {
				t.Errorf("in-stock is checked by the operation: %+v", inStock)
			}
			if priceKnown.Status != "finding" || !strings.HasPrefix(priceKnown.Source, "catalog/reservation.") || len(priceKnown.Findings) == 0 {
				t.Errorf("price-known is declared but Reserve never calls it: %+v", priceKnown)
			}

			discount := showRulesJSON(t, root, "Discount")
			if len(discount.Rules.Checks) != 1 || discount.Rules.Checks[0].Status != "not_observable" ||
				discount.Rules.Checks[0].Source != "" || discount.Rules.Checks[0].Expected != "" ||
				len(discount.Rules.Checks[0].Reasons) == 0 || !strings.Contains(discount.Rules.Checks[0].Reasons[0], "in a language ArcLint analyzes here ("+fixture.language+")") {
				t.Errorf("Discount's code is Rust, which ArcLint does not analyze: %+v", discount.Rules)
			}

			colour := showRulesJSON(t, root, "Colour")
			if len(colour.Rules.Checks) != 0 || colour.Rules.Checked ||
				colour.Rules.Record != `arclint domain define invariant <key> --owner Colour --statement "<statement>" --context catalog` {
				t.Errorf("Colour records no rule and names how to record one: %+v", colour.Rules)
			}

			// A key recorded in two contexts names both, each with the
			// command that shows it alone.
			_, stderr, code := runBin(t, root, os.Environ(), "domain", "show", "well-formed")
			if code != 2 ||
				!strings.Contains(stderr, "arclint domain show invariant well-formed --owner Sku --context catalog") ||
				!strings.Contains(stderr, "arclint domain show invariant well-formed --owner Discount --context pricing") {
				t.Errorf("ambiguous bare key: exit %d: %s", code, stderr)
			}
			stdout := mustRunDomain(t, root, "domain", "show", "well-formed", "--context", "pricing")
			if !strings.Contains(stdout, "Invariant: well-formed\n") || !strings.Contains(stdout, "Owner: Discount\n") ||
				!strings.Contains(stdout, "Status: not observable in the languages ArcLint analyzes\n") {
				t.Errorf("a context narrows the bare key to one entry:\n%s", stdout)
			}
		})
	}
}

// TestDomainShowTextNamesEnforcementAndAbsence reads the human form of
// a show: each rule's enforcing line and status, the line an entry
// recording no rule prints, and a name found in another case.
func TestDomainShowTextNamesEnforcementAndAbsence(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules.arclint.yaml", "runtime: [go]\nzones:\n  catalog:\n    paths: [\"catalog/**\"]\n  pricing:\n    paths: [\"pricing/**\"]\n")
	write(t, root, "domain.arclint.yaml", catalogDomain)
	for path, content := range showRulesFixtures["go"].files {
		write(t, root, path, content)
	}

	stdout := mustRunDomain(t, root, "domain", "show", "reservation")
	for _, want := range []string{
		"Domain Service: Reservation\n",
		"  in-stock (on Reserve)  A Reservation reserves only a Product in stock.\n    enforced at: catalog/reservation.go:14\n    status: conforms\n",
		"  price-known (on Reserve)  A Reservation reserves only a Product with a known price.\n    enforced at: catalog/reservation.go:22\n    status: finding\n      finding: ",
		"Statuses come from the structural checks of the current code",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("show reservation lacks %q:\n%s", want, stdout)
		}
	}

	stdout = mustRunDomain(t, root, "domain", "show", "Colour")
	want := "Rules: Colour records no invariant; record one with: arclint domain define invariant <key> --owner Colour --statement \"<statement>\" --context catalog\n"
	if !strings.HasSuffix(stdout, want) {
		t.Errorf("show Colour ends with %q:\n%s", want, stdout)
	}

	// The first argument completes with the concepts and every
	// recorded name, since a name alone finds its entry.
	stdout, stderr, code := runBin(t, root, os.Environ(), "__complete", "domain", "show", "")
	if code != 0 {
		t.Fatalf("__complete domain show: exit %d: %s", code, stderr)
	}
	for _, want := range []string{"aggregate", "Colour", "Reservation", "price-known", "well-formed", "pricing"} {
		if !containsLine(stdout, want) {
			t.Errorf("__complete domain show misses %q:\n%s", want, stdout)
		}
	}

	_, stderr, code = runBin(t, root, os.Environ(), "domain", "show", "value_object")
	if code != 2 || !strings.Contains(stderr, "arclint domain show value_object <name>") {
		t.Errorf("a concept alone: exit %d: %s", code, stderr)
	}
	_, stderr, code = runBin(t, root, os.Environ(), "domain", "show", "Basket")
	if code == 0 || !strings.Contains(stderr, `no entry named "Basket"`) {
		t.Errorf("an unrecorded name: exit %d: %s", code, stderr)
	}
}
