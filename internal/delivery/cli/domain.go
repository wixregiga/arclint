package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
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
				Flags:        []Flag{contextFlag()},
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
				CompleteArgs: completeConcepts(),
				Run:          defineRunner(define, render),
			},
			{
				Name:         "remove",
				Short:        "remove a domain definition",
				Long:         removeLong,
				Example:      removeExample,
				Aliases:      []string{"rm"},
				MaxArgs:      2,
				Flags:        []Flag{contextFlag()},
				CompleteArgs: completeShowArgs(list),
				Run:          removeRunner(remove, render),
			},
			{
				Name:    "schema",
				Short:   "print the JSON Schema accepted for ubiquitous-language.yaml",
				Long:    schemaLong,
				Example: schemaExample,
				MaxArgs: 0,
				Run:     domainSchemaRunner(),
			},
		},
	}
}

func contextFlag() Flag {
	return Flag{Name: "context", Doc: "bounded context that owns the definition"}
}

func initDomainRunner(initialize application.InitDomain, render Renderer) func(Context) error {
	return func(ctx Context) error {
		result, err := initialize.Execute()
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
		result, err := show.Execute(ctx.Args[0], ctx.String("context"), ctx.Args[1])
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

func defineFlags() []Flag {
	return []Flag{
		contextFlag(),
		{Name: "definition", Doc: "project-specific meaning of the definition"},
		{Name: "alias", Repeat: true, Doc: "recognized alternative name; may be repeated"},
		{Name: "clear-aliases", Bool: true, Doc: "remove every alias from the definition"},
		{Name: "owner", Doc: "enforcing owner for invariant, assertion, or business_rule"},
		{Name: "id", Doc: "cluster or assertion identity; required when creating an assertion"},
		{Name: "on", Doc: "operation an assertion is checked on; required when creating an assertion"},
		{Name: "guided", Bool: true, Doc: "start an interactive authoring session"},
	}
}

func defineRunner(define application.DefineDomainDefinition, render Renderer) func(Context) error {
	return func(ctx Context) error {
		if ctx.Bool("guided") {
			if len(ctx.Args) > 0 || ctx.Changed("definition") || ctx.Changed("alias") ||
				ctx.Changed("clear-aliases") || ctx.Changed("owner") || ctx.Changed("id") ||
				ctx.Changed("on") || ctx.Changed("context") {
				return ConfigError(fmt.Errorf("--guided cannot be combined with a type, name, or mutation flags"))
			}
			return runGuidedDefine(ctx, define, render)
		}
		if len(ctx.Args) != 2 {
			return ConfigError(fmt.Errorf("define requires <concept> <name>, or --guided"))
		}
		req := application.DefineDomainRequest{
			Concept:      ctx.Args[0],
			Context:      ctx.String("context"),
			Name:         ctx.Args[1],
			Aliases:      ctx.Strings("alias"),
			ClearAliases: ctx.Bool("clear-aliases"),
			Owner:        ctx.String("owner"),
			ID:           ctx.String("id"),
			On:           ctx.String("on"),
		}
		if ctx.Changed("definition") {
			def := ctx.String("definition")
			req.Definition = &def
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
		result, err := remove.Execute(ctx.Args[0], ctx.String("context"), ctx.Args[1])
		if err != nil {
			return domainError(err)
		}
		if err := render.Render(ctx.Stdout, DomainRemoveReport{Result: result}); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}

func domainSchemaRunner() func(Context) error {
	return func(ctx Context) error {
		data, err := vocab.Schema()
		if err != nil {
			return fmt.Errorf("domain schema: %w", err)
		}
		if _, err := ctx.Stdout.Write(data); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}

func isInvariantShow(c vocab.Concept) bool {
	return c == vocab.ConceptInvariant || c == vocab.ConceptAssertion || c == vocab.ConceptBusinessRule
}

// --- guided authoring -----------------------------------------------------

func runGuidedDefine(ctx Context, define application.DefineDomainDefinition, render Renderer) error {
	in := ctx.Stdin
	if in == nil {
		in = strings.NewReader("")
	}
	sc := bufio.NewScanner(in)
	readLine := func() (string, error) {
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				return "", fmt.Errorf("read input: %w", err)
			}
			return "", io.EOF
		}
		return strings.TrimSpace(sc.Text()), nil
	}
	prompt := func(line string) error {
		if _, err := fmt.Fprintln(ctx.Stdout, line); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
	promptBlank := func() error { return prompt("") }

	if err := prompt("What are you defining?"); err != nil {
		return err
	}
	options := []struct {
		title   string
		concept vocab.Concept
	}{
		{"Entity", vocab.ConceptEntity},
		{"Value Object", vocab.ConceptValueObject},
		{"Invariant", vocab.ConceptInvariant},
		{"Domain Event", vocab.ConceptDomainEvent},
		{"Bounded Context", vocab.ConceptBoundedContext},
		{"Business Rule", vocab.ConceptBusinessRule},
	}
	for i, opt := range options {
		if err := prompt(fmt.Sprintf("  %d) %s", i+1, opt.title)); err != nil {
			return err
		}
	}
	var chosen vocab.Concept
	var chosenTitle string
	for {
		line, err := readLine()
		if err != nil {
			return guidedAborted(err)
		}
		if c, title, ok := parseGuidedConcept(line, options); ok {
			chosen = c
			chosenTitle = title
			break
		}
		if err := prompt("Please choose 1-" + fmt.Sprint(len(options)) + " or the concept title."); err != nil {
			return err
		}
	}
	doc := chosen.Doc()
	if err := promptBlank(); err != nil {
		return err
	}
	if err := prompt(doc.Title); err != nil {
		return err
	}
	if err := prompt(doc.Meaning); err != nil {
		return err
	}
	if err := promptBlank(); err != nil {
		return err
	}

	contextName := ""
	if chosen != vocab.ConceptBoundedContext {
		if err := prompt("Bounded context:"); err != nil {
			return err
		}
		for {
			line, err := readLine()
			if err != nil {
				return guidedAborted(err)
			}
			if line != "" {
				contextName = line
				break
			}
			if err := prompt("A bounded context name is required."); err != nil {
				return err
			}
		}
	}

	termPrompt := "Project term:"
	if isInvariantShow(chosen) {
		termPrompt = "Invariant statement:"
	}
	if err := prompt(termPrompt); err != nil {
		return err
	}
	var term string
	for {
		line, err := readLine()
		if err != nil {
			return guidedAborted(err)
		}
		if line != "" {
			term = line
			break
		}
		if err := prompt(fmt.Sprintf("Define it in the project's Ubiquitous Language: %s", termPrompt)); err != nil {
			return err
		}
	}

	req := application.DefineDomainRequest{
		Concept: string(chosen),
		Context: contextName,
		Name:    term,
	}

	switch {
	case chosen == vocab.ConceptBoundedContext:
		if err := promptBlank(); err != nil {
			return err
		}
		if err := prompt(fmt.Sprintf("  %s: %s", chosenTitle, term)); err != nil {
			return err
		}
	case isInvariantShow(chosen):
		if err := prompt("Owner:"); err != nil {
			return err
		}
		for {
			line, err := readLine()
			if err != nil {
				return guidedAborted(err)
			}
			if line != "" {
				req.Owner = line
				break
			}
			if err := prompt("Owner is required for invariants."); err != nil {
				return err
			}
		}
		if err := promptBlank(); err != nil {
			return err
		}
		if err := prompt(fmt.Sprintf("  %s: %s", chosenTitle, term)); err != nil {
			return err
		}
		if err := prompt(fmt.Sprintf("  Owner: %s", req.Owner)); err != nil {
			return err
		}
	default:
		if err := prompt("Proposed definition:"); err != nil {
			return err
		}
		definition, err := readLine()
		if err != nil {
			return guidedAborted(err)
		}
		if err := prompt("Recognized aliases:"); err != nil {
			return err
		}
		aliasLine, err := readLine()
		if err != nil {
			return guidedAborted(err)
		}
		aliases := splitCommaList(aliasLine)
		var aggregate *bool
		if chosen == vocab.ConceptEntity {
			if err := prompt(fmt.Sprintf("Is %s an Aggregate? [y/N]", term)); err != nil {
				return err
			}
			ans, err := readLine()
			if err != nil {
				return guidedAborted(err)
			}
			yes := isYes(ans)
			aggregate = &yes
		}
		if err := promptBlank(); err != nil {
			return err
		}
		if err := prompt("Proposed definition:"); err != nil {
			return err
		}
		if err := prompt(fmt.Sprintf("  %s: %s", chosenTitle, term)); err != nil {
			return err
		}
		if aggregate != nil && *aggregate {
			if err := prompt("  Aggregate: yes"); err != nil {
				return err
			}
		}
		if definition != "" {
			if err := prompt(fmt.Sprintf("  Definition: %s", definition)); err != nil {
				return err
			}
			def := definition
			req.Definition = &def
		}
		if len(aliases) > 0 {
			if err := prompt(fmt.Sprintf("  Aliases: %s", strings.Join(aliases, ", "))); err != nil {
				return err
			}
			req.Aliases = aliases
		}
		req.Aggregate = aggregate
	}

	if err := promptBlank(); err != nil {
		return err
	}
	if err := prompt("Write this definition to " + vocab.UbiquitousLanguageFileName + "? [y/N]"); err != nil {
		return err
	}
	confirm, err := readLine()
	if err != nil {
		return guidedAborted(err)
	}
	if !isYes(confirm) {
		if _, err := fmt.Fprintln(ctx.Stdout, "Nothing written."); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
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

func parseGuidedConcept(line string, options []struct {
	title   string
	concept vocab.Concept
},
) (vocab.Concept, string, bool) {
	if line == "" {
		return "", "", false
	}
	for i, opt := range options {
		if line == fmt.Sprintf("%d", i+1) || strings.EqualFold(line, opt.title) {
			return opt.concept, opt.title, true
		}
	}
	return "", "", false
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
	values := make([]string, 0, 5)
	for _, c := range vocab.Concepts() {
		values = append(values, string(c))
	}
	return filterCandidates(toComplete, values)
}

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
	for _, ctx := range result.Language.ListContexts() {
		switch concept {
		case vocab.ConceptEntity, vocab.ConceptAggregate, vocab.ConceptAggregateRoot:
			for _, e := range ctx.Entities {
				if concept == vocab.ConceptEntity || e.Aggregate {
					names = append(names, e.Name)
				}
			}
		case vocab.ConceptValueObject:
			for _, d := range ctx.ValueObjects {
				names = append(names, d.Name)
			}
		case vocab.ConceptDomainEvent:
			for _, d := range ctx.Events {
				names = append(names, d.Name)
			}
		case vocab.ConceptInvariant, vocab.ConceptBusinessRule:
			for _, inv := range ctx.Invariants {
				names = append(names, inv.Statement)
			}
		case vocab.ConceptAssertion:
			for _, a := range ctx.Assertions {
				names = append(names, a.Statement)
			}
		case vocab.ConceptSpecification:
			for _, s := range ctx.Specifications {
				names = append(names, s.Name)
			}
		case vocab.ConceptBoundedContext:
			names = append(names, ctx.Name)
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

ArcLint records that language as a structured Project Domain Model organized by
bounded contexts containing entities, value objects, invariants, and domain
events, plus optional context-map relations.

ArcLint makes this knowledge available to context, enabled lint rules, patterns,
extensions, and agent integrations.

Running arclint domain without a subcommand is the same as:
  arclint domain overview`

const initDomainLong = `Initialize the project's ubiquitous language file.

The file is created beside the resolved rules.yaml with the current document
version and an editor schema hint. If it already exists, ArcLint leaves it
unchanged.`

const initDomainExample = `  arclint domain init`

const overviewLong = `Summarize the project's domain model for understanding.

The overview presents stored definitions grouped by bounded context. It does not
infer source boundaries, relationships, or path relevance.

Running arclint domain without a subcommand runs this command.`

const overviewExample = `  arclint domain
  arclint domain overview
  arclint domain overview --format json`

const listLong = `List the project's domain definitions.

Without a type, definitions are grouped by bounded context under entities,
value objects, invariants, and domain events.`

const listExample = `  arclint domain list
  arclint domain list entities
  arclint domain list aggregates
  arclint domain list invariants
  arclint domain list domain_events
  arclint domain list --context Ordering
  arclint domain list --format json`

const showLong = `Show one domain definition.

Names are matched exactly. Quote names containing spaces. Pass --context when
the same name exists in more than one bounded context.`

const showExample = `  arclint domain show entity Order
  arclint domain show aggregate Order --context Ordering
  arclint domain show value_object Money
  arclint domain show invariant "Every Order must identify its Customer."
  arclint domain show domain_event OrderPlaced
  arclint domain show entity "Purchase Order"
  arclint domain show entity Order --format json`

const explainLong = `Explain ArcLint's supported domain concepts.

Without a type, this command summarizes every supported concept. With a type,
it explains that concept and gives questions that help authors recognize it in
their project.`

const explainExample = `  arclint domain explain
  arclint domain explain entity
  arclint domain explain aggregate
  arclint domain explain invariant
  arclint domain explain business_rule
  arclint domain explain --format json`

const defineLong = `Create or update a domain definition.

If the named definition does not exist, ArcLint creates it. If it already
exists, ArcLint updates only the fields supplied by this command.

Running the same command repeatedly produces no additional change.

Defining an Aggregate designates the named Entity as an Aggregate. If the
Entity does not exist yet, ArcLint creates it as part of the same operation.

Entity, value_object, and domain_event require --definition at create.
Invariant, assertion, and business_rule use the name argument as the statement
and require --owner at create. bounded_context takes only a name.

Pass --context when the project has zero or multiple bounded contexts.`

const defineExample = `  arclint domain define bounded_context Ordering

  arclint domain define entity Order --context Ordering \
    --definition "A customer's request to purchase products"

  arclint domain define aggregate Order --context Ordering

  arclint domain define value_object Money --context Ordering \
    --definition "A monetary amount expressed in a particular currency"

  arclint domain define invariant "Every Order must identify its Customer." \
    --context Ordering --owner Order

  arclint domain define domain_event OrderPlaced --context Ordering \
    --definition "An Order has been accepted for processing."

  arclint domain define entity Order --context Ordering \
    --alias "Purchase Order" \
    --alias "Customer Order"

  arclint domain define --guided`

const removeLong = `Remove a domain definition.

This command changes only the project domain model. It never deletes or modifies
source files.

Removing an Aggregate removes the Aggregate designation while preserving the
Entity. Removing an Entity also removes its Aggregate designation.`

const removeExample = `  arclint domain remove aggregate Order --context Ordering
  arclint domain remove entity LegacyOrder --context Ordering
  arclint domain remove value_object LegacyOrderID --context Ordering
  arclint domain remove invariant "Obsolete rule" --context Ordering
  arclint domain rm domain_event LegacyOrderCreated --context Ordering
  arclint domain remove value_object LegacyOrderID --format json`

const schemaLong = `Print the JSON Schema accepted for ubiquitous-language.yaml.

The schema is the machine-readable contract for direct YAML authoring and
editor completion.`

const schemaExample = `  arclint domain schema
  arclint domain schema > .agents/skills/domain-librarian/library.schema.json`
