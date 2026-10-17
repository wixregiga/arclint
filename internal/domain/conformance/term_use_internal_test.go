package conformance

import (
	"testing"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// The one door enforces reached-through-resolution: only a reference
// the language resolved to the term's located declaration, a member of
// it, or a constructor of it becomes a TermUse.
func TestTermUseIsReachedThroughResolution(t *testing.T) {
	money := fileFacts{path: "ordering/money.go", lang: rule.LanguageGo, decls: []Declaration{
		{Kind: declKindStruct, Name: "Money", StartLine: 3},
		{Kind: declKindMethod, Name: "Add", Owner: "Money", StartLine: 8},
		{Kind: declKindFunc, Name: "NewMoney", StartLine: 12, Results: []string{"Money", "error"}},
		{Kind: declKindFunc, Name: "Zero", StartLine: 20, Results: []string{"int"}},
	}}
	elsewhere := fileFacts{path: "app/money.go", lang: rule.LanguageGo, decls: []Declaration{
		{Kind: declKindStruct, Name: "Money", StartLine: 1},
	}}
	term := usedTerm{context: "ordering", name: "Money", role: UsedValueObject, candidates: []locDecl{{file: money, decl: money.decls[0]}}}
	site := useSite{path: "app/place.go", line: 9, way: UseByCall, via: "x"}
	reached := func(f fileFacts, i int) locDecl { return locDecl{file: f, decl: f.decls[i]} }

	for _, ok := range []struct {
		name    string
		ref     Reference
		reached locDecl
	}{
		{"the declaration", Reference{Resolution: ResolutionDeclared, File: money.path, Name: "Money"}, reached(money, 0)},
		{"a member", Reference{Resolution: ResolutionDeclared, File: money.path, Owner: "Money", Name: "Add"}, reached(money, 1)},
		{"a constructor", Reference{Resolution: ResolutionDeclared, File: money.path, Name: "NewMoney"}, reached(money, 2)},
	} {
		use, err := newTermUse(ok.ref, ok.reached, term, site)
		if err != nil {
			t.Errorf("%s: %v", ok.name, err)
			continue
		}
		if use.Term() != "Money" || use.Path() != "app/place.go" || use.Line() != 9 || use.Role() != UsedValueObject {
			t.Errorf("%s: use = %+v", ok.name, use)
		}
	}

	for _, bad := range []struct {
		name    string
		ref     Reference
		reached locDecl
		term    usedTerm
	}{
		{"an undetermined reference", Reference{Resolution: ResolutionUndetermined, Reason: "no type"}, reached(money, 0), term},
		{"an external reference", Reference{Resolution: ResolutionExternal, Package: "fmt", Name: "Money"}, reached(money, 0), term},
		{"a same-named type elsewhere", Reference{Resolution: ResolutionDeclared, File: elsewhere.path, Name: "Money"}, reached(elsewhere, 0), term},
		{"a reference that reaches another declaration", Reference{Resolution: ResolutionDeclared, File: money.path, Name: "Add", Owner: "Money"}, reached(money, 0), term},
		{"a function of the unit that does not build it", Reference{Resolution: ResolutionDeclared, File: money.path, Name: "Zero"}, reached(money, 3), term},
		{"a term two declarations spell", Reference{Resolution: ResolutionDeclared, File: money.path, Name: "Money"}, reached(money, 0),
			usedTerm{context: "ordering", name: "Money", role: UsedValueObject, candidates: []locDecl{reached(money, 0), reached(elsewhere, 0)}}},
	} {
		if _, err := newTermUse(bad.ref, bad.reached, bad.term, site); err == nil {
			t.Errorf("%s became a TermUse", bad.name)
		}
	}
}
