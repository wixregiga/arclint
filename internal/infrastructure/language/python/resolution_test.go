package python

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func resolved(t *testing.T, sources map[string]string) map[string]conformance.LanguageFacts {
	t.Helper()
	root, files := writeFiles(t, sources)
	facts, err := NewProducer().Facts(root, files, []rule.Fact{rule.FactResolvedCalls, rule.FactFieldTypes})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conformance.NewObservations(files, facts); err != nil {
		t.Fatal(err)
	}
	return facts
}

func TestFieldDeclarations(t *testing.T) {
	f := resolved(t, map[string]string{"model.py": `from dataclasses import dataclass, InitVar as IV
from typing import ClassVar as CV, NamedTuple
@dataclass
class Model:
    x: str
    config: CV[str]
    init: IV[str]
    def init(self):
        self.y: int = 1
        self.x = 'duplicate'
        self.z = 0
    def other(self):
        self.z: str = 'still untyped'
        self.later: str
class Pair(NamedTuple):
    left: str
    right: int
`})["model.py"]
	var got []string
	for _, d := range f.Declarations {
		if d.Kind == "field" {
			got = append(got, d.Owner+"."+d.Name)
			if d.Name == "z" && !reflect.DeepEqual(d.Holds, []conformance.Reference{unknown("z has no declared type")}) {
				t.Errorf("z: %+v", d)
			}
		}
	}
	want := []string{"Model.x", "Model.y", "Model.z", "Model.later", "Pair.left", "Pair.right"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fields %v want %v", got, want)
	}
}

func TestResolvedImportsFieldsAndCalls(t *testing.T) {
	facts := resolved(t, map[string]string{
		"pkg/__init__.py": "from .model import Repo as Repository\nfrom .model import work\n",
		"pkg/model.py": `from typing import Protocol
class Base:
    def save(self): pass
class Repo(Base): pass
class Port(Protocol):
    def execute(self): pass
def work(): pass
`,
		"consumer.py": `from __future__ import annotations
import pkg.model as module
from pkg import Repository as R, work as run
from typing import Optional, Callable, Any
import os
class Service:
    repo: Optional['R']
    items: list[R]
    mapping: dict[str, R | None]
    callback: Callable[[R], R]
    dynamic: Any
    def execute(self, p: R):
        self.repo.save()
        p.save()
        value: R
        value.save()
        R.save(p)
        R()
        module.work()
        run()
        os.getcwd()
        print(len([]))
module.work()
`,
	})
	f := facts["consumer.py"]
	wants := []conformance.Reference{
		declared("pkg/model.py", "Base", "save"), declared("pkg/model.py", "Base", "save"), declared("pkg/model.py", "Base", "save"), declared("pkg/model.py", "Base", "save"),
		declared("pkg/model.py", "", "Repo"), declared("pkg/model.py", "", "work"), declared("pkg/model.py", "", "work"), external("os", "getcwd"), external("", "print"), external("", "len"), declared("pkg/model.py", "", "work"),
	}
	if len(f.Calls) != len(wants) {
		t.Fatalf("calls: %+v", f.Calls)
	}
	for i, c := range f.Calls {
		if c.Target != wants[i] {
			t.Errorf("call %d: %+v want %+v", i, c, wants[i])
		}
		if i < 4 && c.Receiver != declared("pkg/model.py", "", "Repo") {
			t.Errorf("receiver %d: %+v", i, c)
		}
		if i < 10 && c.EnclosingOwner != "Service" {
			t.Errorf("owner: %+v", c)
		}
	}
	for _, d := range f.Declarations {
		if d.Kind != "field" {
			continue
		}
		var want []conformance.Reference
		switch d.Name {
		case "repo":
			want = []conformance.Reference{external("typing", "Optional"), declared("pkg/model.py", "", "Repo")}
		case "items":
			want = []conformance.Reference{external("", "list"), declared("pkg/model.py", "", "Repo")}
		case "mapping":
			want = []conformance.Reference{external("", "dict"), external("", "str"), declared("pkg/model.py", "", "Repo")}
		case "callback":
			want = []conformance.Reference{external("typing", "Callable")}
		case "dynamic":
			want = []conformance.Reference{unknown("Any does not declare a concrete type")}
		}
		if !reflect.DeepEqual(d.Holds, want) {
			t.Errorf("field %s: %+v want %+v", d.Name, d.Holds, want)
		}
		ownTypes := map[string]conformance.Reference{
			"repo": declared("pkg/model.py", "", "Repo"), "items": external("", "list"), "mapping": external("", "dict"),
			"callback": external("typing", "Callable"), "dynamic": unknown("Any does not declare a concrete type"),
		}
		if d.Type != ownTypes[d.Name] {
			t.Errorf("field %s own type: %+v want %+v", d.Name, d.Type, ownTypes[d.Name])
		}
	}
}

// An explicit TypeAlias stands for the type it names; a plain
// assignment or a type statement stays undetermined, never the
// typing.TypeAlias marker itself.
func TestFieldTypeThroughTypeAlias(t *testing.T) {
	facts := resolved(t, map[string]string{
		"ids.py":   "from typing import TypeAlias\nclass OrderID: pass\nTypedID: TypeAlias = OrderID\nStoredID = OrderID\n",
		"order.py": "from ids import TypedID, StoredID\nclass Order:\n    typed: TypedID\n    quoted: 'TypedID'\n    stored: StoredID\n",
	})
	id := declared("ids.py", "", "OrderID")
	for _, d := range facts["order.py"].Declarations {
		if d.Kind != "field" {
			continue
		}
		switch d.Name {
		case "typed", "quoted":
			if d.Type != id || !reflect.DeepEqual(d.Holds, []conformance.Reference{id}) {
				t.Errorf("field %s: type %+v holds %+v, want OrderID", d.Name, d.Type, d.Holds)
			}
		case "stored":
			if d.Type.Resolution != conformance.ResolutionUndetermined {
				t.Errorf("field stored: type %+v, want undetermined", d.Type)
			}
		}
	}
}

// Optional, Union with None, and Final, from typing or
// typing_extensions, wrap a field's own type.
func TestFieldTypeThroughTypingExtensions(t *testing.T) {
	facts := resolved(t, map[string]string{
		"pyproject.toml": "[project]\nname='app'\ndependencies=['typing_extensions']\n",
		"ids.py":         "class OrderID: pass\n",
		"order.py":       "from typing import Final\nfrom typing_extensions import Optional, Union, Final as XFinal\nfrom ids import OrderID\nclass Order:\n    a: Optional[OrderID]\n    b: Union[OrderID, None]\n    c: Final[OrderID]\n    d: XFinal[OrderID]\n",
	})
	id := declared("ids.py", "", "OrderID")
	for _, d := range facts["order.py"].Declarations {
		if d.Kind == "field" && d.Type != id {
			t.Errorf("field %s own type %+v, want OrderID", d.Name, d.Type)
		}
	}
}

// A typing_extensions ClassVar, imported directly or through a module
// alias, marks a class-level value, so it is no field.
func TestTypingExtensionsClassVarIsNoField(t *testing.T) {
	facts := resolved(t, map[string]string{
		"pyproject.toml": "[project]\nname='app'\ndependencies=['typing_extensions']\n",
		"order.py":       "import typing_extensions as te\nfrom typing_extensions import ClassVar\nclass Order:\n    a: ClassVar[int]\n    b: te.ClassVar[int]\n    kept: int\n",
	})
	var fields []string
	for _, d := range facts["order.py"].Declarations {
		if d.Kind == "field" {
			fields = append(fields, d.Name)
		}
	}
	if !reflect.DeepEqual(fields, []string{"kept"}) {
		t.Errorf("fields %v, want only kept", fields)
	}
}

// A method's instance is its first parameter, whatever it is spelled,
// and a ClassVar rebound in the class body no longer marks the
// annotations after it.
func TestInstanceParameterAndClassBodyRebinding(t *testing.T) {
	facts := resolved(t, map[string]string{
		"order.py": "from typing import ClassVar\nclass OrderID: pass\nclass Order:\n    ClassVar = list\n    kept: ClassVar[OrderID]\n    def __init__(this, id: OrderID):\n        this.id: OrderID = id\n    @staticmethod\n    def make(other):\n        other.ignored = 1\n",
	})
	id := declared("order.py", "", "OrderID")
	fields := map[string]conformance.Reference{}
	for _, d := range facts["order.py"].Declarations {
		if d.Kind == "field" {
			fields[d.Name] = d.Type
		}
	}
	if len(fields) != 2 || fields["id"] != id {
		t.Errorf("fields %+v, want kept and id, with id an OrderID", fields)
	}
	if _, ok := fields["kept"]; !ok {
		t.Errorf("fields %+v: kept is annotated with the rebound ClassVar", fields)
	}
}

// An imported ClassVar rebound later in the module no longer marks a
// class-level value, so the field it annotates is kept.
func TestReboundClassVarKeepsTheField(t *testing.T) {
	facts := resolved(t, map[string]string{
		"order.py": "from typing import ClassVar, Optional\nClassVar = Optional\nclass OrderID: pass\nclass Order:\n    id: ClassVar[OrderID]\n",
	})
	var fields []string
	for _, d := range facts["order.py"].Declarations {
		if d.Kind == "field" {
			fields = append(fields, d.Name)
		}
	}
	if !reflect.DeepEqual(fields, []string{"id"}) {
		t.Errorf("fields %v, want id: ClassVar is rebound", fields)
	}
}

// A return annotation runs where its function is defined, so a call in
// it resolves in that scope.
func TestReturnAnnotationCallsResolve(t *testing.T) {
	facts := resolved(t, map[string]string{
		"repo.py":  "class Repo:\n    def load(self): pass\nrepo = Repo()\n",
		"order.py": "from repo import Repo\nclass Order:\n    def place(self, r: Repo):\n        def inner() -> r.load():\n            pass\n",
	})
	calls := facts["order.py"].Calls
	if len(calls) != 1 || calls[0].Callee != "load" || calls[0].EnclosingOwner != "Order" || calls[0].Target != declared("repo.py", "Repo", "load") {
		t.Fatalf("calls %+v, want load in Order.place resolved to Repo.load", calls)
	}
}

// staticmethod and classmethod reached through an import from builtins
// or a plain alias still mark a method that takes no instance.
func TestAliasedMethodDecoratorsTakeNoInstance(t *testing.T) {
	facts := resolved(t, map[string]string{
		"order.py": "import builtins as b\nfrom builtins import staticmethod as sm\ncm = classmethod\nclass Order:\n    def __init__(self):\n        self.kept = 1\n    @cm\n    def make(cls, id):\n        cls.made = id\n    @b.staticmethod\n    def build(x):\n        x.built = 1\n    @sm\n    def other(y):\n        y.othered = 1\n",
	})
	var fields []string
	for _, d := range facts["order.py"].Declarations {
		if d.Kind == "field" {
			fields = append(fields, d.Name)
		}
	}
	if !reflect.DeepEqual(fields, []string{"kept"}) {
		t.Errorf("fields %v, want only kept: the decorated methods take no instance", fields)
	}
	rebound := resolved(t, map[string]string{
		"order.py": "from builtins import staticmethod as sm\ndef passthrough(f): return f\nsm = passthrough\nclass Order:\n    @sm\n    def __init__(self, id):\n        self.id = id\n",
	})
	fields = nil
	for _, d := range rebound["order.py"].Declarations {
		if d.Kind == "field" {
			fields = append(fields, d.Name)
		}
	}
	if !reflect.DeepEqual(fields, []string{"id"}) {
		t.Errorf("fields %v, want id: sm is rebound, so it no longer names staticmethod", fields)
	}
	shadowed := resolved(t, map[string]string{
		"order.py": "from builtins import staticmethod as sm\nclass Order:\n    sm = lambda f: f\n    @sm\n    def __init__(self, id):\n        self.id = id\n",
	})
	fields = nil
	for _, d := range shadowed["order.py"].Declarations {
		if d.Kind == "field" {
			fields = append(fields, d.Name)
		}
	}
	if !reflect.DeepEqual(fields, []string{"id"}) {
		t.Errorf("fields %v, want id: the class body rebinds sm", fields)
	}
	// A class-local def shadows the import too, and the instance may be
	// spelled anything: the field still gets its type, and the check
	// never aborts.
	byDef := resolved(t, map[string]string{
		"order.py": "from builtins import staticmethod as sm\nclass OrderID: pass\nclass Order:\n    def sm(f): return f\n    @sm\n    def __init__(obj):\n        obj.id: OrderID = OrderID()\n",
	})
	var id conformance.Declaration
	for _, d := range byDef["order.py"].Declarations {
		if d.Kind == "field" && d.Name == "id" {
			id = d
		}
	}
	if id.Type != declared("order.py", "", "OrderID") {
		t.Errorf("field id %+v, want its type OrderID", id)
	}
	// An annotated class-level assignment shadows too; a method named
	// after the builtin is still decorated by the builtin.
	annotated := resolved(t, map[string]string{
		"order.py": "sm = staticmethod\nclass Order:\n    sm: object = lambda f: f\n    @sm\n    def __init__(self):\n        self.id = 1\n    @staticmethod\n    def staticmethod(x):\n        x.service = 1\n",
	})
	fields = nil
	for _, d := range annotated["order.py"].Declarations {
		if d.Kind == "field" {
			fields = append(fields, d.Name)
		}
	}
	if !reflect.DeepEqual(fields, []string{"sm", "id"}) {
		t.Errorf("fields %v, want sm and id: sm is shadowed in the class, and the static method declares nothing", fields)
	}
}

// In a class defined inside a function, a module-level alias does not
// decide a decorator: a parameter there may shadow it.
func TestNestedClassIgnoresModuleDecoratorAliases(t *testing.T) {
	facts := resolved(t, map[string]string{
		"order.py": "sm = staticmethod\ndef build(sm=lambda f: f):\n    class Order:\n        @sm\n        def attach(self, service):\n            self.service = service\n        @staticmethod\n        def make(x):\n            x.ignored = 1\n    return Order\n",
	})
	var fields []string
	for _, d := range facts["order.py"].Declarations {
		if d.Kind == "field" {
			fields = append(fields, d.Name)
		}
	}
	if !reflect.DeepEqual(fields, []string{"service"}) {
		t.Errorf("fields %v, want service: the alias may be shadowed, and staticmethod itself still counts", fields)
	}
}

// A leading *args is a tuple, never the instance; a lambda in a return
// annotation keeps its own parameter.
func TestSplatAndAnnotationLambdaScopes(t *testing.T) {
	facts := resolved(t, map[string]string{
		"repo.py":  "class OrderRepository:\n    def count(self, value): return 0\n",
		"order.py": "from repo import OrderRepository\nclass Order(OrderRepository):\n    def place(*args):\n        return args.count(None)\n    def other(self, r: OrderRepository):\n        def inner() -> (lambda r: r.count(None))([]): pass\n",
	})
	counts := 0
	for _, c := range facts["order.py"].Calls {
		if c.Callee != "count" {
			continue
		}
		counts++
		if c.Target.Resolution == conformance.ResolutionDeclared {
			t.Errorf("call %+v reaches the repository; *args is a tuple and the lambda's r is its own", c)
		}
	}
	if counts != 2 {
		t.Fatalf("calls %+v, want two count calls", facts["order.py"].Calls)
	}
}

// A class body runs where the class is defined, so a class inside a
// method runs its body's calls in that method; its own methods belong to
// it. *args holds a tuple, whatever its annotation names.
func TestNestedClassBodiesAndVariadicParameters(t *testing.T) {
	facts := resolved(t, map[string]string{
		"repo.py": "class OrderRepository:\n    def save(self): pass\n",
		"order.py": "from repo import OrderRepository\nclass Order:\n    def place(self, repo: OrderRepository, *repos: OrderRepository):\n        class Nested:\n            marker = repo.save()\n            def run(self):\n                pass\n        repos.count(None)\n",
	})
	calls := facts["order.py"].Calls
	if len(calls) != 2 {
		t.Fatalf("calls %+v", calls)
	}
	if c := calls[0]; c.Callee != "save" || c.EnclosingOwner != "Order" || c.Enclosing != "place" || c.Target != declared("repo.py", "OrderRepository", "save") {
		t.Errorf("class-body call %+v, want Order.place calling OrderRepository.save", c)
	}
	if c := calls[1]; c.Callee != "count" || c.Receiver != external("", "tuple") || c.Target.Resolution == conformance.ResolutionDeclared {
		t.Errorf("*args call %+v, want a call on a tuple, never the repository", c)
	}
}

// A default or an annotation runs where its function is defined, so a
// call in a method's default belongs to the class body, not the method.
func TestDefaultCallsRunWhereTheFunctionIsDefined(t *testing.T) {
	facts := resolved(t, map[string]string{
		"repo.py": "class Repo:\n    def load(self): pass\nrepo = Repo()\n",
		"order.py": `from repo import repo
class Order:
    def place(self, x=repo.load()):
        repo.load()
`,
	})
	var got []string
	for _, c := range facts["order.py"].Calls {
		got = append(got, c.Callee+"@"+c.EnclosingOwner+"."+c.Enclosing)
	}
	if want := []string{"load@.", "load@Order.place"}; !reflect.DeepEqual(got, want) {
		t.Errorf("calls %v, want %v", got, want)
	}
}

func TestUndeterminedReasons(t *testing.T) {
	cases := []struct{ name, body, reason string }{
		{"unannotated", "def f(p):\n    p.save()\n", "p has no declared type"},
		{"any", "from typing import Any\ndef f(p: Any):\n    p.save()\n", "Any does not declare a concrete type"},
		{"getattr", "def f(p):\n    getattr(p, 'save')()\n", "computed member call"},
		{"setattr", "def f(p):\n    setattr(p, 'save', 0)\n", "setattr uses dynamic attribute access"},
		{"star", "from pkg.model import *\ndef f():\n    Repo()\n", "star import may bind Repo"},
		{"kwargs", "from pkg.model import Repo\ndef f(**kw):\n    Repo(**kw)\n", "**kwargs plumbing has no declared target"},
		{"patch", "from pkg.model import Repo\nRepo.save = lambda: None\ndef f(p: Repo):\n    p.save()\n", "monkey patching changes Repo.save"},
		{"descriptor", "class C:\n    @property\n    def save(self): pass\ndef f(p: C):\n    p.save()\n", "decorated declaration save may be a descriptor or replacement"},
		{"metaclass", "class C(metaclass=type): pass\ndef f(p: C):\n    p.save()\n", "class C uses a metaclass or class keyword"},
		{"super", "class C:\n    def f(self):\n        super().save()\n", "super() has no resolvable base"},
		{"unknown", "def f():\n    Missing()\n", "Missing is neither declared in the file nor imported"},
		{"computed", "def f(p):\n    p[0]()\n", "computed member call"},
		{"namespace", "from namespace.mod import C\ndef f():\n    C()\n", "namespace.mod uses a namespace package"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := resolved(t, map[string]string{"consumer.py": tc.body, "pkg/__init__.py": "", "pkg/model.py": "class Repo:\n    def save(self): pass\n", "namespace/mod.py": "class C: pass\n"})["consumer.py"]
			if len(f.Calls) == 0 {
				t.Fatal("no call")
			}
			if f.Calls[0].Target != unknown(tc.reason) {
				t.Errorf("%+v want reason %q", f.Calls[0], tc.reason)
			}
		})
	}
}

func TestFactRequestsAreLazy(t *testing.T) {
	root, files := writeFiles(t, map[string]string{"x.py": "class C:\n    x: str\n    def f(self):\n        print(self.x)\n"})
	for _, req := range [][]rule.Fact{nil, {rule.FactDeclarations}, {rule.FactCalls}, {rule.FactFieldTypes}, {rule.FactResolvedCalls}} {
		got, err := NewProducer().Facts(root, files, req)
		if err != nil {
			t.Fatal(err)
		}
		f := got["x.py"]
		resolvedCalls := len(req) > 0 && req[0] == rule.FactResolvedCalls
		fields := len(req) > 0 && req[0] == rule.FactFieldTypes
		if f.ResolvedCallsAvailable != resolvedCalls || f.FieldTypesAvailable != fields {
			t.Fatal(f)
		}
		for _, d := range f.Declarations {
			if !fields && len(d.Holds) > 0 {
				t.Fatal("unrequested Holds")
			}
		}
		for _, c := range f.Calls {
			if !resolvedCalls && (!c.Target.IsZero() || !c.Receiver.IsZero()) {
				t.Fatal("unrequested resolution")
			}
			if c.EnclosingOwner != "C" {
				t.Fatal("missing syntactic owner", c)
			}
		}
	}
}

func TestC3AndProtocol(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": `from typing import Protocol
class A:
    def m(self): pass
class B(A): pass
class C(A):
    def m(self): pass
class D(B,C): pass
class Port(Protocol):
    def run(self): pass
class Use:
    port: Port
    def f(self,p: D):
        p.m()
        self.port.run()
`})["x.py"]
	if f.Calls[0].Target != declared("x.py", "C", "m") || f.Calls[0].Receiver != declared("x.py", "", "D") {
		t.Fatal(f.Calls[0])
	}
	if f.Calls[1].Target != declared("x.py", "Port", "run") {
		t.Fatal(f.Calls[1])
	}
}

func TestCompetingRoots(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": "from model import C\nC()\n", "model.py": "class C: pass\n", "src/model.py": "class C: pass\n"})["x.py"]
	if !strings.Contains(f.Calls[0].Target.Reason, "competing source roots") {
		t.Fatal(f.Calls)
	}
}

func TestExternalTypesAndConservativeMembers(t *testing.T) {
	f := resolved(t, map[string]string{
		"pyproject.toml": "[project]\nname='app'\ndependencies=['dependency']\n",
		"x.py": `import dependency as dep
from dependency import Repo as R
class C:
    repo: R
    def f(self,p: R):
        dep.work()
        p.save()
`,
	})["x.py"]
	if f.Calls[0].Target != external("dependency", "work") {
		t.Fatal(f.Calls[0])
	}
	if f.Calls[1].Receiver != external("dependency", "Repo") || f.Calls[1].Target != unknown("external receiver Repo has no observed member declaration for save") {
		t.Fatal(f.Calls[1])
	}
}

func TestLexicalScopeShadowingAndCallOccurrences(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": `class R:
    def save(self): pass
def outer(p: R):
    def inner(p):
        p.save()
    p.save()
    [p.save() for p in []]
    getattr(p, 'save')()
    return R().save()
R()
`})["x.py"]
	for _, c := range f.Calls {
		if c.Enclosing == "inner" && c.EnclosingOwner != "" {
			t.Fatal(c)
		}
	}
	if f.Calls[0].Target != unknown("p has no declared type") {
		t.Fatal(f.Calls[0])
	}
	if f.Calls[1].Target != declared("x.py", "R", "save") {
		t.Fatal(f.Calls[1])
	}
	if f.Calls[2].Target.Resolution != conformance.ResolutionUndetermined {
		t.Fatal(f.Calls[2])
	}
	if f.Calls[3].Target != unknown("computed member call") || f.Calls[4].Target != unknown("getattr uses dynamic attribute access") {
		t.Fatal(f.Calls[3:5])
	}
	if len(f.Calls) != 8 {
		t.Fatal(f.Calls)
	}
}

func TestMoreAnnotationShapes(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": `from typing import Annotated, Union, Callable as Fn, ClassVar
import dataclasses as dc
class X: pass
class User:
    class_field: ClassVar[X]
    init: dc.InitVar[X]
    forward: "dict[str, list[X | None]]"
    multi: Union[X, str]
    decorated: Annotated[X, 'metadata']
    callback: Fn[[X], X]
    missing: Unknown
    def f(self,p: X | str):
        p.save()
`})["x.py"]
	got := map[string][]conformance.Reference{}
	for _, d := range f.Declarations {
		if d.Kind == "field" {
			got[d.Name] = d.Holds
		}
	}
	want := map[string][]conformance.Reference{
		"forward":   {external("", "dict"), external("", "str"), external("", "list"), declared("x.py", "", "X")},
		"multi":     {external("typing", "Union"), declared("x.py", "", "X"), external("", "str")},
		"decorated": {external("typing", "Annotated"), declared("x.py", "", "X")},
		"callback":  {external("typing", "Callable")},
		"missing":   {unknown("Unknown is neither declared in the file nor imported")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v want %+v", got, want)
	}
	if f.Calls[0].Target != unknown("union annotation has no single receiver type") {
		t.Fatal(f.Calls[0])
	}
}

func TestSuperAndUnresolvedBases(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": `class Base:
    def run(self): pass
class Child(Base):
    def run(self):
        super().run()
class Unknown(Missing): pass
def f(p: Unknown):
    p.run()
`})["x.py"]
	if f.Calls[0].Target != declared("x.py", "Base", "run") || f.Calls[0].Receiver != declared("x.py", "", "Child") {
		t.Fatal(f.Calls[0])
	}
	if f.Calls[2].Target != unknown("base Missing of Unknown does not resolve to a repository class") {
		t.Fatal(f.Calls[2])
	}
}

func TestModuleMonkeyPatching(t *testing.T) {
	f := resolved(t, map[string]string{"m.py": "def work(): pass\n", "x.py": "import m\nm.work = lambda: None\ndef f():\n    m.work()\n"})["x.py"]
	if f.Calls[0].Target != unknown("monkey patching changes work") {
		t.Fatal(f.Calls)
	}
}

func TestGrammarBasedReceiverAnnotations(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": `from typing import Union as U, Optional as O
class C:
    def run(self): pass
def f(a: U[C,str], b: list[C | str], c: O[C]):
    a.run()
    b.append(C())
    c.run()
`})["x.py"]
	if f.Calls[0].Target != unknown("union annotation has no single receiver type") {
		t.Fatal(f.Calls[0])
	}
	if f.Calls[1].Receiver != external("", "list") {
		t.Fatal(f.Calls[1])
	}
	if f.Calls[3].Target != declared("x.py", "C", "run") {
		t.Fatal(f.Calls[3])
	}
}

func TestDescriptorSetattrAndKwargs(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": `class D:
    def __get__(self,instance,owner): pass
class C:
    descriptor: D
    def save(self): pass
def f(p: C, **kw):
    p.save(**kw)
    setattr(C, 'save', lambda: None)
    p.save()
`})["x.py"]
	if f.Calls[0].Receiver != declared("x.py", "", "C") || f.Calls[0].Target != unknown("**kwargs plumbing has no declared target") {
		t.Fatal(f.Calls[0])
	}
	if f.Calls[2].Target != unknown("monkey patching changes C.save") {
		t.Fatal(f.Calls[2])
	}
	for _, d := range f.Declarations {
		if d.Name == "descriptor" && !reflect.DeepEqual(d.Holds, []conformance.Reference{unknown("descriptor type D needs descriptor resolution")}) {
			t.Fatal(d)
		}
	}
}

func TestCallingFieldDoesNotCallItsAnnotation(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": `from typing import Callable
class C:
    callback: Callable[[], int]
    constructor: type
    def f(self):
        self.callback()
        self.constructor()
`})["x.py"]
	if f.Calls[0].Target != unknown("callable field callback has no declared call target") || f.Calls[1].Target != unknown("callable field constructor has no declared call target") {
		t.Fatal(f.Calls)
	}
}

func TestGeneratedBuiltinsAndDottedExternalModule(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": `import os.path
import builtins
from builtins import getattr as lookup
class C:
    exception: FileNotFoundError
    invalid: len
def f(p):
    FileNotFoundError()
    os.path.join('a', 'b')
    builtins.getattr(p,'x')
    lookup(p,'x')
`})["x.py"]
	if f.Calls[0].Target != external("", "FileNotFoundError") || f.Calls[1].Target != external("os.path", "join") {
		t.Fatal(f.Calls)
	}
	for _, c := range f.Calls[2:] {
		if c.Target != unknown("getattr uses dynamic attribute access") {
			t.Fatal(c)
		}
	}
	for _, d := range f.Declarations {
		if d.Name == "invalid" && !reflect.DeepEqual(d.Holds, []conformance.Reference{unknown("len is a builtin value, not a declared type")}) {
			t.Fatal(d)
		}
	}
}

func TestDecoratorsMustBeEstablished(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": `def dataclass(c): return lambda: None
@dataclass
class Replaced: pass
class C:
    @staticmethod
    def f(self):
        self.run()
def f():
    Replaced()
`})["x.py"]
	if f.Calls[0].Target != unknown("self has no declared type") {
		t.Fatal(f.Calls[0])
	}
	if f.Calls[1].Target != unknown("decorated declaration Replaced may be a descriptor or replacement") {
		t.Fatal(f.Calls[1])
	}
}

func TestDataclassGeneratedMemberHasNoSourceDeclaration(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": `from dataclasses import dataclass
@dataclass
class User:
    name: str
class Anonymous(User):
    def __init__(self):
        super().__init__('anonymous')
User('name')
`})["x.py"]
	if f.Calls[0].Target != unknown("dataclass generated member __init__ has no source declaration") {
		t.Fatal(f.Calls[0])
	}
	if f.Calls[2].Target != declared("x.py", "", "User") {
		t.Fatal(f.Calls[2])
	}
}

func TestUntypedAliasesAndValuesDoNotBecomeReceiverTypes(t *testing.T) {
	facts := resolved(t, map[string]string{"x.py": `class Repo:
    def save(self): pass
with open('file') as Repo:
    Repo.save()
`, "y.py": `class Repo:
    def save(self): pass
try: pass
except Exception as Repo:
    Repo.save()
`, "z.py": `class C:
    def f(self):
        self.missing.save()
def function(): pass
function.save()
`})
	for _, file := range []string{"x.py", "y.py"} {
		for _, c := range facts[file].Calls {
			if c.Callee == "save" && c.Target != unknown("Repo has competing bindings") {
				t.Fatal(file, c)
			}
		}
	}
	for _, c := range facts["z.py"].Calls {
		if c.Target.Resolution != conformance.ResolutionUndetermined || c.Receiver.Resolution != conformance.ResolutionUndetermined {
			t.Fatal(c)
		}
	}
}

func TestStarImportCanReplaceAnEarlierDeclaration(t *testing.T) {
	f := resolved(t, map[string]string{"x.py": "class Repo: pass\nfrom dependency import *\nRepo()\n"})["x.py"]
	if f.Calls[0].Target != unknown("star import may bind Repo") {
		t.Fatal(f.Calls[0])
	}
}

// TestResultTypesFollowTheReturnAnnotation names the declaration a
// def's return annotation is, evaluated around the def, and leaves a
// result that names one of the def's own PEP 695 type parameters
// undetermined, whatever else shares its name.
func TestResultTypesFollowTheReturnAnnotation(t *testing.T) {
	facts := resolved(t, map[string]string{
		"password.py": "class EncryptedPassword:\n    pass\n\n\nclass Encrypter:\n    def encrypt(self, plain: str) -> EncryptedPassword: ...\n\n    def generic[EncryptedPassword](self, plain: EncryptedPassword) -> EncryptedPassword: ...\n\n    def bare(self, plain: str): ...\n",
	})
	types := map[string]conformance.Reference{}
	for _, d := range facts["password.py"].Declarations {
		if d.Kind == declMethod {
			types[d.Name] = d.Type
		}
	}
	if got := types["encrypt"]; got.Resolution != conformance.ResolutionDeclared || got.File != "password.py" || got.Name != "EncryptedPassword" {
		t.Errorf("encrypt result = %+v, want the declared EncryptedPassword", got)
	}
	if got := types["generic"]; got.Resolution != conformance.ResolutionUndetermined || !strings.Contains(got.Reason, "type parameter") {
		t.Errorf("generic result = %+v, want undetermined as a type parameter", got)
	}
	if got := types["bare"]; got.Resolution != conformance.ResolutionUndetermined || !strings.Contains(got.Reason, "no return annotation") {
		t.Errorf("bare result = %+v, want undetermined without an annotation", got)
	}

	facts = resolved(t, map[string]string{
		"captured.py": "from typing import Protocol\n\n\nclass EncryptedPassword:\n    pass\n\n\nmatch str:\n    case EncryptedPassword:\n        pass\n\n\nclass Encrypter(Protocol):\n    def encrypt(self, plain: str) -> EncryptedPassword: ...\n",
	})
	for _, d := range facts["captured.py"].Declarations {
		if d.Kind == declMethod && d.Name == "encrypt" && d.Type.Resolution != conformance.ResolutionUndetermined {
			t.Errorf("a result naming a name a match pattern rebinds = %+v, want undetermined", d.Type)
		}
	}

	facts = resolved(t, map[string]string{
		"generic.py": "from typing import Protocol\n\n\nclass EncryptedPassword:\n    pass\n\n\nclass Encrypter[EncryptedPassword](Protocol):\n    def encrypt(self, plain: str) -> EncryptedPassword: ...\n",
	})
	for _, d := range facts["generic.py"].Declarations {
		if d.Kind == declMethod && d.Name == "encrypt" && (d.Type.Resolution != conformance.ResolutionUndetermined || !strings.Contains(d.Type.Reason, "type parameter")) {
			t.Errorf("a method result naming its class's type parameter = %+v, want undetermined", d.Type)
		}
	}
}

// TestAbstractMarkReachesTheLibrary marks a class whose Protocol base is
// the library's, and leaves one unmarked whose Protocol comes from a
// module of the repository that shares the library's name.
func TestAbstractMarkReachesTheLibrary(t *testing.T) {
	root, files := writeFiles(t, map[string]string{
		"typing_extensions.py": "class Protocol:\n    pass\n",
		"local.py":             "from typing_extensions import Protocol\n\n\nclass Encrypter(Protocol):\n    def encrypt(self, plain: str) -> str: ...\n",
		"library.py":           "from typing import Protocol\n\n\nclass Encrypter((Protocol)):\n    def encrypt(self, plain: str) -> str: ...\n",
	})
	for _, requested := range [][]rule.Fact{{rule.FactDeclarations}, {rule.FactDeclarations, rule.FactFieldTypes}} {
		facts, err := NewProducer().Facts(root, files, requested)
		if err != nil {
			t.Fatal(err)
		}
		for file, want := range map[string]bool{"local.py": false, "library.py": true} {
			for _, d := range facts[file].Declarations {
				if d.Kind == declClass && d.Name == "Encrypter" && d.Abstract != want {
					t.Errorf("%v: %s Encrypter abstract = %v, want %v", requested, file, d.Abstract, want)
				}
			}
		}
	}
}
