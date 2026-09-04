package cli

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

const (
	flagRegistry  = "registry"
	flagRemote    = "remote"
	flagLanguages = "languages"
	flagDir       = "dir"
)

// PatternCommands bundles the Pattern distribution use cases the
// patterns command group adapts.
type PatternCommands struct {
	List    application.ListPatterns
	Vendor  application.VendorPattern
	Install application.InstallPattern
	Export  application.ExportPattern
	// Registry is the default Registry location the --registry flag
	// carries, as the composition root resolved it.
	Registry string
}

// NewPatternsCommand adapts the Pattern distribution use cases into the
// patterns command group. The bare command lists every Pattern that
// resolves offline, --remote lists what a Registry publishes, and the
// subcommands vendor, install, and export one Pattern by reference or
// name. A check never reaches a Registry; only these commands do, and
// only when a selection resolves neither embedded nor local.
func NewPatternsCommand(commands PatternCommands, render Renderer) Command {
	registryFlag := Flag{
		Name:    flagRegistry,
		Default: commands.Registry,
		Doc:     "Pattern Registry location: an https URL or a file:// tree (ARCLINT_REGISTRY)",
	}
	return Command{
		Name:  "patterns",
		Short: "list, vendor, install, and export Pattern distribution packages",
		Long: "Patterns are distributed architecture contracts a ruleset extends by reference.\n" +
			"They resolve offline first: the Patterns embedded in this binary, then the\n" +
			"repository's own .arclint/patterns tree. A Registry is consulted only by the\n" +
			"commands below, and only for a Pattern that resolves nowhere offline.",
		Example: "  arclint patterns\n" +
			"  arclint patterns --remote\n" +
			"  arclint patterns install vertical\n" +
			"  arclint patterns vendor acme/layers@1.2.0\n" +
			"  arclint patterns export vertical --dir ./registry",
		Flags: []Flag{
			{Name: flagRemote, Bool: true, Doc: "list the Patterns the Registry publishes instead of the offline ones"},
			registryFlag,
		},
		Run: func(ctx Context) error {
			report := PatternsReport{}
			var err error
			if ctx.Bool(flagRemote) {
				report.Registry = ctx.String(flagRegistry)
				report.Patterns, err = commands.List.Remote(report.Registry)
			} else {
				report.Patterns, err = commands.List.Execute()
			}
			if err != nil {
				return ConfigError(err)
			}
			if err := render.Render(ctx.Stdout, report); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
		Subcommands: []Command{
			{
				Name:         "vendor",
				Short:        "copy one Pattern under .arclint/patterns with its manifest",
				Long:         "Vendoring writes the Pattern's files under .arclint/patterns/<namespace>/<name>/ with a manifest.json recording every file's digest, so the copy is verified byte for byte on every load and the repository never needs the Registry again.",
				Example:      "  arclint patterns vendor vertical\n  arclint patterns vendor acme/layers@1.2.0 --registry https://patterns.example.com",
				Flags:        []Flag{registryFlag},
				MaxArgs:      1,
				CompleteArgs: completePatternSelections(commands.List),
				Run: func(ctx Context) error {
					selection, err := patternSelection(ctx)
					if err != nil {
						return err
					}
					result, err := commands.Vendor.Execute(application.VendorPatternRequest{
						Selection: selection,
						Registry:  ctx.String(flagRegistry),
					})
					if err != nil {
						return ConfigError(err)
					}
					if err := render.Render(ctx.Stdout, PatternVendorReport{Result: result}); err != nil {
						return fmt.Errorf("write output: %w", err)
					}
					return nil
				},
			},
			{
				Name:  "install",
				Short: "extend " + rule.RulesetFileName + " with one Pattern, vendoring it when it came from the Registry",
				Long: "Installing records the Pattern under extends in " + rule.RulesetFileName + " with every Binding the Pattern suggests, " +
					"binding a Module the ruleset already declares to its declared paths. A Pattern fetched from the Registry " +
					"is vendored first. Without a " + rule.RulesetFileName + ", one is drafted that extends the Pattern.",
				Example: "  arclint patterns install vertical\n  arclint patterns install acme/layers --languages go,ts",
				Flags: []Flag{
					registryFlag,
					{Name: flagLanguages, Doc: "comma-separated runtime targets written when " + rule.RulesetFileName + " is created: go, ts, py (default: the Pattern's coverage)", Complete: completeLanguages},
				},
				MaxArgs:      1,
				CompleteArgs: completePatternSelections(commands.List),
				Run: func(ctx Context) error {
					selection, err := patternSelection(ctx)
					if err != nil {
						return err
					}
					result, err := commands.Install.Execute(application.InstallPatternRequest{
						Selection: selection,
						Registry:  ctx.String(flagRegistry),
						Languages: splitList(ctx.String(flagLanguages)),
					})
					if err != nil {
						return ConfigError(err)
					}
					if err := render.Render(ctx.Stdout, PatternInstallReport{Result: result}); err != nil {
						return fmt.Errorf("write output: %w", err)
					}
					return nil
				},
			},
			{
				Name:    "export",
				Short:   "publish one offline Pattern into a Registry tree on disk",
				Long:    "Exporting writes the Pattern's version directory with its manifest.json under <dir>/<namespace>/<name>/<version>/ and updates <dir>/index.json, so any static file host that serves the tree is a Registry.",
				Example: "  arclint patterns export vertical --dir ./registry\n  arclint patterns export acme/layers@1.2.0 --dir ../arclint-pattern-registry",
				Flags: []Flag{
					{Name: flagDir, Doc: "Registry tree root to publish into (required)"},
				},
				MaxArgs:      1,
				CompleteArgs: completePatternSelections(commands.List),
				Run: func(ctx Context) error {
					selection, err := patternSelection(ctx)
					if err != nil {
						return err
					}
					dir := strings.TrimSpace(ctx.String(flagDir))
					if dir == "" {
						return ConfigError(fmt.Errorf("export pattern: --dir names the registry tree to publish into"))
					}
					result, err := commands.Export.Execute(application.ExportPatternRequest{
						Selection: selection,
						Dir:       dir,
					})
					if err != nil {
						return ConfigError(err)
					}
					if err := render.Render(ctx.Stdout, PatternExportReport{Result: result}); err != nil {
						return fmt.Errorf("write output: %w", err)
					}
					return nil
				},
			},
		},
	}
}

// PatternSourceLabel names where a listed Pattern resolves from and
// what the repository carries of it, so every renderer spells the same
// words: embedded in the binary, vendored or authored under
// .arclint/patterns, both when the repository carries its own copy of
// an embedded Pattern, or published by a Registry.
func PatternSourceLabel(row application.PatternSummary) string {
	local := ""
	switch {
	case row.Vendored:
		local = "vendored"
	case row.Authored:
		local = "authored"
	}
	switch row.Source {
	case distribution.SourceEmbedded:
		if local != "" {
			return "embedded, " + local
		}
		return "embedded"
	case distribution.SourceLocal:
		if local == "" {
			return "local"
		}
		return local
	case distribution.SourceRegistry:
		return "registry"
	default:
		return string(row.Source)
	}
}

// ShortDigest is the leading twelve hex digits of a published digest
// spelling, for listings; the full spelling stays in JSON.
func ShortDigest(digest string) string {
	if i := strings.IndexByte(digest, ':'); i >= 0 {
		digest = digest[i+1:]
	}
	if len(digest) > 12 {
		return digest[:12]
	}
	return digest
}

// patternSelection reads the one positional argument the pattern
// subcommands take: a reference, namespace/name, or bare name.
func patternSelection(ctx Context) (string, error) {
	if len(ctx.Args) != 1 || strings.TrimSpace(ctx.Args[0]) == "" {
		return "", ConfigError(fmt.Errorf("a pattern is required: namespace/name@version, namespace/name, or a name (arclint patterns lists them)"))
	}
	return strings.TrimSpace(ctx.Args[0]), nil
}

// splitList splits a comma-separated flag value, dropping blanks.
func splitList(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// completePatternSelections offers every offline Pattern by exact
// reference for the subcommands' positional argument. Completion
// degrades to no candidates when a source fails: the shell callback
// has nowhere to report the error.
func completePatternSelections(list application.ListPatterns) func(args []string, toComplete string) []AutoCompleteCandidate {
	return func(args []string, _ string) []AutoCompleteCandidate {
		if len(args) > 0 {
			return nil
		}
		rows, err := list.Execute()
		if err != nil {
			return nil
		}
		candidates := make([]AutoCompleteCandidate, 0, len(rows))
		for _, row := range rows {
			doc := string(row.Source)
			if row.Documentation != "" {
				doc += ": " + row.Documentation
			}
			candidates = append(candidates, AutoCompleteCandidate{
				Value: row.Namespace + "/" + row.Name + "@" + row.Version,
				Doc:   doc,
			})
		}
		return candidates
	}
}
