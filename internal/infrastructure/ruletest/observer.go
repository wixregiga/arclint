package ruletest

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
	filesystemobservation "github.com/wixregiga/arclint/internal/infrastructure/observation/filesystem"
)

// Observer implements the application's FixtureObserver port by
// materializing the fixture in a temporary directory and running the
// real filesystem observation source over it with the supplied fact
// producers — the fixture passes through the same parsers production
// uses, never simulated facts.
type Observer struct {
	producers []filesystemobservation.FactProducer
}

// NewObserver accepts the fact producers of the languages fixtures may
// exercise; producers may cover any subset of the supported languages.
func NewObserver(producers ...filesystemobservation.FactProducer) Observer {
	return Observer{producers: producers}
}

// Observe writes the fixture files under a fresh temporary root,
// observes it for the requested languages, scan policy, and fact
// classes, and removes the temporary tree afterwards.
func (o Observer) Observe(files []rule.TestFile, languages []rule.Language,
	scan rule.Scan, facts []rule.Fact,
) (obs conformance.Observations, err error) {
	root, err := os.MkdirTemp("", "arclint-ruletest-")
	if err != nil {
		return conformance.Observations{}, fmt.Errorf("fixture root: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(root); removeErr != nil && err == nil {
			obs = conformance.Observations{}
			err = fmt.Errorf("remove fixture tree: %w", removeErr)
		}
	}()
	for _, f := range files {
		if !filepath.IsLocal(filepath.FromSlash(f.Path)) {
			return conformance.Observations{}, fmt.Errorf("fixture file %q: path escapes the fixture root", f.Path)
		}
		target := filepath.Join(root, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return conformance.Observations{}, fmt.Errorf("fixture file %q: %w", f.Path, err)
		}
		if err := os.WriteFile(target, []byte(f.Content), 0o600); err != nil {
			return conformance.Observations{}, fmt.Errorf("fixture file %q: %w", f.Path, err)
		}
	}
	source, err := filesystemobservation.NewSource(root, o.producers...)
	if err != nil {
		return conformance.Observations{}, fmt.Errorf("fixture observation: %w", err)
	}
	obs, err = source.Observe(languages, scan, facts)
	if err != nil {
		return conformance.Observations{}, fmt.Errorf("fixture observation: %w", err)
	}
	return obs, nil
}
