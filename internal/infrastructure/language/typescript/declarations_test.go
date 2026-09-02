package typescript

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

func TestTypeScriptextractDeclarations(t *testing.T) {
	src := []byte(`import { thing } from "./other";

export class Repo {
  private items: string[] = [];
  count: number = 0;
  find(id: string): string { return id; }
  private hidden(): void {}
}

export interface Port {
  load(id: string): Promise<string>;
  retries: number;
}

export type MemberID = string;

export enum Status { Open, Closed }

export function topLevel(a: number): number {
  return a + 1;
}

const arrow = (x: number) => x * 2;

export const exportedArrow = (y: string) => y;

let counter = 0;
`)
	got := extractDeclarations("src/domain/repo.ts", src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	want := []conformance.Declaration{
		{Kind: "class", Name: "Repo", Exported: true, StartLine: 3, EndLine: 8},
		{Kind: "field", Name: "items", Owner: "Repo", StartLine: 4, EndLine: 4},
		{Kind: "field", Name: "count", Owner: "Repo", Exported: true, StartLine: 5, EndLine: 5},
		{
			Kind: "method", Name: "find", Owner: "Repo", Exported: true, StartLine: 6, EndLine: 6,
			Params: []conformance.DeclarationParam{{Name: "id", Type: "string"}}, Results: []string{"string"},
		},
		{
			Kind: "method", Name: "hidden", Owner: "Repo", StartLine: 7, EndLine: 7,
			Results: []string{"void"},
		},
		{Kind: "interface", Name: "Port", Exported: true, StartLine: 10, EndLine: 13},
		{
			Kind: "method", Name: "load", Owner: "Port", Exported: true, StartLine: 11, EndLine: 11,
			Params: []conformance.DeclarationParam{{Name: "id", Type: "string"}}, Results: []string{"Promise<string>"},
		},
		{Kind: "field", Name: "retries", Owner: "Port", Exported: true, StartLine: 12, EndLine: 12},
		{Kind: "type", Name: "MemberID", Exported: true, StartLine: 15, EndLine: 15},
		{Kind: "enum", Name: "Status", Exported: true, StartLine: 17, EndLine: 17},
		{
			Kind: "func", Name: "topLevel", Exported: true, StartLine: 19, EndLine: 21,
			Params: []conformance.DeclarationParam{{Name: "a", Type: "number"}}, Results: []string{"number"},
		},
		{
			Kind: "func", Name: "arrow", StartLine: 23, EndLine: 23,
			Params: []conformance.DeclarationParam{{Name: "x", Type: "number"}},
		},
		{
			Kind: "func", Name: "exportedArrow", Exported: true, StartLine: 25, EndLine: 25,
			Params: []conformance.DeclarationParam{{Name: "y", Type: "string"}},
		},
		{Kind: "var", Name: "counter", StartLine: 27, EndLine: 27},
	}
	diffDecls(t, got.Decls, want)
}

func TestJavaScriptextractDeclarations(t *testing.T) {
	// Exported means "under an ESM export statement": plain JS without
	// export markers is structurally unexported (CommonJS module.exports
	// assignment is invisible at this tier, by design). Class members
	// keep their own visibility.
	src := []byte(`class Store {
  save(x) { return x; }
}

function helper() {}

const fn = () => 1;
`)
	got := extractDeclarations("src/store.js", src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	want := []conformance.Declaration{
		{Kind: "class", Name: "Store", StartLine: 1, EndLine: 3},
		{
			Kind: "method", Name: "save", Owner: "Store", Exported: true, StartLine: 2, EndLine: 2,
			Params: []conformance.DeclarationParam{{Name: "x"}},
		},
		{Kind: "func", Name: "helper", StartLine: 5, EndLine: 5},
		{Kind: "func", Name: "fn", StartLine: 7, EndLine: 7},
	}
	diffDecls(t, got.Decls, want)
}

// TestTypeScriptSignatureFacts pins the M10 signature tier for TS/JS:
// optional markers, defaults, rest params, destructuring left unnamed,
// generic and union types as normalized source text.
func TestTypeScriptSignatureextractDeclarations(t *testing.T) {
	src := []byte(`export interface Port {
  load(id: string, opts?: LoadOpts): Promise<Book | null>;
}
export class Repo {
  find(id: string, ...rest: string[]): Book | null { return null; }
  make({ a, b }: MakeOpts, x = 3): void {}
}
const arrow = (x: number, y = 2): number => x;
const bare = x => x;
`)
	got := extractDeclarations("src/port.ts", src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	want := []conformance.Declaration{
		{Kind: "interface", Name: "Port", Exported: true, StartLine: 1, EndLine: 3},
		{
			Kind: "method", Name: "load", Owner: "Port", Exported: true, StartLine: 2, EndLine: 2,
			Params: []conformance.DeclarationParam{
				{Name: "id", Type: "string"},
				{Name: "opts", Type: "LoadOpts", Optional: true},
			},
			Results: []string{"Promise<Book | null>"},
		},
		{Kind: "class", Name: "Repo", Exported: true, StartLine: 4, EndLine: 7},
		{
			Kind: "method", Name: "find", Owner: "Repo", Exported: true, StartLine: 5, EndLine: 5,
			Params: []conformance.DeclarationParam{
				{Name: "id", Type: "string"},
				{Name: "rest", Type: "string[]", Variadic: true},
			},
			Results: []string{"Book | null"},
		},
		{
			Kind: "method", Name: "make", Owner: "Repo", Exported: true, StartLine: 6, EndLine: 6,
			Params: []conformance.DeclarationParam{
				{Type: "MakeOpts"},
				{Name: "x", Optional: true},
			},
			Results: []string{"void"},
		},
		{
			Kind: "func", Name: "arrow", StartLine: 8, EndLine: 8,
			Params: []conformance.DeclarationParam{
				{Name: "x", Type: "number"},
				{Name: "y", Optional: true},
			},
			Results: []string{"number"},
		},
		{
			Kind: "func", Name: "bare", StartLine: 9, EndLine: 9,
			Params: []conformance.DeclarationParam{{Name: "x"}},
		},
	}
	diffDecls(t, got.Decls, want)
}

func TestTypeScriptextractCalls(t *testing.T) {
	src := []byte(`export class Event {
  constructor() {
    this.publishedFrozen();
  }
  publishedFrozen(): Error | null { return null; }
  publish(): Error | null {
    return this.publishedFrozen();
  }
}
`)
	got := extractDeclarations("src/event.ts", src)
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
	for _, k := range []string{"publishedFrozen@constructor", "publishedFrozen@publish"} {
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
