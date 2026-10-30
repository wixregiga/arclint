package mapserve

import (
	"embed"
	"fmt"
	"io/fs"
)

// Built from web/ by make generate ONLY=web. Committed assets keep go build
// and the installed binary independent of Node and the working directory.
//
//go:embed static/web
var studioBuild embed.FS

func studioFiles() (fs.FS, error) {
	files, err := fs.Sub(studioBuild, "static/web")
	if err != nil {
		return nil, fmt.Errorf("studio assets: %w", err)
	}
	return files, nil
}
