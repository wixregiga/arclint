package application

import (
	"fmt"
	"path"
	"sort"

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

func (uc GetArchitecturalContext) observeDependencies(cfg rule.Configured) (*ObservedDependencies, error) {
	if uc.observations == nil {
		return nil, fmt.Errorf("observe dependencies: no observation source is configured")
	}
	obs, err := uc.observations.Observe(cfg.Languages, cfg.Scan, []rule.Fact{rule.FactImports})
	if err != nil {
		return nil, fmt.Errorf("observe dependencies: %w", err)
	}
	return dependenciesOf(obs, cfg), nil
}

func dependenciesOf(obs conformance.Observations, cfg rule.Configured) *ObservedDependencies {
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
	fileZones := map[string][]string{}
	dirZones := map[string]map[string]bool{}
	files := obs.Files()
	d.Coverage.FilesObserved = len(files)
	for _, file := range files {
		zones := []string{}
		for _, zone := range cfg.Zones {
			if zone.Contains(file.Path) {
				zones = append(zones, string(zone.Name()))
			}
		}
		sort.Strings(zones)
		fileZones[file.Path] = zones
		dir := path.Dir(file.Path)
		if dirZones[dir] == nil {
			dirZones[dir] = map[string]bool{}
		}
		for _, zone := range zones {
			dirZones[dir][zone] = true
		}
	}
	for _, file := range files {
		language := rule.LanguageOf(file.Path)
		facts, found := obs.FactsFor(file.Path)
		available := enabled[language] && found && facts.Supports(rule.FactImports)
		d.Files = append(d.Files, DependencyFile{Path: file.Path, Zones: fileZones[file.Path], Language: string(language), ImportsAvailable: available})
		if !enabled[language] {
			continue
		}
		d.Coverage.SourceFiles++
		if !available {
			message := "The configured language supplied no usable import facts."
			if facts.ParseFailure != "" {
				message = facts.ParseFailure
			}
			diagnostic(file.Path, "IMPORTS_UNAVAILABLE", message)
			continue
		}
		d.Coverage.FilesWithImports++
		for _, imp := range facts.Imports {
			edge := DependencyImport{SourcePath: file.Path, Specifier: imp.Path, Line: imp.Line, Classification: imp.Class, SourceZones: fileZones[file.Path], TargetZones: []string{}, TargetKind: "unresolved"}
			switch imp.Class {
			case conformance.ImportInternal:
				switch {
				case imp.TargetFile != "":
					edge.TargetKind, edge.TargetPath = "file", imp.TargetFile
					if zones, exists := fileZones[imp.TargetFile]; exists {
						edge.TargetZones = zones
					} else {
						diagnostic(file.Path, "TARGET_NOT_OBSERVED", "Resolved import target was not observed: "+imp.TargetFile)
					}
				case imp.TargetDir != "":
					edge.TargetKind, edge.TargetPath = "directory", imp.TargetDir
					if zones, exists := dirZones[imp.TargetDir]; exists {
						for zone := range zones {
							edge.TargetZones = append(edge.TargetZones, zone)
						}
						sort.Strings(edge.TargetZones)
					} else {
						diagnostic(file.Path, "TARGET_NOT_OBSERVED", "Resolved import directory was not observed: "+imp.TargetDir)
					}
				default:
					diagnostic(file.Path, "TARGET_UNRESOLVED", "Internal import has no resolved repository target: "+imp.Path)
				}
			case conformance.ImportUnknown:
				diagnostic(file.Path, "IMPORT_UNKNOWN", "Import classification is unknown: "+imp.Path)
			case conformance.ImportStdlib, conformance.ImportExternal, conformance.ImportCgo:
				// Nonlocal imports have no repository target to resolve.
			}
			d.Edges = append(d.Edges, edge)
		}
	}
	return d
}
