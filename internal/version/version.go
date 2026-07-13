// Package version carries the arclint version string.
package version

// Version is the arclint version reported by --version and injected into
// templates as the arclint_version built-in. A var (not a const) so release
// builds can override it:
//
//	go build -ldflags "-X github.com/wixregiga/arclint/internal/version.Version=v1.2.3"
var Version = "0.0.0-dev"
