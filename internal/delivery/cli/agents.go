package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

// NewAgentsCommand is the agents command group: md installs or prints the
// AGENTS.md architecture block; skill emits domain-librarian artifacts.
func NewAgentsCommand(
	publish application.PublishAgentsContext,
	publishProtocol application.PublishSkillProtocol,
	publishVocabulary application.PublishSkillVocabulary,
	publishSchema application.PublishLibrarySchema,
) Command {
	return Command{
		Name:  "agents",
		Short: "AGENTS.md architecture block and domain-librarian skill artifacts",
		Subcommands: []Command{
			newAgentsMDCommand(publish),
			newAgentsSkillCommand(publishProtocol, publishVocabulary, publishSchema),
		},
	}
}

// newAgentsMDCommand renders the AGENTS.md architecture block to stdout,
// or installs it under --write.
func newAgentsMDCommand(publish application.PublishAgentsContext) Command {
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
			return reportArtifactWrite(ctx, changed, path)
		},
	}
}

// newAgentsSkillCommand writes SKILL.md, VOCAB.yaml, and
// library.schema.json into --dir (default DomainLibrarianSkillDir).
func newAgentsSkillCommand(
	protocol application.PublishSkillProtocol,
	vocabulary application.PublishSkillVocabulary,
	schema application.PublishLibrarySchema,
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
			out := &printer{w: ctx.Stdout}

			changed, path, err := protocol.Execute(dir)
			if err != nil {
				return ConfigError(err)
			}
			reportArtifactWriteTo(out, changed, path)

			changed, path, err = vocabulary.Execute(dir)
			if err != nil {
				return ConfigError(err)
			}
			reportArtifactWriteTo(out, changed, path)

			changed, path, err = schema.Execute(dir)
			if err != nil {
				return ConfigError(err)
			}
			reportArtifactWriteTo(out, changed, path)

			if out.err != nil {
				return fmt.Errorf("write output: %w", out.err)
			}
			return nil
		},
	}
}

func reportArtifactWrite(ctx Context, changed bool, path string) error {
	out := &printer{w: ctx.Stdout}
	reportArtifactWriteTo(out, changed, path)
	if out.err != nil {
		return fmt.Errorf("write output: %w", out.err)
	}
	return nil
}

func reportArtifactWriteTo(out *printer, changed bool, path string) {
	if changed {
		out.printf("wrote %s\n", path)
		return
	}
	out.printf("%s already current\n", path)
}
