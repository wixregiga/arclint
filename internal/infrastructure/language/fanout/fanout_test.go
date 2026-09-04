package fanout_test

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/infrastructure/language/fanout"
)

func observed(n int) []conformance.ObservedFile {
	files := make([]conformance.ObservedFile, n)
	for i := range files {
		files[i] = conformance.ObservedFile{Path: fmt.Sprintf("pkg%03d/file_%04d.go", i%7, i)}
	}
	return files
}

// TestAnalyzeGathersEveryAcceptedFileOnce proves the gathered map is
// what a sequential loop would build: every accepted file exactly
// once under its own path, nothing for rejected files, and an empty
// map when nothing is accepted.
func TestAnalyzeGathersEveryAcceptedFileOnce(t *testing.T) {
	files := observed(1000)
	accept := func(rel string) bool { return rel[len(rel)-4] != '5' } // drop paths ending in 5.go
	var calls atomic.Int64
	var mu sync.Mutex
	seen := map[string]int{}
	out := fanout.Analyze(files, accept, func() fanout.Analyzer {
		return func(rel string) conformance.LanguageFacts {
			calls.Add(1)
			mu.Lock()
			seen[rel]++
			mu.Unlock()
			return conformance.LanguageFacts{Package: rel}
		}
	})
	wantAccepted := 0
	for _, f := range files {
		if !accept(f.Path) {
			if _, ok := out[f.Path]; ok {
				t.Errorf("rejected file %s was analyzed", f.Path)
			}
			continue
		}
		wantAccepted++
		got, ok := out[f.Path]
		if !ok {
			t.Errorf("accepted file %s missing from the gathered facts", f.Path)
			continue
		}
		if got.Package != f.Path {
			t.Errorf("facts of %s carry Package %q: results crossed between files", f.Path, got.Package)
		}
		if seen[f.Path] != 1 {
			t.Errorf("%s analyzed %d times, want once", f.Path, seen[f.Path])
		}
	}
	if len(out) != wantAccepted || int(calls.Load()) != wantAccepted {
		t.Errorf("gathered %d facts from %d analyses, want %d of each", len(out), calls.Load(), wantAccepted)
	}

	empty := fanout.Analyze(files, func(string) bool { return false }, func() fanout.Analyzer {
		t.Error("a worker was built with no accepted files")
		return nil
	})
	if len(empty) != 0 {
		t.Errorf("no accepted files gathered %d facts", len(empty))
	}
}

// TestAnalyzeBuildsOneAnalyzerPerWorker proves per-goroutine state is
// honored: the pool never exceeds GOMAXPROCS workers nor the file
// count, each worker builds its analyzer exactly once, and an analyzer
// is used by its own goroutine only. The unsynchronized per-worker
// counter is the race detector's witness for that last claim.
func TestAnalyzeBuildsOneAnalyzerPerWorker(t *testing.T) {
	for _, n := range []int{1, 3, 4 * runtime.GOMAXPROCS(0)} {
		var workers atomic.Int64
		var mu sync.Mutex
		var perWorker []*int
		out := fanout.Analyze(observed(n), func(string) bool { return true }, func() fanout.Analyzer {
			workers.Add(1)
			count := new(int)
			mu.Lock()
			perWorker = append(perWorker, count)
			mu.Unlock()
			return func(rel string) conformance.LanguageFacts {
				*count++
				return conformance.LanguageFacts{}
			}
		})
		if len(out) != n {
			t.Fatalf("n=%d: gathered %d facts", n, len(out))
		}
		bound := min(runtime.GOMAXPROCS(0), n)
		if got := int(workers.Load()); got < 1 || got > bound {
			t.Errorf("n=%d: built %d analyzers, want between 1 and %d", n, got, bound)
		}
		total := 0
		for _, c := range perWorker {
			total += *c
		}
		if total != n {
			t.Errorf("n=%d: workers analyzed %d files in total", n, total)
		}
	}
}
