package application

import "fmt"

// ExportPatternRequest selects the offline Pattern to publish and the
// Registry tree to write it into.
type ExportPatternRequest struct {
	// Selection is namespace/name@version, namespace/name, or a name.
	Selection string
	// Dir is the Registry tree root on disk.
	Dir string
}

// ExportPatternResult reports what the Registry tree now holds.
type ExportPatternResult struct {
	Reference  string
	Digest     string
	VersionDir string
	IndexPath  string
	// Replaced reports that the index already listed the version and
	// its entry was rewritten.
	Replaced bool
}

// ExportPattern publishes one offline Pattern into a Registry tree:
// the version directory with its Manifest, and the index entry, so
// any static file host serves it to arclint patterns install.
type ExportPattern struct {
	resolver  patternResolver
	publisher PatternPublisher
}

// NewExportPattern requires the publisher and the offline sources.
func NewExportPattern(publisher PatternPublisher, sources ...PatternSource) (ExportPattern, error) {
	if publisher == nil {
		return ExportPattern{}, fmt.Errorf("export pattern: missing pattern publisher")
	}
	if len(sources) == 0 {
		return ExportPattern{}, fmt.Errorf("export pattern: at least one pattern source required")
	}
	if err := validSources("export pattern", sources); err != nil {
		return ExportPattern{}, err
	}
	return ExportPattern{resolver: patternResolver{sources: sources}, publisher: publisher}, nil
}

// Execute resolves the selection offline and publishes it.
func (uc ExportPattern) Execute(req ExportPatternRequest) (ExportPatternResult, error) {
	if req.Dir == "" {
		return ExportPatternResult{}, fmt.Errorf("export pattern: registry directory required")
	}
	a, err := uc.resolver.resolve(req.Selection, "")
	if err != nil {
		return ExportPatternResult{}, fmt.Errorf("export pattern: %w", err)
	}
	published, err := uc.publisher.Publish(req.Dir, a.Available)
	if err != nil {
		return ExportPatternResult{}, fmt.Errorf("export pattern %s: %w", a.Reference(), err)
	}
	return ExportPatternResult{
		Reference:  a.Reference().String(),
		Digest:     a.Digest().String(),
		VersionDir: published.VersionDir,
		IndexPath:  published.IndexPath,
		Replaced:   published.Replaced,
	}, nil
}
