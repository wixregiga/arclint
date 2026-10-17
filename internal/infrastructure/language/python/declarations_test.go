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

// TestPythonAbstractClassDeclarations marks the classes whose own
// declaration states that they are abstract, through the module's
// imports, and leaves every other class unmarked: a local class that
// shadows nothing named ABC, a subclass of a marked class, and a name
// the module binds twice.
func TestPythonAbstractClassDeclarations(t *testing.T) {
	src := []byte(`import abc
import typing as t
from abc import ABC, ABCMeta
from typing import Generic, Protocol, TypeVar
from typing_extensions import Protocol as ExtProtocol

T = TypeVar("T")
Base = ABC


class ByABC(ABC):
    pass


class ByModule(abc.ABC):
    pass


class ByMetaclass(metaclass=ABCMeta):
    pass


class ByModuleMetaclass(object, metaclass=abc.ABCMeta):
    pass


class ByProtocol(Protocol):
    pass


class ByGenericProtocol(Protocol[T]):
    pass


class ByAliasedModule(t.Protocol):
    pass


class ByExtension(ExtProtocol):
    pass


class ByAlias(Base):
    pass


class Concrete(ByABC):
    pass


class Plain(Generic[T]):
    pass
`)
	got := extractDeclarations(newParsers(), src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	abstract := map[string]bool{}
	for _, d := range got.Decls {
		if d.Kind == "class" {
			abstract[d.Name] = d.Abstract
		}
	}
	want := map[string]bool{
		"ByABC": true, "ByModule": true, "ByMetaclass": true, "ByModuleMetaclass": true,
		"ByProtocol": true, "ByGenericProtocol": true, "ByAliasedModule": true, "ByExtension": true,
		"ByAlias": true, "Concrete": false, "Plain": false,
	}
	if !reflect.DeepEqual(abstract, want) {
		t.Fatalf("abstract classes:\n got %v\nwant %v", abstract, want)
	}

	local := []byte(`class ABC:
    pass


class Shadowed(ABC):
    pass
`)
	got = extractDeclarations(newParsers(), local)
	for _, d := range got.Decls {
		if d.Abstract {
			t.Fatalf("a local class named ABC declares nothing abstract: %+v", d)
		}
	}

	rebound := []byte(`from abc import ABC
from shapes import ABC


class Twice(ABC):
    pass
`)
	got = extractDeclarations(newParsers(), rebound)
	for _, d := range got.Decls {
		if d.Abstract {
			t.Fatalf("a name the module binds twice decides nothing: %+v", d)
		}
	}
}

// TestPythonAbstractClassShadowedByAnEnclosingScope leaves a nested
// class unmarked when an enclosing class or function binds the name its
// base spells, since the base is that binding and not the import.
func TestPythonAbstractClassShadowedByAnEnclosingScope(t *testing.T) {
	src := []byte(`from abc import ABC


class Outer:
    class ABC:
        pass

    class Encrypter(ABC):
        pass


def build(ABC):
    class ByParameter(ABC):
        pass
    return ByParameter


def assigned():
    ABC = object

    class ByAssignment(ABC):
        pass
    return ByAssignment


def unshadowed():
    class Inner(ABC):
        pass
    return Inner


class Holder:
    ABC = object

    def build(self):
        class InMethod(ABC):
            pass
        return InMethod
`)
	got := extractDeclarations(newParsers(), src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	abstract := map[string]bool{}
	for _, d := range got.Decls {
		if d.Kind == "class" {
			abstract[d.Name] = d.Abstract
		}
	}
	// A method's scope skips its class's namespace, so Holder.ABC does
	// not shadow the import inside build.
	want := map[string]bool{"Outer": false, "ABC": false, "Encrypter": false, "ByParameter": false, "ByAssignment": false, "Inner": true, "Holder": false, "InMethod": true}
	if !reflect.DeepEqual(abstract, want) {
		t.Fatalf("abstract classes:\n got %v\nwant %v", abstract, want)
	}
}

// TestPythonAbstractClassAfterADestructuringRebinding leaves a class
// unmarked when the module rebinds the imported name by destructuring.
func TestPythonAbstractClassAfterADestructuringRebinding(t *testing.T) {
	got := extractDeclarations(newParsers(), []byte("from abc import ABC\n\nABC, other = object, None\n\n\nclass Encrypter(ABC):\n    pass\n"))
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	for _, d := range got.Decls {
		if d.Abstract {
			t.Fatalf("a rebound ABC declares nothing abstract: %+v", d)
		}
	}
}
