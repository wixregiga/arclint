// tsoracle emits the actual in-process TypeScript adapter facts for the
// independent compiler-checker oracle. It is not linked into arclint.
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/infrastructure/language/typescript"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: go run ./tools/tsoracle <repository-root>")
	}
	root, err := filepath.Abs(os.Args[1])
	if err != nil {
		return fmt.Errorf("tsoracle: %w", err)
	}
	var files []conformance.ObservedFile
	err = filepath.WalkDir(root, func(abs string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("tsoracle: %w", err)
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".claude", "dist", "coverage":
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return fmt.Errorf("tsoracle: %w", err)
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("tsoracle: %w", err)
		}
		files = append(files, conformance.ObservedFile{Path: filepath.ToSlash(rel), Size: info.Size()})
		return nil
	})
	if err != nil {
		return fmt.Errorf("tsoracle: %w", err)
	}
	facts, err := typescript.NewProducer().Facts(root, files, []rule.Fact{rule.FactResolvedCalls, rule.FactFieldTypes})
	if err != nil {
		return fmt.Errorf("tsoracle: %w", err)
	}
	if _, err := conformance.NewObservations(files, facts); err != nil {
		return fmt.Errorf("tsoracle: %w", err)
	}
	err = json.NewEncoder(os.Stdout).Encode(struct {
		Root  string
		Files []conformance.ObservedFile
		Facts map[string]conformance.LanguageFacts
	}{root, files, facts})
	if err != nil {
		return fmt.Errorf("encode facts: %w", err)
	}
	return nil
}
