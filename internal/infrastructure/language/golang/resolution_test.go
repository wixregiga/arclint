package golang_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	golang "github.com/wixregiga/arclint/internal/infrastructure/language/golang"
	filesystemobservation "github.com/wixregiga/arclint/internal/infrastructure/observation/filesystem"
)

var resolvedFacts = []rule.Fact{rule.FactDeclarations, rule.FactCalls, rule.FactResolvedCalls, rule.FactFieldTypes}

// orderModule is a module whose order package holds the shapes the
// role rules judge: calls through an interface field and a method
// promoted into it, stdlib calls and methods, a function-typed field,
// builtins, conversions, generics, and fields of every holding shape.
func orderModule(t *testing.T) (string, []conformance.ObservedFile) {
	t.Helper()
	root := t.TempDir()
	files := []conformance.ObservedFile{
		write(t, root, "go.mod", "module example.com/shop\n\ngo 1.24\n"),
		write(t, root, "store/store.go", `package store

type Store[T any] interface {
	Put(T) error
}

type Box[T any] struct{ V T }
`),
		write(t, root, "order/repository.go", `package order

import "example.com/shop/store"

type Repository interface {
	store.Store[Order]
	Find(id ID) (Order, error)
}

type Alias = Repository
`),
		write(t, root, "order/order.go", `package order

import (
	"time"

	"example.com/shop/store"
)

type ID string

type Line struct{ n int }

type Ptr[T any] = *T

type Lines []Line

type Factory[T any] func() T

type Order struct {
	id      ID
	lines   []*Line
	byID    map[ID]Line
	visit   func(Line)
	created time.Time
	repo    Repository
	alias   Alias
	boxed   store.Box[Line]
	clock   func() time.Time
	nested  struct{ first Line }
	notify  interface{ Send() }
	viaPtr  Ptr[ID]
	defined Lines
	factory Factory[Line]
}

func (o *Order) Place(at time.Time) error {
	_ = len(o.lines)
	_ = ID("x")
	if o.created.Before(at) {
		return nil
	}
	_ = o.clock()
	o.notify.Send()
	_, _ = o.repo.Find(o.id)
	return o.repo.Put(*o)
}

func Total(o Order) int {
	type view Order
	_ = view(o)
	return len(o.lines)
}
`),
		write(t, root, "order/order_test.go", `package order

func helper(o *Order) { _ = o.Place }
func useHelper() { helper(nil) }
`),
		write(t, root, "order/ignored.go", "//go:build ignore\n\npackage order\n\nfunc ignored() { Total(Order{}) }\n"),
	}
	return root, files
}

func factsOf(t *testing.T, root string, files []conformance.ObservedFile, requested []rule.Fact) map[string]conformance.LanguageFacts {
	t.Helper()
	facts, err := golang.NewProducer().Facts(root, files, requested)
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}
	if _, err := conformance.NewObservations(files, facts); err != nil {
		t.Fatalf("the producer's facts are refused: %v", err)
	}
	return facts
}

func callAt(t *testing.T, f conformance.LanguageFacts, callee string, enclosing string) conformance.Call {
	t.Helper()
	for _, c := range f.Calls {
		if c.Callee == callee && c.Enclosing == enclosing {
			return c
		}
	}
	t.Fatalf("no call %s in %s among %+v", callee, enclosing, f.Calls)
	return conformance.Call{}
}

func fieldNamed(t *testing.T, f conformance.LanguageFacts, owner, name string) conformance.Declaration {
	t.Helper()
	for _, d := range f.Declarations {
		if d.Kind == "field" && d.Owner == owner && d.Name == name {
			return d
		}
	}
	t.Fatalf("no field %s.%s", owner, name)
	return conformance.Declaration{}
}

func declaredRef(file, owner, name string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionDeclared, File: file, Owner: owner, Name: name}
}

func localRef(name string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionLocal, Name: name}
}

func externalRef(pkg, owner, name string) conformance.Reference {
	return conformance.Reference{Resolution: conformance.ResolutionExternal, Package: pkg, Owner: owner, Name: name}
}

// Calls resolve to the declaration the Go type checker selects, with
// the static type of the value a method is called on; a call whose
// function the code does not declare is undetermined, with the reason.
func TestResolvedCallsFollowTheTypeChecker(t *testing.T) {
	root, files := orderModule(t)
	facts := factsOf(t, root, files, resolvedFacts)
	order := facts["order/order.go"]
	if !order.ResolvedCallsAvailable || !order.FieldTypesAvailable {
		t.Fatalf("resolution was requested but not supplied: %+v", order)
	}
	cases := []struct {
		callee, enclosing string
		target, receiver  conformance.Reference
	}{
		{"Find", "Place", declaredRef("order/repository.go", "Repository", "Find"), declaredRef("order/repository.go", "", "Repository")},
		{"Put", "Place", declaredRef("store/store.go", "Store", "Put"), declaredRef("order/repository.go", "", "Repository")},
		{"Before", "Place", externalRef("", "", "Before"), externalRef("time", "", "Time")},
		{"len", "Place", externalRef("", "", "len"), conformance.Reference{}},
		{"ID", "Place", declaredRef("order/order.go", "", "ID"), conformance.Reference{}},
		{"len", "Total", externalRef("", "", "len"), conformance.Reference{}},
		{"Send", "Place", localRef("Send"), localRef("interface{Send()}")},
		{"view", "Total", localRef("view"), conformance.Reference{}},
	}
	for _, c := range cases {
		got := callAt(t, order, c.callee, c.enclosing)
		if got.Target != c.target || got.Receiver != c.receiver {
			t.Errorf("%s in %s: target %+v receiver %+v, want %+v and %+v", c.callee, c.enclosing, got.Target, got.Receiver, c.target, c.receiver)
		}
		if c.enclosing == "Place" && got.EnclosingOwner != "Order" {
			t.Errorf("%s in Place: enclosing owner %q, want Order", c.callee, got.EnclosingOwner)
		}
	}
	clock := callAt(t, order, "clock", "Place")
	if clock.Target.Resolution != conformance.ResolutionUndetermined || !strings.Contains(clock.Target.Reason, "clock is a field of function type") {
		t.Errorf("clock: %+v, want undetermined because the field holds a function value", clock.Target)
	}
	helper := callAt(t, facts["order/order_test.go"], "helper", "useHelper")
	if helper.Target.Reason != "test files are not type-checked" {
		t.Errorf("test file call: %+v", helper.Target)
	}
	ignored := callAt(t, facts["order/ignored.go"], "Total", "ignored")
	if ignored.Target.Resolution != conformance.ResolutionUndetermined || !strings.Contains(ignored.Target.Reason, "does not build this file") {
		t.Errorf("build-constrained call: %+v", ignored.Target)
	}
}

// A field holds its type and, through pointers, collections, type
// arguments, aliases, and unnamed structs, the declarations inside;
// never the types inside a function signature.
func TestFieldTypesHoldWhatTheFieldStores(t *testing.T) {
	root, files := orderModule(t)
	order := factsOf(t, root, files, resolvedFacts)["order/order.go"]
	cases := map[string][]conformance.Reference{
		"id":      {declaredRef("order/order.go", "", "ID")},
		"lines":   {declaredRef("order/order.go", "", "Line")},
		"byID":    {declaredRef("order/order.go", "", "ID"), declaredRef("order/order.go", "", "Line")},
		"visit":   nil,
		"created": {externalRef("time", "", "Time")},
		"repo":    {declaredRef("order/repository.go", "", "Repository")},
		"alias":   {declaredRef("order/repository.go", "", "Alias"), declaredRef("order/repository.go", "", "Repository")},
		"boxed":   {declaredRef("store/store.go", "", "Box"), declaredRef("order/order.go", "", "Line")},
		"clock":   nil,
		"nested":  {declaredRef("order/order.go", "", "Line")},
		"notify":  nil,
		"defined": {declaredRef("order/order.go", "", "Lines"), declaredRef("order/order.go", "", "Line")},
		"factory": {declaredRef("order/order.go", "", "Factory")},
	}
	ownTypes := map[string]conformance.Reference{
		"id":      declaredRef("order/order.go", "", "ID"),
		"lines":   localRef("[]*Line"),
		"created": externalRef("time", "", "Time"),
		"repo":    declaredRef("order/repository.go", "", "Repository"),
		"boxed":   declaredRef("store/store.go", "", "Box"),
		"alias":   declaredRef("order/repository.go", "", "Repository"),
		"viaPtr":  declaredRef("order/order.go", "", "ID"),
		"clock":   localRef("func() time.Time"),
	}
	for name, want := range ownTypes {
		if got := fieldNamed(t, order, "Order", name).Type; got != want {
			t.Errorf("field %s own type %+v, want %+v", name, got, want)
		}
	}
	for name, want := range cases {
		got := fieldNamed(t, order, "Order", name).Holds
		if len(got) != len(want) {
			t.Errorf("field %s holds %+v, want %+v", name, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("field %s holds %+v, want %+v", name, got, want)
				break
			}
		}
	}
}

// Without the resolved classes requested, the producer resolves
// nothing and runs no toolchain load.
func TestResolutionOnlyWhenRequested(t *testing.T) {
	root, files := orderModule(t)
	order := factsOf(t, root, files, []rule.Fact{rule.FactDeclarations, rule.FactCalls})["order/order.go"]
	if order.ResolvedCallsAvailable || order.FieldTypesAvailable {
		t.Fatal("resolution supplied without being requested")
	}
	for _, c := range order.Calls {
		if !c.Target.IsZero() || !c.Receiver.IsZero() {
			t.Fatalf("call %s carries a resolution", c.Callee)
		}
	}
	if c := callAt(t, order, "Find", "Place"); c.EnclosingOwner != "Order" {
		t.Errorf("enclosing owner %q, want Order: it is part of the syntactic call fact", c.EnclosingOwner)
	}
}

// Resolution runs no toolchain: with no go command on PATH it still
// resolves, a call into a required dependency that was never
// downloaded is external, an import no go.mod requires is
// undetermined with the reason, and go.mod is never rewritten.
func TestResolutionRunsNoToolchain(t *testing.T) {
	root := t.TempDir()
	goMod := "module example.com/broken\n\ngo 1.24\n\nrequire example.com/absent v1.0.0\n"
	files := []conformance.ObservedFile{
		write(t, root, "go.mod", goMod),
		write(t, root, "app/app.go", "package app\n\nimport (\n\t\"example.com/absent\"\n\t\"example.com/unrequired\"\n)\n\ntype App struct {\n\tdep absent.Thing\n\tother unrequired.Thing\n}\n\nfunc Run() {\n\tabsent.Do()\n\tunrequired.Do()\n}\n"),
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GOFLAGS", "-mod=mod")
	app := factsOf(t, root, files, resolvedFacts)["app/app.go"]
	after, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != goMod {
		t.Fatalf("go.mod was rewritten:\n%s", after)
	}
	if _, err := os.Stat(filepath.Join(root, "go.sum")); err == nil {
		t.Fatal("go.sum was written")
	}
	if got := callAt(t, app, "Do", "Run").Target; got != externalRef("example.com/absent", "", "Do") {
		t.Errorf("call into a required dependency: %+v, want external", got)
	}
	if got := fieldNamed(t, app, "App", "dep").Holds; len(got) != 1 || got[0] != externalRef("example.com/absent", "", "Thing") {
		t.Errorf("field of a dependency's type holds %+v, want the external Thing", got)
	}
	var unrequired conformance.Call
	for _, c := range app.Calls {
		if c.Callee == "Do" && c.Target.Package != "example.com/absent" {
			unrequired = c
		}
	}
	if unrequired.Target.Resolution != conformance.ResolutionUndetermined || !strings.Contains(unrequired.Target.Reason, "not provided by any module the go.mod requires") {
		t.Errorf("call into an unrequired import: %+v, want undetermined with the reason", unrequired.Target)
	}
}

// TestResolutionOverRealRepositories proves the resolved facts on the
// pinned real repositories of the toolchain suite and on this
// repository against the Go toolchain's own type check: the
// producer's output is accepted by the observation contract, every
// declared reference names a declaration the referenced file really
// holds, and no reference contradicts the toolchain's. A reference
// the producer leaves undetermined where the toolchain resolves it is
// lost coverage, counted and logged, never a contradiction.
//
// It clones six repositories and runs the Go toolchain over them, so it
// runs only when asked: make test-oracle, or ARCLINT_ORACLE=1, after a
// change to Go resolution.
func TestResolutionOverRealRepositories(t *testing.T) {
	if os.Getenv("ARCLINT_ORACLE") == "" {
		t.Skip("the Go toolchain comparison over real repositories; run make test-oracle")
	}
	self, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	dirs := map[string]string{"arclint": self}
	names := []string{"arclint"}
	for _, spec := range repos {
		names = append(names, spec.name)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			repoDir, ok := dirs[name]
			if !ok {
				for _, spec := range repos {
					if spec.name == name {
						repoDir = ensureClone(t, spec)
					}
				}
			}
			source, err := filesystemobservation.NewSource(repoDir)
			if err != nil {
				t.Fatal(err)
			}
			observed, err := source.Observe(nil, rule.Scan{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			mods, err := golang.Modules(repoDir, observed.Files())
			if err != nil {
				t.Fatal(err)
			}
			_, workspaceErr := os.Stat(filepath.Join(repoDir, "go.work"))
			if name != "arclint" {
				for _, m := range mods {
					// The suite's own ground truth fills the module cache.
					goList(t, filepath.Join(repoDir, filepath.FromSlash(m.Dir)), workspaceErr == nil)
				}
			}
			before := moduleFiles(t, repoDir, observed.Files())
			facts := factsOf(t, repoDir, observed.Files(), resolvedFacts)
			if after := moduleFiles(t, repoDir, observed.Files()); !equalBytes(before, after) {
				t.Fatal("resolution changed go.mod or go.sum")
			}
			// go/packages races inside go/types under -race (Go 1.27
			// fills shared instantiated types lazily), so a race build
			// checks arclint's own facts only; make test-oracle runs the
			// comparison without -race.
			oracle := map[string]conformance.LanguageFacts{}
			if !raceDetector {
				oracle, err = golang.ToolchainFacts(repoDir, observed.Files(), resolvedFacts)
				if err != nil {
					t.Fatal(err)
				}
			}

			declared := map[string]map[string]bool{}
			for path, f := range facts {
				names := map[string]bool{}
				for _, d := range f.Declarations {
					names[d.Owner+"."+d.Name] = true
				}
				declared[path] = names
			}
			counts := map[conformance.Resolution]int{}
			reasons := map[string]int{}
			var wrong, contradictions []string
			lost := 0
			compare := func(where string, got, want conformance.Reference, wantTyped bool) {
				if got.IsZero() && want.IsZero() {
					return
				}
				counts[got.Resolution]++
				switch got.Resolution {
				case conformance.ResolutionDeclared:
					if !declared[got.File][got.Owner+"."+got.Name] {
						wrong = append(wrong, where+" -> "+got.File+" "+got.Owner+"."+got.Name)
					}
				case conformance.ResolutionUndetermined:
					reasons[reasonKind(got.Reason)]++
				}
				if !wantTyped {
					return
				}
				if got.Resolution == conformance.ResolutionUndetermined {
					if want.Resolution != conformance.ResolutionUndetermined {
						lost++
					}
					return
				}
				if !agrees(got, want) {
					contradictions = append(contradictions, fmt.Sprintf("%s: %+v, toolchain %+v", where, got, want))
				}
			}
			for path, f := range facts {
				o, typed := oracle[path]
				typed = typed && o.ResolvedCallsAvailable && !strings.HasSuffix(path, "_test.go") && oracleTyped(o)
				if typed && len(o.Calls) != len(f.Calls) {
					t.Fatalf("%s: %d calls, toolchain %d", path, len(f.Calls), len(o.Calls))
				}
				for i, c := range f.Calls {
					where := path + ":" + strconv.Itoa(c.Line) + " " + c.Callee
					var want conformance.Call
					if typed {
						want = o.Calls[i]
					}
					compare(where, c.Target, want.Target, typed)
					compare(where+" (receiver)", c.Receiver, want.Receiver, typed && !want.Receiver.IsZero())
				}
				fields := map[string][]conformance.Reference{}
				types := map[string]conformance.Reference{}
				if typed {
					for _, d := range o.Declarations {
						fields[d.Owner+"."+d.Name+":"+strconv.Itoa(d.StartLine)] = d.Holds
						if d.Kind == "field" {
							types[d.Owner+"."+d.Name+":"+strconv.Itoa(d.StartLine)] = d.Type
						}
					}
				}
				for _, d := range f.Declarations {
					where := path + ":" + strconv.Itoa(d.StartLine) + " field " + d.Name
					open := false
					for _, ref := range d.Holds {
						compare(where, ref, conformance.Reference{}, false)
						open = open || ref.Resolution == conformance.ResolutionUndetermined
					}
					if !typed {
						continue
					}
					if wantType, ok := types[d.Owner+"."+d.Name+":"+strconv.Itoa(d.StartLine)]; ok {
						compare(where+" (type)", d.Type, wantType, true)
					}
					got, want := declaredSet(d.Holds), declaredSet(fields[d.Owner+"."+d.Name+":"+strconv.Itoa(d.StartLine)])
					for ref := range got {
						if !want[ref] {
							contradictions = append(contradictions, where+": holds "+ref+", the toolchain does not")
						}
					}
					for ref := range want {
						if !got[ref] {
							if open {
								lost++
								continue
							}
							contradictions = append(contradictions, where+": misses "+ref+" the toolchain holds")
						}
					}
				}
			}
			sort.Strings(wrong)
			if len(wrong) > 0 {
				t.Fatalf("%d declared references name no declaration of the referenced file, first: %s", len(wrong), strings.Join(wrong[:min(len(wrong), 10)], "; "))
			}
			sort.Strings(contradictions)
			if len(contradictions) > 0 {
				t.Fatalf("%d references contradict the toolchain, first: %s", len(contradictions), strings.Join(contradictions[:min(len(contradictions), 10)], "; "))
			}
			t.Logf("%s: declared %d, external %d, local %d, undetermined %d; %d the toolchain resolves", name,
				counts[conformance.ResolutionDeclared], counts[conformance.ResolutionExternal], counts[conformance.ResolutionLocal], counts[conformance.ResolutionUndetermined], lost)
			var top []string
			for r, n := range reasons {
				top = append(top, strconv.Itoa(n)+" "+r)
			}
			sort.Strings(top)
			t.Logf("undetermined by reason: %s", strings.Join(top, " | "))
		})
	}
}

// oracleTyped reports a file the toolchain type-checked without
// errors; a file of a package it could not fully type, such as one
// whose dependencies are not downloaded, is no ground truth.
func oracleTyped(f conformance.LanguageFacts) bool {
	for _, c := range f.Calls {
		if strings.HasPrefix(c.Target.Reason, golang.OracleUnchecked) {
			return false
		}
	}
	for _, d := range f.Declarations {
		if strings.HasPrefix(d.Type.Reason, golang.OracleUnchecked) {
			return false
		}
	}
	return true
}

// agrees reports a resolved reference that does not contradict the
// toolchain's: the same declaration, or the same side of the
// repository's boundary where arclint knows less of the declaration
// outside it.
func agrees(got, want conformance.Reference) bool {
	switch got.Resolution {
	case conformance.ResolutionDeclared, conformance.ResolutionLocal:
		return got == want
	case conformance.ResolutionExternal:
		switch want.Resolution {
		case conformance.ResolutionExternal:
			return got.Name == want.Name && (got.Package == "" || got.Package == want.Package)
		case conformance.ResolutionUndetermined:
			// A function value reached through a type outside the
			// repository: no repository declaration either way.
			return true
		}
	}
	return false
}

func declaredSet(refs []conformance.Reference) map[string]bool {
	out := map[string]bool{}
	for _, r := range refs {
		if r.Resolution == conformance.ResolutionDeclared {
			out[r.File+" "+r.Owner+"."+r.Name] = true
		}
	}
	return out
}

// reasonKind groups undetermined reasons by their wording, without the
// names and the toolchain's detail that make each one unique.
func reasonKind(reason string) string {
	if head, _, ok := strings.Cut(reason, ": "); ok {
		reason = head
	}
	if _, rest, ok := strings.Cut(reason, " is a "); ok {
		return "<name> is a " + rest
	}
	if _, rest, ok := strings.Cut(reason, " has the type parameter "); ok {
		_, tail, _ := strings.Cut(rest, ", ")
		return "<value> has a type parameter, " + tail
	}
	return reason
}

func moduleFiles(t *testing.T, root string, files []conformance.ObservedFile) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, f := range files {
		base := filepath.Base(f.Path)
		if base != "go.mod" && base != "go.sum" && base != "go.work" && base != "go.work.sum" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if err != nil {
			t.Fatal(err)
		}
		out[f.Path] = data
	}
	return out
}

func equalBytes(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if !bytes.Equal(v, b[k]) {
			return false
		}
	}
	return true
}
