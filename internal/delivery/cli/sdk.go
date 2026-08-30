package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

// NewSDKCommand adapts the extension-SDK use case: install the typed
// SDK declarations beside the repository's extensions.
func NewSDKCommand(initialize application.InitializeExtensionSDK, render Renderer) Command {
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
					if err := render.Render(ctx.Stdout, SDKInitReport{Paths: paths}); err != nil {
						return fmt.Errorf("write output: %w", err)
					}
					return nil
				},
			},
		},
	}
}
