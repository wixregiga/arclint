package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/rule"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// NewDomainCommand adapts the project domain-model use cases into the
// domain command family: init, overview (also the bare `domain`
// default), list, show, explain, define (including --guided),
// remove/rm, and schema. Presentation is closed over the injected
// Renderer; raw schema bytes bypass it.
func NewDomainCommand(
	initialize application.InitDomain,
	overview application.GetDomainOverview,
	list application.ListDomainDefinitions,
	show application.ShowDomainDefinition,
	define application.DefineDomainDefinition,
	remove application.RemoveDomainDefinition,
	publishSchema application.PublishDomainSchema,
	render Renderer,
) Command {
	runOverview := overviewRunner(overview, render)
	return Command{
		Name:    "domain",
		Short:   "inspect and maintain the project's ubiquitous language",
		Long:    domainGroupLong,
		MaxArgs: -1,
		Run: func(ctx Context) error {
			if len(ctx.Args) > 0 {
				return ConfigError(fmt.Errorf("unknown command %q for `arclint domain`", ctx.Args[0]))
			}
			return runOverview(ctx)
		},
		Subcommands: []Command{
			{
				Name:    "init",
				Short:   "initialize the project's ubiquitous language file",
				Long:    initDomainLong,
				Example: initDomainExample,
				MaxArgs: 0,
				Flags:   []Flag{{Name: "project", Doc: "name of the software whose domain the file records; the repository directory's name by default"}},
				Run:     initDomainRunner(initialize, render),
			},
			{
				Name:    "overview",
				Short:   "summarize the project's ubiquitous language for understanding",
				Long:    overviewLong,
				Example: overviewExample,
				MaxArgs: 0,
				Run:     runOverview,
			},
			{
				Name:         "list",
				Short:        "list the project's domain definitions",
				Long:         listLong,
				Example:      listExample,
				MaxArgs:      1,
				Flags:        []Flag{contextFlag()},
				CompleteArgs: completeListings(),
				Run:          listRunner(list, render),
			},
			{
				Name:         "show",
				Short:        "show one domain definition",
				Long:         showLong,
				Example:      showExample,
				MaxArgs:      2,
				Flags:        []Flag{contextFlag(), ownerFlag()},
				CompleteArgs: completeShowArgs(list),
				Run:          showRunner(show, render),
			},
			{
				Name:         "explain",
				Short:        "explain ArcLint's supported domain concepts",
				Long:         explainLong,
				Example:      explainExample,
				MaxArgs:      1,
				CompleteArgs: completeConcepts(),
				Run:          explainRunner(render),
			},
			{
				Name:         "define",
				Short:        "create or update a domain definition",
				Long:         defineLong,
				Example:      defineExample,
				MaxArgs:      2,
				Flags:        defineFlags(),
				CompleteArgs: completeShowArgs(list),
				Run:          defineRunner(define, render),
			},
			{
				Name:         "remove",
				Short:        "remove a domain definition",
				Long:         removeLong,
				Example:      removeExample,
				Aliases:      []string{"rm"},
				MaxArgs:      2,
				Flags:        []Flag{contextFlag(), ownerFlag()},
				CompleteArgs: completeShowArgs(list),
				Run:          removeRunner(remove, render),
			},
			newSchemaCommand(
				"print or write the JSON Schema accepted for "+vocab.UbiquitousLanguageFileName,
				schemaLong,
				schemaExample,
				publishSchema,
				render,
			),
		},
	}
}

func contextFlag() Flag {
	return Flag{Name: "context", Doc: "bounded context the entry is recorded in; optional while the project records exactly one"}
}

func ownerFlag() Flag {
	return Flag{Name: "owner", Doc: "the aggregate an entity or assertion belongs to, or the aggregate or value object an invariant belongs to"}
}

func initDomainRunner(initialize application.InitDomain, render Renderer) func(Context) error {
	return func(ctx Context) error {
		result, err := initialize.Execute(ctx.String("project"))
		if err != nil {
			return domainError(err)
		}
		if err := render.Render(ctx.Stdout, DomainInitReport{Result: result}); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}

func domainError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, application.ErrDomainUsage):
		return ConfigError(err)
	default:
		return &ExitError{Code: ExitViolations, Message: err.Error()}
	}
}

func overviewRunner(overview application.GetDomainOverview, render Renderer) func(Context) error {
	return func(ctx Context) error {
		result, err := overview.Execute()
		if err != nil {
			return domainError(err)
		}
		if err := render.Render(ctx.Stdout, DomainOverviewReport{Overview: result}); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}

func listRunner(list application.ListDomainDefinitions, render Renderer) func(Context) error {
	return func(ctx Context) error {
		listing := ""
		if len(ctx.Args) == 1 {
			listing = ctx.Args[0]
		}
		result, err := list.Execute(listing, ctx.String("context"))
		if err != nil {
			return domainError(err)
		}
		if err := render.Render(ctx.Stdout, DomainListReport{Listing: result}); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}

func showRunner(show application.ShowDomainDefinition, render Renderer) func(Context) error {
	return func(ctx Context) error {
		if len(ctx.Args) != 2 {
			return ConfigError(fmt.Errorf("show requires <concept> <name>"))
		}
		result, err := show.Execute(ctx.Args[0], ctx.String("context"), ctx.String("owner"), ctx.Args[1])
		if err != nil {
			return domainError(err)
		}
		if err := render.Render(ctx.Stdout, DomainShowReport{View: result}); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}

func explainRunner(render Renderer) func(Context) error {
	return func(ctx Context) error {
		var docs []vocab.ConceptDoc
		single := false
		if len(ctx.Args) == 0 {
			for _, c := range vocab.Concepts() {
				docs = append(docs, c.Doc())
			}
		} else {
			concept, err := vocab.ParseConcept(ctx.Args[0])
			if err != nil {
				return ConfigError(err)
			}
			docs = []vocab.ConceptDoc{concept.Doc()}
			single = true
		}
		if err := render.Render(ctx.Stdout, DomainExplainReport{Docs: docs, Single: single}); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}

// The flags of `domain define` that record a property. Each maps to
// one recorded property of the meta-model; the concept named on the
// command line decides which of them it takes.
const (
	flagDefinition   = "definition"
	flagStatement    = "statement"
	flagOn           = "on"
	flagText         = "text"
	flagIdentity     = "identity"
	flagAlias        = "alias"
	flagClearAliases = "clear-aliases"
	flagRepository   = "repository"
	flagFactory      = "factory"
	flagRaisedBy     = "raised-by"
)

func defineFlags() []Flag {
	return []Flag{
		contextFlag(),
		ownerFlag(),
		{Name: flagDefinition, Doc: "what the term means in this project (bounded_context, aggregate, entity, value_object, domain_event, domain_service, specification)"},
		{Name: flagStatement, Doc: "what an invariant or assertion holds, in the project's words"},
		{Name: flagOn, Doc: "the operation of the root an assertion is checked on"},
		{Name: flagText, Doc: "the open question, for a question"},
		{Name: flagIdentity, Doc: "the value object that identifies an aggregate's root or an entity"},
		{Name: flagAlias, Repeat: true, Doc: "another name the project uses for the term; may be repeated"},
		{Name: flagClearAliases, Bool: true, Doc: "record the term with no aliases"},
		{Name: flagRepository, Doc: "the declaration that is an aggregate's repository"},
		{Name: flagFactory, Doc: "the declaration that is an aggregate's factory"},
		{Name: flagRaisedBy, Doc: "the aggregate that raises a domain event; an empty value records none"},
		{Name: "guided", Bool: true, Doc: "start an interactive authoring session"},
	}
}

// propertyFlags lists the flags a plain define reads into the change;
// any of them set alongside --guided is a usage error.
var propertyFlags = []string{
	flagDefinition, flagStatement, flagOn, flagText, flagIdentity,
	flagAlias, flagClearAliases,
	flagRepository, flagFactory, flagRaisedBy,
}

// changeFromFlags reads the recording decision off the command line:
// a property flag that was passed is set, even to an empty value; one
// that was not stays untouched.
func changeFromFlags(ctx Context) (vocab.Change, error) {
	if ctx.Changed(flagAlias) && ctx.Bool(flagClearAliases) {
		return vocab.Change{}, ConfigError(fmt.Errorf("--%s and --%s cannot be combined", flagAlias, flagClearAliases))
	}
	ch := vocab.Change{}
	if ctx.Changed(flagDefinition) {
		ch.SetDefinition, ch.Definition = true, ctx.String(flagDefinition)
	}
	if ctx.Changed(flagStatement) {
		ch.SetStatement, ch.Statement = true, ctx.String(flagStatement)
	}
	if ctx.Changed(flagOn) {
		ch.SetOn, ch.On = true, ctx.String(flagOn)
	}
	if ctx.Changed(flagText) {
		ch.SetText, ch.Text = true, ctx.String(flagText)
	}
	if ctx.Changed(flagIdentity) {
		ch.SetIdentity, ch.Identity = true, ctx.String(flagIdentity)
	}
	if ctx.Changed(flagAlias) {
		ch.SetAliases, ch.Aliases = true, ctx.Strings(flagAlias)
	}
	if ctx.Bool(flagClearAliases) {
		ch.SetAliases, ch.Aliases = true, nil
	}
	if ctx.Changed(flagRepository) {
		ch.SetRepository, ch.Repository = true, ctx.String(flagRepository)
	}
	if ctx.Changed(flagFactory) {
		ch.SetFactory, ch.Factory = true, ctx.String(flagFactory)
	}
	if ctx.Changed(flagRaisedBy) {
		ch.SetRaisedBy, ch.RaisedBy = true, ctx.String(flagRaisedBy)
	}
	return ch, nil
}

func defineRunner(define application.DefineDomainDefinition, render Renderer) func(Context) error {
	return func(ctx Context) error {
		if ctx.Bool("guided") {
			combined := len(ctx.Args) > 0 || ctx.Changed("context") || ctx.Changed("owner")
			for _, name := range propertyFlags {
				combined = combined || ctx.Changed(name)
			}
			if combined {
				return ConfigError(fmt.Errorf("--guided cannot be combined with a concept, a name, or property flags"))
			}
			return runGuidedDefine(ctx, define, render)
		}
		if len(ctx.Args) != 2 {
			return ConfigError(fmt.Errorf("define requires <concept> <name>, or --guided"))
		}
		change, err := changeFromFlags(ctx)
		if err != nil {
			return err
		}
		req := application.DefineDomainRequest{
			Concept: ctx.Args[0],
			Context: ctx.String("context"),
			Owner:   ctx.String("owner"),
			Name:    ctx.Args[1],
			Change:  change,
		}
		result, err := define.Execute(req)
		if err != nil {
			return domainError(err)
		}
		if err := render.Render(ctx.Stdout, NewDomainDefineReport(result, req)); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}

func removeRunner(remove application.RemoveDomainDefinition, render Renderer) func(Context) error {
	return func(ctx Context) error {
		if len(ctx.Args) != 2 {
			return ConfigError(fmt.Errorf("remove requires <concept> <name>"))
		}
		result, err := remove.Execute(ctx.Args[0], ctx.String("context"), ctx.String("owner"), ctx.Args[1])
		if err != nil {
			return domainError(err)
		}
		if err := render.Render(ctx.Stdout, DomainRemoveReport{Result: result}); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}

// --- guided authoring -----------------------------------------------------

// guidedOption is one concept the guided session offers, in the order
// a modeler meets them: the context first, then what lives in it.
type guidedOption struct {
	title   string
	concept vocab.Concept
}

func guidedOptions() []guidedOption {
	return []guidedOption{
		{"Bounded Context", vocab.ConceptBoundedContext},
		{"Aggregate", vocab.ConceptAggregate},
		{"Entity", vocab.ConceptEntity},
		{"Value Object", vocab.ConceptValueObject},
		{"Invariant", vocab.ConceptInvariant},
		{"Assertion", vocab.ConceptAssertion},
		{"Domain Event", vocab.ConceptDomainEvent},
		{"Domain Service", vocab.ConceptDomainService},
		{"Specification", vocab.ConceptSpecification},
		{"Question", vocab.ConceptQuestion},
	}
}

// guidedSession reads answers line by line and echoes prompts; an
// exhausted input aborts the session.
type guidedSession struct {
	scanner *bufio.Scanner
	out     io.Writer
}

func (s *guidedSession) prompt(line string) error {
	if _, err := fmt.Fprintln(s.out, line); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func (s *guidedSession) readLine() (string, error) {
	if !s.scanner.Scan() {
		if err := s.scanner.Err(); err != nil {
			return "", fmt.Errorf("read input: %w", err)
		}
		return "", io.EOF
	}
	return strings.TrimSpace(s.scanner.Text()), nil
}

// ask prompts until a non-empty answer arrives, repeating the prompt
// with the reminder when the answer is blank.
func (s *guidedSession) ask(question, reminder string) (string, error) {
	if err := s.prompt(question); err != nil {
		return "", err
	}
	for {
		line, err := s.readLine()
		if err != nil {
			return "", guidedAborted(err)
		}
		if line != "" {
			return line, nil
		}
		if err := s.prompt(reminder); err != nil {
			return "", err
		}
	}
}

// askOptional prompts once and accepts a blank answer.
func (s *guidedSession) askOptional(question string) (string, error) {
	if err := s.prompt(question); err != nil {
		return "", err
	}
	line, err := s.readLine()
	if err != nil {
		return "", guidedAborted(err)
	}
	return line, nil
}

func runGuidedDefine(ctx Context, define application.DefineDomainDefinition, render Renderer) error {
	in := ctx.Stdin
	if in == nil {
		in = strings.NewReader("")
	}
	s := &guidedSession{scanner: bufio.NewScanner(in), out: ctx.Stdout}

	if err := s.prompt("What are you defining?"); err != nil {
		return err
	}
	options := guidedOptions()
	for i, opt := range options {
		if err := s.prompt(fmt.Sprintf("  %d) %s", i+1, opt.title)); err != nil {
			return err
		}
	}
	var chosen guidedOption
	for {
		line, err := s.readLine()
		if err != nil {
			return guidedAborted(err)
		}
		if opt, ok := parseGuidedConcept(line, options); ok {
			chosen = opt
			break
		}
		if err := s.prompt(fmt.Sprintf("Please choose 1-%d or the concept title.", len(options))); err != nil {
			return err
		}
	}
	doc := chosen.concept.Doc()
	for _, line := range []string{"", doc.Title, doc.Meaning, ""} {
		if err := s.prompt(line); err != nil {
			return err
		}
	}

	req := application.DefineDomainRequest{Concept: string(chosen.concept)}
	var summary []string
	var err error
	if chosen.concept != vocab.ConceptBoundedContext {
		req.Context, err = s.ask("Bounded context:", "A bounded context name is required.")
		if err != nil {
			return err
		}
	}
	switch chosen.concept {
	case vocab.ConceptEntity, vocab.ConceptAssertion:
		req.Owner, err = s.ask("Aggregate it belongs to:", "The owning aggregate is required.")
	case vocab.ConceptInvariant:
		req.Owner, err = s.ask("Aggregate or value object it belongs to:", "The owner is required: the aggregate whose root enforces it, or the value object whose constructor does.")
	default:
		// Every other concept belongs to its context alone.
	}
	if err != nil {
		return err
	}
	if req.Owner != "" {
		summary = append(summary, fmt.Sprintf("  Owner: %s", req.Owner))
	}

	switch chosen.concept {
	case vocab.ConceptInvariant, vocab.ConceptAssertion:
		req.Name, err = s.ask("Key (short, kebab-case; the method that enforces it is named after it):", "A key is required.")
	case vocab.ConceptQuestion:
		req.Name, err = s.ask("Key (short, kebab-case):", "A key is required.")
	default:
		req.Name, err = s.ask("Name:", "A name is required.")
	}
	if err != nil {
		return err
	}
	summary = append([]string{fmt.Sprintf("  %s: %s", chosen.title, req.Name)}, summary...)

	switch chosen.concept {
	case vocab.ConceptInvariant, vocab.ConceptAssertion:
		req.Change.Statement, err = s.ask("Statement (what always holds, in the project's words):", "A statement is required.")
		if err != nil {
			return err
		}
		req.Change.SetStatement = true
		summary = append(summary, "  Statement: "+req.Change.Statement)
		if chosen.concept == vocab.ConceptAssertion {
			req.Change.On, err = s.ask("Checked on (the operation of the root):", "The operation is required.")
			if err != nil {
				return err
			}
			req.Change.SetOn = true
			summary = append(summary, "  On: "+req.Change.On)
		}
	case vocab.ConceptQuestion:
		req.Change.Text, err = s.ask("Question:", "The question text is required.")
		if err != nil {
			return err
		}
		req.Change.SetText = true
		summary = append(summary, "  Text: "+req.Change.Text)
	default:
		req.Change.Definition, err = s.ask("Definition (what it means in this project):", "A definition is required.")
		if err != nil {
			return err
		}
		req.Change.SetDefinition = true
		summary = append(summary, "  Definition: "+req.Change.Definition)
	}

	if chosen.concept == vocab.ConceptAggregate {
		req.Change.Identity, err = s.ask("Identity (the value object that identifies the root):", "An identity is required.")
		if err != nil {
			return err
		}
		req.Change.SetIdentity = true
		summary = append(summary, "  Identity: "+req.Change.Identity)
	}
	switch chosen.concept {
	case vocab.ConceptAggregate, vocab.ConceptEntity, vocab.ConceptValueObject:
		line, err := s.askOptional("Aliases (comma-separated, optional):")
		if err != nil {
			return err
		}
		if aliases := splitCommaList(line); len(aliases) > 0 {
			req.Change.SetAliases, req.Change.Aliases = true, aliases
			summary = append(summary, "  Aliases: "+strings.Join(aliases, ", "))
		}
	case vocab.ConceptDomainEvent:
		line, err := s.askOptional("Raised by (the aggregate that raises it, optional):")
		if err != nil {
			return err
		}
		if line != "" {
			req.Change.SetRaisedBy, req.Change.RaisedBy = true, line
			summary = append(summary, "  Raised by: "+line)
		}
	default:
		// Only terms carry aliases and only events name a raiser.
	}

	if err := s.prompt(""); err != nil {
		return err
	}
	if err := s.prompt("Proposed definition:"); err != nil {
		return err
	}
	for _, line := range summary {
		if err := s.prompt(line); err != nil {
			return err
		}
	}
	if err := s.prompt(""); err != nil {
		return err
	}
	if err := s.prompt("Write this definition to " + vocab.UbiquitousLanguageFileName + "? [y/N]"); err != nil {
		return err
	}
	confirm, err := s.readLine()
	if err != nil {
		return guidedAborted(err)
	}
	if !isYes(confirm) {
		return s.prompt("Nothing written.")
	}

	result, err := define.Execute(req)
	if err != nil {
		return domainError(err)
	}
	if err := render.Render(ctx.Stdout, NewDomainDefineReport(result, req)); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func parseGuidedConcept(line string, options []guidedOption) (guidedOption, bool) {
	if line == "" {
		return guidedOption{}, false
	}
	for i, opt := range options {
		if line == fmt.Sprintf("%d", i+1) || strings.EqualFold(line, opt.title) {
			return opt, true
		}
	}
	return guidedOption{}, false
}

func splitCommaList(line string) []string {
	if strings.TrimSpace(line) == "" {
		return nil
	}
	parts := strings.Split(line, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func isYes(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func guidedAborted(err error) error {
	if err == io.EOF {
		return &ExitError{Code: ExitViolations, Message: "guided authoring aborted"}
	}
	return err
}

// --- completion -----------------------------------------------------------

func completeListings() func(args []string, toComplete string) []AutoCompleteCandidate {
	return func(args []string, toComplete string) []AutoCompleteCandidate {
		if len(args) > 0 {
			return nil
		}
		values := make([]string, 0, len(vocab.Concepts()))
		for _, c := range vocab.Concepts() {
			values = append(values, vocab.Listing(c))
		}
		return filterCandidates(toComplete, values)
	}
}

func completeConcepts() func(args []string, toComplete string) []AutoCompleteCandidate {
	return func(args []string, toComplete string) []AutoCompleteCandidate {
		if len(args) > 0 {
			return nil
		}
		return conceptSingularCandidates(toComplete)
	}
}

func completeShowArgs(list application.ListDomainDefinitions) func(args []string, toComplete string) []AutoCompleteCandidate {
	return func(args []string, toComplete string) []AutoCompleteCandidate {
		switch len(args) {
		case 0:
			return conceptSingularCandidates(toComplete)
		case 1:
			return definitionNameCandidates(list, args[0], toComplete)
		default:
			return nil
		}
	}
}

func conceptSingularCandidates(toComplete string) []AutoCompleteCandidate {
	values := make([]string, 0, len(vocab.Concepts()))
	for _, c := range vocab.Concepts() {
		values = append(values, string(c))
	}
	return filterCandidates(toComplete, values)
}

// definitionNameCandidates completes the name argument of show,
// define, and remove with what the project records under the concept:
// keys for invariants, assertions, and questions; names for everything
// else. Define still accepts names outside these suggestions.
func definitionNameCandidates(list application.ListDomainDefinitions, conceptArg, toComplete string) []AutoCompleteCandidate {
	concept, err := vocab.ParseConcept(conceptArg)
	if err != nil {
		return nil
	}
	result, err := list.Execute("", "")
	if err != nil {
		return nil
	}
	var names []string
	for _, ctx := range result.Language.Contexts {
		switch concept {
		case vocab.ConceptBoundedContext:
			names = append(names, ctx.Name)
		case vocab.ConceptAggregate, vocab.ConceptAggregateRoot:
			for _, a := range ctx.Aggregates {
				names = append(names, a.Name)
			}
		case vocab.ConceptEntity:
			for _, a := range ctx.Aggregates {
				for _, e := range a.Entities {
					names = append(names, e.Name)
				}
			}
		case vocab.ConceptValueObject:
			for _, v := range ctx.ValueObjects {
				names = append(names, v.Name)
			}
		case vocab.ConceptInvariant, vocab.ConceptBusinessRule:
			for _, oi := range ctx.Invariants() {
				names = append(names, oi.Invariant.Key)
			}
			if concept == vocab.ConceptBusinessRule {
				for _, oa := range ctx.Assertions() {
					names = append(names, oa.Assertion.Key)
				}
			}
		case vocab.ConceptAssertion:
			for _, oa := range ctx.Assertions() {
				names = append(names, oa.Assertion.Key)
			}
		case vocab.ConceptDomainEvent:
			for _, e := range ctx.Events {
				names = append(names, e.Name)
			}
		case vocab.ConceptDomainService:
			for _, s := range ctx.Services {
				names = append(names, s.Name)
			}
		case vocab.ConceptSpecification:
			for _, s := range ctx.Specifications {
				names = append(names, s.Name)
			}
		case vocab.ConceptQuestion:
			for _, q := range ctx.Questions {
				names = append(names, q.Key)
			}
		}
	}
	sort.Strings(names)
	return filterCandidates(toComplete, names)
}

func filterCandidates(toComplete string, values []string) []AutoCompleteCandidate {
	out := make([]AutoCompleteCandidate, 0, len(values))
	for _, v := range values {
		if toComplete == "" || strings.HasPrefix(v, toComplete) {
			out = append(out, AutoCompleteCandidate{Value: v})
		}
	}
	return out
}

// --- help text ------------------------------------------------------------

const domainGroupLong = `Inspect and maintain the project's ubiquitous language and domain model.

A project's Ubiquitous Language is the shared language used by its developers,
domain experts, documentation, tests, and code.

ArcLint records that language in ` + vocab.UbiquitousLanguageFileName + ` as bounded contexts,
each holding its aggregates (with their entities, invariants, and assertions),
value objects (with their invariants), domain events, domain services,
specifications, and open questions, plus the context map's relations.

The moment a domain is recorded, the invariants of the Domain-Driven Design
building blocks apply to it: arclint check reports an aggregate whose root or
identity the code does not declare, an invariant no method enforces, and the
like, with nothing added to ` + rule.RulesetFileName + `.

ArcLint makes this knowledge available to context, rules, patterns,
extensions, and agent integrations.

Running arclint domain without a subcommand is the same as:
  arclint domain overview`

const initDomainLong = `Initialize the project's ubiquitous language file, ` + vocab.UbiquitousLanguageFileName + `.

The file is created beside the resolved ` + rule.RulesetFileName + ` with the current
document version, an editor schema hint, and the project's name: the name of the
software whose domain it records, the repository directory's name unless
--project says otherwise. If the file already exists, ArcLint leaves it
unchanged.`

const initDomainExample = `  arclint domain init
  arclint domain init --project boxoffice`

const overviewLong = `Summarize the project's domain model for understanding.

The overview presents the recorded model grouped by bounded context: every
aggregate with its members, invariants, and assertions; every value object with
its invariants; then events, services, specifications, and open questions. When
the repository has been scanned, each invariant, assertion, and specification
names the declaration that carries it, or says that none does.

Running arclint domain without a subcommand runs this command.`

const overviewExample = `  arclint domain
  arclint domain overview
  arclint domain overview --format json`

const listLong = `List the project's domain definitions.

Without a listing, every recorded entry is named under its bounded context.
With one, only that concept's entries are listed. Invariants, assertions, and
questions list by key.`

const listExample = `  arclint domain list
  arclint domain list aggregates
  arclint domain list entities
  arclint domain list value_objects
  arclint domain list invariants
  arclint domain list events
  arclint domain list --context ordering
  arclint domain list --format json`

const showLong = `Show one domain definition.

Names and keys are matched exactly. Pass --context when the project records
several bounded contexts and the name is recorded in more than one; pass
--owner when an invariant or assertion key is recorded under more than one
owner in the same context.`

const showExample = `  arclint domain show bounded_context ordering
  arclint domain show aggregate Order
  arclint domain show entity OrderLine
  arclint domain show value_object Money
  arclint domain show invariant has-lines --owner Order
  arclint domain show assertion paid-in-full
  arclint domain show domain_event OrderShipped
  arclint domain show question resale
  arclint domain show aggregate Order --context ordering --format json`

const explainLong = `Explain ArcLint's supported domain concepts.

Without a concept, this command summarizes every supported concept. With one,
it explains that concept, cites the sources its meaning comes from, and gives
questions that help authors recognize it in their project.`

const explainExample = `  arclint domain explain
  arclint domain explain aggregate
  arclint domain explain entity
  arclint domain explain invariant
  arclint domain explain assertion
  arclint domain explain --format json`

const defineLong = `Create or update a domain definition.

If the named entry does not exist, ArcLint records it. If it already exists,
ArcLint changes only the properties the command passes; a property passed empty
is recorded empty. Running the same command again changes nothing.

Shell completion suggests existing names of the selected concept from the
recorded vocabulary. A new name remains valid because define also creates entries.

Every concept takes the properties its building block records, and needs its
required ones when first recorded:

  bounded_context  --definition
  aggregate        --definition --identity; optional --alias --repository --factory
  entity           --owner <aggregate> --definition; optional --identity --alias
  value_object     --definition; optional --alias
  invariant        --owner <aggregate or value object> --statement
  assertion        --owner <aggregate> --on --statement
  business_rule    an invariant, or an assertion when --on is passed
  domain_event     --definition; optional --raised-by <aggregate>
  domain_service   --definition
  specification    --definition
  question         --text

The name of an invariant, assertion, or question is its key: short and
kebab-case, since the method that enforces an aggregate's invariant or checks
an assertion is named after it.

A bounded context must be recorded before anything is recorded in it. Pass
--context when the project records several.`

const defineExample = `  arclint domain define bounded_context ordering \
    --definition "Taking and fulfilling orders."

  arclint domain define aggregate Order \
    --definition "A customer's request to purchase products." \
    --identity OrderID --alias "Purchase Order"

  arclint domain define entity OrderLine --owner Order \
    --definition "One product and quantity on an Order."

  arclint domain define value_object Money \
    --definition "An amount in one currency."

  arclint domain define invariant has-lines --owner Order \
    --statement "An Order carries at least one OrderLine."

  arclint domain define invariant never-negative --owner Money \
    --statement "Money is never negative."

  arclint domain define assertion paid-in-full --owner Order --on Ship \
    --statement "An Order ships only once paid in full."

  arclint domain define domain_event OrderShipped --raised-by Order \
    --definition "The Order left the warehouse."

  arclint domain define question resale --text "Can an Order be resold?"

  arclint domain define --guided`

const removeLong = `Remove a domain definition.

This command changes only the project domain model. It never deletes or modifies
source files.

Removing an aggregate removes its entities, invariants, and assertions with it,
and a domain event it raised no longer names what raises it. Removing a bounded
context removes the relations that name it. The command says what went with the
entry.`

const removeExample = `  arclint domain remove aggregate Order
  arclint domain remove entity OrderLine --owner Order
  arclint domain remove value_object LegacyOrderID
  arclint domain remove invariant has-lines --owner Order
  arclint domain rm domain_event OrderShipped
  arclint domain remove value_object LegacyOrderID --format json`

const schemaLong = `Print the JSON Schema accepted for ` + vocab.UbiquitousLanguageFileName + `, or write
it under the project's schema directory with --write so the file's modeline
can name a local copy.

The schema is the machine-readable contract for direct YAML authoring and
editor completion.`

const schemaExample = `  arclint domain schema
  arclint domain schema --write
  arclint domain schema --write --dir docs/schemas`
