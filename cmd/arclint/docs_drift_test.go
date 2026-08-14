package main

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestCLIDocsDrift ties docs/site/content/docs/cli.md to the real
// command tree: a command or a check flag that the page does not
// mention fails here, so the reference cannot rot silently. The rule
// reference page has the same protection through gendocs; this guard
// covers the hand-written CLI page without a generator.
func TestCLIDocsDrift(t *testing.T) {
	data, err := os.ReadFile("../../docs/site/content/docs/cli.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)

	root := newRootCmd()
	var walk func(cmd *cobra.Command, path string)
	walk = func(cmd *cobra.Command, path string) {
		for _, c := range cmd.Commands() {
			if c.Hidden || c.Name() == "help" {
				continue
			}
			p := path + " " + c.Name()
			if !strings.Contains(doc, "arclint"+p) {
				t.Errorf("cli.md does not mention `arclint%s`", p)
			}
			walk(c, p)
		}
	}
	walk(root, "")

	check, _, err := root.Find([]string{"check"})
	if err != nil {
		t.Fatal(err)
	}
	check.Flags().VisitAll(func(f *pflag.Flag) {
		if !strings.Contains(doc, "--"+f.Name) {
			t.Errorf("cli.md does not mention check --%s", f.Name)
		}
	})
}
