// Command genclassifying generates the Classifying page of the docs site
// from the sources it explains: the building blocks, their invariants,
// guidance, and citations from the Domain-Driven Design meta-model; the
// clarification questions and distillation rules the domain-librarian
// skill carries; and the code of the worked example under docs/examples,
// quoted between ANCHOR markers from examples that pass arclint check,
// the Go one also compiling and passing its tests. Nothing on the page is
// written twice: a change to the meta-model, the questions, or the
// example changes the page, and a test fails until it is regenerated.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

//go:generate go run . -root ../..

// pagePath is where the page lives, relative to the repository root.
const pagePath = "docs/site/content/docs/classifying.md"

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()

	repo, err := os.OpenRoot(*root)
	if err != nil {
		log.Fatal(err)
	}
	page, err := render(repo.FS())
	if err != nil {
		log.Fatal(err)
	}
	// The write is confined to the repository root so no path the
	// generator computes can reach outside it.
	if err := repo.WriteFile(pagePath, page, 0o644); err != nil {
		log.Fatal(err)
	}
	if err := repo.Close(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s\n", pagePath)
}
