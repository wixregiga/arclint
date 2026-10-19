package mapserve

import _ "embed"

// The page and its script are embedded at build time, so the binary
// serves them without a working directory to read from.
var (
	//go:embed static/index.html
	studioPage []byte
	//go:embed static/studio.js
	studioScript []byte
)
