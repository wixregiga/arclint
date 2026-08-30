package out

import (
	"errors"
	"testing"
)

type failingWriter struct {
	calls int
	err   error
}

func (w *failingWriter) Write([]byte) (int, error) {
	w.calls++
	return 0, w.err
}

func TestPrinterLatchesFirstWriteError(t *testing.T) {
	writeErr := errors.New("write failed")
	writer := &failingWriter{err: writeErr}
	printer := &Printer{W: writer}

	printer.Printf("first")
	printer.Println("second")

	if !errors.Is(printer.Err, writeErr) {
		t.Fatalf("error = %v, want first write error", printer.Err)
	}
	if writer.calls != 1 {
		t.Fatalf("write calls = %d, want 1", writer.calls)
	}
}
