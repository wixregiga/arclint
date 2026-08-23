package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

// NewSDKCommand adapts the extension-SDK use case: install the typed
// SDK declarations beside the repository's extensions.
func NewSDKCommand(initialize application.InitializeExtensionSDK) Command {
	return Command{
		Name:  "sdk",
		Short: "extension SDK utilities",
		Subcommands: []Command{
			{
				Name:  commandInit,
				Short: "write arclint.d.ts and tsconfig.json into .arclint/extensions",
				Run: func(ctx Context) error {
					paths, err := initialize.Execute()
					if err != nil {
						return ConfigError(err)
					}
					out := &printer{w: ctx.Stdout}
					for _, p := range paths {
						out.printf("wrote %s\n", p)
					}
					if out.err != nil {
						return fmt.Errorf("write output: %w", out.err)
					}
					return nil
				},
			},
		},
	}
}
