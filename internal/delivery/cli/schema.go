package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

// schemaPublisher is what a published JSON Schema use case offers the
// CLI: the bytes for stdout, or a write under a directory reporting
// whether the file changed.
type schemaPublisher interface {
	Render() ([]byte, error)
	Execute(dir string) (changed bool, path string, err error)
}

// newSchemaCommand builds a schema subcommand shared by the rules and
// domain groups: without --write the schema is printed as raw bytes
// (bypassing the Renderer, like every schema output); with --write it
// lands under --dir (default the project's schema directory) and the
// write is reported through the Renderer.
func newSchemaCommand(short, long, example string, publish schemaPublisher, render Renderer) Command {
	return Command{
		Name:    "schema",
		Short:   short,
		Long:    long,
		Example: example,
		MaxArgs: 0,
		Flags: []Flag{
			{Name: "write", Bool: true, Doc: "write the schema under --dir instead of printing it"},
			{Name: "dir", Default: application.SchemaDirectory, Doc: "directory --write puts the schema into"},
		},
		Run: func(ctx Context) error {
			dir := ctx.String("dir")
			if !ctx.Bool("write") {
				if dir != application.SchemaDirectory {
					return ConfigError(fmt.Errorf("--dir only applies with --write"))
				}
				data, err := publish.Render()
				if err != nil {
					return ConfigError(err)
				}
				if _, err := ctx.Stdout.Write(data); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
				return nil
			}
			changed, path, err := publish.Execute(dir)
			if err != nil {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, ArtifactStatusReport{
				Writes: []ArtifactWrite{{Changed: changed, Path: path}},
			}); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}
