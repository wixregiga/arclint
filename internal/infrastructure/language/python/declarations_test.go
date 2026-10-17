package python

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

func TestPythonextractDeclarations(t *testing.T) {
	src := []byte(`import os

class Member:
    def loans(self):
        return 0

    @property
    def blocked(self):
        return False

    def _internal(self):
        pass

class _Private:
    pass

def top_level(x):
    def nested(y):
        return y
    return nested(x)

async def fetch():
    pass

if True:
    def conditional():
        pass
`)
	got := extractDeclarations(newParsers(), src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	self := []conformance.DeclarationParam{{Name: "self"}}
	want := []conformance.Declaration{
		{Kind: "class", Name: "Member", Exported: true, StartLine: 3, EndLine: 12},
		{Kind: "method", Name: "loans", Owner: "Member", Exported: true, StartLine: 4, EndLine: 5, Params: self},
		// Spans cover the definition node itself; decorator lines are
		// excluded by design (the walker recurses into the definition).
		{Kind: "method", Name: "blocked", Owner: "Member", Exported: true, StartLine: 8, EndLine: 9, Params: self},
		{Kind: "method", Name: "_internal", Owner: "Member", StartLine: 11, EndLine: 12, Params: self},
		{Kind: "class", Name: "_Private", StartLine: 14, EndLine: 15},
		{
			Kind: "func", Name: "top_level", Exported: true, StartLine: 17, EndLine: 20,
			Params: []conformance.DeclarationParam{{Name: "x"}},
		},
		{
			Kind: "func", Name: "nested", Owner: "top_level", Exported: true, StartLine: 18, EndLine: 19,
			Params: []conformance.DeclarationParam{{Name: "y"}},
		},
		{Kind: "func", Name: "fetch", Exported: true, StartLine: 22, EndLine: 23},
		{Kind: "func", Name: "conditional", Exported: true, StartLine: 26, EndLine: 27},
	}
	diffDecls(t, got.Decls, want)
}

// TestPythonSignatureFacts pins the M10 signature tier: annotations as
// source text, defaults marked optional, splats keep their prefix and
// the separators / and * are not parameters.
func TestPythonSignatureextractDeclarations(t *testing.T) {
	src := []byte(`class Repo:
    def find(self, book_id: str, limit: int = 10) -> 'Book':
        pass

    def mixed(self, a, /, b, *args, c=1, **kwargs):
        pass

def top(x: int) -> dict[str, int]:
    return {}
`)
	got := extractDeclarations(newParsers(), src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	want := []conformance.Declaration{
		{Kind: "class", Name: "Repo", Exported: true, StartLine: 1, EndLine: 6},
		{
			Kind: "method", Name: "find", Owner: "Repo", Exported: true, StartLine: 2, EndLine: 3,
			Params: []conformance.DeclarationParam{
				{Name: "self"},
				{Name: "book_id", Type: "str"},
				{Name: "limit", Type: "int", Optional: true},
			},
			Results: []string{"'Book'"},
		},
		{
			Kind: "method", Name: "mixed", Owner: "Repo", Exported: true, StartLine: 5, EndLine: 6,
			Params: []conformance.DeclarationParam{
				{Name: "self"},
				{Name: "a"},
				{Name: "b"},
				{Name: "*args", Variadic: true},
				{Name: "c", Optional: true},
				{Name: "**kwargs", Variadic: true},
			},
		},
		{
			Kind: "func", Name: "top", Exported: true, StartLine: 8, EndLine: 9,
			Params:  []conformance.DeclarationParam{{Name: "x", Type: "int"}},
			Results: []string{"dict[str, int]"},
		},
	}
	diffDecls(t, got.Decls, want)
}

func TestPythonextractCalls(t *testing.T) {
	src := []byte(`class Event:
    def __init__(self):
        self.published_frozen()

    def published_frozen(self):
        return None

    def publish(self):
        return self.published_frozen()
`)
	got := extractDeclarations(newParsers(), src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	seen := map[string]bool{}
	for _, c := range got.Calls {
		if c.Line == 0 {
			t.Errorf("call %q has no line", c.Callee)
		}
		seen[c.Callee+"@"+c.Enclosing] = true
	}
	for _, k := range []string{"published_frozen@__init__", "published_frozen@publish"} {
		if !seen[k] {
			t.Errorf("missing call %s; have %v", k, seen)
		}
	}
}

func diffDecls(t *testing.T, got, want []conformance.Declaration) {
	t.Helper()
	if reflect.DeepEqual(got, want) {
		return
	}
	for i := 0; i < len(got) || i < len(want); i++ {
		var g, w conformance.Declaration
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		marker := "  "
		if !reflect.DeepEqual(g, w) {
			marker = "!!"
		}
		t.Logf("%s got %+v want %+v", marker, g, w)
	}
	t.Fail()
}

// A class deriving ABC or Protocol, or naming ABCMeta as its metaclass,
// carries the abstract mark, whether it writes the base bare, through
// its module, or with type arguments; any other class carries none.
func TestPythonMarksAbstractClasses(t *testing.T) {
	src := []byte(`import abc
import typing
from abc import ABC, ABCMeta
from typing import Protocol, Generic, TypeVar

class Discounter(ABC):
    pass

class Pricing(abc.ABC):
    pass

class Ledger(Protocol):
    pass

class Store(Protocol[T]):
    pass

class Typed(typing.Protocol):
    pass

class Meta(Base, metaclass=ABCMeta):
    pass

class QualifiedMeta(metaclass=abc.ABCMeta):
    pass

class Concrete(Discounter):
    pass

class Plain:
    pass

class Generic2(Generic[T]):
    pass
`)
	got := extractDeclarations(newParsers(), src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	abstract := map[string]bool{}
	for _, d := range got.Decls {
		if d.Kind != "class" {
			t.Errorf("unexpected %s %s", d.Kind, d.Name)
			continue
		}
		abstract[d.Name] = d.Abstract
	}
	want := map[string]bool{
		"Discounter": true, "Pricing": true, "Ledger": true, "Store": true, "Typed": true,
		"Meta": true, "QualifiedMeta": true,
		"Concrete": false, "Plain": false, "Generic2": false,
	}
	if !reflect.DeepEqual(abstract, want) {
		t.Fatalf("abstract marks:\n got %v\nwant %v", abstract, want)
	}
}

// A def or class with its own type parameters is generic; and a def in
// the body of an elif, else, except, or finally clause, or of a loop,
// or of a match case, is declared at the level the clause stands at.
func TestPythonMarksGenericsAndReadsClauseBodies(t *testing.T) {
	src := []byte(`def first[T](xs: list[T]) -> T:
    return xs[0]

class Box[T]:
    pass

def plain(x):
    return x

if FAST:
    pass
elif SLOW:
    def test_elif():
        pass
else:
    def test_else():
        pass

try:
    pass
except ImportError:
    def test_except():
        pass
finally:
    def test_finally():
        pass

for _ in range(1):
    def test_loop():
        pass

match MODE:
    case "fast":
        def test_match():
            pass
`)
	got := extractDeclarations(newParsers(), src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	generic := map[string]bool{}
	for _, d := range got.Decls {
		if d.Owner != "" {
			t.Errorf("%s %s is owned by %q, want module level", d.Kind, d.Name, d.Owner)
		}
		generic[d.Name] = d.Generic
	}
	want := map[string]bool{"first": true, "Box": true, "plain": false,
		"test_elif": false, "test_else": false, "test_except": false, "test_finally": false, "test_loop": false, "test_match": false}
	for name, g := range want {
		if have, ok := generic[name]; !ok || have != g {
			t.Errorf("%s generic = %v (declared %v), want %v", name, have, ok, g)
		}
	}
}
