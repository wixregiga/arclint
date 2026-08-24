// Command arclint enforces architecture contracts over a repository.
// Exit codes: 0 clean, 1 error-severity findings, 2 configuration or
// usage error.
package main

import (
	_ "embed"
	"os"
)

//go:embed VERSION
var version string

func main() {
	os.Exit(run(os.Args[1:]))
}
