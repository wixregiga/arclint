package reportfactory_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/reportfactory"
)

func TestSelectKnown(t *testing.T) {
	for _, name := range []cli.RendererName{cli.RendererPlain, cli.RendererJSON, cli.RendererLipgloss} {
		r, err := reportfactory.Select(name)
		if err != nil {
			t.Fatalf("Select(%q): %v", name, err)
		}
		if r == nil {
			t.Fatalf("Select(%q): nil renderer", name)
		}
		var buf bytes.Buffer
		if err := r.Render(&buf, cli.InitReport{Path: "x"}); err != nil {
			t.Fatalf("Select(%q) Render: %v", name, err)
		}
		if !strings.Contains(buf.String(), "x") {
			t.Fatalf("Select(%q) output missing path: %q", name, buf.String())
		}
	}
}

func TestSelectUnknown(t *testing.T) {
	_, err := reportfactory.Select(cli.RendererName("tables"))
	if err == nil {
		t.Fatal("expected error")
	}
	want := `report adapter "tables": unknown`
	if err.Error() != want {
		t.Fatalf("err = %q, want %q", err.Error(), want)
	}
}

func TestSelectRenderersShortWrite(t *testing.T) {
	r, err := reportfactory.Select(cli.RendererPlain)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Render(&shortWriter{n: 1}, cli.InitReport{Path: "rules.arclint.yaml"}); err == nil {
		t.Fatal("expected short-write error")
	} else if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("err = %v, want ErrShortWrite", err)
	}
}

// shortWriter accepts at most n bytes then returns n < len with nil error.
type shortWriter struct{ n int }

func (s *shortWriter) Write(p []byte) (int, error) {
	if s.n <= 0 {
		return 0, nil
	}
	if len(p) > s.n {
		n := s.n
		s.n = 0
		return n, nil
	}
	s.n -= len(p)
	return len(p), nil
}
