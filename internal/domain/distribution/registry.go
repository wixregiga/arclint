package distribution

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

// DefaultRegistryLocation is arclint's own Registry: the published
// tree of the arclint-pattern-registry repository.
const DefaultRegistryLocation = "https://raw.githubusercontent.com/wixregiga/arclint-pattern-registry/main"

// IndexFileName is the Registry's listing of every published
// reference, at the Registry root.
const IndexFileName = "index.json"

// ManifestFileName is the Manifest document beside a published
// version's files, and beside a VendoredPattern's files.
const ManifestFileName = "manifest.json"

// Registry is a location, reachable by URL, that publishes Patterns by
// PatternReference. The layout is fixed so that a Registry is nothing
// more than files: <root>/index.json, and per version
// <root>/<namespace>/<name>/<version>/manifest.json beside the
// version's files.
type Registry struct {
	location string
}

// NewRegistry accepts an https, http, or file URL. The location is
// kept without a trailing slash so paths join predictably.
func NewRegistry(location string) (Registry, error) {
	location = strings.TrimSpace(location)
	if location == "" {
		return Registry{}, fmt.Errorf("registry: location required")
	}
	u, err := url.Parse(location)
	if err != nil {
		return Registry{}, fmt.Errorf("registry %q: %v", location, err)
	}
	switch u.Scheme {
	case "https", "http":
		if u.Host == "" {
			return Registry{}, fmt.Errorf("registry %q: host required", location)
		}
	case "file":
		if u.Path == "" {
			return Registry{}, fmt.Errorf("registry %q: file URL needs a path", location)
		}
	default:
		return Registry{}, fmt.Errorf("registry %q: scheme must be https, http, or file", location)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return Registry{}, fmt.Errorf("registry %q: query and fragment are not allowed", location)
	}
	return Registry{location: strings.TrimRight(location, "/")}, nil
}

// Location is the Registry root URL without a trailing slash.
func (r Registry) Location() string { return r.location }

// IsZero reports an unconstructed Registry.
func (r Registry) IsZero() bool { return r.location == "" }

// IndexPath is the published index, relative to the Registry root.
func IndexPath() string { return IndexFileName }

// VersionDir is the relative directory one published version lives
// in, inside a Registry tree and inside an export tree alike.
func VersionDir(ref rule.PatternReference) string {
	return ref.Namespace() + "/" + ref.Name() + "/" + ref.Version()
}

// ManifestPath is one published version's Manifest, relative to the
// Registry root.
func ManifestPath(ref rule.PatternReference) string {
	return VersionDir(ref) + "/" + ManifestFileName
}

// FilePath is one file of a published version, relative to the
// Registry root.
func FilePath(ref rule.PatternReference, filePath string) string {
	return VersionDir(ref) + "/" + filePath
}

// URL locates one document by its path relative to the Registry root.
func (r Registry) URL(relative string) string { return r.location + "/" + relative }

// IndexURL locates the published index.
func (r Registry) IndexURL() string { return r.URL(IndexPath()) }

// ManifestURL locates one published version's Manifest.
func (r Registry) ManifestURL(ref rule.PatternReference) string {
	return r.URL(ManifestPath(ref))
}

// FileURL locates one file of a published version.
func (r Registry) FileURL(ref rule.PatternReference, filePath string) string {
	return r.URL(FilePath(ref, filePath))
}

// String is the location.
func (r Registry) String() string { return r.location }
