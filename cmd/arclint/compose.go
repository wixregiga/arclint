// The ARCLINT_V2 feature flag routes the process into the target
// architecture. This composition root selects the concrete outbound
// Adapters, constructs the use cases, selects the Cobra CLI through
// the sealed factory, and runs: no Rule behavior, no use-case
// orchestration.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/term"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	clifactory "github.com/wixregiga/arclint/internal/delivery/cli/factory"
	"github.com/wixregiga/arclint/internal/delivery/cli/reportfactory"
	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	agenthooks "github.com/wixregiga/arclint/internal/infrastructure/agents/hooks"
	markdownagents "github.com/wixregiga/arclint/internal/infrastructure/agents/markdown"
	artifactfs "github.com/wixregiga/arclint/internal/infrastructure/artifact"
	jsonbaseline "github.com/wixregiga/arclint/internal/infrastructure/baseline/json"
	sobekextension "github.com/wixregiga/arclint/internal/infrastructure/extension/sobek"
	"github.com/wixregiga/arclint/internal/infrastructure/filechange"
	golangfacts "github.com/wixregiga/arclint/internal/infrastructure/language/golang"
	pythonfacts "github.com/wixregiga/arclint/internal/infrastructure/language/python"
	typescriptfacts "github.com/wixregiga/arclint/internal/infrastructure/language/typescript"
	filesystemobservation "github.com/wixregiga/arclint/internal/infrastructure/observation/filesystem"
	embeddedpattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/embedded"
	filesystempattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/filesystem"
	registrypattern "github.com/wixregiga/arclint/internal/infrastructure/pattern/registry"
	yamlrule "github.com/wixregiga/arclint/internal/infrastructure/rule/yaml"
	"github.com/wixregiga/arclint/internal/infrastructure/ruletest"
	filesystemscaffold "github.com/wixregiga/arclint/internal/infrastructure/scaffold/filesystem"
	vocabfs "github.com/wixregiga/arclint/internal/infrastructure/vocab"
	cmlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/cml"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

func run(args []string) int {
	configError := func(err error) int {
		fmt.Fprintln(os.Stderr, "arclint: "+err.Error())
		return cli.ExitConfigError
	}
	// Process presentation is resolved before any other pre-Cobra
	// peeling so every command shares one Renderer selection.
	rendererName, args, err := resolvePresentation(args, os.Getenv("NO_COLOR"), term.IsTerminal(os.Stdout.Fd()))
	if err != nil {
		return configError(err)
	}
	renderer, err := reportfactory.Select(rendererName)
	if err != nil {
		return configError(err)
	}
	// init runs before any ruleset exists, so it composes against the
	// working directory instead of a discovered repository root.
	if firstPositional(args) == "init" {
		return runInit(args, renderer, configError)
	}
	rulesPath, err := discoverRulesPath(args)
	if err != nil {
		return configError(err)
	}
	// The recorded vocabulary is an input to rule configuration
	// (expanded Rules resolve against it), so the vocabulary repository
	// composes before the rule repository that consumes it.
	absRulesPath, err := filepath.Abs(rulesPath)
	if err != nil {
		return configError(err)
	}
	root := filepath.Dir(absRulesPath)
	recording, ignored, knowledge, err := composeKnowledge(root, absRulesPath)
	if err != nil {
		if !runsWithoutRuleset(args) {
			return configError(err)
		}
		// Help, completion, the version, the patterns commands and the
		// workflow hooks run in a project whose ruleset or domain file is
		// broken. The hooks still need to know which file the domain is
		// recorded in, so a DomainRecording the ruleset names or discovery
		// settled on is kept even when its file cannot be opened; only when
		// none was settled do they take the default. Reading the language
		// reports what is wrong.
		openErr := err
		if recording.IsZero() {
			if recording, err = vocab.DefaultDomainRecording(); err != nil {
				return configError(err)
			}
		}
		ignored = nil
		knowledge = unopenedRecording{recording: recording, err: openErr}
	}
	// Every command a person or an agent runs says which default-named
	// files it does not read, on stderr, so the output it prints stays what
	// it is. A workflow hook event says nothing: hosts do not show a
	// successful hook's stderr, and the commands the agent runs say it.
	if !hookEvent(args) {
		for _, warning := range ignoredDomainFileWarnings(recording, ignored) {
			fmt.Fprintln(os.Stderr, warning)
		}
	}
	// Patterns a ruleset may extend come from the binary and from the
	// repository's own .arclint/patterns; the rule repository resolves
	// extends through both.
	builtinPatterns := embeddedpattern.NewSource()
	localPatterns, err := filesystempattern.NewSource(filepath.Join(root, ".arclint", "patterns"))
	if err != nil {
		return configError(err)
	}
	repository, err := yamlrule.NewRepository(absRulesPath, knowledge, builtinPatterns, localPatterns)
	if err != nil {
		return configError(err)
	}
	observations, err := filesystemobservation.NewSource(root,
		golangfacts.NewProducer(), typescriptfacts.NewProducer(), pythonfacts.NewProducer())
	if err != nil {
		return configError(err)
	}
	// Extension sources the extended Patterns distribute reach the host
	// from the loaded ruleset, never from a copy on disk.
	extensions, err := sobekextension.NewEvaluator(root, func() ([]rule.ConfiguredExtension, error) {
		cfg, err := repository.ConfiguredRules()
		if err != nil {
			return nil, fmt.Errorf("pattern extensions: %w", err)
		}
		return cfg.Extensions, nil
	})
	if err != nil {
		return configError(err)
	}
	baselines, err := jsonbaseline.NewStore(root)
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
	patternCommands, err := composePatternCommands(root, absRulesPath, builtinPatterns, localPatterns)
	if err != nil {
		return configError(err)
	}
	getContext, err := application.NewGetArchitecturalContext(repository, knowledge)
	if err != nil {
		return configError(err)
	}
	getContext = getContext.WithObservations(observations)
	publisher, err := markdownagents.NewPublisher(root)
	if err != nil {
		return configError(err)
	}
	publishAgents, err := application.NewPublishAgentsContext(repository, knowledge, extensions, publisher)
	if err != nil {
		return configError(err)
	}
	inspectAgents, err := application.NewInspectAgentsContext(publishAgents, publisher)
	if err != nil {
		return configError(err)
	}
	artifactWriter, err := artifactfs.NewWriter(root)
	if err != nil {
		return configError(err)
	}
	publishSkillProtocol, err := application.NewPublishSkillProtocol(artifactWriter)
	if err != nil {
		return configError(err)
	}
	publishSkillVocabulary, err := application.NewPublishSkillVocabulary(artifactWriter)
	if err != nil {
		return configError(err)
	}
	publishDomainSchema, err := application.NewPublishDomainSchema(artifactWriter)
	if err != nil {
		return configError(err)
	}
	publishRuleSchema, err := application.NewPublishRuleSchema(artifactWriter)
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
		extensions, domainRecordingParser{}, yamlrule.RecordingParser{})
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
	initialize, err := application.NewInitializeRepository(scaffoldWriter, builtinPatterns, localPatterns)
	if err != nil {
		return configError(err)
	}
	// A model initialized or first defined here is named for the
	// repository directory until the project records its own name.
	project := filepath.Base(root)
	initDomain, err := application.NewInitDomain(knowledge, project)
	if err != nil {
		return configError(err)
	}
	getDomainOverview, err := application.NewGetDomainOverview(knowledge)
	if err != nil {
		return configError(err)
	}
	getDomainOverview = getDomainOverview.WithObservations(repository, observations)
	listDomainDefinitions, err := application.NewListDomainDefinitions(knowledge)
	if err != nil {
		return configError(err)
	}
	showDomainDefinition, err := application.NewShowDomainDefinition(knowledge)
	if err != nil {
		return configError(err)
	}
	defineDomainDefinition, err := application.NewDefineDomainDefinition(knowledge, project)
	if err != nil {
		return configError(err)
	}
	removeDomainDefinition, err := application.NewRemoveDomainDefinition(knowledge)
	if err != nil {
		return configError(err)
	}
	recordings := domainRecordings{root: root, source: recording}
	exportDomainRecording, err := application.NewExportDomainRecording(knowledge, recordings)
	if err != nil {
		return configError(err)
	}
	rulesetEditor, err := yamlrule.NewEditor(absRulesPath)
	if err != nil {
		return configError(err)
	}
	convertDomainRecording, err := application.NewConvertDomainRecording(exportDomainRecording, recordings, rulesetEditor)
	if err != nil {
		return configError(err)
	}
	progressStore := agenthooks.NewProgressStore(root, recording.Path())
	guideWorkflow, err := application.NewGuideWorkflow(progressStore, repository, knowledge, agenthooks.TestConventions{})
	if err != nil {
		return configError(err)
	}
	// A Windows Agent Host reaches a WSL project through wsl.exe and
	// reports its paths through the \\wsl.localhost share.
	wslDistro := os.Getenv("WSL_DISTRO_NAME")
	// The hooks run this binary by its path; one that cannot be located
	// fails install and status with the reason, not every other command.
	binary, err := agenthooks.InvokedBinary(os.Args[0])
	if err != nil {
		binary = ""
	}
	hookEvents, err := agenthooks.NewEvents(root, absRulesPath, recording.Path(), wslDistro, binary, guideWorkflow, progressStore)
	if err != nil {
		return configError(err)
	}
	installer := agenthooks.NewInstaller(root, wslDistro, binary, agenthooks.UserConfigurations(wslDistro))
	hooksCommand := cli.NewHooksCommand(installer, hookEvents, renderer)
	rootCommand := cli.Root(buildVersion(version),
		cli.NewCheckCommand(assess, listRules, inspectAgents, renderer),
		cli.NewInitCommand(initialize, renderer),
		cli.NewRulesCommand(listRules, showRule, ruleTests, publishRuleSchema, renderer),
		cli.NewContextCommand(getContext, renderer),
		cli.NewDomainCommand(initDomain, getDomainOverview, listDomainDefinitions, showDomainDefinition, defineDomainDefinition, removeDomainDefinition, publishDomainSchema, exportDomainRecording, convertDomainRecording, recording.Path(), renderer),
		cli.NewAgentsCommand(publishAgents, publishSkillProtocol, publishSkillVocabulary, publishDomainSchema, renderer, hooksCommand),
		cli.NewBaselineCommand(capture, refresh, renderer),
		cli.NewPatternsCommand(patternCommands, renderer),
		cli.NewSDKCommand(initializeSDK, renderer),
	)
	// Context, the full check and domain changes report themselves to the
	// workflow hooks, which then need not guess from shell text.
	rootCommand = cli.RecordHookActivity(rootCommand, agenthooks.NewActivityLog(root, recording.Path()))
	adapter, err := clifactory.Select(cli.AdapterCobra)
	if err != nil {
		return configError(err)
	}
	outcome := adapter.Run(rootCommand, cli.Invocation{Args: args, Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
	return outcome.ExitCode
}

// composeKnowledge resolves the project's DomainRecording and opens the
// vocabulary repository that reads and writes it. The ruleset is read
// first because it may name the DomainRecording and because the
// vocabulary is an input to loading it. It also returns the
// default-named files that exist but are not the DomainRecording.
func composeKnowledge(root, rulesPath string) (vocab.DomainRecording, []vocab.DomainRecording, vocab.Repository, error) {
	named, err := yamlrule.ReadRecording(rulesPath)
	if err != nil {
		return vocab.DomainRecording{}, nil, nil, fmt.Errorf("domain file: %w", err)
	}
	recordings := domainRecordings{root: root}
	recording, ignored, err := vocab.ResolveDomainRecording(named, recordings.Exists)
	if err != nil {
		return vocab.DomainRecording{}, nil, nil, fmt.Errorf("domain file: %w", err)
	}
	knowledge, err := recordings.Open(recording)
	if err != nil {
		return recording, ignored, nil, err
	}
	return recording, ignored, knowledge, nil
}

// unopenedRecording stands for a project's DomainRecording whose file
// could not be opened, for the commands that run without the project's
// domain: it names the file, and reading or writing the language
// reports why it could not be opened.
type unopenedRecording struct {
	recording vocab.DomainRecording
	err       error
}

func (u unopenedRecording) Recording() vocab.DomainRecording { return u.recording }

func (u unopenedRecording) RecordedLanguage() (vocab.UbiquitousLanguage, bool, error) {
	return vocab.UbiquitousLanguage{}, false, fmt.Errorf("domain file %s: %w", u.recording.Path(), u.err)
}

func (u unopenedRecording) Record(vocab.UbiquitousLanguage) error {
	return fmt.Errorf("domain file %s: %w", u.recording.Path(), u.err)
}

// ignoredDomainFileWarnings tells, once per ignored file, that the
// project keeps a default-named file beside its DomainRecording which no
// command reads, and how to settle it.
func ignoredDomainFileWarnings(recording vocab.DomainRecording, ignored []vocab.DomainRecording) []string {
	warnings := make([]string, 0, len(ignored))
	for _, file := range ignored {
		warnings = append(warnings, fmt.Sprintf("arclint: %s is ignored; the project's domain file is %s. Name the one to use under domain.file in %s, or remove the other.",
			file.Path(), recording.Path(), rule.RulesetFileName))
	}
	return warnings
}

// domainRecordings implements application.DomainRecordings: it selects
// the vocabulary adapter for a DomainRecording by the format the
// DomainRecording is written in, tells whether a DomainRecording's file
// exists, and removes it. root holds rules.arclint.yaml. source is zero
// for the project's own domain file; for a translation it is that file,
// and every DomainRecording opened is the target a person named.
type domainRecordings struct {
	root   string
	source vocab.DomainRecording
}

// Open returns the vocabulary repository of one DomainRecording. A file
// that reaches outside the repository, or reaches rules.arclint.yaml,
// through a link is refused, as the same path spelled out is. A
// translation target refused that way, or one that reaches the
// project's domain file, is a usage error of the command that named it.
func (d domainRecordings) Open(recording vocab.DomainRecording) (vocab.Repository, error) {
	location, err := d.locate(recording)
	if err != nil {
		if !d.source.IsZero() && (errors.Is(err, vocabfs.ErrRecordingLocation) || errors.Is(err, yamlrule.ErrDomainFileIsRuleset)) {
			return nil, fmt.Errorf("%w: %w", application.ErrDomainUsage, err)
		}
		return nil, err
	}
	if !d.source.IsZero() {
		from, err := vocabfs.ResolveRecording(d.root, d.source)
		if err != nil {
			return nil, fmt.Errorf("resolve source domain recording: %w", err)
		}
		same, err := sameFile(location, from)
		if err != nil {
			return nil, err
		}
		if same {
			return nil, fmt.Errorf("%w: %s resolves to the project's domain file %s; name another file", application.ErrDomainUsage, recording.Path(), d.source.Path())
		}
	}
	switch recording.Format() {
	case vocab.DomainRecordingYAML:
		knowledge, err := yamlvocab.NewRepository(d.root, recording)
		if err != nil {
			return nil, fmt.Errorf("domain file: %w", err)
		}
		return knowledge, nil
	case vocab.DomainRecordingCML:
		knowledge, err := cmlvocab.NewRepository(d.root, recording)
		if err != nil {
			return nil, fmt.Errorf("domain file: %w", err)
		}
		return knowledge, nil
	default:
		return nil, fmt.Errorf("domain file %s: no vocabulary adapter reads a %q recording", recording.Path(), recording.Format())
	}
}

// locate resolves a DomainRecording's file through every link, refusing
// a location outside the repository or at rules.arclint.yaml. A refusal
// names the invariant it applies; any other failure says what could not
// be resolved.
func (d domainRecordings) locate(recording vocab.DomainRecording) (string, error) {
	location, err := vocabfs.ResolveRecording(d.root, recording)
	if errors.Is(err, vocabfs.ErrRecordingLocation) {
		return "", fmt.Errorf("%w", err)
	}
	if err != nil {
		return "", fmt.Errorf("resolve domain recording: %w", err)
	}
	if err := yamlrule.CheckDomainFileLocation(filepath.Join(d.root, rule.RulesetFileName), recording, location); err != nil {
		return "", fmt.Errorf("%w", err)
	}
	return location, nil
}

// sameFile reports whether two resolved locations are one file. Equal
// paths are; otherwise, when both exist, the file system decides, so a
// hard link or a name that differs only in case on a case-insensitive
// file system counts as the same file. A location with no file yet is
// no other file.
func sameFile(a, b string) (bool, error) {
	if a == b {
		return true, nil
	}
	first, err := os.Stat(a)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect %s: %w", a, err)
	}
	second, err := os.Stat(b)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect %s: %w", b, err)
	}
	return os.SameFile(first, second), nil
}

// domainRecordingParser implements application.VocabularySource for the
// rule-test harness: a fixture's domain file is parsed by the adapter of
// the format its DomainRecording is written in.
type domainRecordingParser struct{}

// ParseUbiquitousLanguage parses one fixture domain file.
func (domainRecordingParser) ParseUbiquitousLanguage(content []byte, recording vocab.DomainRecording) (vocab.UbiquitousLanguage, error) {
	var (
		lang vocab.UbiquitousLanguage
		err  error
	)
	switch recording.Format() {
	case vocab.DomainRecordingYAML:
		lang, err = yamlvocab.Parser{}.ParseUbiquitousLanguage(content, recording)
	case vocab.DomainRecordingCML:
		lang, err = cmlvocab.Parser{}.ParseUbiquitousLanguage(content, recording)
	default:
		return vocab.UbiquitousLanguage{}, fmt.Errorf("domain file %s: no vocabulary adapter reads a %q recording", recording.Path(), recording.Format())
	}
	if err != nil {
		// No prefix: the adapter's message already names the domain file.
		return vocab.UbiquitousLanguage{}, fmt.Errorf("%w", err)
	}
	return lang, nil
}

// Exists reports whether a DomainRecording's file is there: a file that
// is absent, or a directory in its place, is not.
func (d domainRecordings) Exists(recording vocab.DomainRecording) (bool, error) {
	location, err := vocabfs.ResolveRecording(d.root, recording)
	if err != nil {
		return false, fmt.Errorf("resolve domain recording: %w", err)
	}
	info, err := os.Stat(location)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("%w", err)
	}
	return !info.IsDir(), nil
}

// PlanRecord works out, in memory, the file a DomainRecording would
// hold with lang written into it, through the adapter of its format, and
// reads that file back. Nothing is written; the change writes it.
func (d domainRecordings) PlanRecord(recording vocab.DomainRecording, lang vocab.UbiquitousLanguage) (application.PlannedRecording, error) {
	location, err := d.locate(recording)
	if err != nil {
		return application.PlannedRecording{}, err
	}
	var (
		data     []byte
		readBack vocab.UbiquitousLanguage
	)
	switch recording.Format() {
	case vocab.DomainRecordingYAML:
		repo, err := yamlvocab.NewRepository(d.root, recording)
		if err != nil {
			return application.PlannedRecording{}, fmt.Errorf("domain file: %w", err)
		}
		if data, err = repo.Draft(lang); err != nil {
			return application.PlannedRecording{}, fmt.Errorf("%w", err)
		}
		readBack, err = yamlvocab.Parse(data, recording)
		if err != nil {
			return application.PlannedRecording{}, fmt.Errorf("read back %s: %w", recording.Path(), err)
		}
	case vocab.DomainRecordingCML:
		repo, err := cmlvocab.NewRepository(d.root, recording)
		if err != nil {
			return application.PlannedRecording{}, fmt.Errorf("domain file: %w", err)
		}
		if data, err = repo.Draft(lang); err != nil {
			return application.PlannedRecording{}, fmt.Errorf("%w", err)
		}
		readBack, err = cmlvocab.Parse(data, recording)
		if err != nil {
			return application.PlannedRecording{}, fmt.Errorf("read back %s: %w", recording.Path(), err)
		}
	default:
		return application.PlannedRecording{}, fmt.Errorf("domain file %s: no vocabulary adapter writes a %q recording", recording.Path(), recording.Format())
	}
	change, err := filechange.Write(location, data)
	if err != nil {
		return application.PlannedRecording{}, fmt.Errorf("%w", err)
	}
	return application.PlannedRecording{Change: change, Language: readBack}, nil
}

// PlanRemoval works out removing a DomainRecording's file. The
// recording's directory entry is removed, a file symlink included, so a
// conversion cannot leave a dangling default-named recording; putting
// it back restores the same entry. Nothing is removed until the change
// is applied.
func (d domainRecordings) PlanRemoval(recording vocab.DomainRecording) (application.FileChange, error) {
	if _, err := vocabfs.ResolveRecording(d.root, recording); err != nil {
		return nil, fmt.Errorf("resolve domain recording: %w", err)
	}
	change, err := filechange.Remove(filepath.Join(d.root, filepath.FromSlash(recording.Path())))
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return change, nil
}

// composePatternCommands wires the Pattern distribution use cases: the
// offline sources every command resolves through, the Registry client
// only the patterns commands may call, the vendored store under
// .arclint/patterns, and the ruleset editor for install. The Registry
// location defaults from ARCLINT_REGISTRY, then arclint's own Registry;
// a private Registry authenticates with GITHUB_TOKEN or GH_TOKEN.
func composePatternCommands(root, rulesPath string, builtinPatterns embeddedpattern.Source, localPatterns filesystempattern.Source) (cli.PatternCommands, error) {
	registry := registrypattern.NewClient(registryToken())
	listPatterns, err := application.NewListPatterns(registry, builtinPatterns, localPatterns)
	if err != nil {
		return cli.PatternCommands{}, fmt.Errorf("compose patterns: %w", err)
	}
	vendorPattern, err := application.NewVendorPattern(localPatterns, registry, builtinPatterns, localPatterns)
	if err != nil {
		return cli.PatternCommands{}, fmt.Errorf("compose patterns vendor: %w", err)
	}
	editor, err := yamlrule.NewEditor(rulesPath)
	if err != nil {
		return cli.PatternCommands{}, fmt.Errorf("compose patterns install: %w", err)
	}
	scaffold, err := filesystemscaffold.NewWriter(root)
	if err != nil {
		return cli.PatternCommands{}, fmt.Errorf("compose patterns install: %w", err)
	}
	installPattern, err := application.NewInstallPattern(localPatterns, editor, scaffold, registry, builtinPatterns, localPatterns)
	if err != nil {
		return cli.PatternCommands{}, fmt.Errorf("compose patterns install: %w", err)
	}
	exportPattern, err := application.NewExportPattern(registrypattern.NewPublisher(), builtinPatterns, localPatterns)
	if err != nil {
		return cli.PatternCommands{}, fmt.Errorf("compose patterns export: %w", err)
	}
	return cli.PatternCommands{
		List:     listPatterns,
		Vendor:   vendorPattern,
		Install:  installPattern,
		Export:   exportPattern,
		Registry: registryLocation(),
	}, nil
}

// registryLocation is the default --registry value: ARCLINT_REGISTRY
// when set, otherwise arclint's own published Registry.
func registryLocation() string {
	if location := strings.TrimSpace(os.Getenv("ARCLINT_REGISTRY")); location != "" {
		return location
	}
	return distribution.DefaultRegistryLocation
}

// registryToken is the bearer token a private Registry is read with:
// GITHUB_TOKEN first, then GH_TOKEN, matching the GitHub tooling that
// serves raw repository content.
func registryToken() string {
	for _, name := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if token := strings.TrimSpace(os.Getenv(name)); token != "" {
			return token
		}
	}
	return ""
}

// firstPositional returns the first non-flag argument, skipping
// composition-level process flags (--format, --no-color).
func firstPositional(args []string) string {
	skipNext := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		switch {
		case skipNext:
			skipNext = false
		case a == "--format":
			skipNext = true
		case strings.HasPrefix(a, "--format="),
			a == "--no-color",
			strings.HasPrefix(a, "--no-color="):
		case strings.HasPrefix(a, "-"):
		default:
			return a
		}
	}
	return ""
}

// runInit composes the initialization use case against the working
// directory.
func runInit(args []string, renderer cli.Renderer, configError func(error) int) int {
	cwd, err := os.Getwd()
	if err != nil {
		return configError(err)
	}
	writer, err := filesystemscaffold.NewWriter(cwd)
	if err != nil {
		return configError(err)
	}
	localPatterns, err := filesystempattern.NewSource(filepath.Join(cwd, ".arclint", "patterns"))
	if err != nil {
		return configError(err)
	}
	initialize, err := application.NewInitializeRepository(writer, embeddedpattern.NewSource(), localPatterns)
	if err != nil {
		return configError(err)
	}
	adapter, err := clifactory.Select(cli.AdapterCobra)
	if err != nil {
		return configError(err)
	}
	rootCommand := cli.Root(buildVersion(version), cli.NewInitCommand(initialize, renderer))
	outcome := adapter.Run(rootCommand, cli.Invocation{Args: args, Stdout: os.Stdout, Stderr: os.Stderr})
	return outcome.ExitCode
}

// discoverRulesPath finds the project's ruleset before any adapter
// exists: the composition root needs the repository root to wire
// concrete Adapters. The ruleset is rules.arclint.yaml, discovered
// upward from the working directory; `check [path]` selects the
// repository by path, and discovery walks upward from it.
func discoverRulesPath(args []string) (string, error) {
	start := "."
	if len(args) > 1 && args[0] == "check" && !strings.HasPrefix(args[1], "-") {
		start = args[1]
	}
	discovered, err := yamlrule.DiscoverPath(start, rule.RulesetFileName)
	if err != nil {
		// No subcommand means --version or --help: neither needs a
		// repository, and construction alone never loads the ruleset.
		// The completion machinery must answer without one too: the
		// shell re-executes the binary on TAB in arbitrary directories,
		// and completion callbacks degrade to no candidates when the
		// ruleset fails to load. The patterns commands list, vendor,
		// and export without a ruleset, and install drafts one where
		// none exists, so they compose against the working directory.
		if runsWithoutRuleset(args) {
			fallback, absErr := filepath.Abs(rule.RulesetFileName)
			if absErr != nil {
				return "", fmt.Errorf("rules path: %w", absErr)
			}
			return fallback, nil
		}
		return "", fmt.Errorf("resolve ruleset: %w", err)
	}
	return discovered, nil
}

// runsWithoutRuleset reports the commands that answer without a usable
// ruleset: no subcommand (the version or help), help, completion, the
// patterns commands, and the workflow hooks.
// hookEvent reports `arclint agents hooks event`, the command a host's
// workflow hook runs.
func hookEvent(rest []string) bool {
	return len(rest) >= 3 && rest[0] == "agents" && rest[1] == "hooks" && rest[2] == "event"
}

func runsWithoutRuleset(rest []string) bool {
	fp := firstPositional(rest)
	return fp == "" || fp == "help" || fp == "completion" ||
		fp == "__complete" || fp == "__completeNoDesc" || fp == "patterns" ||
		(len(rest) >= 2 && rest[0] == "agents" && rest[1] == "hooks")
}
