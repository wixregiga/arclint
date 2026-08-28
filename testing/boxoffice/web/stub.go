//go:build !embedweb

// Package web carries the built web app into the single binary.
// This is the tagless stub: no node toolchain ran, so there is no
// embedded app and the server says so instead of pretending.
package web

import "io/fs"

// Dist reports that no web app was embedded in this build.
func Dist() (fs.FS, bool) {
	return nil, false
}
