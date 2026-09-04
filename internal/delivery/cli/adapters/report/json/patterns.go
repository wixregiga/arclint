package jsonreport

import (
	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
)

// --- patterns (lowerCamel) ---

type patternDoc struct {
	Reference     string   `json:"reference"`
	Namespace     string   `json:"namespace"`
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Source        string   `json:"source"`
	Vendored      bool     `json:"vendored"`
	Authored      bool     `json:"authored"`
	Digest        string   `json:"digest,omitempty"`
	Documentation string   `json:"documentation,omitempty"`
	Rules         int      `json:"rules"`
	Extensions    int      `json:"extensions"`
	Coverage      []string `json:"coverage,omitempty"`
}

type patternsListDoc struct {
	Registry string       `json:"registry,omitempty"`
	Patterns []patternDoc `json:"patterns"`
}

func patternsDoc(r cli.PatternsReport) patternsListDoc {
	docs := make([]patternDoc, 0, len(r.Patterns))
	for _, row := range r.Patterns {
		docs = append(docs, patternDoc{
			Reference:     row.Namespace + "/" + row.Name + "@" + row.Version,
			Namespace:     row.Namespace,
			Name:          row.Name,
			Version:       row.Version,
			Source:        string(row.Source),
			Vendored:      row.Vendored,
			Authored:      row.Authored,
			Digest:        row.Digest,
			Documentation: row.Documentation,
			Rules:         row.Rules,
			Extensions:    row.Extensions,
			Coverage:      append([]string(nil), row.Coverage...),
		})
	}
	return patternsListDoc{Registry: r.Registry, Patterns: docs}
}

type patternVendorDoc struct {
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
	Source    string `json:"source"`
	Path      string `json:"path,omitempty"`
	Replaced  string `json:"replaced,omitempty"`
	Unchanged bool   `json:"unchanged"`
}

func patternVendorDocOf(res application.VendorPatternResult) patternVendorDoc {
	return patternVendorDoc{
		Reference: res.Reference,
		Digest:    res.Digest,
		Source:    string(res.Source),
		Path:      res.Path,
		Replaced:  res.Replaced,
		Unchanged: res.Unchanged,
	}
}

type boundModuleDoc struct {
	Module string   `json:"module"`
	Paths  []string `json:"paths"`
}

type patternInstallDoc struct {
	Reference       string           `json:"reference"`
	Digest          string           `json:"digest"`
	Source          string           `json:"source"`
	VendoredPath    string           `json:"vendoredPath,omitempty"`
	VendorReplaced  string           `json:"vendorReplaced,omitempty"`
	RulesetPath     string           `json:"rulesetPath"`
	RulesetCreated  bool             `json:"rulesetCreated"`
	RulesetReplaced string           `json:"rulesetReplaced,omitempty"`
	Bound           []boundModuleDoc `json:"bound"`
	Unbound         []string         `json:"unbound"`
	Adopted         []string         `json:"adopted,omitempty"`
}

func patternInstallDocOf(res application.InstallPatternResult) patternInstallDoc {
	bound := make([]boundModuleDoc, 0, len(res.Bound))
	for _, b := range res.Bound {
		bound = append(bound, boundModuleDoc{Module: b.Module, Paths: append([]string(nil), b.Paths...)})
	}
	unbound := make([]string, 0, len(res.Unbound))
	unbound = append(unbound, res.Unbound...)
	return patternInstallDoc{
		Reference:       res.Reference,
		Digest:          res.Digest,
		Source:          string(res.Source),
		VendoredPath:    res.VendoredPath,
		VendorReplaced:  res.VendorReplaced,
		RulesetPath:     res.RulesetPath,
		RulesetCreated:  res.RulesetCreated,
		RulesetReplaced: res.RulesetReplaced,
		Bound:           bound,
		Unbound:         unbound,
		Adopted:         append([]string(nil), res.Adopted...),
	}
}

type patternExportDoc struct {
	Reference  string `json:"reference"`
	Digest     string `json:"digest"`
	VersionDir string `json:"versionDir"`
	IndexPath  string `json:"indexPath"`
	Replaced   bool   `json:"replaced"`
}

func patternExportDocOf(res application.ExportPatternResult) patternExportDoc {
	return patternExportDoc{
		Reference:  res.Reference,
		Digest:     res.Digest,
		VersionDir: res.VersionDir,
		IndexPath:  res.IndexPath,
		Replaced:   res.Replaced,
	}
}
