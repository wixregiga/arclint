// Package fanout analyzes observed files on a bounded worker pool.
// Every language producer walks the same shape: select the files it
// owns, analyze each one independently, gather Language Facts by
// path. The per-file work (a read, a parse, a classification against a
// read-only resolver) shares nothing between files, so it runs on one
// goroutine per CPU. Each worker builds its own analyzer, which is
// where per-goroutine state lives: a Go token.FileSet, a tree-sitter
// parser. The gathered map is what a sequential loop would build.
package fanout

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// Analyzer analyzes one repo-relative file. A worker owns one Analyzer
// and is the only goroutine that calls it.
type Analyzer func(rel string) conformance.LanguageFacts

// Analyze produces the facts of every accepted file. newWorker runs
// once per worker goroutine, before that worker takes any file; the
// pool has min(GOMAXPROCS, accepted files) workers, and files are
// handed out dynamically so one slow file never idles the others.
func Analyze(files []conformance.ObservedFile, accept func(rel string) bool,
	newWorker func() Analyzer,
) map[string]conformance.LanguageFacts {
	var selected []string
	for _, f := range files {
		if accept(f.Path) {
			selected = append(selected, f.Path)
		}
	}
	out := make(map[string]conformance.LanguageFacts, len(selected))
	if len(selected) == 0 {
		return out
	}
	workers := min(runtime.GOMAXPROCS(0), len(selected))
	results := make([]conformance.LanguageFacts, len(selected))
	var next atomic.Int64
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			analyze := newWorker()
			for {
				i := int(next.Add(1)) - 1
				if i >= len(selected) {
					return
				}
				results[i] = analyze(selected[i])
			}
		}()
	}
	wg.Wait()
	for i, rel := range selected {
		out[rel] = results[i]
	}
	return out
}
