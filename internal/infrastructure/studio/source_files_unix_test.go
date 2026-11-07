//go:build unix

package studio_test

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/infrastructure/studio"
)

// A named pipe is refused at once instead of waiting for a writer.
func TestSourceFilesRefuseAPipeWithoutWaiting(t *testing.T) {
	project := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(project, "pipe.go"), 0o600); err != nil {
		t.Skipf("no named pipes here: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := studio.SourceFiles{Project: project}.ReadSource("pipe.go", 1024)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a pipe was read")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reading a pipe blocked")
	}
}
