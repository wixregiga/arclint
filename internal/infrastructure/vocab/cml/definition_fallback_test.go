package cmlvocab

import "testing"

// A term without a doc string takes its definition from its leading
// comment, but an ArcLint directive in that comment is carrier metadata,
// never part of the definition.
func TestDefinitionFallbackLeavesDirectivesOut(t *testing.T) {
	for _, tc := range []struct{ name, comment, want string }{
		{"a directive after the prose", "// A monetary amount.\n// @arclint name \"money value\"", "A monetary amount."},
		{"a directive before the prose", "// @arclint question \"who-sets-it\" \"Who sets it?\"\n// A monetary amount.", "A monetary amount."},
		{"only a directive", "// @arclint name \"money value\"", ""},
		{"a block comment", "/* A monetary amount.\n   Never negative. */", "A monetary amount.\nNever negative."},
	} {
		if got := definition("", tc.comment); got != tc.want {
			t.Errorf("%s: definition = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := definition("The doc string.", "// @arclint name \"x\""); got != "The doc string." {
		t.Errorf("a doc string = %q, want it kept", got)
	}
}
