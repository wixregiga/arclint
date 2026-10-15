package conformance_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// resolvedGo is one parsed Go file whose calls and field types are
// resolved.
func resolvedGo(pkg string, decls []conformance.Declaration, calls ...conformance.Call) conformance.LanguageFacts {
	f := goDecls(pkg, decls, calls...)
	f.ResolvedCallsAvailable = true
	return f
}

func declared(file, owner, name string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionDeclared, File: file, Owner: owner, Name: name}
}

func external(pkg, owner, name string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionExternal, Package: pkg, Owner: owner, Name: name}
}

func undetermined(reason string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionUndetermined, Reason: reason}
}

// evaluationOf returns the one evaluation of a rule over a context.
func evaluationOf(t *testing.T, a conformance.Assessment, id, context string) conformance.Evaluation {
	t.Helper()
	for _, e := range a.Evaluations() {
		if e.Rule().Qualified() == id && e.Subject().String() == "context:"+context {
			return e
		}
	}
	t.Fatalf("no evaluation of %s for context %s", id, context)
	return conformance.Evaluation{}
}

func orderingWith(t *testing.T, agg vocab.Aggregate, services ...vocab.DomainService) vocab.UbiquitousLanguage {
	t.Helper()
	return oneContext(t, vocab.BoundedContext{
		Name: "ordering", Definition: "d", Line: 1,
		Aggregates: []vocab.Aggregate{agg},
		Services:   services,
	})
}

// An aggregate's root or member calling a repository is a finding,
// whether the call reaches a method the repository declares or a
// method promoted into it from an embedded interface; a call to
// anything else conforms, and a call resolution cannot follow leaves
// the rule undetermined, with the reason, unless a violation is
// proven.
func TestRepositoryNotCalledFromAggregates(t *testing.T) {
	const orderGo, repoGo, storeGo = "internal/order/order.go", "internal/order/repository.go", "internal/store/store.go"
	knowledge := orderingWith(t, vocab.Aggregate{
		Name: "Order", Definition: "d", Identity: "OrderID", Line: 2, Repository: "OrderRepository",
		Entities: []vocab.Entity{{Name: "OrderLine", Definition: "d", Line: 3}},
	})
	r := builtInRule(t, "repository/not-called-from-aggregates")
	repo := resolvedGo("order", []conformance.Declaration{{Kind: "interface", Name: "Repository", Exported: true, StartLine: 3}})
	store := resolvedGo("store", []conformance.Declaration{{Kind: "interface", Name: "Store", Exported: true, StartLine: 3}})
	orderDecls := []conformance.Declaration{
		{Kind: "struct", Name: "Order", Exported: true, StartLine: 5},
		{Kind: "struct", Name: "Line", Exported: true, StartLine: 9},
		{Kind: "method", Name: "Place", Owner: "Order", Exported: true, StartLine: 12, Results: []string{"error"}},
		{Kind: "method", Name: "Reprice", Owner: "Line", Exported: true, StartLine: 20, Results: []string{"error"}},
	}
	run := func(calls ...conformance.Call) conformance.Assessment {
		facts := map[string]conformance.LanguageFacts{
			orderGo: resolvedGo("order", orderDecls, calls...),
			repoGo:  repo,
			storeGo: store,
		}
		return runOne(t, r, nil, observed(orderGo, repoGo, storeGo), facts, knowledge)
	}

	t.Run("a root calling the repository's own method", func(t *testing.T) {
		a := run(conformance.Call{Callee: "Save", Line: 14, Enclosing: "Place", EnclosingOwner: "Order",
			Target: declared(repoGo, "Repository", "Save"), Receiver: declared(repoGo, "", "Repository")})
		if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != orderGo+":14" {
			t.Fatalf("violations at %v, want the call in Place", got)
		}
		if msg := a.Violations()[0].Message(); !strings.Contains(msg, "Order.Place calls Save on the repository Repository") {
			t.Errorf("message %q does not name the caller and the repository", msg)
		}
	})

	t.Run("a member calling a method promoted from an embedded interface", func(t *testing.T) {
		a := run(conformance.Call{Callee: "Put", Line: 22, Enclosing: "Reprice", EnclosingOwner: "Line",
			Target: declared(storeGo, "Store", "Put"), Receiver: declared(repoGo, "", "Repository")})
		if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != orderGo+":22" {
			t.Fatalf("violations at %v, want the call in Reprice: the value it is called on is the repository", got)
		}
	})

	t.Run("a repository two declarations spell", func(t *testing.T) {
		const otherGo = "internal/order/legacy.go"
		facts := map[string]conformance.LanguageFacts{
			orderGo: resolvedGo("order", orderDecls, conformance.Call{Callee: "Save", Line: 14, Enclosing: "Place", EnclosingOwner: "Order",
				Target: declared(repoGo, "Repository", "Save"), Receiver: declared(repoGo, "", "Repository")}),
			repoGo:  repo,
			storeGo: store,
			otherGo: resolvedGo("order", []conformance.Declaration{{Kind: "interface", Name: "OrderRepository", Exported: true, StartLine: 3}}),
		}
		a := runOne(t, r, nil, observed(orderGo, repoGo, storeGo, otherGo), facts, knowledge)
		e := evaluationOf(t, a, r.ID().Qualified(), "ordering")
		if e.Outcome() != conformance.OutcomeUndetermined || len(e.Unresolved()) != 1 || !strings.HasPrefix(e.Unresolved()[0], "repository OrderRepository is spelled by") {
			t.Fatalf("outcome %s with unresolved %v, want undetermined: either declaration may be the repository", e.Outcome(), e.Unresolved())
		}
	})

	t.Run("a file of the aggregate's package that does not parse", func(t *testing.T) {
		const brokenGo = "internal/order/broken.go"
		facts := map[string]conformance.LanguageFacts{
			orderGo:  resolvedGo("order", orderDecls),
			repoGo:   repo,
			storeGo:  store,
			brokenGo: {Language: rule.LanguageGo, ParseFailure: "expected '}'"},
		}
		a := runOne(t, r, nil, observed(orderGo, repoGo, storeGo, brokenGo), facts, knowledge)
		if e := evaluationOf(t, a, r.ID().Qualified(), "ordering"); e.Outcome() != conformance.OutcomeUndetermined {
			t.Fatalf("outcome %s, want undetermined: a method of Order may be in the file that does not parse", e.Outcome())
		}
	})

	t.Run("calls to anything else conform", func(t *testing.T) {
		a := run(
			conformance.Call{Callee: "Now", Line: 13, Enclosing: "Place", EnclosingOwner: "Order", Target: external("time", "", "Now")},
			// A conversion to the repository type runs none of its operations.
			conformance.Call{Callee: "Repository", Line: 15, Enclosing: "Place", EnclosingOwner: "Order", Target: declared(repoGo, "", "Repository")},
			conformance.Call{Callee: "Put", Line: 14, Enclosing: "Place", EnclosingOwner: "Order",
				Target: declared(storeGo, "Store", "Put"), Receiver: declared(storeGo, "", "Store")},
		)
		if e := evaluationOf(t, a, r.ID().Qualified(), "ordering"); e.Outcome() != conformance.OutcomeConforms {
			t.Fatalf("outcome %s, want conforms", e.Outcome())
		}
	})

	t.Run("an unresolved call leaves the rule undetermined and says where", func(t *testing.T) {
		a := run(conformance.Call{Callee: "Save", Line: 15, Enclosing: "Place", EnclosingOwner: "Order",
			Target: undetermined("the package failed to load")})
		e := evaluationOf(t, a, r.ID().Qualified(), "ordering")
		if e.Outcome() != conformance.OutcomeUndetermined {
			t.Fatalf("outcome %s, want undetermined", e.Outcome())
		}
		if got := strings.Join(e.Unresolved(), "\n"); got != orderGo+":15: call Save in Order.Place: the package failed to load" {
			t.Errorf("unresolved %q", got)
		}
		var summary string
		for _, d := range a.Diagnostics() {
			if d.Kind() == conformance.DiagnosticCoverage && strings.Contains(d.Message(), r.ID().Qualified()) {
				summary = d.Message()
			}
		}
		if !strings.Contains(summary, "1 subject(s) undetermined") || !strings.Contains(summary, "the package failed to load") {
			t.Errorf("coverage summary %q does not surface the unresolved call", summary)
		}
	})

	t.Run("a proven violation decides despite an unresolved call", func(t *testing.T) {
		a := run(
			conformance.Call{Callee: "Save", Line: 14, Enclosing: "Place", EnclosingOwner: "Order",
				Target: declared(repoGo, "Repository", "Save"), Receiver: declared(repoGo, "", "Repository")},
			conformance.Call{Callee: "Load", Line: 15, Enclosing: "Place", EnclosingOwner: "Order", Target: undetermined("no type")},
		)
		if e := evaluationOf(t, a, r.ID().Qualified(), "ordering"); e.Outcome() != conformance.OutcomeViolates {
			t.Fatalf("outcome %s, want violates", e.Outcome())
		}
	})

	t.Run("a call outside the aggregate's methods is not the aggregate's", func(t *testing.T) {
		a := run(conformance.Call{Callee: "Save", Line: 30, Enclosing: "PlaceOrder", EnclosingOwner: "",
			Target: declared(repoGo, "Repository", "Save"), Receiver: declared(repoGo, "", "Repository")})
		if got := anchorsOf(a)[r.ID().Qualified()]; len(got) != 0 {
			t.Fatalf("violations at %v, want none: a function of the package is no method of the aggregate", got)
		}
	})
}

// Outside an aggregate only its root is held: a field of another type
// holding a member is a finding, the root and its members may hold
// each other, and a field type resolution cannot follow leaves the
// rule undetermined.
func TestRootOnlyReferences(t *testing.T) {
	const orderGo, invoiceGo = "internal/order/order.go", "internal/billing/invoice.go"
	knowledge := orderingWith(t, vocab.Aggregate{
		Name: "Order", Definition: "d", Identity: "OrderID", Line: 2,
		Entities: []vocab.Entity{{Name: "OrderLine", Definition: "d", Line: 3}},
	})
	r := builtInRule(t, "aggregate/root-only-references")
	order := goDecls("order", []conformance.Declaration{
		{Kind: "struct", Name: "Order", Exported: true, StartLine: 5},
		{Kind: "field", Name: "lines", Owner: "Order", StartLine: 6, Holds: []conformance.Reference{declared(orderGo, "", "Line")}, Type: conformance.Reference{Resolution: conformance.ResolutionLocal, Name: "[]Line"}},
		{Kind: "struct", Name: "Line", Exported: true, StartLine: 9},
	})
	run := func(invoiceField conformance.Declaration) conformance.Assessment {
		facts := map[string]conformance.LanguageFacts{
			orderGo: order,
			invoiceGo: goDecls("billing", []conformance.Declaration{
				{Kind: "struct", Name: "Invoice", Exported: true, StartLine: 3},
				invoiceField,
			}),
		}
		return runOne(t, r, nil, observed(orderGo, invoiceGo), facts, knowledge)
	}

	t.Run("a member held outside its aggregate", func(t *testing.T) {
		a := run(conformance.Declaration{Kind: "field", Name: "line", Owner: "Invoice", StartLine: 4,
			Holds: []conformance.Reference{declared(orderGo, "", "Line")}, Type: declared(orderGo, "", "Line")})
		if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != invoiceGo+":4" {
			t.Fatalf("violations at %v, want Invoice.line alone: Order may hold its own Line", got)
		}
	})

	t.Run("the root held outside its aggregate conforms", func(t *testing.T) {
		a := run(conformance.Declaration{Kind: "field", Name: "order", Owner: "Invoice", StartLine: 4,
			Holds: []conformance.Reference{declared(orderGo, "", "Order")}, Type: declared(orderGo, "", "Order")})
		if e := evaluationOf(t, a, r.ID().Qualified(), "ordering"); e.Outcome() != conformance.OutcomeConforms {
			t.Fatalf("outcome %s, want conforms", e.Outcome())
		}
	})

	t.Run("a file that does not parse leaves the aggregate undetermined", func(t *testing.T) {
		const brokenGo = "internal/shipping/shipment.go"
		facts := map[string]conformance.LanguageFacts{
			orderGo:   order,
			invoiceGo: goDecls("billing", []conformance.Declaration{{Kind: "struct", Name: "Invoice", Exported: true, StartLine: 3}}),
			brokenGo:  {Language: rule.LanguageGo, ParseFailure: "expected '}'"},
		}
		a := runOne(t, r, nil, observed(orderGo, invoiceGo, brokenGo), facts, knowledge)
		e := evaluationOf(t, a, r.ID().Qualified(), "ordering")
		if e.Outcome() != conformance.OutcomeUndetermined || len(e.Unresolved()) != 1 || !strings.HasPrefix(e.Unresolved()[0], brokenGo+": the file does not parse") {
			t.Fatalf("outcome %s with unresolved %v, want undetermined naming the unparsed file", e.Outcome(), e.Unresolved())
		}
	})

	t.Run("a file whose parser supplied nothing leaves the aggregate undetermined", func(t *testing.T) {
		const rejectedTS = "web/src/shipping/shipment.ts"
		facts := map[string]conformance.LanguageFacts{
			orderGo:    order,
			invoiceGo:  goDecls("billing", []conformance.Declaration{{Kind: "struct", Name: "Invoice", Exported: true, StartLine: 3}}),
			rejectedTS: {Language: rule.LanguageTypeScript, ImportsAvailable: true},
		}
		a := runOne(t, r, nil, observed(orderGo, invoiceGo, rejectedTS), facts, knowledge)
		e := evaluationOf(t, a, r.ID().Qualified(), "ordering")
		if e.Outcome() != conformance.OutcomeUndetermined || len(e.Unresolved()) != 1 || !strings.HasPrefix(e.Unresolved()[0], rejectedTS+": its declarations and calls are not available") {
			t.Fatalf("outcome %s with unresolved %v, want undetermined naming the unread file", e.Outcome(), e.Unresolved())
		}
	})

	t.Run("a root two declarations spell leaves the aggregate undetermined", func(t *testing.T) {
		const otherOrder = "internal/order/order_v2.go"
		facts := map[string]conformance.LanguageFacts{
			orderGo:    order,
			otherOrder: goDecls("order", []conformance.Declaration{{Kind: "struct", Name: "Order", Exported: true, StartLine: 3}}),
			invoiceGo: goDecls("billing", []conformance.Declaration{
				{Kind: "struct", Name: "Invoice", Exported: true, StartLine: 3},
				{Kind: "field", Name: "line", Owner: "Invoice", StartLine: 4, Holds: []conformance.Reference{declared(orderGo, "", "Line")}, Type: declared(orderGo, "", "Line")},
			}),
		}
		a := runOne(t, r, nil, observed(orderGo, otherOrder, invoiceGo), facts, knowledge)
		e := evaluationOf(t, a, r.ID().Qualified(), "ordering")
		found := false
		for _, u := range e.Unresolved() {
			found = found || strings.HasPrefix(u, "root of aggregate Order is spelled by")
		}
		if e.Outcome() != conformance.OutcomeUndetermined || !found {
			t.Fatalf("outcome %s with unresolved %v, want undetermined naming the ambiguous root", e.Outcome(), e.Unresolved())
		}
	})

	t.Run("an aggregate without members conforms whatever spells its root", func(t *testing.T) {
		const otherOrder = "internal/order/order_v2.go"
		lone := orderingWith(t, vocab.Aggregate{Name: "Order", Definition: "d", Identity: "OrderID", Line: 2})
		facts := map[string]conformance.LanguageFacts{
			orderGo:    order,
			otherOrder: goDecls("order", []conformance.Declaration{{Kind: "struct", Name: "Order", Exported: true, StartLine: 3}}),
		}
		a := runOne(t, r, nil, observed(orderGo, otherOrder), facts, lone)
		if e := evaluationOf(t, a, r.ID().Qualified(), "ordering"); e.Outcome() != conformance.OutcomeConforms {
			t.Fatalf("outcome %s with unresolved %v, want conforms: no member can be held", e.Outcome(), e.Unresolved())
		}
	})

	t.Run("a member two declarations spell leaves the aggregate undetermined", func(t *testing.T) {
		const otherLine = "internal/order/line_v2.go"
		facts := map[string]conformance.LanguageFacts{
			orderGo:   order,
			otherLine: goDecls("order", []conformance.Declaration{{Kind: "struct", Name: "OrderLine", Exported: true, StartLine: 3}}),
			invoiceGo: goDecls("billing", []conformance.Declaration{{Kind: "struct", Name: "Invoice", Exported: true, StartLine: 3}}),
		}
		a := runOne(t, r, nil, observed(orderGo, otherLine, invoiceGo), facts, knowledge)
		e := evaluationOf(t, a, r.ID().Qualified(), "ordering")
		if e.Outcome() != conformance.OutcomeUndetermined || len(e.Unresolved()) != 1 || !strings.HasPrefix(e.Unresolved()[0], "member OrderLine of aggregate Order is spelled by") {
			t.Fatalf("outcome %s with unresolved %v, want undetermined: either declaration may be the member", e.Outcome(), e.Unresolved())
		}
	})

	t.Run("an unresolved field type outside the aggregate", func(t *testing.T) {
		a := run(conformance.Declaration{Kind: "field", Name: "item", Owner: "Invoice", StartLine: 4,
			Holds: []conformance.Reference{undetermined("Item is neither declared in the file nor imported")}, Type: undetermined("Item is neither declared in the file nor imported")})
		e := evaluationOf(t, a, r.ID().Qualified(), "ordering")
		if e.Outcome() != conformance.OutcomeUndetermined || len(e.Unresolved()) != 1 {
			t.Fatalf("outcome %s with unresolved %v, want undetermined with one", e.Outcome(), e.Unresolved())
		}
	})
}

// No field of an aggregate's root or member holds a recorded domain
// service; a service passed into one command is not held.
func TestServiceNotHeldByAggregates(t *testing.T) {
	const orderGo, pricingGo = "internal/order/order.go", "internal/order/pricing.go"
	knowledge := orderingWith(t, vocab.Aggregate{Name: "Order", Definition: "d", Identity: "OrderID", Line: 2},
		vocab.DomainService{Name: "Pricing", Definition: "d", Line: 4})
	r := builtInRule(t, "domain_service/not-held-by-aggregates")
	facts := map[string]conformance.LanguageFacts{
		orderGo: goDecls("order", []conformance.Declaration{
			{Kind: "struct", Name: "Order", Exported: true, StartLine: 5},
			{Kind: "field", Name: "pricing", Owner: "Order", StartLine: 6, Holds: []conformance.Reference{declared(pricingGo, "", "Pricing")}, Type: declared(pricingGo, "", "Pricing")},
			{Kind: "field", Name: "total", Owner: "Order", StartLine: 7, Holds: []conformance.Reference{external("", "", "int")}, Type: external("", "", "int")},
			{Kind: "method", Name: "Reprice", Owner: "Order", Exported: true, StartLine: 10, Params: []conformance.DeclarationParam{{Name: "p", Type: "Pricing"}}, Results: []string{"error"}},
		}),
		pricingGo: goDecls("order", []conformance.Declaration{{Kind: "struct", Name: "Pricing", Exported: true, StartLine: 3}}),
	}
	a := runOne(t, r, nil, observed(orderGo, pricingGo), facts, knowledge)
	if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != orderGo+":6" {
		t.Fatalf("violations at %v, want the pricing field alone", got)
	}
}

// An entity with a recorded identity declares a field holding it, in
// Go spelled through the package that completes the name (order.ID is
// OrderID).
func TestEntityIdentity(t *testing.T) {
	const orderGo = "internal/order/order.go"
	knowledge := orderingWith(t, vocab.Aggregate{
		Name: "Order", Definition: "d", Identity: "OrderID", Line: 2,
		Entities: []vocab.Entity{{Name: "OrderLine", Definition: "d", Identity: "LineNumber", Line: 3}},
	})
	r := builtInRule(t, "entity/identity")
	run := func(lineField conformance.Reference) conformance.Assessment {
		facts := map[string]conformance.LanguageFacts{
			orderGo: goDecls("order", []conformance.Declaration{
				{Kind: "type", Name: "ID", Exported: true, StartLine: 3},
				{Kind: "type", Name: "LineNumber", Exported: true, StartLine: 4},
				{Kind: "struct", Name: "Order", Exported: true, StartLine: 5},
				{Kind: "field", Name: "id", Owner: "Order", StartLine: 6, Holds: []conformance.Reference{declared(orderGo, "", "ID")}, Type: declared(orderGo, "", "ID")},
				{Kind: "struct", Name: "Line", Exported: true, StartLine: 9},
				{Kind: "field", Name: "number", Owner: "Line", StartLine: 10, Holds: []conformance.Reference{lineField}, Type: lineField},
			}),
		}
		return runOne(t, r, nil, observed(orderGo), facts, knowledge)
	}
	if a := run(declared(orderGo, "", "LineNumber")); evaluationOf(t, a, r.ID().Qualified(), "ordering").Outcome() != conformance.OutcomeConforms {
		t.Fatalf("both entities hold their identity; outcome %s", evaluationOf(t, a, r.ID().Qualified(), "ordering").Outcome())
	}
	a := run(external("", "", "int"))
	if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != orderGo+":9" {
		t.Fatalf("violations at %v, want Line, whose number is an int, not its LineNumber", got)
	}
	a = run(undetermined("the package failed to load"))
	if e := evaluationOf(t, a, r.ID().Qualified(), "ordering"); e.Outcome() != conformance.OutcomeUndetermined {
		t.Fatalf("outcome %s, want undetermined: the unresolved field may hold the identity", e.Outcome())
	}
	// A member two declarations spell cannot be judged, so its identity
	// is not proven present.
	ambiguous := map[string]conformance.LanguageFacts{
		orderGo: goDecls("order", []conformance.Declaration{
			{Kind: "type", Name: "ID", Exported: true, StartLine: 3},
			{Kind: "type", Name: "LineNumber", Exported: true, StartLine: 4},
			{Kind: "struct", Name: "Order", Exported: true, StartLine: 5},
			{Kind: "field", Name: "id", Owner: "Order", StartLine: 6, Holds: []conformance.Reference{declared(orderGo, "", "ID")}, Type: declared(orderGo, "", "ID")},
			{Kind: "struct", Name: "Line", Exported: true, StartLine: 9},
		}),
		"internal/order/line_v2.go": goDecls("order", []conformance.Declaration{{Kind: "struct", Name: "OrderLine", Exported: true, StartLine: 3}}),
	}
	a = runOne(t, r, nil, observed(orderGo, "internal/order/line_v2.go"), ambiguous, knowledge)
	if e := evaluationOf(t, a, r.ID().Qualified(), "ordering"); e.Outcome() != conformance.OutcomeUndetermined || len(e.Unresolved()) != 1 || !strings.HasPrefix(e.Unresolved()[0], "member OrderLine of aggregate Order is spelled by") {
		t.Fatalf("outcome %s with unresolved %v, want undetermined naming the ambiguous member", e.Outcome(), e.Unresolved())
	}
	// An identity no readable file declares, beside a file that could
	// not be read, is not proven missing.
	gapFacts := map[string]conformance.LanguageFacts{
		orderGo: goDecls("order", []conformance.Declaration{
			{Kind: "type", Name: "ID", Exported: true, StartLine: 3},
			{Kind: "struct", Name: "Order", Exported: true, StartLine: 5},
			{Kind: "field", Name: "id", Owner: "Order", StartLine: 6, Holds: []conformance.Reference{declared(orderGo, "", "ID")}, Type: declared(orderGo, "", "ID")},
			{Kind: "struct", Name: "Line", Exported: true, StartLine: 9},
		}),
		"internal/order/numbers.go": {Language: rule.LanguageGo, ParseFailure: "expected '}'"},
	}
	a = runOne(t, r, nil, observed(orderGo, "internal/order/numbers.go"), gapFacts, knowledge)
	if e := evaluationOf(t, a, r.ID().Qualified(), "ordering"); e.Outcome() != conformance.OutcomeUndetermined || len(a.Violations()) != 0 {
		t.Fatalf("outcome %s with %d violations, want undetermined: LineNumber may be declared in the unread file", e.Outcome(), len(a.Violations()))
	}
	// A list of identities holds LineNumber values but is not the
	// entity's own identity.
	facts := map[string]conformance.LanguageFacts{
		orderGo: goDecls("order", []conformance.Declaration{
			{Kind: "type", Name: "ID", Exported: true, StartLine: 3},
			{Kind: "type", Name: "LineNumber", Exported: true, StartLine: 4},
			{Kind: "struct", Name: "Order", Exported: true, StartLine: 5},
			{Kind: "field", Name: "id", Owner: "Order", StartLine: 6, Holds: []conformance.Reference{declared(orderGo, "", "ID")}, Type: declared(orderGo, "", "ID")},
			{Kind: "struct", Name: "Line", Exported: true, StartLine: 9},
			{Kind: "field", Name: "numbers", Owner: "Line", StartLine: 10,
				Holds: []conformance.Reference{declared(orderGo, "", "LineNumber")},
				Type:  conformance.Reference{Resolution: conformance.ResolutionLocal, Name: "[]LineNumber"}},
		}),
	}
	a = runOne(t, r, nil, observed(orderGo), facts, knowledge)
	if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != orderGo+":9" {
		t.Fatalf("violations at %v, want Line, whose list of LineNumbers is not its identity", got)
	}
}

// The remediation of a missing identity field asks first whether the
// identity holds a part that can change: an identity composed of a
// recorded value object that is no identity, such as a role's name,
// is right only if that part never changes while the entity stays
// the same. A leaf identity keeps the plain instruction.
func TestEntityIdentityRemediation(t *testing.T) {
	const roleGo, tenantGo = "internal/role/role.go", "internal/tenant/tenant.go"
	knowledge := oneContext(t, vocab.BoundedContext{
		Name: "access", Definition: "d", Line: 1,
		Aggregates: []vocab.Aggregate{
			{Name: "Role", Definition: "d", Identity: "RoleID", Line: 2},
			{Name: "Tenant", Definition: "d", Identity: "TenantID", Line: 3},
		},
		ValueObjects: []vocab.ValueObject{
			{Name: "RoleID", Definition: "d", Line: 4},
			{Name: "RoleName", Definition: "d", Line: 5},
			{Name: "TenantID", Definition: "d", Line: 6},
		},
	})
	r := builtInRule(t, "entity/identity")
	tenant := goDecls("tenant", []conformance.Declaration{
		{Kind: "struct", Name: "ID", Exported: true, StartLine: 3},
		{Kind: "struct", Name: "Tenant", Exported: true, StartLine: 5},
		{Kind: "field", Name: "id", Owner: "Tenant", StartLine: 6, Holds: []conformance.Reference{declared(tenantGo, "", "ID")}, Type: declared(tenantGo, "", "ID")},
	})
	remediation := func(t *testing.T, idFields ...conformance.Declaration) string {
		t.Helper()
		decls := append([]conformance.Declaration{
			{Kind: "struct", Name: "ID", Exported: true, StartLine: 3},
			{Kind: "type", Name: "Name", Exported: true, StartLine: 8},
			{Kind: "struct", Name: "Role", Exported: true, StartLine: 10},
			{Kind: "field", Name: "name", Owner: "Role", StartLine: 11, Holds: []conformance.Reference{declared(roleGo, "", "Name")}, Type: declared(roleGo, "", "Name")},
		}, idFields...)
		facts := map[string]conformance.LanguageFacts{roleGo: goDecls("role", decls), tenantGo: tenant}
		a := runOne(t, r, nil, observed(roleGo, tenantGo), facts, knowledge)
		if got := anchorsOf(a)[r.ID().Qualified()]; strings.Join(got, ",") != roleGo+":10" {
			t.Fatalf("violations at %v, want Role, which holds no RoleID", got)
		}
		return a.Violations()[0].Remediation()
	}

	t.Run("an identity composed of a name", func(t *testing.T) {
		got := remediation(t,
			conformance.Declaration{Kind: "field", Name: "TenantID", Owner: "ID", Exported: true, StartLine: 4, Holds: []conformance.Reference{declared(tenantGo, "", "ID")}, Type: declared(tenantGo, "", "ID")},
			conformance.Declaration{Kind: "field", Name: "Name", Owner: "ID", Exported: true, StartLine: 5, Holds: []conformance.Reference{declared(roleGo, "", "Name")}, Type: declared(roleGo, "", "Name")},
		)
		want := "RoleID includes RoleName, which is not an identity. If RoleName can change while the Role stays the same, it is not part of the identity; change the identity before adding a field. If it cannot change, declare a field of type ID on Role."
		if got != want {
			t.Fatalf("remediation\n%s\nwant\n%s\nTenantID is an identity, so only RoleName is asked about", got, want)
		}
	})

	t.Run("a leaf identity", func(t *testing.T) {
		got := remediation(t)
		if want := "declare a field of type ID on Role; its identity is what tells one Role from another"; got != want {
			t.Fatalf("remediation %q, want %q", got, want)
		}
	})
}

// Observations refuse a resolution that could pass for what it is
// not: a declared reference into a file never observed, an
// undetermined one without its reason, and a resolution on facts whose
// resolution was not supplied.
func TestObservationsRefuseDishonestResolution(t *testing.T) {
	const file = "internal/order/order.go"
	cases := map[string]conformance.LanguageFacts{
		"declared into an unobserved file": resolvedGo("order", nil, conformance.Call{Callee: "Save", Line: 3, Target: declared("internal/gone.go", "", "Save")}),
		"undetermined without a reason":    resolvedGo("order", nil, conformance.Call{Callee: "Save", Line: 3, Target: conformance.Reference{Resolution: conformance.ResolutionUndetermined}}),
		"a call left unresolved":           resolvedGo("order", nil, conformance.Call{Callee: "Save", Line: 3}),
		"a target nobody asked for":        goDecls("order", nil, conformance.Call{Callee: "Save", Line: 3, Target: external("time", "", "Now")}),
		"field types without declarations": {Language: rule.LanguageGo, FieldTypesAvailable: true},
		"a field type nobody asked for": {Language: rule.LanguageGo, DeclarationsAvailable: true, Declarations: []conformance.Declaration{
			{Kind: "field", Name: "id", Owner: "Order", Holds: []conformance.Reference{external("", "", "int")}, Type: external("", "", "int")},
		}},
	}
	for name, facts := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := conformance.NewObservations(observed(file), map[string]conformance.LanguageFacts{file: facts}); err == nil {
				t.Fatal("observations accepted a dishonest resolution")
			}
		})
	}
}
