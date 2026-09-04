package python

import (
	"reflect"
	"testing"
)

// TestExtractForms covers exactly the statement forms the research report
// (multi-language-rule-engines.md §4) commits to: import and from-import,
// legal at any indentation, with parenthesized multi-line and backslash
// continuation.
func TestExtractForms(t *testing.T) {
	src := `import os
import os.path as p
import first, second.sub as s
from x.y import z
from . import sibling
from ..parent import thing
from pkg import (
    m1,
    m2,
)
def f():
    import inside_function
    if True:
        from cond import q
import third, \
    fourth
`
	got := extract(src)
	var mods []string
	for _, ri := range got {
		mods = append(mods, ri.module)
	}
	want := []string{
		"os", "os.path", "first", "second.sub", "x.y", ".", "..parent",
		"pkg", "inside_function", "cond", "third", "fourth",
	}
	if !reflect.DeepEqual(mods, want) {
		t.Errorf("extracted %v\nwant %v", mods, want)
	}
	if got[0].line != 1 || got[7].line != 7 {
		t.Errorf("line anchors: first=%d pkg=%d", got[0].line, got[7].line)
	}
}

// TestDocumentedFalseNegatives: computed imports stay invisible at this
// tier, the documented contract.
func TestDocumentedFalseNegatives(t *testing.T) {
	src := `import importlib
mod = importlib.import_module("boto3")
dyn = __import__("requests")
s = "import fake_in_string"
# import commented
def g():
    """
    import fake_in_docstring
    from fake import thing
    """
    return s
t = '''
import fake_in_single_triple
'''
`
	got := extract(src)
	if len(got) != 1 || got[0].module != "importlib" {
		t.Errorf("extracted %+v, want only the literal `import importlib`", got)
	}
}
