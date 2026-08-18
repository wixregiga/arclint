package cli

import (
	"fmt"
	"io"
)

// printer latches the first write error of a multi-write renderer:
// once a write fails, every later call is a no-op and err keeps the
// original failure for the caller to return. The rendered report is
// the product — a truncated report (closed pipe, full disk on
// redirect) must surface as a command error, never as exit 0.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) printf(format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, args...)
}

func (p *printer) println(args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintln(p.w, args...)
}
