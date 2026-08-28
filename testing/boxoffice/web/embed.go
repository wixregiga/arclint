//go:build embedweb

// Package web carries the built web app into the single binary.
// Build with -tags embedweb after `make web` so web/dist exists;
// without the tag the stub reports no embedded app.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var files embed.FS

// Dist returns the built web app when it was embedded.
func Dist() (fs.FS, bool) {
	sub, err := fs.Sub(files, "dist")
	if err != nil {
		return nil, false
	}
	return sub, true
}
