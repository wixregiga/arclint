// Package out provides a first-error latching writer used by report adapters.
package out

import (
	"fmt"
	"io"
)

// Printer latches the first write error of a multi-write renderer: once a
// write fails, every later call is a no-op and Err keeps the original
// failure for the caller to return.
type Printer struct {
	W   io.Writer
	Err error
}

// Printf formats and writes, latching the first error including short writes
// that return n < len with a nil error.
func (p *Printer) Printf(format string, args ...any) {
	if p.Err != nil {
		return
	}
	p.Err = WriteString(p.W, fmt.Sprintf(format, args...))
}

// Println writes a line, latching the first error including short writes
// that return n < len with a nil error.
func (p *Printer) Println(args ...any) {
	if p.Err != nil {
		return
	}
	p.Err = WriteString(p.W, fmt.Sprintln(args...))
}

// WriteFull writes b entirely or returns the first error / io.ErrShortWrite.
func WriteFull(w io.Writer, b []byte) error {
	n, err := w.Write(b)
	if err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	if n < len(b) {
		return io.ErrShortWrite
	}
	return nil
}

// WriteString writes s entirely or returns the first error / io.ErrShortWrite.
func WriteString(w io.Writer, s string) error {
	n, err := io.WriteString(w, s)
	if err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	if n < len(s) {
		return io.ErrShortWrite
	}
	return nil
}
