package typescript

import (
	"fmt"
	"sort"
	"strings"
	"testing"

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
