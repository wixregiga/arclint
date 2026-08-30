package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

// NewAgentsCommand is the agents command group: md installs or prints the
// AGENTS.md architecture block; skill emits domain-librarian artifacts.
// Status lines for write operations use the injected Renderer; raw
// markdown remains a raw byte product.
func NewAgentsCommand(
	publish application.PublishAgentsContext,
	publishProtocol application.PublishSkillProtocol,
	publishVocabulary application.PublishSkillVocabulary,
	publishSchema application.PublishLibrarySchema,
	render Renderer,
) Command {
	return Command{
		Name:  "agents",
		Short: "AGENTS.md architecture block and domain-librarian skill artifacts",
		Subcommands: []Command{
			newAgentsMDCommand(publish, render),
			newAgentsSkillCommand(publishProtocol, publishVocabulary, publishSchema, render),
		},
	}
}

// newAgentsMDCommand renders the AGENTS.md architecture block to stdout,
// or installs it under --write.
func newAgentsMDCommand(publish application.PublishAgentsContext, render Renderer) Command {
	return Command{
		Name:    "md",
		Aliases: []string{"markdown", "agentsmd"},
		Short:   "print or install the generated AGENTS.md architecture block",
		Flags: []Flag{
			{Name: "write", Bool: true, Doc: "install or refresh the block in <repo-root>/AGENTS.md"},
		},
		Run: func(ctx Context) error {
			if !ctx.Bool("write") {
				block, err := publish.Render()
				if err != nil {
					return ConfigError(err)
				}
				if _, err := fmt.Fprint(ctx.Stdout, block); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
				return nil
			}
			changed, path, err := publish.Execute()
			if err != nil {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, AgentsStatusReport{
				Writes: []ArtifactWrite{{Changed: changed, Path: path}},
			}); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}

// newAgentsSkillCommand writes SKILL.md, VOCAB.yaml, and
// library.schema.json into --dir (default DomainLibrarianSkillDir).
func newAgentsSkillCommand(
	protocol application.PublishSkillProtocol,
	vocabulary application.PublishSkillVocabulary,
	schema application.PublishLibrarySchema,
	render Renderer,
) Command {
	return Command{
		Name:  "skill",
		Short: "write domain-librarian skill artifacts (SKILL.md, VOCAB.yaml, library.schema.json)",
		Flags: []Flag{
			{
				Name:    "dir",
				Default: application.DomainLibrarianSkillDir,
				Doc:     "directory to write skill artifacts into",
			},
		},
		Run: func(ctx Context) error {
			dir := ctx.String("dir")
			writes := make([]ArtifactWrite, 0, 3)

			changed, path, err := protocol.Execute(dir)
			if err != nil {
				return ConfigError(err)
			}
			writes = append(writes, ArtifactWrite{Changed: changed, Path: path})

			changed, path, err = vocabulary.Execute(dir)
			if err != nil {
				return ConfigError(err)
			}
			writes = append(writes, ArtifactWrite{Changed: changed, Path: path})

			changed, path, err = schema.Execute(dir)
			if err != nil {
				return ConfigError(err)
			}
			writes = append(writes, ArtifactWrite{Changed: changed, Path: path})

			if err := render.Render(ctx.Stdout, AgentsStatusReport{Writes: writes}); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}
