package typescript

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// spellImplementations spells every implementation of a file's classes
// as Class -> Abstraction [evidence] with each operation reached.
func spellImplementations(f conformance.LanguageFacts) []string {
	var out []string
	for _, d := range f.Declarations {
		for _, impl := range d.Implements {
			var ops []string
			for _, op := range impl.Operations {
				if op.Resolution == conformance.ResolutionDeclared {
					ops = append(ops, fmt.Sprintf("%s.%s@%s", op.Owner, op.Name, op.File))
				} else {
					ops = append(ops, fmt.Sprintf("%s:%s(%s)", op.Name, op.Resolution, op.Reason))
				}
			}
			abs := impl.Abstraction.Name + "@" + impl.Abstraction.File
			if impl.Abstraction.Resolution == conformance.ResolutionUndetermined {
				abs = impl.Abstraction.Name + "?(" + impl.Abstraction.Reason + ")"
			}
			out = append(out, fmt.Sprintf("%s -> %s [%s] %s", d.Name, abs, impl.Evidence, strings.Join(ops, " ")))
		}
	}
	sort.Strings(out)
	return out
}

// An implements or extends clause names a class's abstractions exactly,
// followed through base classes and through interfaces that extend
// interfaces, and each operation is the member the class's own lookup
// reaches, inherited ones included. A class that only has the methods
// of an interface is a heuristic implementation of it, and a clause
// resolution cannot follow is undetermined with the name it writes.
func TestImplementationsFollowTheClauses(t *testing.T) {
	sources := map[string]string{
		"src/user/password.ts": `export class EncryptedPassword { constructor(readonly value: string) {} }
export interface Matcher { matches(p: string, e: EncryptedPassword): boolean; }
export interface Encrypter extends Matcher { encrypt(p: string): Promise<EncryptedPassword>; }
export abstract class Hasher { abstract hash(p: string): string; describe(): string { return "hasher"; } }
export interface Named { label: string; }
`,
		"src/adapters/encrypters.ts": `import { EncryptedPassword, type Encrypter, Hasher } from "../user/password.js";
import type { Missing } from "./missing.js";
import type { Iterable as Ext } from "node:stream";

export class Pbkdf2Encrypter implements Encrypter {
  async encrypt(p: string): Promise<EncryptedPassword> { return new EncryptedPassword(p); }
  matches(p: string, e: EncryptedPassword): boolean { return false; }
}

export class Salted extends Pbkdf2Encrypter {
  override async encrypt(p: string): Promise<EncryptedPassword> { return new EncryptedPassword(p + "salt"); }
}

export class Sha extends Hasher { hash(p: string): string { return p; } }

export class LooksLikeOne {
  async encrypt(p: string): Promise<EncryptedPassword> { return new EncryptedPassword(p); }
  matches(p: string, e: EncryptedPassword): boolean { return true; }
}

export class Unknown implements Missing {}

export class Library implements Ext {}
`,
	}
	got := spellImplementations(resolutionFixture(t, sources, rule.FactDeclarations, rule.FactImplementations)["src/adapters/encrypters.ts"])
	want := []string{
		"LooksLikeOne -> Encrypter@src/user/password.ts [heuristic] LooksLikeOne.encrypt@src/adapters/encrypters.ts LooksLikeOne.matches@src/adapters/encrypters.ts",
		"LooksLikeOne -> Matcher@src/user/password.ts [heuristic] LooksLikeOne.matches@src/adapters/encrypters.ts",
		"Pbkdf2Encrypter -> Encrypter@src/user/password.ts [exact] Pbkdf2Encrypter.encrypt@src/adapters/encrypters.ts Pbkdf2Encrypter.matches@src/adapters/encrypters.ts",
		"Pbkdf2Encrypter -> Matcher@src/user/password.ts [exact] Pbkdf2Encrypter.matches@src/adapters/encrypters.ts",
		"Salted -> Encrypter@src/user/password.ts [exact] Salted.encrypt@src/adapters/encrypters.ts Pbkdf2Encrypter.matches@src/adapters/encrypters.ts",
		"Salted -> Matcher@src/user/password.ts [exact] Pbkdf2Encrypter.matches@src/adapters/encrypters.ts",
		"Sha -> Hasher@src/user/password.ts [exact] Sha.hash@src/adapters/encrypters.ts Hasher.describe@src/user/password.ts",
		"Unknown -> Missing?(import target is not an observed TypeScript declaration) [undetermined] ",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("implementations:\n got %s\nwant %s", strings.Join(got, "\n     "), strings.Join(want, "\n     "))
	}
}

// Implementations are read only when asked for.
func TestImplementationsNotRequested(t *testing.T) {
	sources := map[string]string{
		"a.ts": `export interface Port { run(): void } export class Adapter implements Port { run() {} }`,
	}
	f := resolutionFixture(t, sources, rule.FactDeclarations, rule.FactFieldTypes)["a.ts"]
	if f.ImplementationsAvailable || len(spellImplementations(f)) != 0 {
		t.Errorf("implementations supplied without being requested: %+v", f)
	}
	f = resolutionFixture(t, sources, rule.FactDeclarations, rule.FactImplementations)["a.ts"]
	if !f.Supports(rule.FactImplementations) || len(spellImplementations(f)) != 1 {
		t.Errorf("implementations not supplied: %q", spellImplementations(f))
	}
}

// A clause naming a type alias of an interface names the interface, and
// a static method is marked, so it is never taken for the instance
// operation of the same name.
func TestImplementationsFollowAliasesAndMarkStatics(t *testing.T) {
	sources := map[string]string{
		"src/port.ts": `export interface Encrypter { encrypt(s: string): string; }
export type Alias = Encrypter;
export type Again = Alias;
`,
		"src/adapter.ts": `import { type Again } from "./port.js";

export class Adapter implements Again {
  encrypt(s: string): string { return s; }
  static encrypt(s: string): string { return s; }
}
`,
	}
	f := resolutionFixture(t, sources, rule.FactDeclarations, rule.FactImplementations)["src/adapter.ts"]
	got := spellImplementations(f)
	want := []string{"Adapter -> Encrypter@src/port.ts [exact] Adapter.encrypt@src/adapter.ts"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("implementations:\n got %q\nwant %q", got, want)
	}
	var statics []bool
	for _, d := range f.Declarations {
		if d.Kind == "method" && d.Name == "encrypt" {
			statics = append(statics, d.Static)
		}
	}
	if len(statics) != 2 || statics[0] || !statics[1] {
		t.Errorf("static marks of encrypt: %v, want [false true]", statics)
	}
}

// Each interface an inheritance graph reaches is visited once, so a
// graph whose paths multiply at every level is read in time linear in
// its edges.
func TestImplementationsVisitEachBaseOnce(t *testing.T) {
	var b strings.Builder
	b.WriteString("export interface I0 { run(): void; }\nexport interface I1 extends I0 {}\n")
	const depth = 40
	for i := 2; i <= depth; i++ {
		fmt.Fprintf(&b, "export interface I%d extends I%d, I%d {}\n", i, i-1, i-2)
	}
	fmt.Fprintf(&b, "export class Runner implements I%d { run(): void {} }\n", depth)
	done := make(chan []string, 1)
	go func() {
		done <- spellImplementations(resolutionFixture(t, map[string]string{"src/graph.ts": b.String()}, rule.FactDeclarations, rule.FactImplementations)["src/graph.ts"])
	}()
	select {
	case got := <-done:
		if len(got) != depth+1 {
			t.Errorf("Runner implements %d interfaces, want %d: %q", len(got), depth+1, got)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("reading the inheritance graph did not finish in 20s")
	}
}
