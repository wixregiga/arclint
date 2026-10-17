package conformance_test

import (
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// resolvedFile is the facts of one parsed file with resolved calls and
// field types, the way a language reports them when asked for uses.
func resolvedFile(lang rule.Language, pkg string, decls []conformance.Declaration, calls []conformance.Call) conformance.LanguageFacts {
	return conformance.LanguageFacts{
		Language:               lang,
		Package:                pkg,
		DeclarationsAvailable:  true,
		Declarations:           decls,
		CallsAvailable:         true,
		Calls:                  calls,
		ResolvedCallsAvailable: true,
		FieldTypesAvailable:    true,
	}
}

// usesOf locates the recorded contexts in files observed with every
// reference fact, and finds the uses of the files under the worksite
// prefixes.
func usesOf(t *testing.T, contexts []vocab.BoundedContext, files map[string]conformance.LanguageFacts, worksite ...string) conformance.TermUses {
	t.Helper()
	return usesObserving(t, contexts, nil, files, nil, worksite...)
}

// usesObserving is usesOf over a walk that also observed files no
// language supplied facts for.
func usesObserving(t *testing.T, contexts []vocab.BoundedContext, zones []rule.Zone, files map[string]conformance.LanguageFacts, unread []string, worksite ...string) conformance.TermUses {
	t.Helper()
	observed := make([]conformance.ObservedFile, 0, len(files)+len(unread))
	for p := range files {
		observed = append(observed, conformance.ObservedFile{Path: p})
	}
	for _, p := range unread {
		observed = append(observed, conformance.ObservedFile{Path: p})
	}
	sort.Slice(observed, func(i, j int) bool { return observed[i].Path < observed[j].Path })
	obs, err := conformance.NewObservations(observed, files)
	if err != nil {
		t.Fatalf("NewObservations: %v", err)
	}
	knowledge, err := newRecordedLanguage("p", "", contexts, nil)
	if err != nil {
		t.Fatalf("newRecordedLanguage: %v", err)
	}
	facts, err := conformance.NewInspectionFacts(zones, obs, rule.FactFileTree, rule.FactDeclarations, rule.FactCalls, rule.FactResolvedCalls, rule.FactFieldTypes)
	if err != nil {
		t.Fatal(err)
	}
	c, err := conformance.NewCarriers(facts, knowledge)
	if err != nil {
		t.Fatalf("NewCarriers: %v", err)
	}
	uses, err := c.Uses(func(p string) bool {
		for _, w := range worksite {
			if p == w || strings.HasPrefix(p, w+"/") {
				return true
			}
		}
		return false
	}, []rule.Language{rule.LanguageGo, rule.LanguageTypeScript, rule.LanguagePython})
	if err != nil {
		t.Fatalf("Uses: %v", err)
	}
	return uses
}

// specOf spells an undetermined use back as the spec it was built from.
func specOf(u conformance.UndeterminedUse) conformance.UndeterminedUseSpec {
	return conformance.UndeterminedUseSpec{Path: u.Path(), Line: u.Line(), What: u.What(), Reason: u.Reason(), Context: u.Context(), Term: u.Term()}
}

// spelled renders each use as term role owner way via path:line, in
// order; the owner is the aggregate, or the aggregates a value object
// identifies joined by +.
func spelled(uses []conformance.TermUse) []string {
	out := make([]string, len(uses))
	for i, u := range uses {
		owner := u.Aggregate()
		if ids := u.Identifies(); len(ids) > 0 {
			owner = strings.Join(ids, "+")
		}
		out[i] = strings.Join([]string{u.Term(), string(u.Role()), owner, string(u.Way()), u.Via(), u.Path() + ":" + strconv.Itoa(u.Line())}, " ")
	}
	return out
}

// orderingContext records an Order aggregate with a member Line and a
// repository, a Money value object, and a Pricing service.
func orderingContext() vocab.BoundedContext {
	order := aggregateOf("Order", "OrderID", []string{"has-lines"}, nil)
	order.Entities = []vocab.Entity{{Name: "Line", Definition: "one line of an Order"}}
	order.Repository = "OrderRepository"
	ctx := contextOf("ordering", []vocab.Aggregate{order}, []vocab.ValueObject{valueObject("Money")}, nil)
	ctx.Services = []vocab.DomainService{{Name: "Pricing", Definition: "prices an order"}}
	return ctx
}

// goOrdering is the ordering context in Go: the root's command in a
// second file of its package, a constructor function for Money, and a
// repository interface.
func goOrdering() map[string]conformance.LanguageFacts {
	return map[string]conformance.LanguageFacts{
		"ordering/order.go": resolvedFile(rule.LanguageGo, "ordering", []conformance.Declaration{
			{Kind: "struct", Name: "Order", Exported: true, StartLine: 3},
			{Kind: "struct", Name: "Line", Exported: true, StartLine: 9},
			{Kind: "method", Name: "EnsureHasLines", Owner: "Order", Exported: true, StartLine: 12},
		}, nil),
		"ordering/order_commands.go": resolvedFile(rule.LanguageGo, "ordering", []conformance.Declaration{
			{Kind: "method", Name: "Place", Owner: "Order", Exported: true, StartLine: 5},
		}, nil),
		"ordering/money.go": resolvedFile(rule.LanguageGo, "ordering", []conformance.Declaration{
			{Kind: "struct", Name: "Money", Exported: true, StartLine: 3},
			{Kind: "func", Name: "NewMoney", Exported: true, StartLine: 7, Results: []string{"Money", "error"}},
			{Kind: "func", Name: "Sum", Exported: true, StartLine: 15, Results: []string{"[]Money"}},
		}, nil),
		"ordering/pricing.go": resolvedFile(rule.LanguageGo, "ordering", []conformance.Declaration{
			{Kind: "struct", Name: "Pricing", Exported: true, StartLine: 3},
			{Kind: "method", Name: "Quote", Owner: "Pricing", Exported: true, StartLine: 6},
		}, nil),
		"ordering/repository.go": resolvedFile(rule.LanguageGo, "ordering", []conformance.Declaration{
			{Kind: "interface", Name: "OrderRepository", Exported: true, StartLine: 3},
			{Kind: "method", Name: "Save", Owner: "OrderRepository", Exported: true, StartLine: 4},
		}, nil),
	}
}

// A use case reaches the root through a command declared in another
// file of its package, a service through its receiver, and a
// repository through a field; a call that only spells a recorded name
// and an external call make no use. An unresolvable call is
// undetermined with its reason, and so is a call to a function beside
// Money whose written result names it, since result types are not
// resolved.
func TestUsesFollowResolutionInGo(t *testing.T) {
	files := goOrdering()
	files["app/place.go"] = resolvedFile(rule.LanguageGo, "app", []conformance.Declaration{
		{Kind: "struct", Name: "PlaceOrder", Exported: true, StartLine: 5},
		{Kind: "field", Name: "orders", Owner: "PlaceOrder", StartLine: 6, Holds: []conformance.Reference{declared("ordering/repository.go", "", "OrderRepository")}, Type: declared("ordering/repository.go", "", "OrderRepository")},
		{Kind: "field", Name: "events", Owner: "PlaceOrder", StartLine: 7, Holds: []conformance.Reference{undetermined("the package bus does not type-check")}, Type: undetermined("the package bus does not type-check")},
		{Kind: "func", Name: "Money", StartLine: 30},
		{Kind: "method", Name: "Run", Owner: "PlaceOrder", Exported: true, StartLine: 10},
	}, []conformance.Call{
		{Callee: "NewMoney", Line: 11, Enclosing: "Run", Target: declared("ordering/money.go", "", "NewMoney")},
		{Callee: "Place", Line: 12, Enclosing: "Run", Target: declared("ordering/order_commands.go", "Order", "Place"), Receiver: declared("ordering/order.go", "", "Order")},
		{Callee: "Quote", Line: 13, Enclosing: "Run", Target: declared("ordering/pricing.go", "Pricing", "Quote"), Receiver: declared("ordering/pricing.go", "", "Pricing")},
		{Callee: "Money", Line: 14, Enclosing: "Run", Target: declared("app/place.go", "", "Money")},
		{Callee: "Println", Line: 15, Enclosing: "Run", Target: external("fmt", "", "Println")},
		{Callee: "Sum", Line: 16, Enclosing: "Run", Target: declared("ordering/money.go", "", "Sum")},
		{Callee: "handle", Line: 17, Enclosing: "Run", Target: undetermined("a call through a function value is not followed")},
	})
	uses := usesOf(t, []vocab.BoundedContext{orderingContext()}, files, "app")
	want := []string{
		"OrderRepository repository Order field PlaceOrder.orders app/place.go:6",
		"Order aggregate  call Place app/place.go:12",
		"Pricing domain_service  call Quote app/place.go:13",
	}
	if got := spelled(uses.Uses); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("uses =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if u := uses.Uses[1]; u.Context() != "ordering" {
		t.Fatalf("use context = %q", u.Context())
	}
	wantUndetermined := []conformance.UndeterminedUseSpec{
		{Path: "app/place.go", Line: 7, What: "field PlaceOrder.events", Reason: "the package bus does not type-check"},
		{Path: "app/place.go", Line: 11, What: "call NewMoney", Reason: "NewMoney returns Money as written; result types are not resolved", Context: "ordering", Term: "Money"},
		{Path: "app/place.go", Line: 16, What: "call Sum", Reason: "Sum returns []Money as written; result types are not resolved", Context: "ordering", Term: "Money"},
		{Path: "app/place.go", Line: 17, What: "call handle", Reason: "a call through a function value is not followed"},
	}
	if len(uses.Undetermined) != len(wantUndetermined) {
		t.Fatalf("undetermined = %+v", uses.Undetermined)
	}
	for i, u := range wantUndetermined {
		if got := specOf(uses.Undetermined[i]); got != u {
			t.Fatalf("undetermined[%d] = %+v, want %+v", i, got, u)
		}
	}
}

// In TypeScript and Python a member belongs to the class of its file:
// a method of a same-named class in another file is not the term's.
// A static factory method is a member and makes a use; a member entity
// is reported with its aggregate.
func TestUsesReadMembersByFileInTypeScriptAndPython(t *testing.T) {
	for _, tc := range []struct {
		lang rule.Language
		ext  string
	}{{rule.LanguageTypeScript, ".ts"}, {rule.LanguagePython, ".py"}} {
		t.Run(string(tc.lang), func(t *testing.T) {
			order, line, money, other := "ordering/order"+tc.ext, "ordering/line"+tc.ext, "ordering/money"+tc.ext, "legacy/money"+tc.ext
			app := "app/place" + tc.ext
			files := map[string]conformance.LanguageFacts{
				order: resolvedFile(tc.lang, "", []conformance.Declaration{
					{Kind: "class", Name: "Order", Exported: true, StartLine: 1},
					{Kind: "method", Name: "place", Owner: "Order", Exported: true, StartLine: 4},
				}, nil),
				line: resolvedFile(tc.lang, "", []conformance.Declaration{
					{Kind: "class", Name: "Line", Exported: true, StartLine: 1},
					{Kind: "method", Name: "amount", Owner: "Line", Exported: true, StartLine: 2},
				}, nil),
				money: resolvedFile(tc.lang, "", []conformance.Declaration{
					{Kind: "class", Name: "Money", Exported: true, StartLine: 1},
					{Kind: "method", Name: "from", Owner: "Money", Exported: true, StartLine: 2, Results: []string{"Money"}},
				}, nil),
				other: resolvedFile(tc.lang, "", []conformance.Declaration{
					{Kind: "class", Name: "Wallet", Exported: true, StartLine: 1},
					{Kind: "method", Name: "from", Owner: "Money", Exported: true, StartLine: 2},
				}, nil),
				app: resolvedFile(tc.lang, "", nil, []conformance.Call{
					{Callee: "from", Line: 3, Target: declared(money, "Money", "from")},
					{Callee: "place", Line: 4, Target: declared(order, "Order", "place"), Receiver: declared(order, "", "Order")},
					{Callee: "amount", Line: 5, Target: declared(line, "Line", "amount")},
					{Callee: "from", Line: 6, Target: declared(other, "Money", "from")},
				}),
			}
			uses := usesOf(t, []vocab.BoundedContext{orderingContext()}, files, "app")
			want := []string{
				"Money value_object  call from " + app + ":3",
				"Order aggregate  call place " + app + ":4",
				"Line entity Order call amount " + app + ":5",
			}
			if got := spelled(uses.Uses); strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Fatalf("uses =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			if len(uses.Undetermined) != 0 {
				t.Fatalf("undetermined = %+v", uses.Undetermined)
			}
		})
	}
}

// Uses reports what the worksite reaches wherever it is declared, the
// worksite included: which entries a listing shows as declared here is
// the listing's decision. A term two declarations spell leaves a
// reference to either undetermined, naming the term, rather than
// picking one.
func TestUsesReportEveryTermAndNeverPickATwin(t *testing.T) {
	ctx := orderingContext()
	ctx.ValueObjects = append(ctx.ValueObjects, valueObject("Currency"))
	files := goOrdering()
	for _, p := range []string{"billing/currency.go", "legacy/currency.go"} {
		files[p] = resolvedFile(rule.LanguageGo, path.Base(path.Dir(p)), []conformance.Declaration{
			{Kind: "struct", Name: "Currency", Exported: true, StartLine: 2},
			{Kind: "method", Name: "Code", Owner: "Currency", Exported: true, StartLine: 5},
		}, nil)
	}
	files["app/place.go"] = resolvedFile(rule.LanguageGo, "app", nil, []conformance.Call{
		{Callee: "Place", Line: 4, Target: declared("ordering/order_commands.go", "Order", "Place")},
		{Callee: "Quote", Line: 5, Target: declared("ordering/pricing.go", "Pricing", "Quote")},
		{Callee: "Code", Line: 6, Target: declared("billing/currency.go", "Currency", "Code")},
	})
	uses := usesOf(t, []vocab.BoundedContext{ctx}, files, "app", "ordering/order.go")
	want := []string{"Order aggregate  call Place app/place.go:4", "Pricing domain_service  call Quote app/place.go:5"}
	if got := spelled(uses.Uses); !slices.Equal(got, want) {
		t.Fatalf("uses = %v, want %v", got, want)
	}
	if len(uses.Undetermined) != 1 {
		t.Fatalf("undetermined = %+v", uses.Undetermined)
	}
	u := uses.Undetermined[0]
	if u.Line() != 6 || u.Context() != "ordering" || u.Term() != "Currency" || !strings.Contains(u.Reason(), "spelling Currency") || !strings.Contains(u.Reason(), "billing/currency.go") || !strings.Contains(u.Reason(), "legacy/currency.go") {
		t.Fatalf("twin reference = %+v", u)
	}
}

// A worksite file that does not parse, or whose language supplied no
// resolved calls or field types, is undetermined as a whole: a missing
// use is never read as no use.
func TestUsesReportUnreadableFiles(t *testing.T) {
	files := goOrdering()
	files["app/broken.go"] = conformance.LanguageFacts{Language: rule.LanguageGo, ParseFailure: "expected declaration"}
	files["app/plain.go"] = conformance.LanguageFacts{Language: rule.LanguageGo, DeclarationsAvailable: true, CallsAvailable: true}
	uses := usesOf(t, []vocab.BoundedContext{orderingContext()}, files, "app")
	got := map[string]string{}
	for _, u := range uses.Undetermined {
		got[u.Path()+" "+u.What()] = u.Reason()
	}
	if !strings.Contains(got["app/broken.go file"], "does not parse") {
		t.Fatalf("parse failure not undetermined: %+v", uses.Undetermined)
	}
	if got["app/plain.go calls"] == "" || got["app/plain.go fields"] == "" {
		t.Fatalf("unresolved file not undetermined: %+v", uses.Undetermined)
	}
}

// A worksite file of a configured language that its language supplied
// no facts for, such as a TypeScript declaration file, is undetermined;
// a file of no configured language is no code to read.
func TestUsesReportFilesTheirLanguageDidNotRead(t *testing.T) {
	uses := usesObserving(t, []vocab.BoundedContext{orderingContext()}, nil, goOrdering(), []string{"app/consumer.d.ts", "app/README.md"}, "app")
	if len(uses.Undetermined) != 1 {
		t.Fatalf("undetermined = %+v", uses.Undetermined)
	}
	if u := uses.Undetermined[0]; u.Path() != "app/consumer.d.ts" || u.What() != "file" || u.Line() != 0 || u.Reason() == "" {
		t.Fatalf("unread file = %+v", u)
	}
}

// A function never makes a use: the language resolves the call to the
// function, not to the type it returns, and result types are not
// resolved. A call to a function beside a term whose written result
// names it is undetermined, whatever wraps or qualifies the name, since
// a type parameter or an import can spell it too; a function whose
// result names no term, or one away from the term, is a call to a
// declaration that is no recorded entry.
func TestUsesLeaveFunctionResultsUndetermined(t *testing.T) {
	files := goOrdering()
	files["ordering/legacy.go"] = resolvedFile(rule.LanguageGo, "ordering", []conformance.Declaration{
		{Kind: "func", Name: "LegacyMoney", Exported: true, StartLine: 3, Results: []string{"legacy.Money"}},
		{Kind: "func", Name: "Echo", Exported: true, StartLine: 6, Results: []string{"Money"}},
		{Kind: "func", Name: "Generic", Exported: true, StartLine: 9, Results: []string{"Money[T]"}},
		{Kind: "func", Name: "Count", Exported: true, StartLine: 12, Results: []string{"int"}},
	}, nil)
	files["ordering/pricing_helpers.py"] = resolvedFile(rule.LanguagePython, "", []conformance.Declaration{
		{Kind: "func", Name: "make_order", StartLine: 1, Results: []string{"Order"}},
	}, nil)
	files["app/place.go"] = resolvedFile(rule.LanguageGo, "app", nil, []conformance.Call{
		{Callee: "LegacyMoney", Line: 3, Target: declared("ordering/legacy.go", "", "LegacyMoney")},
		{Callee: "Echo", Line: 4, Target: declared("ordering/legacy.go", "", "Echo")},
		{Callee: "Generic", Line: 5, Target: declared("ordering/legacy.go", "", "Generic")},
		{Callee: "Count", Line: 6, Target: declared("ordering/legacy.go", "", "Count")},
		{Callee: "make_order", Line: 7, Target: declared("ordering/pricing_helpers.py", "", "make_order")},
	})
	uses := usesOf(t, []vocab.BoundedContext{orderingContext()}, files, "app")
	if len(uses.Uses) != 0 {
		t.Fatalf("a function made a use: %v", spelled(uses.Uses))
	}
	var lines []int
	for _, u := range uses.Undetermined {
		lines = append(lines, u.Line())
		if !strings.Contains(u.Reason(), "Money") || !strings.HasSuffix(u.Reason(), " as written; result types are not resolved") {
			t.Errorf("undetermined reason = %q", u.Reason())
		}
	}
	if !slices.Equal(lines, []int{3, 4, 5}) {
		t.Fatalf("undetermined lines = %v: %+v", lines, uses.Undetermined)
	}
}

// A member never belongs to a type of another language: a Go method
// beside a TypeScript class of the same name, each recorded in its own
// context, makes a use of the Go type alone, and a Go function whose
// written result names the TypeScript class raises no question about
// it.
func TestUsesNeverCrossLanguages(t *testing.T) {
	zone := func(name, glob string) rule.Zone {
		g, err := rule.NewGlob(glob)
		if err != nil {
			t.Fatal(err)
		}
		z, err := rule.NewZone(rule.ZoneName(name), "the "+name+" code", []rule.Glob{g})
		if err != nil {
			t.Fatal(err)
		}
		return z
	}
	gomodel := contextOf("gomodel", nil, []vocab.ValueObject{valueObject("Money")}, nil)
	tsmodel := contextOf("tsmodel", nil, []vocab.ValueObject{valueObject("Money")}, nil)
	files := map[string]conformance.LanguageFacts{
		"model/money.go": resolvedFile(rule.LanguageGo, "model", []conformance.Declaration{
			{Kind: "struct", Name: "Money", Exported: true, StartLine: 3},
			{Kind: "method", Name: "Add", Owner: "Money", Exported: true, StartLine: 6},
			{Kind: "func", Name: "Zero", Exported: true, StartLine: 9, Results: []string{"Money"}},
		}, nil),
		"model/money.ts": resolvedFile(rule.LanguageTypeScript, "", []conformance.Declaration{
			{Kind: "class", Name: "Money", Exported: true, StartLine: 1},
			{Kind: "method", Name: "add", Owner: "Money", Exported: true, StartLine: 2},
		}, nil),
		"app/run.go": resolvedFile(rule.LanguageGo, "app", nil, []conformance.Call{
			{Callee: "Add", Line: 4, Target: declared("model/money.go", "Money", "Add"), Receiver: declared("model/money.go", "", "Money")},
			{Callee: "Zero", Line: 5, Target: declared("model/money.go", "", "Zero")},
		}),
	}
	zones := []rule.Zone{zone("gomodel", "model/*.go"), zone("tsmodel", "model/*.ts")}
	uses := usesObserving(t, []vocab.BoundedContext{gomodel, tsmodel}, zones, files, nil, "app")
	var got []string
	for _, u := range uses.Uses {
		got = append(got, u.Context()+" "+u.Term()+" "+u.Via())
	}
	if want := []string{"gomodel Money Add"}; !slices.Equal(got, want) {
		t.Fatalf("uses = %v, want %v", got, want)
	}
	if len(uses.Undetermined) != 1 || uses.Undetermined[0].Context() != "gomodel" || uses.Undetermined[0].Line() != 5 {
		t.Fatalf("undetermined = %+v", uses.Undetermined)
	}
}

// In Go a member belongs only to a type of its own package: an external
// test package in the same directory declaring a type of the same name
// makes no use of its own when the production type is reached.
func TestUsesKeepGoMembersInTheirPackage(t *testing.T) {
	zone := func(name, glob string) rule.Zone {
		g, err := rule.NewGlob(glob)
		if err != nil {
			t.Fatal(err)
		}
		z, err := rule.NewZone(rule.ZoneName(name), "the "+name+" code", []rule.Glob{g})
		if err != nil {
			t.Fatal(err)
		}
		return z
	}
	model := contextOf("model", nil, []vocab.ValueObject{valueObject("Money")}, nil)
	fixture := contextOf("fixture", nil, []vocab.ValueObject{valueObject("Money")}, nil)
	files := map[string]conformance.LanguageFacts{
		"model/money.go": resolvedFile(rule.LanguageGo, "model", []conformance.Declaration{
			{Kind: "struct", Name: "Money", Exported: true, StartLine: 3},
			{Kind: "method", Name: "Add", Owner: "Money", Exported: true, StartLine: 6},
			{Kind: "func", Name: "Zero", Exported: true, StartLine: 9, Results: []string{"Money"}},
		}, nil),
		"model/money_test.go": resolvedFile(rule.LanguageGo, "model_test", []conformance.Declaration{
			{Kind: "struct", Name: "Money", Exported: true, StartLine: 5},
		}, nil),
		"app/run.go": resolvedFile(rule.LanguageGo, "app", nil, []conformance.Call{
			{Callee: "Add", Line: 4, Target: declared("model/money.go", "Money", "Add")},
			{Callee: "Zero", Line: 5, Target: declared("model/money.go", "", "Zero")},
		}),
	}
	zones := []rule.Zone{zone("model", "model/money.go"), zone("fixture", "model/money_test.go")}
	uses := usesObserving(t, []vocab.BoundedContext{model, fixture}, zones, files, nil, "app")
	var got []string
	for _, u := range uses.Uses {
		got = append(got, u.Context()+" "+u.Term()+" "+u.Via())
	}
	if want := []string{"model Money Add"}; !slices.Equal(got, want) {
		t.Fatalf("uses = %v, want %v", got, want)
	}
	if len(uses.Undetermined) != 1 || uses.Undetermined[0].Context() != "model" {
		t.Fatalf("undetermined = %+v", uses.Undetermined)
	}
}

// A reached declaration is the located one only by its whole identity: a
// top-level function sharing the name of a recorded class nested in the
// same file reaches no recorded entry.
func TestUsesMatchTheWholeDeclaration(t *testing.T) {
	files := map[string]conformance.LanguageFacts{
		"model.py": resolvedFile(rule.LanguagePython, "", []conformance.Declaration{
			{Kind: "class", Name: "Container", Exported: true, StartLine: 1},
			{Kind: "class", Name: "Money", Owner: "Container", Exported: true, StartLine: 2},
			{Kind: "func", Name: "Money", Exported: true, StartLine: 5, Results: []string{"int"}},
		}, nil),
		"app.py": resolvedFile(rule.LanguagePython, "", nil, []conformance.Call{
			{Callee: "Money", Line: 2, Target: declared("model.py", "", "Money")},
		}),
	}
	uses := usesOf(t, []vocab.BoundedContext{contextOf("model", nil, []vocab.ValueObject{valueObject("Money")}, nil)}, files, "app.py")
	if len(uses.Uses) != 0 || len(uses.Undetermined) != 0 {
		t.Fatalf("a function sharing the class's name made a use: uses %v, undetermined %+v", spelled(uses.Uses), uses.Undetermined)
	}
}

// A reference names no line, so when its file declares the name twice
// it reaches a recorded entry only if every such declaration leads to
// it: a getter and setter of one property are both members of the
// class, while a function and a class of one name leave the reference
// undetermined.
func TestUsesWeighEveryDeclarationOfTheName(t *testing.T) {
	files := map[string]conformance.LanguageFacts{
		"model.py": resolvedFile(rule.LanguagePython, "", []conformance.Declaration{
			{Kind: "func", Name: "Money", Exported: true, StartLine: 1, Results: []string{"int"}},
			{Kind: "class", Name: "Money", Exported: true, StartLine: 4},
			{Kind: "method", Name: "cents", Owner: "Money", Exported: true, StartLine: 6},
			{Kind: "method", Name: "cents", Owner: "Money", Exported: true, StartLine: 10},
		}, nil),
		"app.py": resolvedFile(rule.LanguagePython, "", nil, []conformance.Call{
			{Callee: "Money", Line: 2, Target: declared("model.py", "", "Money")},
			{Callee: "cents", Line: 3, Target: declared("model.py", "Money", "cents")},
		}),
	}
	uses := usesOf(t, []vocab.BoundedContext{contextOf("model", nil, []vocab.ValueObject{valueObject("Money")}, nil)}, files, "app.py")
	if got := spelled(uses.Uses); len(got) != 1 || got[0] != "Money value_object  call cents app.py:3" {
		t.Fatalf("uses = %v", got)
	}
	if len(uses.Undetermined) != 1 || uses.Undetermined[0].Line() != 2 || !strings.Contains(uses.Undetermined[0].Reason(), "which 2 declarations spell, and not all of them are Money") {
		t.Fatalf("undetermined = %+v", uses.Undetermined)
	}
}

// The one door enforces states-its-reason: an undetermined use always
// says why resolution cannot confirm it, and where it is.
func TestUndeterminedUseStatesItsReason(t *testing.T) {
	ok := conformance.UndeterminedUseSpec{Path: "app/run.go", Line: 4, What: "call run", Reason: "run has no declared type", Context: "ordering", Term: "Money"}
	u, err := conformance.NewUndeterminedUse(ok)
	if err != nil || specOf(u) != ok {
		t.Fatalf("NewUndeterminedUse(%+v) = %+v, %v", ok, u, err)
	}
	for name, bad := range map[string]conformance.UndeterminedUseSpec{
		"no reason":                  {Path: "app/run.go", Line: 4, What: "call run"},
		"a blank reason":             {Path: "app/run.go", Line: 4, What: "call run", Reason: "  "},
		"no path":                    {Line: 4, What: "call run", Reason: "r"},
		"a negative line":            {Path: "app/run.go", Line: -1, What: "call run", Reason: "r"},
		"nothing written":            {Path: "app/run.go", Line: 4, Reason: "r"},
		"a term without its context": {Path: "app/run.go", Line: 4, What: "call run", Reason: "r", Term: "Money"},
	} {
		if _, err := conformance.NewUndeterminedUse(bad); err == nil {
			t.Errorf("%s became an UndeterminedUse", name)
		}
	}
}

// An identity also recorded as a value object keeps the aggregate it
// identifies, as an implied identity does.
func TestUsesKeepTheAggregateARecordedIdentityIdentifies(t *testing.T) {
	ctx := orderingContext()
	ctx.ValueObjects = append(ctx.ValueObjects, valueObject("OrderID"))
	files := goOrdering()
	files["ordering/order_id.go"] = resolvedFile(rule.LanguageGo, "ordering", []conformance.Declaration{
		{Kind: "struct", Name: "OrderID", Exported: true, StartLine: 3},
	}, nil)
	files["app/place.go"] = resolvedFile(rule.LanguageGo, "app", []conformance.Declaration{
		{Kind: "struct", Name: "PlaceOrder", Exported: true, StartLine: 2},
		{Kind: "field", Name: "id", Owner: "PlaceOrder", StartLine: 3, Holds: []conformance.Reference{declared("ordering/order_id.go", "", "OrderID")}, Type: declared("ordering/order_id.go", "", "OrderID")},
	}, nil)
	uses := usesOf(t, []vocab.BoundedContext{ctx}, files, "app")
	if got := spelled(uses.Uses); len(got) != 1 || got[0] != "OrderID value_object Order field PlaceOrder.id app/place.go:3" {
		t.Fatalf("uses = %v", got)
	}
}

// An identity two aggregates share names both.
func TestUsesNameEveryAggregateAnIdentityIdentifies(t *testing.T) {
	ctx := orderingContext()
	invoice := aggregateOf("Invoice", "OrderID", []string{"totals-match"}, nil)
	ctx.Aggregates = append(ctx.Aggregates, invoice)
	ctx.ValueObjects = append(ctx.ValueObjects, valueObject("OrderID"))
	files := goOrdering()
	files["ordering/order_id.go"] = resolvedFile(rule.LanguageGo, "ordering", []conformance.Declaration{
		{Kind: "struct", Name: "OrderID", Exported: true, StartLine: 3},
	}, nil)
	files["ordering/invoice.go"] = resolvedFile(rule.LanguageGo, "ordering", []conformance.Declaration{
		{Kind: "struct", Name: "Invoice", Exported: true, StartLine: 3},
	}, nil)
	files["app/place.go"] = resolvedFile(rule.LanguageGo, "app", []conformance.Declaration{
		{Kind: "struct", Name: "PlaceOrder", Exported: true, StartLine: 2},
		{Kind: "field", Name: "id", Owner: "PlaceOrder", StartLine: 3, Holds: []conformance.Reference{declared("ordering/order_id.go", "", "OrderID")}, Type: declared("ordering/order_id.go", "", "OrderID")},
	}, nil)
	uses := usesOf(t, []vocab.BoundedContext{ctx}, files, "app")
	if got := spelled(uses.Uses); len(got) != 1 || got[0] != "OrderID value_object Order+Invoice field PlaceOrder.id app/place.go:3" {
		t.Fatalf("uses = %v", got)
	}
}
