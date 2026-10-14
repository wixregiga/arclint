package typescript

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// BenchmarkBoxoffice measures the production web tree, not a generated fixture.
// Setup and directory walking are outside the measured producer calls.
func BenchmarkBoxoffice(b *testing.B) {
	target := os.Getenv("ARCLINT_TS_BENCH_ROOT")
	if target == "" {
		target = "../../../../testing/boxoffice/web"
	}
	root, err := filepath.Abs(target)
	if err != nil {
		b.Fatal(err)
	}
	var files []conformance.ObservedFile
	err = filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, conformance.ObservedFile{Path: filepath.ToSlash(rel), Size: info.Size()})
		return nil
	})
	if err != nil {
		b.Fatal(err)
	}
	for _, tt := range []struct {
		name  string
		facts []rule.Fact
	}{
		{"Imports", nil},
		{"Syntactic", []rule.Fact{rule.FactDeclarations, rule.FactCalls}},
		{"Resolved", []rule.Fact{rule.FactResolvedCalls, rule.FactFieldTypes}},
	} {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := NewProducer().Facts(root, files, tt.facts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
