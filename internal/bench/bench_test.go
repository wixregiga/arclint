//go:build bench

// Package bench_test measures the performance bounds against a
// compiled binary: cold start under 100ms, and a 5,000-file repository
// checked in low single-digit seconds. Both repositories are
// synthesized in temp dirs; the bench depends on no fixtures. Run
// with:
//
//	go build -o /tmp/arclint ./cmd/arclint
//	ARCLINT_BIN=/tmp/arclint go test -tags bench -v ./internal/bench/
package bench_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func binPath(t *testing.T) string {
	t.Helper()
	bin := os.Getenv("ARCLINT_BIN")
	if bin == "" {
		t.Fatal("set ARCLINT_BIN to a compiled arclint binary")
	}
	return bin
}

// timeRuns executes the binary and returns per-run wall times.
// wantExit tolerates exit code 1 (violations are a normal outcome).
func timeRuns(t *testing.T, runs int, dir string, args ...string) []time.Duration {
	t.Helper()
	bin := binPath(t)
	var times []time.Duration
	for i := 0; i < runs; i++ {
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		start := time.Now()
		out, err := cmd.CombinedOutput()
		elapsed := time.Since(start)
		if err != nil {
			if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() != 1 {
				t.Fatalf("run %d: %v\n%s", i, err, out)
			}
		}
		times = append(times, elapsed)
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	return times
}

func median(d []time.Duration) time.Duration { return d[len(d)/2] }

// TestColdStart: full process lifetime (spawn, load rules, walk,
// parse, check, report) on a small repository stays under 100ms
// median.
func TestColdStart(t *testing.T) {
	root := t.TempDir()
	writeSyntheticRepo(t, root, 6, 4) // 6 packages x 4 files
	times := timeRuns(t, 11, root, "check", ".")
	med, max := median(times), times[len(times)-1]
	t.Logf("cold start over 11 runs: median %s, min %s, max %s", med, times[0], max)
	if med > 100*time.Millisecond {
		t.Errorf("median cold start %s exceeds the 100ms bound", med)
	}
}

// TestFiveThousandFiles: a synthetic 5,000-file repository with real
// contracts checks in low single-digit seconds.
func TestFiveThousandFiles(t *testing.T) {
	root := t.TempDir()
	writeSyntheticRepo(t, root, 500, 10) // 500 packages x 10 files = 5,000

	times := timeRuns(t, 3, root, "check", ".")
	med := median(times)
	t.Logf("5,000-file check over 3 runs: median %s, min %s, max %s", med, times[0], times[len(times)-1])
	if med > 5*time.Second {
		t.Errorf("median %s exceeds the low-single-digit-seconds bound (5s)", med)
	}
}

func writeSyntheticRepo(t *testing.T, root string, pkgs, filesPer int) {
	t.Helper()
	write := func(rel, content string) {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/synth\n\ngo 1.24\n\nrequire github.com/pkg/errors v0.9.1\n")
	write("rules.yaml", `runtime: [go]
modules:
  entities: "internal/entities/**"
  features: "internal/features/**"
  shared: "internal/shared/**"
rules:
  entities/pure:
    description: "Entities import nothing internal and no third-party package."
    on: entities
    imports:
      internal: []
      external: forbid
  features/inward:
    description: "Features import only shared and entities."
    on: features
    imports:
      internal: [shared, entities]
  features/snake:
    description: "Feature files use snake_case."
    on: features
    files: "internal/features/**/*.go"
    naming: snake_case
  dependencies/acyclic:
    description: "Module dependencies contain no cycle."
    acyclic: {}
`)
	for p := 0; p < pkgs; p++ {
		var area, imports string
		switch p % 3 {
		case 0:
			area = fmt.Sprintf("internal/entities/e%03d", p)
			imports = "\t\"strings\"\n"
		case 1:
			area = fmt.Sprintf("internal/features/f%03d", p)
			imports = fmt.Sprintf("\t_ \"example.com/synth/internal/shared/s%03d\"\n", p%50)
		default:
			area = fmt.Sprintf("internal/shared/s%03d", p%50+100)
			imports = "\t\"fmt\"\n"
		}
		for f := 0; f < filesPer; f++ {
			name := fmt.Sprintf("file_%02d.go", f)
			content := fmt.Sprintf("package p%03d\n\nimport (\n%s)\n\nfunc F%d() string { return \"x\" }\n", p, imports, f)
			write(area+"/"+name, content)
		}
	}
	// The shared packages the features import must exist.
	for s := 0; s < 50; s++ {
		write(fmt.Sprintf("internal/shared/s%03d/s.go", s), fmt.Sprintf("package s%03d\n", s))
	}
}
