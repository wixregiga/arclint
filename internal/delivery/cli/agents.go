package cli

import (
	"fmt"
	"strings"

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
	publishSchema application.PublishDomainSchema,
	render Renderer,
	extraCommands ...Command,
) Command {
	return Command{
		Name:  "agents",
		Short: "AGENTS.md architecture block and domain-librarian skill artifacts",
		Subcommands: append([]Command{
			newAgentsMDCommand(publish, render),
			newAgentsSkillCommand(publishProtocol, publishVocabulary, publishSchema, render),
		}, extraCommands...),
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
			if err := render.Render(ctx.Stdout, ArtifactStatusReport{
				Writes: []ArtifactWrite{{Changed: changed, Path: path}},
			}); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}

// newAgentsSkillCommand writes SKILL.md and VOCAB.yaml into every
// --dir (default: every directory an agent harness loads the skill
// from) and the Ubiquitous Language schema for human YAML editors into
// the project's schema directory.
func newAgentsSkillCommand(
	protocol application.PublishSkillProtocol,
	vocabulary application.PublishSkillVocabulary,
	schema application.PublishDomainSchema,
	render Renderer,
) Command {
	return Command{
		Name:  "skill",
		Short: "write domain-librarian skill artifacts (SKILL.md, VOCAB.yaml) and the domain schema for human YAML editors",
		Flags: []Flag{
			{
				Name:   "dir",
				Repeat: true,
				Doc:    "directory to write skill artifacts into; repeatable; default " + strings.Join(application.SkillDirectories(nil), " and ") + "; the schema always lands under " + application.SchemaDirectory,
			},
		},
		Run: func(ctx Context) error {
			dirs := application.SkillDirectories(ctx.Strings("dir"))
			writes := make([]ArtifactWrite, 0, 2*len(dirs)+1)

			for _, dir := range dirs {
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
			}

			changed, path, err := schema.Execute(application.SchemaDirectory)
			if err != nil {
				return ConfigError(err)
			}
			writes = append(writes, ArtifactWrite{Changed: changed, Path: path})

			if err := render.Render(ctx.Stdout, ArtifactStatusReport{Writes: writes}); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}
