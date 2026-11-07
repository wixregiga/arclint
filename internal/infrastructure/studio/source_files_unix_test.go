//go:build unix

package studio_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// A folder swapped again and again for a link to another folder of the
// project, while reads run, never makes a read return the other
// folder's file: the walk holds each folder it opened and checks it is
// the folder it saw.
func TestSourceFilesNeverFollowAFolderSwappedForALink(t *testing.T) {
	project := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(project, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("src/config.go", "package src\n")
	write(".aws/config.go", "aws_secret_access_key = SWAPPED-CREDENTIAL\n")
	src, real := filepath.Join(project, "src"), filepath.Join(project, "src.real")

	stop := make(chan struct{})
	swapped := make(chan int)
	go func() {
		n := 0
		defer func() { swapped <- n }()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if os.Rename(src, real) != nil || os.Symlink(".aws", src) != nil {
				return
			}
			runtime.Gosched()
			if os.Remove(src) != nil || os.Rename(real, src) != nil {
				return
			}
			n++
		}
	}()
	files := studio.SourceFiles{Project: project}
	deadline := time.Now().Add(1500 * time.Millisecond)
	reads := 0
	for time.Now().Before(deadline) {
		got, err := files.ReadSource("src/config.go", 1024)
		if err == nil && strings.Contains(string(got), "SWAPPED-CREDENTIAL") {
			close(stop)
			<-swapped
			t.Fatalf("read %d returned the linked folder's file: %q", reads, got)
		}
		reads++
	}
	close(stop)
	if n := <-swapped; n == 0 {
		t.Skip("the folder could not be swapped here")
	}
}

// A folder swapped again and again for a named pipe, while reads run,
// never makes a read wait: every read answers, file or refusal, within
// a bound.
func TestSourceFilesNeverWaitOnAFolderSwappedForAPipe(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "src", "a.go"), []byte("package src\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src, real := filepath.Join(project, "src"), filepath.Join(project, "src.real")
	stop := make(chan struct{})
	swapped := make(chan int)
	go func() {
		n := 0
		defer func() { swapped <- n }()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if os.Rename(src, real) != nil || syscall.Mkfifo(src, 0o600) != nil {
				return
			}
			runtime.Gosched()
			if os.Remove(src) != nil || os.Rename(real, src) != nil {
				return
			}
			n++
		}
	}()
	files := studio.SourceFiles{Project: project}
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = files.ReadSource("src/a.go", 1024)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			close(stop)
			<-swapped
			t.Fatal("a read waited on a folder swapped for a pipe")
		}
	}
	close(stop)
	if n := <-swapped; n == 0 {
		t.Skip("the folder could not be swapped here")
	}
}
