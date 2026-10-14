package typescript

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func resolutionFixture(t *testing.T, sources map[string]string, requested ...rule.Fact) map[string]conformance.LanguageFacts {
	t.Helper()
	root, files := writeFiles(t, sources)
	facts, err := NewProducer().Facts(root, files, requested)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conformance.NewObservations(files, facts); err != nil {
		t.Fatal(err)
	}
	return facts
}

func TestResolutionImportGraph(t *testing.T) {
	sources := map[string]string{
		"tsconfig.json":   `{"compilerOptions":{"baseUrl":".","paths":{"@/*":["src/*"]}}}`,
		"src/original.ts": `export function work() {} export default function def() {} export class Repo { save(){} static open(){} }`,
		"src/barrel.ts":   `export {work as renamed, default} from './original'; export * from './original';`,
		"src/main.ts":     `import run, { renamed as go, Repo } from '@/barrel'; import * as ns from './barrel'; function main(){ go(); run(); ns.renamed(); Repo.open(); }`,
	}
	got := resolutionFixture(t, sources, rule.FactResolvedCalls)["src/main.ts"]
	want := []conformance.Reference{declared("src/original.ts", "", "work"), declared("src/original.ts", "", "def"), declared("src/original.ts", "", "work"), declared("src/original.ts", "Repo", "open")}
	if !got.ResolvedCallsAvailable || !got.CallsAvailable || got.DeclarationsAvailable {
		t.Fatalf("availability: %+v", got)
	}
	for i, c := range got.Calls {
		if c.Target != want[i] {
			t.Errorf("%s: got %+v want %+v", c.Callee, c.Target, want[i])
		}
	}
}

func TestResolutionReceiversAndFields(t *testing.T) {
	sources := map[string]string{
		"repo.ts": `export class Base { save() {} } export class Repo extends Base { static open(){} } export interface Port { save():void } export type Alias = Repo;`,
		"main.ts": `import {Repo, Port, Alias} from './repo';
 class Root {
   repo: Repo;
   collection: Promise<Map<string, Repo[] | null>>;
   callback: (p: Repo) => Repo;
   alias: Alias;
   constructor(private readonly port: Port) {}
   run(p: Repo) { const local: Repo = p; const created = new Repo(); this.repo.save(); this.port.save(); p.save(); local.save(); created.save(); Repo.open(); }
 }
 interface Holder { repository: Repo; }
 `,
	}
	got := resolutionFixture(t, sources, rule.FactResolvedCalls, rule.FactFieldTypes)["main.ts"]
	for _, c := range got.Calls {
		if c.EnclosingOwner != "Root" {
			t.Errorf("owner: %+v", c)
		}
		targetOwner, receiver := "Base", "Repo"
		if c.Callee == "open" {
			targetOwner = "Repo"
		}
		if c.Receiver.Name == "Port" {
			targetOwner, receiver = "Port", "Port"
		}
		if c.Target != declared("repo.ts", targetOwner, c.Callee) || c.Receiver != declared("repo.ts", "", receiver) {
			t.Errorf("call: %+v", c)
		}
	}
	held := map[string][]conformance.Reference{}
	for _, d := range got.Declarations {
		if d.Kind == "field" {
			held[d.Owner+"."+d.Name] = d.Holds
		}
	}
	repo := declared("repo.ts", "", "Repo")
	wants := map[string][]conformance.Reference{
		"Root.repo": {repo}, "Root.port": {declared("repo.ts", "", "Port")}, "Holder.repository": {repo},
		"Root.collection": {external("", "Promise"), external("", "Map"), external("", "string"), external("", "Array"), repo, external("", "null")},
		"Root.callback":   nil, "Root.alias": {declared("repo.ts", "", "Alias"), repo},
	}
	if !reflect.DeepEqual(held, wants) {
		t.Errorf("holds:\n got %#v\nwant %#v", held, wants)
	}
}

// A field of a generic function alias holds a function, not the
// alias's type arguments.
func TestCallableAliasHoldsNoTypeArguments(t *testing.T) {
	got := resolutionFixture(t, map[string]string{
		"svc.ts": `export class Pricing {} export type Factory<T> = () => T; export type Maker<T> = Factory<T>; export type Box<T> = { v: T };`,
		"main.ts": `import {Pricing, Factory, Maker, Box} from './svc';
 class Order { make: Factory<Pricing>; again: Maker<Pricing>; boxed: Box<Pricing>; }`,
	}, rule.FactResolvedCalls, rule.FactFieldTypes)["main.ts"]
	pricing := declared("svc.ts", "", "Pricing")
	for _, d := range got.Declarations {
		if d.Kind != "field" {
			continue
		}
		holdsPricing := false
		for _, ref := range d.Holds {
			holdsPricing = holdsPricing || ref == pricing
		}
		if holdsPricing != (d.Name == "boxed") {
			t.Errorf("field %s holds %+v", d.Name, d.Holds)
		}
	}
}

// A field's own type is the declaration its annotation names, through
// parentheses and a union with null or undefined; a collection of a
// type is the collection, and a static property is no field at all.
func TestResolutionFieldOwnType(t *testing.T) {
	sources := map[string]string{
		"id.ts": `export class OrderID {} export type StoredID = OrderID; export type MaybeID = StoredID | null;
 export type Maybe<T> = T | undefined; export type Wrapped<T = OrderID> = Maybe<T>; export type Loop = Loop;`,
		"main.ts": `import {OrderID, StoredID, MaybeID, Maybe, Wrapped, Loop} from './id';
 class Order {
   generic: Maybe<OrderID>;
   nested: Maybe<Maybe<StoredID>>;
   defaulted: Wrapped;
   loop: Loop;
   id: OrderID;
   stored: StoredID;
   maybeStored: MaybeID;
   maybe?: OrderID | undefined;
   nullable: (OrderID | null);
   ids: OrderID[];
   either: OrderID | string;
   static shared: OrderID;
 }
 `,
	}
	got := resolutionFixture(t, sources, rule.FactResolvedCalls, rule.FactFieldTypes)["main.ts"]
	own := map[string]conformance.Reference{}
	for _, d := range got.Declarations {
		if d.Kind == "field" {
			own[d.Name] = d.Type
		}
	}
	id := declared("id.ts", "", "OrderID")
	wants := map[string]conformance.Reference{
		"id": id, "maybe": id, "nullable": id, "stored": id, "maybeStored": id,
		"generic": id, "nested": id, "defaulted": id, "loop": uncertain("cyclic type alias cannot be resolved"),
		"ids":    external("", "Array"),
		"either": {Resolution: conformance.ResolutionLocal, Name: "OrderID | string"},
	}
	if !reflect.DeepEqual(own, wants) {
		t.Errorf("field types:\n got %#v\nwant %#v", own, wants)
	}
}

func TestResolutionUncertaintyReasons(t *testing.T) {
	tests := []struct{ name, source, reason string }{
		{"inferred", `class R{m(){}} function f(){ const r=make(); r.m(); }`, "binding has no written annotation or resolvable new initializer"},
		{"any", `function f(r:any){r.m()}`, "any has no established declaration"},
		{"unknown", `function f(r:unknown){r.m()}`, "unknown has no established declaration"},
		{"computed", `function f(r:R,k:string){r[k]()}`, "computed member call"},
		{"optional", `function f(r){r?.m?.()}`, "binding has no written annotation or resolvable new initializer"},
		{"missing", `function f(){ missing() }`, "missing is neither declared in the file nor imported"},
		{"merge", `interface R{m():void} interface R{x:string} function f(r:R){r.m()}`, "global declaration merging is not resolved"},
		{"ambient", `declare module "x" { export class R{} } function f(){ console.log() }`, "ambient modules or namespaces require declaration merging"},
		{"decorator", `function decorate(x:any){} @decorate class R{m(){}} function f(r:R){r.m()}`, "decorated type may change members"},
		{"conditional", `class R{m(){}} function f(r: string extends number ? R : R){r.m()}`, "conditional type is not resolved"},
		{"mapped", `class R{m(){}} function f(r: {[K in keyof R]: R[K]}){r.m()}`, "mapped type is not resolved"},
		{"indexed", `interface R{x:string} function f(r:R['x']){r.trim()}`, "indexed-access type is not resolved"},
		{"generic", `function f<R>(r:R){r.m()}`, "generic type parameter has no concrete declaration"},
		{"inference-call", `function f(fn:()=>void){fn()}`, "callee has no declared function implementation"},
		{"reassigned", `class R{m(){}} function f(){let r=new R();r=make();r.m()}`, "inferred new binding is reassigned"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fa := resolutionFixture(t, map[string]string{"main.ts": tt.source}, rule.FactResolvedCalls)["main.ts"]
			found := false
			for _, c := range fa.Calls {
				if c.Target == uncertain(tt.reason) {
					found = true
				}
			}
			if !found {
				t.Fatalf("reason %q not found: %+v", tt.reason, fa.Calls)
			}
		})
	}
}

func TestResolutionUncertainImportGraph(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		reason string
	}{
		{"conflict", map[string]string{"a.ts": `export function f(){}`, "b.ts": `export function f(){}`, "barrel.ts": `export * from './a';export * from './b';`}, "conflicting star exports cannot be resolved"},
		{"cycle", map[string]string{"barrel.ts": `export * from './b';`, "b.ts": `export * from './barrel';`}, "cyclic re-export cannot be resolved"},
		{"unreachable", map[string]string{"barrel.ts": `export {f} from './gone';`}, "import target is not an observed TypeScript declaration"},
		{"declaration-file", map[string]string{"barrel.ts": `export {f} from './decl.js';`, "decl.d.ts": `export declare function f():void;`}, "import reaches a .d.ts-only declaration the adapter skips"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tt.files["main.ts"] = `import {f} from './barrel';function main(){f()}`
			fa := resolutionFixture(t, tt.files, rule.FactResolvedCalls)["main.ts"]
			if len(fa.Calls) != 1 || fa.Calls[0].Target != uncertain(tt.reason) {
				t.Fatalf("calls: %+v", fa.Calls)
			}
		})
	}
}

func TestResolutionExternalAndShadowing(t *testing.T) {
	files := map[string]string{"package.json": `{"dependencies":{"pkg":"1"}}`, "main.ts": `import {send as s, Client} from 'pkg';import * as fs from 'node:fs';class R {client:Client;run(){s();fs.readFile();console.log();Promise.resolve();}} function shadow(console:unknown){console.log()} `}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls, rule.FactFieldTypes)["main.ts"]
	want := []conformance.Reference{external("pkg", "send"), external("node:fs", "readFile"), {Resolution: conformance.ResolutionExternal, Owner: "Console", Name: "log"}, {Resolution: conformance.ResolutionExternal, Owner: "PromiseConstructor", Name: "resolve"}, uncertain("unknown has no established declaration")}
	for i, c := range fa.Calls {
		if c.Target != want[i] {
			t.Errorf("%d: %+v want %+v", i, c, want[i])
		}
	}
	for _, d := range fa.Declarations {
		if d.Kind == "field" && !reflect.DeepEqual(d.Holds, []conformance.Reference{external("pkg", "Client")}) {
			t.Errorf("holds %+v", d)
		}
	}
}

func TestResolutionNotRequested(t *testing.T) {
	files := map[string]string{"main.ts": `class R{x:string;constructor(private p: R){} m(){this.m()}}`}
	for _, requested := range [][]rule.Fact{nil, {rule.FactDeclarations}, {rule.FactCalls}, {rule.FactDeclarations, rule.FactCalls}} {
		fa := resolutionFixture(t, files, requested...)["main.ts"]
		if fa.ResolvedCallsAvailable || fa.FieldTypesAvailable {
			t.Fatalf("unrequested availability: %+v", fa)
		}
		for _, c := range fa.Calls {
			if !c.Target.IsZero() || !c.Receiver.IsZero() || c.EnclosingOwner != "R" {
				t.Fatalf("call: %+v", c)
			}
		}
		for _, d := range fa.Declarations {
			if len(d.Holds) > 0 {
				t.Fatal("unrequested holds")
			}
		}
	}
}

func TestResolutionFieldUncertainty(t *testing.T) {
	files := map[string]string{"main.ts": `class R { inferred = make(); any:any; unknown:unknown; conditional: string extends number ? R : string; mapped: {[K in keyof R]: string}; indexed: R['any']; }`}
	fa := resolutionFixture(t, files, rule.FactFieldTypes)["main.ts"]
	want := map[string]string{"inferred": "field has no written type annotation", "any": "any has no established declaration", "unknown": "unknown has no established declaration", "conditional": "conditional type is not resolved", "mapped": "mapped type is not resolved", "indexed": "indexed-access type is not resolved"}
	for _, d := range fa.Declarations {
		if d.Kind == "field" && !reflect.DeepEqual(d.Holds, []conformance.Reference{uncertain(want[d.Name])}) {
			t.Errorf("field %+v expected %q", d, want[d.Name])
		}
	}
}

// Real-repository regressions: a barrel-exported instance is not a class
// constructor, and its inherited method belongs to the base declaration.
func TestResolutionImportedInstances(t *testing.T) {
	files := map[string]string{
		"base.ts":       `export class Base { execute(){} }`,
		"controller.ts": `import {Base} from './base';export class Controller extends Base {}`,
		"barrel.ts":     `import {Controller} from './controller';export const controller = new Controller();export const inferred = make();`,
		"main.ts":       `import {controller, inferred} from './barrel';controller.execute();inferred.m();`,
	}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls)["main.ts"]
	if fa.Calls[0].Target != declared("base.ts", "Base", "execute") || fa.Calls[0].Receiver != declared("controller.ts", "", "Controller") {
		t.Fatalf("imported instance: %+v", fa.Calls[0])
	}
	if fa.Calls[1].Target != uncertain("binding has no written annotation or resolvable new initializer") {
		t.Fatalf("imported inference: %+v", fa.Calls[1])
	}
}

func TestResolutionLexicalBindings(t *testing.T) {
	files := map[string]string{"main.ts": `class R{m(){}}class Other{m(){}}function run(p:R){ {let p:Other;p.m()} p.m(); if(true){var q:Other}q.m(); function inner(p:Other){p.m()} }`}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls)["main.ts"]
	owners := []string{"Other", "R", "Other", "Other"}
	for i, c := range fa.Calls {
		if c.Target != declared("main.ts", owners[i], "m") || c.Receiver != declared("main.ts", "", owners[i]) || c.EnclosingOwner != "" {
			t.Errorf("binding %d: %+v", i, c)
		}
	}
}

func TestResolutionOptionalKnownReceiver(t *testing.T) {
	fa := resolutionFixture(t, map[string]string{"main.ts": `class R{m(){}}function f(p:R){p?.m?.()}`}, rule.FactResolvedCalls)["main.ts"]
	if len(fa.Calls) != 1 || fa.Calls[0].Target != declared("main.ts", "R", "m") {
		t.Fatalf("calls: %+v", fa.Calls)
	}
}

func TestResolutionBuiltinDeclarations(t *testing.T) {
	fa := resolutionFixture(t, map[string]string{"main.ts": `function f(s:string){console.log();Object.freeze({});Promise.resolve();s.split(' ')}`}, rule.FactResolvedCalls)["main.ts"]
	owners := []string{"Console", "ObjectConstructor", "PromiseConstructor", "String"}
	for i, c := range fa.Calls {
		want := external("", c.Callee)
		want.Owner = owners[i]
		if c.Target != want {
			t.Errorf("builtin %d: %+v", i, c)
		}
	}
	if fa.Calls[3].Receiver != external("", "string") {
		t.Fatalf("primitive receiver: %+v", fa.Calls[3])
	}
}

func TestResolutionAmbientModuleDoesNotPoisonOtherFiles(t *testing.T) {
	files := map[string]string{"ambient.ts": `declare module "changed" { export function f():void; }`, "package.json": `{"dependencies":{"pkg":"1","changed":"1"}}`, "main.ts": `import {f} from 'changed';import {g} from 'pkg';export function local(){};local();g();f();`}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls)["main.ts"]
	wants := []conformance.Reference{declared("main.ts", "", "local"), external("pkg", "g"), uncertain("ambient modules or namespaces require declaration merging")}
	for i, c := range fa.Calls {
		if c.Target != wants[i] {
			t.Errorf("call %d: %+v", i, c)
		}
	}
}

func TestResolutionNamespaceClassesAndInstances(t *testing.T) {
	files := map[string]string{"repo.ts": `export class Base{m(){}static open(){}}export class R extends Base{}export const r=new R();export const fn=make();`, "main.ts": `import * as ns from './repo';ns.R.open();ns.r.m();ns.fn();class C{static r:ns.R;static f(){this.r.m();this.f()}}`}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls)["main.ts"]
	want := []conformance.Reference{declared("repo.ts", "Base", "open"), declared("repo.ts", "Base", "m"), uncertain("callee has no declared function implementation"), declared("repo.ts", "Base", "m"), declared("main.ts", "C", "f")}
	for i, c := range fa.Calls {
		if c.Target != want[i] {
			t.Errorf("namespace call %d: %+v want %+v", i, c, want[i])
		}
	}
}

func TestResolutionInferredFieldIsUndetermined(t *testing.T) {
	fa := resolutionFixture(t, map[string]string{"main.ts": `class R{m(){}}class C{r=new R();f(){this.r.m()}}`}, rule.FactResolvedCalls)["main.ts"]
	if fa.Calls[0].Target != uncertain("field has no written type annotation") {
		t.Fatalf("inferred field: %+v", fa.Calls)
	}
}

func TestResolutionTypeOnlyImports(t *testing.T) {
	files := map[string]string{"repo.ts": `export class R{static m(){}}export function f(){}`, "main.ts": `import type {R,f} from './repo';R.m();f();class C{r:R;}`}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls, rule.FactFieldTypes)["main.ts"]
	for _, c := range fa.Calls {
		if c.Target != uncertain("type-only import cannot be used as a value") {
			t.Fatalf("call %+v", c)
		}
	}
	for _, d := range fa.Declarations {
		if d.Kind == "field" && !reflect.DeepEqual(d.Holds, []conformance.Reference{declared("repo.ts", "", "R")}) {
			t.Fatalf("type-only field: %+v", d)
		}
	}
}

func TestResolutionAnonymousDefaults(t *testing.T) {
	for _, source := range []string{`export default function(){}`, `export default () => {}`} {
		fa := resolutionFixture(t, map[string]string{"original.ts": source, "main.ts": `import f from './original';f()`}, rule.FactResolvedCalls, rule.FactDeclarations)["main.ts"]
		if fa.Calls[0].Target != declared("original.ts", "", "default") {
			t.Fatalf("anonymous default %s: %+v", source, fa.Calls)
		}
	}
	fa := resolutionFixture(t, map[string]string{"original.ts": `export default class {static open(){}}`, "main.ts": `import C from './original';C.open()`}, rule.FactResolvedCalls)["main.ts"]
	if fa.Calls[0].Target != declared("original.ts", "default", "open") {
		t.Fatalf("anonymous class: %+v", fa.Calls)
	}
}

func TestResolutionNamespaceExportDoesNotBecomeStarExport(t *testing.T) {
	files := map[string]string{"original.ts": `export function f(){}`, "barrel.ts": `export * as ns from './original';`, "main.ts": `import {f,ns} from './barrel';f();ns.f()`}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls)["main.ts"]
	for _, c := range fa.Calls {
		if c.Target.Resolution != conformance.ResolutionUndetermined || c.Target.Reason == "" {
			t.Fatalf("namespace export falsely resolved: %+v", c)
		}
	}
}

func TestResolutionBuiltinMembersComeFromCompilerDeclarations(t *testing.T) {
	fa := resolutionFixture(t, map[string]string{"main.ts": `class C{static call(){}static f(){}}C.call();C.bind(null);function run(s:string){s.trim();s.noSuchMember();console.noSuchMember()}`}, rule.FactResolvedCalls)["main.ts"]
	if fa.Calls[0].Target != declared("main.ts", "C", "call") {
		t.Fatalf("own static member: %+v", fa.Calls[0])
	}
	bind := external("", "bind")
	bind.Owner = "Function"
	if fa.Calls[1].Target != bind {
		t.Fatalf("constructor prototype: %+v", fa.Calls[1])
	}
	for _, c := range fa.Calls[3:] {
		if c.Target.Resolution != conformance.ResolutionUndetermined || c.Target.Reason == "" || c.Receiver.Resolution != conformance.ResolutionExternal {
			t.Fatalf("invented builtin: %+v", c)
		}
	}
}

func TestResolutionOptionalFieldValues(t *testing.T) {
	files := map[string]string{"main.ts": `class R{}interface I{r?:R;callback?:()=>R}class C{constructor(private readonly r?:R){}}`}
	fa := resolutionFixture(t, files, rule.FactFieldTypes)["main.ts"]
	for _, d := range fa.Declarations {
		if d.Kind != "field" {
			continue
		}
		want := []conformance.Reference{declared("main.ts", "", "R"), external("", "undefined")}
		if d.Name == "callback" {
			want = []conformance.Reference{external("", "undefined")}
		}
		if !reflect.DeepEqual(d.Holds, want) {
			t.Fatalf("optional field %+v want %+v", d, want)
		}
	}
}

func TestResolutionDefaultNewParameterAndFieldInitializer(t *testing.T) {
	files := map[string]string{"main.ts": `class R{m(){}}function run(p=new R()){p.m()}class C{r:R;result=this.r.m();f(){function nested(){this.r.m()}}}`}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls)["main.ts"]
	for _, c := range fa.Calls[:2] {
		if c.Target != declared("main.ts", "R", "m") || c.EnclosingOwner != "" {
			t.Fatalf("lexical receiver: %+v", c)
		}
	}
	if fa.Calls[2].Target != uncertain("this has no enclosing declared class") || fa.Calls[2].EnclosingOwner != "" {
		t.Fatalf("nested this: %+v", fa.Calls[2])
	}
}

func TestResolutionScriptAndSkippedDeclarationMerging(t *testing.T) {
	tests := []struct {
		name   string
		files  map[string]string
		reason string
	}{
		{"scripts", map[string]string{"first.ts": `interface R{m():void}`, "second.ts": `interface R{x:string}function f(r:R){r.m()}`}, "global declaration merging is not resolved"},
		{"skipped-global", map[string]string{"global.d.ts": `interface R{x:string}`, "second.ts": `interface R{m():void}function f(r:R){r.m()}`}, ".d.ts-only global declaration or merging is not resolved"},
		{"augmentation", map[string]string{"augment.d.ts": `import './repo';declare module './repo'{interface R{x:string}}`, "repo.ts": `export class R{m(){}}`, "second.ts": `import {R} from './repo.js';function f(r:R){r.m()}`}, "ambient module augmentation requires declaration merging"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fa := resolutionFixture(t, tt.files, rule.FactResolvedCalls)["second.ts"]
			if fa.Calls[0].Target != uncertain(tt.reason) {
				t.Fatalf("merge: %+v", fa.Calls)
			}
		})
	}
}

func TestResolutionModuleDeclarationsStayIsolated(t *testing.T) {
	files := map[string]string{"first.ts": `export interface R{m():void}`, "second.ts": `export interface R{m():void}export function f(r:R){r.m()}`}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls)["second.ts"]
	if fa.Calls[0].Target != declared("second.ts", "R", "m") {
		t.Fatalf("module isolation: %+v", fa.Calls)
	}
}

func TestResolutionBuiltinMergingAndUnsupportedMembers(t *testing.T) {
	files := map[string]string{"global.d.ts": `interface String{added():void}`, "main.ts": `function f(s:string){s.trim()}interface Window{m():void}function g(w:Window){w.m()}`}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls)["main.ts"]
	if fa.Calls[0].Target != uncertain(".d.ts-only global declaration or merging is not resolved") {
		t.Fatalf("builtin merge: %+v", fa.Calls[0])
	}
	if fa.Calls[1].Target != uncertain("global declaration merging is not resolved") {
		t.Fatalf("default-library merge: %+v", fa.Calls[1])
	}
	fa = resolutionFixture(t, map[string]string{"main.ts": `enum E{A}function f(e:E){e.toString()}class R{n:1n}`}, rule.FactResolvedCalls, rule.FactFieldTypes)["main.ts"]
	if fa.Calls[0].Target != uncertain("receiver declaration has no class or interface member model") {
		t.Fatalf("enum receiver: %+v", fa.Calls[0])
	}
	for _, d := range fa.Declarations {
		if d.Kind == "field" && !reflect.DeepEqual(d.Holds, []conformance.Reference{external("", "bigint")}) {
			t.Fatalf("bigint literal: %+v", d)
		}
	}
}

func TestResolutionLexicalClassNamesDoNotLeak(t *testing.T) {
	files := map[string]string{"main.ts": `const C=class Hidden{repo:R;m(){this.repo.m()}};class R{m(){}}Hidden.m();`}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls, rule.FactFieldTypes)["main.ts"]
	if fa.Calls[0].Target != declared("main.ts", "R", "m") {
		t.Fatalf("class expression this: %+v", fa.Calls[0])
	}
	if fa.Calls[1].Target != uncertain("Hidden is neither declared in the file nor imported") {
		t.Fatalf("class expression leaked: %+v", fa.Calls[1])
	}
	for _, d := range fa.Declarations {
		if d.Kind == "field" && !reflect.DeepEqual(d.Holds, []conformance.Reference{declared("main.ts", "", "R")}) {
			t.Fatalf("class expression field: %+v", d)
		}
	}
}

func TestResolutionNamespaceBasesAndShadowedThis(t *testing.T) {
	files := map[string]string{"base.ts": `export class Base{m(){}}`, "main.ts": `import * as ns from './base';class R extends ns.Base{repo:ns.Base;f<R>(){this.repo.m();this.m()}}`}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls)["main.ts"]
	for _, c := range fa.Calls {
		if c.Target != declared("base.ts", "Base", "m") {
			t.Fatalf("namespace base: %+v", c)
		}
	}
}

func TestResolutionValueUsedAsAnnotationIsUndetermined(t *testing.T) {
	fa := resolutionFixture(t, map[string]string{"main.ts": `const NotType=make();class R{field:NotType}`}, rule.FactFieldTypes)["main.ts"]
	for _, d := range fa.Declarations {
		if d.Kind == "field" && !reflect.DeepEqual(d.Holds, []conformance.Reference{uncertain("type name NotType does not reach an indexed type declaration")}) {
			t.Fatalf("value used as type: %+v", d)
		}
	}
}

func TestResolutionReadonlyArraysAndComputedBuiltinTypes(t *testing.T) {
	files := map[string]string{"main.ts": `class R{rows:readonly R[];tuple:readonly [R,string];computed:ReturnType<()=>R>;f(p:readonly R[]){p.map(x=>x)}}`}
	fa := resolutionFixture(t, files, rule.FactResolvedCalls, rule.FactFieldTypes)["main.ts"]
	target := external("", "map")
	target.Owner = "ReadonlyArray"
	if fa.Calls[0].Target != target || fa.Calls[0].Receiver != external("", "ReadonlyArray") {
		t.Fatalf("readonly receiver: %+v", fa.Calls[0])
	}
	for _, d := range fa.Declarations {
		if d.Kind != "field" {
			continue
		}
		var want []conformance.Reference
		switch d.Name {
		case "rows":
			want = []conformance.Reference{external("", "ReadonlyArray"), declared("main.ts", "", "R")}
		case "tuple":
			want = []conformance.Reference{external("", "ReadonlyArray"), declared("main.ts", "", "R"), external("", "string")}
		case "computed":
			want = []conformance.Reference{external("", "ReturnType"), uncertain("builtin type ReturnType requires conditional type computation")}
		}
		if !reflect.DeepEqual(d.Holds, want) {
			t.Fatalf("field %+v want %+v", d, want)
		}
	}
}

func TestResolutionBuiltinConstructors(t *testing.T) {
	fa := resolutionFixture(t, map[string]string{"main.ts": `function f(){const s=new String('x');s.trim();const x=new Proxy({},{});x.toString();const r=new ReadonlyArray();r.map(x=>x)}`}, rule.FactResolvedCalls)["main.ts"]
	if fa.Calls[0].Receiver != external("", "String") {
		t.Fatalf("wrapper receiver: %+v", fa.Calls[0])
	}
	if fa.Calls[1].Target != uncertain("builtin constructor Proxy has no single named instance type") {
		t.Fatalf("any constructor: %+v", fa.Calls[1])
	}
	if fa.Calls[2].Target != uncertain("new target is not a resolvable class") {
		t.Fatalf("non-value constructor: %+v", fa.Calls[2])
	}
}

// A decorator runs when its class is defined, so its calls belong to
// the scope around the class; a parameter default runs on every call,
// so its calls belong to the method.
func TestDecoratorCallsRunWhereTheClassIsDefined(t *testing.T) {
	fa := resolutionFixture(t, map[string]string{"order.ts": `declare const repo: any;
@repo.entity()
class Order {
  @repo.track()
  place(@repo.inject() x = repo.load()) { repo.save(); }
  [repo.key()]() {}
}
function build() {
  class Inner { @repo.mark() run() {} }
}
`}, rule.FactResolvedCalls)["order.ts"]
	var got []string
	for _, c := range fa.Calls {
		got = append(got, c.Callee+"@"+c.EnclosingOwner+"."+c.Enclosing)
	}
	want := []string{"entity@.", "track@.", "inject@.", "load@Order.place", "save@Order.place", "key@.", "mark@.build"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("calls %v, want %v", got, want)
	}
}

// A decorator is resolved where it runs, so a parameter of the method
// it decorates does not shadow the function it calls.
func TestDecoratorResolvesOutsideTheMethod(t *testing.T) {
	fa := resolutionFixture(t, map[string]string{"order.ts": `function decorate() { return (..._: unknown[]) => {}; }
class Order {
  @decorate()
  place(decorate: () => void) { decorate(); }
}
`}, rule.FactResolvedCalls)["order.ts"]
	if len(fa.Calls) != 2 {
		t.Fatalf("calls %+v", fa.Calls)
	}
	if got := fa.Calls[0].Target; got != declared("order.ts", "", "decorate") {
		t.Errorf("decorator call target %+v, want the outer function", got)
	}
	if got := fa.Calls[1].Target; got.Resolution != conformance.ResolutionUndetermined {
		t.Errorf("parameter call target %+v, want undetermined: the parameter holds a function value", got)
	}
}

// A method of an object literal belongs to no class: inside a method it
// is part of that method, as an arrow function is.
func TestObjectLiteralMethodsHaveNoClassOwner(t *testing.T) {
	fa := resolutionFixture(t, map[string]string{"order.ts": `declare const repo: any;
class Order {
  handlers = { reset() { repo.clear(); } };
  place() { return { run() { repo.load(); }, go: () => repo.save() }; }
  later() { return [function () { repo.flush(); }, function named() { repo.sync(); }]; }
}
const api = { start() { repo.open(); } };
`}, rule.FactResolvedCalls)["order.ts"]
	var got []string
	for _, c := range fa.Calls {
		got = append(got, c.Callee+"@"+c.EnclosingOwner+"."+c.Enclosing)
	}
	want := []string{"clear@.reset", "load@Order.place", "save@Order.place", "flush@Order.later", "sync@.named", "open@.start"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("calls %v, want %v", got, want)
	}
}
