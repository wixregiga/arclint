// Command genschema writes the published JSON Schema for rules.yaml,
// derived from the same Go structs that drive runtime validation.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/wixregiga/arclint/internal/config"
)

func main() {
	out := flag.String("out", "docs/rules.schema.json", "output file")
	flag.Parse()

	data, err := config.SchemaJSON()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%d bytes)", *out, len(data))
}
