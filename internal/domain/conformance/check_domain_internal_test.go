package conformance

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// The domain evaluator and the meta-model agree exactly on which block
// invariants `arclint check` evaluates: every domain-evaluator
// invariant has a check, and every check evaluates a domain-evaluator
// invariant.
func TestDomainChecksMatchTheMetaModel(t *testing.T) {
	expected := map[string]bool{}
	for _, inv := range vocab.DDD().Invariants() {
		if inv.Enforcement.By != vocab.EvaluatorDomain {
			continue
		}
		expected[inv.ID] = true
		if _, ok := domainChecks[inv.ID]; !ok {
			t.Errorf("block invariant %s is evaluated by the domain evaluator and has no check", inv.ID)
		}
	}
	for id := range domainChecks {
		if !expected[id] {
			t.Errorf("check %s evaluates no domain-evaluator block invariant of the meta-model", id)
		}
	}
}

func TestIsSetter(t *testing.T) {
	cases := map[string]bool{
		"Set": true, "SetName": true, "setName": true, "set_name": true, "set": true,
		"settle": false, "Settle": false, "Settings": false, "reset": false, "Name": false, "": false,
	}
	for name, want := range cases {
		if got := isSetter(name); got != want {
			t.Errorf("isSetter(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestTypeSpelledMatchesWordsInTypeCase(t *testing.T) {
	cases := []struct {
		declared, term string
		want           bool
	}{
		{"EventID", "EventID", true},
		{"EventId", "EventID", true},
		{"OrderLine", "OrderLine", true},
		{"Orderline", "OrderLine", true},
		{"orderLine", "OrderLine", false},
		{"order_line", "OrderLine", false},
		{"Event", "EventID", false},
		{"", "Event", false},
	}
	for _, tc := range cases {
		if got := typeSpelled(tc.declared, tc.term); got != tc.want {
			t.Errorf("typeSpelled(%q, %q) = %v, want %v", tc.declared, tc.term, got, tc.want)
		}
	}
}

// A factory method starts with one of the constructor words as a whole
// word: created and offer begin with the letters and not the word.
func TestStartsWithWord(t *testing.T) {
	cases := map[string]bool{
		"create": true, "createOrder": true, "create_order": true, "FromCents": true, "from_cents": true,
		"of": true, "Of": true, "parse": true, "New": true, "NewOrder": true, "build": true,
		"created": false, "offer": false, "newline": false, "builder": false, "parser": false, "Clone": false, "": false,
	}
	for name, want := range cases {
		if got := startsWithWord(name, constructorWords...); got != want {
			t.Errorf("startsWithWord(%q) = %v, want %v", name, got, want)
		}
	}
}

// A type is mentioned as a whole token, optionally through a package
// qualifier; a longer identifier containing the name is not a mention.
func TestMentionsType(t *testing.T) {
	cases := []struct {
		text, qualifier, name string
		want                  bool
	}{
		{"Event", "", "Event", true},
		{"*Event", "", "Event", true},
		{"Promise<Event>", "", "Event", true},
		{"Optional[Event]", "", "Event", true},
		{"\"Event\"", "", "Event", true},
		{"EventID", "", "Event", false},
		{"event.Event", "event", "Event", true},
		{"*event.Event", "event", "Event", true},
		{"[]event.Event", "event", "Event", true},
		{"Event", "event", "Event", false},
		{"other.Event", "event", "Event", false},
		{"event.EventID", "event", "Event", false},
	}
	for _, tc := range cases {
		if got := mentionsType(tc.text, tc.qualifier, tc.name); got != tc.want {
			t.Errorf("mentionsType(%q, %q, %q) = %v, want %v", tc.text, tc.qualifier, tc.name, got, tc.want)
		}
	}
}

// A result headed by a collection holds values it did not build.
func TestIsCollection(t *testing.T) {
	cases := map[string]bool{
		"[]Order": true, "[]*Order": true, "map[string]Order": true, "...Order": true,
		"Order[]": true, "Array<Order>": true, "ReadonlyArray<Order>": true, "Map<string, Order>": true,
		"list[Order]": true, "dict[str, Order]": true, "Sequence[Order]": true, "readonly Order[]": true,
		"Order": false, "*Order": false, "Promise<Order>": false, "Result<Order, Error>": false,
		"Optional[Order]": false, "Order | undefined": false, "\"Order\"": false, "": false,
	}
	for text, want := range cases {
		if got := isCollection(text); got != want {
			t.Errorf("isCollection(%q) = %v, want %v", text, got, want)
		}
	}
}

// The doors of a type: constructor and __init__ on it, factory methods
// on it that start with a constructor word and return it, and every
// function returning it; a method returning it under another name and
// a function returning a collection of it are not doors.
func TestIsConstructor(t *testing.T) {
	cases := []struct {
		name string
		decl Declaration
		want bool
	}{
		{"constructor", Declaration{Kind: "method", Owner: "Money", Name: "constructor"}, true},
		{"__init__", Declaration{Kind: "method", Owner: "Money", Name: "__init__"}, true},
		{"static from", Declaration{Kind: "method", Owner: "Money", Name: "fromCents", Results: []string{"Money"}}, true},
		{"classmethod parse", Declaration{Kind: "method", Owner: "Money", Name: "parse", Results: []string{"\"Money\""}}, true},
		{"Clone", Declaration{Kind: "method", Owner: "Money", Name: "Clone", Results: []string{"Money"}}, false},
		{"from without result", Declaration{Kind: "method", Owner: "Money", Name: "fromCents"}, false},
		{"another type's method", Declaration{Kind: "method", Owner: "Wallet", Name: "create", Results: []string{"Money"}}, false},
		{"New", Declaration{Kind: "func", Name: "New", Results: []string{"Money", "error"}}, true},
		{"NewMoney", Declaration{Kind: "func", Name: "NewMoney", Results: []string{"*Money", "error"}}, true},
		{"FromCents", Declaration{Kind: "func", Name: "FromCents", Results: []string{"Money"}}, true},
		{"Load", Declaration{Kind: "func", Name: "Load", Results: []string{"Money"}}, true},
		{"Sorted returns a slice", Declaration{Kind: "func", Name: "Sorted", Results: []string{"[]Money"}}, false},
		{"unrelated func", Declaration{Kind: "func", Name: "Total", Results: []string{"int"}}, false},
		{"a type", Declaration{Kind: "struct", Name: "Money"}, false},
	}
	for _, tc := range cases {
		if got := isConstructor(tc.decl, "Money"); got != tc.want {
			t.Errorf("%s: isConstructor = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A test file of any supported language holds no door and no command
// of the model.
func TestIsTestFile(t *testing.T) {
	cases := map[string]bool{
		"internal/order/order_test.go": true, "src/order.test.ts": true, "src/order.spec.tsx": true,
		"tests/test_order.py": true, "tests/order_test.py": true, "tests/conftest.py": true,
		"internal/order/order.go": false, "src/order.ts": false, "src/testing/order.ts": false,
		"src/order.py": false, "internal/order/contest.go": false,
	}
	for p, want := range cases {
		if got := isTestFile(p); got != want {
			t.Errorf("isTestFile(%q) = %v, want %v", p, got, want)
		}
	}
}
