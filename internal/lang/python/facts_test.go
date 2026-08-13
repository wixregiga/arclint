package python

import (
	"reflect"
	"testing"

	"github.com/wixregiga/arclint/internal/lang"
)

func TestPythonFacts(t *testing.T) {
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
	got := Facts("src/domain/member.py", src)
	if got.ParseError != "" {
		t.Fatal(got.ParseError)
	}
	want := []lang.Decl{
		{Kind: "class", Name: "Member", Exported: true, StartLine: 3, EndLine: 12},
		{Kind: "method", Name: "loans", Owner: "Member", Exported: true, StartLine: 4, EndLine: 5},
		// Spans cover the definition node itself; decorator lines are
		// excluded by design (the walker recurses into the definition).
		{Kind: "method", Name: "blocked", Owner: "Member", Exported: true, StartLine: 8, EndLine: 9},
		{Kind: "method", Name: "_internal", Owner: "Member", StartLine: 11, EndLine: 12},
		{Kind: "class", Name: "_Private", StartLine: 14, EndLine: 15},
		{Kind: "func", Name: "top_level", Exported: true, StartLine: 17, EndLine: 20},
		{Kind: "func", Name: "nested", Owner: "top_level", Exported: true, StartLine: 18, EndLine: 19},
		{Kind: "func", Name: "fetch", Exported: true, StartLine: 22, EndLine: 23},
		{Kind: "func", Name: "conditional", Exported: true, StartLine: 26, EndLine: 27},
	}
	if !reflect.DeepEqual(got.Decls, want) {
		for i := 0; i < len(got.Decls) || i < len(want); i++ {
			var g, w lang.Decl
			if i < len(got.Decls) {
				g = got.Decls[i]
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
}
