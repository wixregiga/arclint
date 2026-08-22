// The ARCLINT_V2 feature flag routes the process into the target
// architecture. This composition root selects the concrete outbound
// Adapters, constructs the use cases, selects the Cobra CLI through
// the sealed factory, and runs — no Rule behavior, no use-case
// orchestration.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	clifactory "github.com/wixregiga/arclint/internal/delivery/cli/factory"
	markdownagents "github.com/wixregiga/arclint/internal/infrastructure/agents/markdown"
	jsonbaseline "github.com/wixregiga/arclint/internal/infrastructure/baseline/json"
	sobekextension "github.com/wixregiga/arclint/internal/infrastructure/extension/sobek"
	golangfacts "github.com/wixregiga/arclint/internal/infrastructure/language/golang"
	pythonfacts "github.com/wixregiga/arclint/internal/infrastructure/language/python"
	typescriptfacts "github.com/wixregiga/arclint/internal/infrastructure/language/typescript"
	filesystemobservation "github.com/wixregiga/arclint/internal/infrastructure/observation/filesystem"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
	filesystempattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/filesystem"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
	"github.com/wixregiga/arclint/internal/infrastructure/ruletest"
	filesystemscaffold "github.com/wixregiga/arclint/internal/infrastructure/scaffold/filesystem"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

func run(args []string) int {
	configError := func(err error) int {
		fmt.Fprintln(os.Stderr, "arclint: "+err.Error())
		return cli.ExitConfigError
	}
	// init runs before any ruleset exists, so it composes against the
	// working directory instead of a discovered repository root.
	if firstPositional(args) == "init" {
		return runInit(args, configError)
	}
	rulesPath, rest, err := resolveRulesPath(args)
	if err != nil {
		return configError(err)
	}
	repository, err := yamlrule.NewRepository(rulesPath)
	if err != nil {
		return configError(err)
	}
	root := repository.Root()
	observations, err := filesystemobservation.NewSource(root,
		golangfacts.NewProducer(), typescriptfacts.NewProducer(), pythonfacts.NewProducer())
	if err != nil {
		return configError(err)
	}
	extensions, err := sobekextension.NewEvaluator(root)
	if err != nil {
		return configError(err)
	}
	baselines, err := jsonbaseline.NewStore(root)
	if err != nil {
		return configError(err)
	}
	patterns, err := filesystempattern.NewSource(filepath.Join(root, ".arclint", "patterns"))
	if err != nil {
		return configError(err)
	}
	knowledge, err := yamlvocab.NewRepository(root)
	if err != nil {
		return configError(err)
	}

	listRules, err := application.NewListRules(repository)
	if err != nil {
		return configError(err)
	}
	showRule, err := application.NewShowRule(repository)
	if err != nil {
		return configError(err)
	}
	assess, err := application.NewAssessConformance(repository, observations, baselines, extensions, knowledge)
	if err != nil {
		return configError(err)
	}
	capture, err := application.NewCaptureBaseline(assess, baselines)
	if err != nil {
		return configError(err)
	}
	refresh, err := application.NewRefreshBaseline(assess, baselines, baselines)
	if err != nil {
		return configError(err)
	}
	listPatterns, err := application.NewListPatterns(embeddedpattern.NewSource(), patterns)
	if err != nil {
		return configError(err)
	}
	getContext, err := application.NewGetArchitecturalContext(repository, knowledge)
	if err != nil {
		return configError(err)
	}
	publisher, err := markdownagents.NewPublisher(root)
	if err != nil {
		return configError(err)
	}
	publishAgents, err := application.NewPublishAgentsContext(getContext, publisher)
	if err != nil {
		return configError(err)
	}
	sdkWriter, err := sobekextension.NewSDKWriter(root)
	if err != nil {
		return configError(err)
	}
	initializeSDK, err := application.NewInitializeExtensionSDK(sdkWriter)
	if err != nil {
		return configError(err)
	}
	ruleTests, err := application.NewRunRuleTests(repository, ruletest.NewSource(root),
		ruletest.NewObserver(golangfacts.NewProducer(), typescriptfacts.NewProducer(), pythonfacts.NewProducer()),
		extensions, yamlvocab.Parser{})
	if err != nil {
		return configError(err)
	}
	// The main root carries init as well, so help and completion list
	// it; execution still routes through runInit above, before any
	// ruleset is required.
	cwd, err := os.Getwd()
	if err != nil {
		return configError(err)
	}
	scaffoldWriter, err := filesystemscaffold.NewWriter(cwd)
	if err != nil {
		return configError(err)
	}
	initialize, err := application.NewInitializeRepository(scaffoldWriter, embeddedpattern.NewSource())
	if err != nil {
		return configError(err)
	}
	getDomainOverview, err := application.NewGetDomainOverview(knowledge)
	if err != nil {
		return configError(err)
	}
	listDomainDefinitions, err := application.NewListDomainDefinitions(knowledge)
	if err != nil {
		return configError(err)
	}
	showDomainDefinition, err := application.NewShowDomainDefinition(knowledge)
	if err != nil {
		return configError(err)
	}
	defineDomainDefinition, err := application.NewDefineDomainDefinition(knowledge)
	if err != nil {
		return configError(err)
	}
	removeDomainDefinition, err := application.NewRemoveDomainDefinition(knowledge)
	if err != nil {
		return configError(err)
	}
	rootCommand := cli.Root(buildVersion(version),
		cli.NewCheckCommand(assess, listRules),
		cli.NewInitCommand(initialize),
		cli.NewRulesCommand(listRules, showRule, ruleTests),
		cli.NewContextCommand(getContext),
		cli.NewDomainCommand(getDomainOverview, listDomainDefinitions, showDomainDefinition, defineDomainDefinition, removeDomainDefinition),
		cli.NewAgentsCommand(publishAgents),
		cli.NewBaselineCommand(capture, refresh),
		cli.NewPatternsCommand(listPatterns),
		cli.NewSDKCommand(initializeSDK),
	)
	adapter, err := clifactory.Select(cli.AdapterCobra)
	if err != nil {
		return configError(err)
	}
	outcome := adapter.Run(rootCommand, cli.Invocation{Args: rest, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
	return outcome.ExitCode
}

// firstPositional returns the first non-flag argument, skipping the
// composition-level --rules value.
func firstPositional(args []string) string {
	skipNext := false
	for _, a := range args {
		switch {
		case skipNext:
			skipNext = false
		case a == "--rules":
			skipNext = true
		case strings.HasPrefix(a, "-"):
		default:
			return a
		}
	}
	return ""
}

// runInit composes the initialization use case against the working
// directory.
func runInit(args []string, configError func(error) int) int {
	cwd, err := os.Getwd()
	if err != nil {
		return configError(err)
	}
	writer, err := filesystemscaffold.NewWriter(cwd)
	if err != nil {
		return configError(err)
	}
	initialize, err := application.NewInitializeRepository(writer, embeddedpattern.NewSource())
	if err != nil {
		return configError(err)
	}
	adapter, err := clifactory.Select(cli.AdapterCobra)
	if err != nil {
		return configError(err)
	}
	rootCommand := cli.Root(buildVersion(version), cli.NewInitCommand(initialize))
	outcome := adapter.Run(rootCommand, cli.Invocation{Args: args, Stdout: os.Stdout, Stderr: os.Stderr})
	return outcome.ExitCode
}

// resolveRulesPath resolves --rules from the raw arguments before any
// adapter exists: the composition root needs the repository root to
// wire concrete Adapters, so this one flag is consumed here rather
// than in the neutral command descriptions. Without the flag the
// ruleset is discovered upward from the working directory.
func resolveRulesPath(args []string) (string, []string, error) {
	rest := make([]string, 0, len(args))
	path := ""
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--rules":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--rules requires a path")
			}
			i++
			path = args[i]
		case strings.HasPrefix(args[i], "--rules="):
			path = strings.TrimPrefix(args[i], "--rules=")
		default:
			rest = append(rest, args[i])
		}
	}
	if path != "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", nil, fmt.Errorf("rules path: %w", err)
		}
		return abs, rest, nil
	}
	// `check [path]` selects the repository by path; discovery walks
	// upward from it. Every other command discovers from the working
	// directory.
	start := "."
	if len(rest) > 1 && rest[0] == "check" && !strings.HasPrefix(rest[1], "-") {
		start = rest[1]
	}
	discovered, err := yamlrule.DiscoverPath(start, "rules.yaml")
	if err != nil {
		// No subcommand means --version or --help: neither needs a
		// repository, and construction alone never loads the ruleset.
		// The completion machinery must answer without one too — the
		// shell re-executes the binary on TAB in arbitrary directories,
		// and completion callbacks degrade to no candidates when the
		// ruleset fails to load.
		if fp := firstPositional(rest); fp == "" || fp == "help" || fp == "completion" ||
			fp == "__complete" || fp == "__completeNoDesc" {
			fallback, absErr := filepath.Abs("rules.yaml")
			if absErr != nil {
				return "", nil, fmt.Errorf("rules path: %w", absErr)
			}
			return fallback, rest, nil
		}
		return "", nil, fmt.Errorf("resolve ruleset: %w", err)
	}
	return discovered, rest, nil
}
