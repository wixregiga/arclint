package conformance_test

import (
	"path"
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
	return usesObserving(t, contexts, files, nil, worksite...)
}

// usesObserving is usesOf over a walk that also observed files no
// language supplied facts for.
func usesObserving(t *testing.T, contexts []vocab.BoundedContext, files map[string]conformance.LanguageFacts, unread []string, worksite ...string) conformance.TermUses {
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
	facts, err := conformance.NewInspectionFacts(nil, obs, rule.FactFileTree, rule.FactDeclarations, rule.FactCalls, rule.FactResolvedCalls, rule.FactFieldTypes)
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

// spelled renders each use as term role way via path:line, in order.
func spelled(uses []conformance.TermUse) []string {
	out := make([]string, len(uses))
	for i, u := range uses {
		out[i] = strings.Join([]string{u.Term(), string(u.Role()), u.Aggregate(), string(u.Way()), u.Via(), u.Path() + ":" + strconv.Itoa(u.Line())}, " ")
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
// file of its package, a value object through its constructor
// function, a service through its receiver, and a repository through
// a field; a call that only spells a recorded name, an external call,
// and a collection-returning function make no use, and an
// unresolvable call is undetermined with its reason.
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
		"Money value_object  call NewMoney app/place.go:11",
		"Order aggregate  call Place app/place.go:12",
		"Pricing domain_service  call Quote app/place.go:13",
	}
	if got := spelled(uses.Uses); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("uses =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if u := uses.Uses[2]; u.Context() != "ordering" {
		t.Fatalf("use context = %q", u.Context())
	}
	wantUndetermined := []conformance.UndeterminedUse{
		{Path: "app/place.go", Line: 7, What: "field PlaceOrder.events", Reason: "the package bus does not type-check"},
		{Path: "app/place.go", Line: 17, What: "call handle", Reason: "a call through a function value is not followed"},
	}
	if len(uses.Undetermined) != len(wantUndetermined) {
		t.Fatalf("undetermined = %+v", uses.Undetermined)
	}
	for i, u := range wantUndetermined {
		if uses.Undetermined[i] != u {
			t.Fatalf("undetermined[%d] = %+v, want %+v", i, uses.Undetermined[i], u)
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

// A term declared inside the worksite is declared here, not used; a
// term two declarations spell leaves a reference to either undetermined
// rather than picking one.
func TestUsesSkipTheWorksiteAndNeverPickATwin(t *testing.T) {
	ctx := orderingContext()
	ctx.ValueObjects = append(ctx.ValueObjects, valueObject("Currency"))
	files := goOrdering()
	for _, p := range []string{"billing/currency.go", "legacy/currency.go"} {
		files[p] = resolvedFile(rule.LanguageGo, path.Base(path.Dir(p)), []conformance.Declaration{
			{Kind: "struct", Name: "Currency", Exported: true, StartLine: 2},
			{Kind: "func", Name: "NewCurrency", Exported: true, StartLine: 5, Results: []string{"Currency"}},
		}, nil)
	}
	files["app/place.go"] = resolvedFile(rule.LanguageGo, "app", nil, []conformance.Call{
		{Callee: "Place", Line: 4, Target: declared("ordering/order_commands.go", "Order", "Place")},
		{Callee: "NewMoney", Line: 5, Target: declared("ordering/money.go", "", "NewMoney")},
		{Callee: "NewCurrency", Line: 6, Target: declared("billing/currency.go", "", "NewCurrency")},
	})
	uses := usesOf(t, []vocab.BoundedContext{ctx}, files, "app", "ordering/order.go")
	if got := spelled(uses.Uses); len(got) != 1 || got[0] != "Money value_object  call NewMoney app/place.go:5" {
		t.Fatalf("the root declared in the worksite is no use, Money is: %v", got)
	}
	if len(uses.Undetermined) != 1 {
		t.Fatalf("undetermined = %+v", uses.Undetermined)
	}
	u := uses.Undetermined[0]
	if u.Line != 6 || !strings.Contains(u.Reason, "spelling Currency") || !strings.Contains(u.Reason, "billing/currency.go") || !strings.Contains(u.Reason, "legacy/currency.go") {
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
		got[u.Path+" "+u.What] = u.Reason
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
	uses := usesObserving(t, []vocab.BoundedContext{orderingContext()}, goOrdering(), []string{"app/consumer.d.ts", "app/README.md"}, "app")
	if len(uses.Undetermined) != 1 {
		t.Fatalf("undetermined = %+v", uses.Undetermined)
	}
	if u := uses.Undetermined[0]; u.Path != "app/consumer.d.ts" || u.What != "file" || u.Line != 0 || u.Reason == "" {
		t.Fatalf("unread file = %+v", u)
	}
}

// A function makes a use of a type only when it returns the type
// itself where the name can mean nothing else: in Go a function of the
// type's package returning it unqualified, in TypeScript and Python a
// function of the type's file. A qualified, wrapped, or sibling-file
// result is no use.
func TestUsesRecognizeConstructorsOnlyByTheirOwnType(t *testing.T) {
	files := goOrdering()
	files["ordering/legacy.go"] = resolvedFile(rule.LanguageGo, "ordering", []conformance.Declaration{
		{Kind: "func", Name: "LegacyMoney", Exported: true, StartLine: 3, Results: []string{"legacy.Money"}},
		{Kind: "func", Name: "MaybeMoney", Exported: true, StartLine: 6, Results: []string{"Option[Money]"}},
		{Kind: "func", Name: "MoneyRef", Exported: true, StartLine: 9, Results: []string{"*Money"}},
	}, nil)
	files["ordering/pricing_helpers.py"] = resolvedFile(rule.LanguagePython, "", []conformance.Declaration{
		{Kind: "func", Name: "make_order", StartLine: 1, Results: []string{"Order"}},
	}, nil)
	files["app/place.go"] = resolvedFile(rule.LanguageGo, "app", nil, []conformance.Call{
		{Callee: "LegacyMoney", Line: 3, Target: declared("ordering/legacy.go", "", "LegacyMoney")},
		{Callee: "MaybeMoney", Line: 4, Target: declared("ordering/legacy.go", "", "MaybeMoney")},
		{Callee: "MoneyRef", Line: 5, Target: declared("ordering/legacy.go", "", "MoneyRef")},
		{Callee: "make_order", Line: 6, Target: declared("ordering/pricing_helpers.py", "", "make_order")},
	})
	uses := usesOf(t, []vocab.BoundedContext{orderingContext()}, files, "app")
	if got := spelled(uses.Uses); len(got) != 1 || got[0] != "Money value_object  call MoneyRef app/place.go:5" {
		t.Fatalf("uses = %v", got)
	}
}
