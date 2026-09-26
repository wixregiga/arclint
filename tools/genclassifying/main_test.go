package main

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const repoRoot = "../.."

// The committed page must be what the generator writes from the committed
// meta-model, questions, and examples; edit those, run go generate, and
// commit the page with them. The page is never edited by hand.
func TestGeneratedPageIsCurrent(t *testing.T) {
	want, err := os.ReadFile(repoRoot + "/" + pagePath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := render(os.DirFS(repoRoot))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale: run `go generate ./tools/genclassifying/`; never edit it by hand", pagePath)
	}
}

func TestScanQuotesTheRegionBetweenMarkers(t *testing.T) {
	s := &snippets{regions: map[string]snippet{}, used: map[string]bool{}}
	src := strings.Join([]string{
		"package tenant",
		"\t// ANCHOR: outer",
		"",
		"\tfunc A() {",
		"\t\t// ANCHOR: inner",
		"\t\tb()   ",
		"\t\t// ANCHOR_END: inner",
		"\t}",
		"",
		"\t// ANCHOR_END: outer",
	}, "\n")
	if err := s.scan("docs/examples/go", "docs/examples/go/tenant/tenant.go", src); err != nil {
		t.Fatal(err)
	}
	outer, err := s.get("docs/examples/go", "outer")
	if err != nil {
		t.Fatal(err)
	}
	if want := "func A() {\n\tb()\n}"; outer.text != want {
		t.Errorf("outer = %q, want %q", outer.text, want)
	}
	if outer.file != "tenant/tenant.go" {
		t.Errorf("outer.file = %q, want the path inside the example", outer.file)
	}
	if got := s.unused(); !slices.Equal(got, []string{"docs/examples/go#inner"}) {
		t.Errorf("unused = %v, want only the inner region", got)
	}
}

func TestScanRefusesBrokenMarkers(t *testing.T) {
	cases := map[string]string{
		"opened twice":                "# ANCHOR: a\nx\n# ANCHOR: a\ny\n# ANCHOR_END: a\n",
		"closed without being opened": "x\n# ANCHOR_END: a\n",
		"holds no text":               "# ANCHOR: a\n\n# ANCHOR_END: a\n",
		"is never closed":             "# ANCHOR: a\nx\n",
	}
	for want, src := range cases {
		t.Run(want, func(t *testing.T) {
			s := &snippets{regions: map[string]snippet{}, used: map[string]bool{}}
			err := s.scan("docs/examples/go", "docs/examples/go/domain.arclint.yaml", src)
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("scan error = %v, want one saying %q", err, want)
			}
		})
	}
}

func TestScanRefusesOneAnchorInTwoFilesOfAnExample(t *testing.T) {
	s := &snippets{regions: map[string]snippet{}, used: map[string]bool{}}
	src := "// ANCHOR: a\nx\n// ANCHOR_END: a\n"
	if err := s.scan("docs/examples/go", "docs/examples/go/a.go", src); err != nil {
		t.Fatal(err)
	}
	if err := s.scan("docs/examples/go", "docs/examples/go/b.go", src); err == nil {
		t.Fatal("scan accepted anchor a in a second file of the same example")
	}
	if err := s.scan("docs/examples/ts", "docs/examples/ts/a.ts", src); err != nil {
		t.Fatalf("the same anchor in another example is its own region: %v", err)
	}
}

func TestDedentKeepsRelativeIndentation(t *testing.T) {
	got := dedent([]string{"", "    if x {", "", "      y()", "    }", "  "})
	if want := "if x {\n\n  y()\n}"; got != want {
		t.Errorf("dedent = %q, want %q", got, want)
	}
	if got := dedent([]string{"", " \t"}); got != "" {
		t.Errorf("dedent of blank lines = %q, want empty", got)
	}
}

func TestDecidedNamesOnlyTerms(t *testing.T) {
	p := &page{m: vocab.DDD()}
	cases := map[string][]string{
		"entity vs value_object":                        {"entity", "value_object"},
		"assertion owner (aggregate vs domain_service)": {"aggregate", "domain_service"},
		"specification vs invariant or assertion":       {"specification", "invariant", "assertion"},
		"entity vs value_object vs domain_event":        {"entity", "value_object", "domain_event"},
		"value integrity vs cluster invariant":          nil,
		"domain contract vs programming-only guard":     nil,
		"bounded_context assignment":                    {"bounded_context"},
	}
	for decides, want := range cases {
		var got []string
		for _, b := range p.decided(decides) {
			got = append(got, b.Term)
		}
		if !slices.Equal(got, want) {
			t.Errorf("decided(%q) = %v, want %v", decides, got, want)
		}
	}
}

func TestSentenceCase(t *testing.T) {
	for in, want := range map[string]string{
		"Business Rule":  "Business rule",
		"Domain Service": "Domain service",
		"Invariant":      "Invariant",
	} {
		if got := sentenceCase(in); got != want {
			t.Errorf("sentenceCase(%q) = %q, want %q", in, got, want)
		}
	}
}
