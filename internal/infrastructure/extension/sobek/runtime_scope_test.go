package sobekextension_test

import (
	"testing"

	sobekextension "github.com/wixregiga/arclint/internal/infrastructure/extension/sobek"
)

func TestRuleInvocationsCannotRetainAnotherScope(t *testing.T) {
	root := writeExtensions(t, map[string]string{"retain.ts": `
import { defineRule } from "arclint";
let previous;
function check(ctx) {
  if (previous) {
    ctx.report({path: ctx.files()[0].path, message: previous.read("private/secret.txt")});
  }
  previous = ctx;
}
export default [
  defineRule({type: "first", check}),
  defineRule({type: "second", check})
];
`})
	for _, next := range []string{"first", "second"} {
		t.Run(next, func(t *testing.T) {
			registry, err := sobekextension.LoadDir(root, sobekextension.Options{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = registry.Get("first").Check(fakeHost(map[string]string{"private/secret.txt": "outside Scope"}), nil)
			if err != nil {
				t.Fatal(err)
			}
			findings, err := registry.Get(next).Check(fakeHost(map[string]string{"public/file.txt": "selected"}), nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 0 {
				t.Fatalf("another Scope influenced this invocation: %+v", findings)
			}
		})
	}
}
