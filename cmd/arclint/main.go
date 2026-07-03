// Command arclint is the arclint binary: a very fast architecture linter
// and template repo creator. All logic lives in internal packages; this
// file only forwards to the CLI layer.
package main

import (
	"os"

	"github.com/jofyi/arclint/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
