package application

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/domain/conformance"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// ObservedDependencies projects native import facts, not declared permissions or
// Rule outcomes. Directory targets preserve package-granular language resolution.
type ObservedDependencies struct {
	Files       []DependencyFile       `json:"files"`
	Edges       []DependencyImport     `json:"edges"`
	Coverage    DependencyCoverage     `json:"coverage"`
	Diagnostics []DependencyDiagnostic `json:"diagnostics"`
	Limitations []string               `json:"limitations"`
}

// DependencyFile describes one observed file and every declared Zone containing it.
type DependencyFile struct {
	Path             string   `json:"path"`
	Zones            []string `json:"zones"`
	Language         string   `json:"language"`
	ImportsAvailable bool     `json:"importsAvailable"`
}

// DependencyImport describes one parsed import with the native resolver’s target precision.
type DependencyImport struct {
	SourcePath     string                  `json:"sourcePath"`
	TargetPath     string                  `json:"targetPath"`
	TargetKind     string                  `json:"targetKind"`
	Specifier      string                  `json:"specifier"`
	Line           int                     `json:"line"`
	Classification conformance.ImportClass `json:"classification"`
	SourceZones    []string                `json:"sourceZones"`
	TargetZones    []string                `json:"targetZones"`
	TargetObserved bool                    `json:"targetObserved"`
}

// DependencyCoverage describes the availability of import facts within the configured scan.
type DependencyCoverage struct {
	Scope         string   `json:"scope"`
	Languages     []string `json:"languages"`
	FilesObserved int      `json:"filesObserved"`
	SourceFiles   int      `json:"sourceFiles"`
	// FilesWithImports counts available import fact sets, including genuine zero-import files.
	FilesWithImports int  `json:"filesWithImports"`
	Complete         bool `json:"complete"`
}

// DependencyDiagnostic describes an observation gap that prevents complete import coverage.
type DependencyDiagnostic struct {
	Path    string `json:"path,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func dependenciesOf(facts conformance.Facts, cfg rule.Configured) *ObservedDependencies {
	d := &ObservedDependencies{
		Files: []DependencyFile{}, Edges: []DependencyImport{}, Diagnostics: []DependencyDiagnostic{},
		Coverage: DependencyCoverage{Scope: "repository", Languages: []string{}, Complete: true},
		Limitations: []string{
			"Parsed import statements only; not runtime, reflective, or transitive dependencies and not a conformance assessment.",
			"Coverage is limited to configured languages and the repository scan policy; excluded files and directories are not observed.",
			"Directory targets are package resolutions; their Zones are the union of observed files directly in that directory, not exact target-file dependencies.",
		},
	}
	diagnostic := func(file, code, message string) {
		d.Coverage.Complete = false
		d.Diagnostics = append(d.Diagnostics, DependencyDiagnostic{Path: file, Code: code, Message: message})
	}
	enabled := map[rule.Language]bool{}
	for _, language := range cfg.Languages {
		enabled[language] = true
		d.Coverage.Languages = append(d.Coverage.Languages, string(language))
	}
	if len(enabled) == 0 {
		diagnostic("", "NO_LANGUAGES", "No source languages are configured for import observation.")
	}
	if !cfg.Scan.IncludeTestdata {
		d.Limitations = append(d.Limitations, "The scan excludes testdata directories.")
	}
	for _, excluded := range cfg.Scan.Exclude {
		d.Limitations = append(d.Limitations, "Scan exclusion: "+excluded.String())
	}
	files := facts.Files()
	d.Coverage.FilesObserved = len(files)
	for _, file := range files {
		language := rule.LanguageOf(file.Path)
		observed, found := facts.FactsFor(file.Path)
		available := enabled[language] && found && observed.Supports(rule.FactImports)
		d.Files = append(d.Files, DependencyFile{Path: file.Path, Zones: dependencyZoneNames(facts.ZoneOf(file.Path)), Language: string(language), ImportsAvailable: available})
		if !enabled[language] {
			continue
		}
		d.Coverage.SourceFiles++
		if !available {
			message := "The configured language supplied no usable import facts."
			if observed.ParseFailure != "" {
				message = observed.ParseFailure
			}
			diagnostic(file.Path, "IMPORTS_UNAVAILABLE", message)
			continue
		}
		d.Coverage.FilesWithImports++
		for _, imp := range facts.ImportsFor(file.Path) {
			kind, target := imp.Target()
			edge := DependencyImport{
				SourcePath: imp.SourcePath, Specifier: imp.Path, Line: imp.Line,
				Classification: imp.Class, SourceZones: dependencyZoneNames(imp.SourceZones),
				TargetZones: dependencyZoneNames(imp.TargetZones), TargetKind: kind, TargetPath: target, TargetObserved: imp.TargetObserved,
			}
			switch imp.Class {
			case conformance.ImportInternal:
				if target == "" {
					diagnostic(file.Path, "TARGET_UNRESOLVED", "Internal import has no resolved repository target: "+imp.Path)
				} else if !imp.TargetObserved {
					diagnostic(file.Path, "TARGET_NOT_OBSERVED", fmt.Sprintf("Resolved import %s was not observed: %s", kind, target))
				}
			case conformance.ImportUnknown:
				diagnostic(file.Path, "IMPORT_UNKNOWN", "Import classification is unknown: "+imp.Path)
			case conformance.ImportStdlib, conformance.ImportExternal, conformance.ImportCgo:
				// These imports do not have repository targets to observe.
			}
			d.Edges = append(d.Edges, edge)
		}
	}
	return d
}

// This is a wire conversion only; membership is resolved by conformance.Facts.
func dependencyZoneNames(names []rule.ZoneName) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = string(name)
	}
	return out
}
