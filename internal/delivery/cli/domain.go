package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// flagFormat names the shared output-format flag; jsonKeyType and
// jsonKeyName are the recurring JSON document keys.
const (
	flagFormat  = "format"
	jsonKeyType = "type"
	jsonKeyName = "name"
	formatText  = "text"
)

// NewDomainCommand adapts the project domain-model use cases into the
// domain command family: init, overview (also the bare `domain`
// default), list, show, explain, define (including --guided),
// remove/rm, and schema.
func NewDomainCommand(
	initialize application.InitDomain,
	overview application.GetDomainOverview,
	list application.ListDomainDefinitions,
	show application.ShowDomainDefinition,
	define application.DefineDomainDefinition,
	remove application.RemoveDomainDefinition,
) Command {
	runOverview := overviewRunner(overview)
	return Command{
		Name:    "domain",
		Short:   "inspect and maintain the project's ubiquitous language",
		Long:    domainGroupLong,
		MaxArgs: -1,
		Flags:   []Flag{formatFlag()},
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
				Run:     initDomainRunner(initialize),
			},
			{
				Name:    "overview",
				Short:   "summarize the project's ubiquitous language for understanding",
				Long:    overviewLong,
				Example: overviewExample,
				MaxArgs: 0,
				Flags:   []Flag{formatFlag()},
				Run:     runOverview,
			},
			{
				Name:         "list",
				Short:        "list the project's domain definitions",
				Long:         listLong,
				Example:      listExample,
				MaxArgs:      1,
				Flags:        []Flag{formatFlag(), contextFlag()},
				CompleteArgs: completeListings(),
				Run:          listRunner(list),
			},
			{
				Name:         "show",
				Short:        "show one domain definition",
				Long:         showLong,
				Example:      showExample,
				MaxArgs:      2,
				Flags:        []Flag{formatFlag(), contextFlag()},
				CompleteArgs: completeShowArgs(list),
				Run:          showRunner(show),
			},
			{
				Name:         "explain",
				Short:        "explain ArcLint's supported domain concepts",
				Long:         explainLong,
				Example:      explainExample,
				MaxArgs:      1,
				Flags:        []Flag{formatFlag()},
				CompleteArgs: completeConcepts(),
				Run:          explainRunner(),
			},
			{
				Name:         "define",
				Short:        "create or update a domain definition",
				Long:         defineLong,
				Example:      defineExample,
				MaxArgs:      2,
				Flags:        defineFlags(),
				CompleteArgs: completeConcepts(),
				Run:          defineRunner(define),
			},
			{
				Name:         "remove",
				Short:        "remove a domain definition",
				Long:         removeLong,
				Example:      removeExample,
				Aliases:      []string{"rm"},
				MaxArgs:      2,
				Flags:        []Flag{formatFlag(), contextFlag()},
				CompleteArgs: completeShowArgs(list),
				Run:          removeRunner(remove),
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

func initDomainRunner(initialize application.InitDomain) func(Context) error {
	return func(ctx Context) error {
		result, err := initialize.Execute()
		if err != nil {
			return domainError(err)
		}
		var writeErr error
		if result.Created {
			_, writeErr = fmt.Fprintf(ctx.Stdout, "Initialized %s.\n", result.Source)
		} else {
			_, writeErr = fmt.Fprintf(ctx.Stdout, "%s already exists; left unchanged.\n", result.Source)
		}
		if writeErr != nil {
			return fmt.Errorf("write output: %w", writeErr)
		}
		return nil
	}
}

func formatFlag() Flag {
	return Flag{
		Name:    flagFormat,
		Default: formatText,
		Doc:     "output format: text or json",
		Options: []string{formatText, formatJSON},
	}
}

func outputFormat(ctx Context) (string, error) {
	format := ctx.String(flagFormat)
	if format == "" {
		format = formatText
	}
	if format != formatText && format != formatJSON {
		return "", ConfigError(fmt.Errorf("unknown format %q (text, json)", format))
	}
	return format, nil
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

func overviewRunner(overview application.GetDomainOverview) func(Context) error {
	return func(ctx Context) error {
		format, err := outputFormat(ctx)
		if err != nil {
			return err
		}
		result, err := overview.Execute()
		if err != nil {
			return domainError(err)
		}
		if format == formatJSON {
			return writeDomainJSON(ctx.Stdout, overviewJSONDoc(result))
		}
		if !result.Found {
			return writeMissingDomainGuidance(ctx.Stdout)
		}
		return writeOverviewText(ctx.Stdout, result)
	}
}

func listRunner(list application.ListDomainDefinitions) func(Context) error {
	return func(ctx Context) error {
		format, err := outputFormat(ctx)
		if err != nil {
			return err
		}
		listing := ""
		if len(ctx.Args) == 1 {
			listing = ctx.Args[0]
		}
		result, err := list.Execute(listing, ctx.String("context"))
		if err != nil {
			return domainError(err)
		}
		if format == formatJSON {
			return writeDomainJSON(ctx.Stdout, listJSONDoc(result))
		}
		if result.Language.Empty() {
			return writeMissingDomainGuidance(ctx.Stdout)
		}
		return writeListText(ctx.Stdout, result)
	}
}

func showRunner(show application.ShowDomainDefinition) func(Context) error {
	return func(ctx Context) error {
		format, err := outputFormat(ctx)
		if err != nil {
			return err
		}
		if len(ctx.Args) != 2 {
			return ConfigError(fmt.Errorf("show requires <concept> <name>"))
		}
		result, err := show.Execute(ctx.Args[0], ctx.String("context"), ctx.Args[1])
		if err != nil {
			return domainError(err)
		}
		if format == formatJSON {
			return writeDomainJSON(ctx.Stdout, showJSONDoc(result))
		}
		return writeShowText(ctx.Stdout, result)
	}
}

func explainRunner() func(Context) error {
	return func(ctx Context) error {
		format, err := outputFormat(ctx)
		if err != nil {
			return err
		}
		var docs []vocab.ConceptDoc
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
		}
		if format == formatJSON {
			if len(ctx.Args) == 1 {
				return writeDomainJSON(ctx.Stdout, explainJSONDoc(docs[0]))
			}
			items := make([]any, 0, len(docs))
			for _, d := range docs {
				items = append(items, explainJSONDoc(d))
			}
			return writeDomainJSON(ctx.Stdout, items)
		}
		return writeExplainText(ctx.Stdout, docs)
	}
}

func defineFlags() []Flag {
	return []Flag{
		contextFlag(),
		{Name: "definition", Doc: "project-specific meaning of the definition"},
		{Name: "alias", Repeat: true, Doc: "recognized alternative name; may be repeated"},
		{Name: "clear-aliases", Bool: true, Doc: "remove every alias from the definition"},
		{Name: "owner", Doc: "enforcing owner for invariant, assertion, or business_rule"},
		{Name: "guided", Bool: true, Doc: "start an interactive authoring session"},
		formatFlag(),
	}
}

func defineRunner(define application.DefineDomainDefinition) func(Context) error {
	return func(ctx Context) error {
		format, err := outputFormat(ctx)
		if err != nil {
			return err
		}
		if ctx.Bool("guided") {
			if len(ctx.Args) > 0 || ctx.Changed("definition") || ctx.Changed("alias") ||
				ctx.Changed("clear-aliases") || ctx.Changed("owner") || ctx.Changed("context") {
				return ConfigError(fmt.Errorf("--guided cannot be combined with a type, name, or mutation flags"))
			}
			return runGuidedDefine(ctx, define, format)
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
		}
		if ctx.Changed("definition") {
			def := ctx.String("definition")
			req.Definition = &def
		}
		result, err := define.Execute(req)
		if err != nil {
			return domainError(err)
		}
		if format == formatJSON {
			return writeDomainJSON(ctx.Stdout, defineJSONDoc(result, req))
		}
		return writeDefineText(ctx.Stdout, result, req)
	}
}

func removeRunner(remove application.RemoveDomainDefinition) func(Context) error {
	return func(ctx Context) error {
		format, err := outputFormat(ctx)
		if err != nil {
			return err
		}
		if len(ctx.Args) != 2 {
			return ConfigError(fmt.Errorf("remove requires <concept> <name>"))
		}
		result, err := remove.Execute(ctx.Args[0], ctx.String("context"), ctx.Args[1])
		if err != nil {
			return domainError(err)
		}
		if format == formatJSON {
			return writeDomainJSON(ctx.Stdout, removeJSONDoc(result))
		}
		return writeRemoveText(ctx.Stdout, result)
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

// --- text renderers -------------------------------------------------------

func writeMissingDomainGuidance(w io.Writer) error {
	p := &printer{w: w}
	p.println("No recorded Ubiquitous Language found at " + vocab.UbiquitousLanguageFileName + ".")
	p.println()
	p.println("Initialize an empty model:")
	p.println("  arclint domain init")
	p.println()
	p.println("Define one item:")
	p.println("  arclint domain define entity <name> --context <context> --definition <text>")
	p.println()
	p.println("Start guided authoring:")
	p.println("  arclint domain define --guided")
	return p.err
}

func writeOverviewText(w io.Writer, result application.DomainOverview) error {
	p := &printer{w: w}
	lang := result.Language
	counts := lang.Counts()
	p.println("Project domain")
	p.printf("Source: %s\n", result.Source)
	p.println()
	p.printf("%s · %s · %s · %s · %s · %s\n",
		countPhrase(counts.Contexts, "Context", "Contexts"),
		countPhrase(counts.Entities, "Entity", "Entities"),
		countPhrase(counts.Aggregates, "Aggregate", "Aggregates"),
		countPhrase(counts.ValueObjects, "Value Object", "Value Objects"),
		countPhrase(counts.Invariants, "Invariant", "Invariants"),
		countPhrase(counts.Events, "Event", "Events"),
	)

	for _, ctx := range lang.ListContexts() {
		p.println()
		p.printf("Context %s\n", ctx.Name)

		var aggregates, plain []vocab.Entity
		for _, e := range ctx.Entities {
			if e.Aggregate {
				aggregates = append(aggregates, e)
			} else {
				plain = append(plain, e)
			}
		}
		if len(aggregates) > 0 {
			p.println()
			if len(aggregates) == 1 {
				p.println("  Aggregate")
			} else {
				p.println("  Aggregates")
			}
			p.println()
			writeTwoLineEntities(p, aggregates, "  ")
		}
		if len(plain) > 0 {
			p.println()
			p.println("  Entities")
			p.println()
			writePaddedEntityOneLiners(p, plain, "  ")
		}
		if len(ctx.Invariants) > 0 {
			p.println()
			p.println("  Invariants")
			p.println()
			for i, inv := range ctx.Invariants {
				if i > 0 {
					p.println()
				}
				p.printf("    %s\n", inv.Statement)
				p.printf("    owner: %s\n", inv.Owner)
			}
		}
		if len(ctx.ValueObjects) > 0 {
			p.println()
			p.println("  Value objects")
			p.println()
			writePaddedOneLiners(p, ctx.ValueObjects, "  ")
		}
		if len(ctx.Events) > 0 {
			p.println()
			p.println("  Domain events")
			p.println()
			writePaddedOneLiners(p, ctx.Events, "  ")
		}
	}

	if len(lang.Relations) > 0 {
		p.println()
		p.println("Relations")
		p.println()
		for _, rel := range lang.Relations {
			p.printf("  %s -[%s]-> %s\n", rel.From, rel.Kind, rel.To)
		}
	}
	return p.err
}

func writeListText(w io.Writer, result application.DomainListing) error {
	p := &printer{w: w}
	p.println("Project domain")
	contexts := selectedContexts(result)
	if result.Filtered {
		for _, ctx := range contexts {
			p.println()
			p.printf("Context %s\n", ctx.Name)
			switch result.Concept {
			case vocab.ConceptEntity:
				writeEntityListGroup(p, "  Entities", ctx.Entities, true)
			case vocab.ConceptAggregate, vocab.ConceptAggregateRoot:
				var aggs []vocab.Entity
				for _, e := range ctx.Entities {
					if e.Aggregate {
						aggs = append(aggs, e)
					}
				}
				writeEntityListGroup(p, "  "+listGroupHeader(result.Concept), aggs, false)
			case vocab.ConceptValueObject:
				writeListGroup(p, "  Value objects", ctx.ValueObjects)
			case vocab.ConceptInvariant, vocab.ConceptAssertion, vocab.ConceptBusinessRule:
				writeInvariantListGroup(p, "  "+listGroupHeader(result.Concept), ctx.Invariants)
			case vocab.ConceptDomainEvent:
				writeListGroup(p, "  Domain events", ctx.Events)
			case vocab.ConceptBoundedContext:
				p.printf("  %s\n", ctx.Name)
			default:
				writeListGroup(p, "  "+listGroupHeader(result.Concept), nil)
			}
		}
		return p.err
	}
	for _, ctx := range contexts {
		p.println()
		p.printf("Context %s\n", ctx.Name)
		writeEntityListGroup(p, "  Entities", ctx.Entities, true)
		writeListGroup(p, "  Value objects", ctx.ValueObjects)
		writeInvariantListGroup(p, "  Invariants", ctx.Invariants)
		writeListGroup(p, "  Domain events", ctx.Events)
	}
	if result.Context == "" && len(result.Language.Relations) > 0 {
		p.println()
		p.println("Relations")
		for _, rel := range result.Language.Relations {
			p.printf("  %s -[%s]-> %s\n", rel.From, rel.Kind, rel.To)
		}
	}
	return p.err
}

func selectedContexts(result application.DomainListing) []vocab.BoundedContext {
	all := result.Language.ListContexts()
	if result.Context == "" {
		return all
	}
	for _, ctx := range all {
		if ctx.Name == result.Context {
			return []vocab.BoundedContext{ctx}
		}
	}
	return nil
}

func writeListGroup(p *printer, header string, defs []vocab.Definition) {
	if len(defs) == 0 {
		return
	}
	p.println()
	p.println(header)
	for _, d := range sortedDefs(defs) {
		p.printf("    %s\n", d.Name)
	}
}

func writeInvariantListGroup(p *printer, header string, invs []vocab.Invariant) {
	if len(invs) == 0 {
		return
	}
	p.println()
	p.println(header)
	sorted := append([]vocab.Invariant(nil), invs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Statement < sorted[j].Statement })
	for _, inv := range sorted {
		p.printf("    %s (owner: %s)\n", inv.Statement, inv.Owner)
	}
}

func writeEntityListGroup(p *printer, header string, entities []vocab.Entity, markAggregate bool) {
	if len(entities) == 0 {
		return
	}
	p.println()
	p.println(header)
	for _, e := range sortedEntities(entities) {
		if markAggregate && e.Aggregate {
			p.printf("    %s [aggregate]\n", e.Name)
			continue
		}
		p.printf("    %s\n", e.Name)
	}
}

func writeShowText(w io.Writer, result application.DomainDefinitionView) error {
	p := &printer{w: w}
	doc := result.Concept.Doc()
	p.printf("%s: %s\n", doc.Title, result.Definition.Name)
	if result.Context != "" {
		p.printf("Context: %s\n", result.Context)
	}
	if isEntityShow(result.Concept) {
		if result.Aggregate {
			p.println("Aggregate: yes")
		} else if result.Concept == vocab.ConceptEntity {
			p.println("Aggregate: no")
		}
	}
	if isInvariantShow(result.Concept) {
		if result.Owner != "" {
			p.printf("Owner: %s\n", result.Owner)
		}
		return p.err
	}
	if result.Definition.Definition != "" {
		p.printf("Definition: %s\n", result.Definition.Definition)
	}
	if len(result.Definition.Aliases) > 0 {
		p.println("Aliases:")
		for _, alias := range result.Definition.Aliases {
			p.printf("  %s\n", alias)
		}
	}
	return p.err
}

func isEntityShow(c vocab.Concept) bool {
	return c == vocab.ConceptEntity || c == vocab.ConceptAggregate || c == vocab.ConceptAggregateRoot
}

func isInvariantShow(c vocab.Concept) bool {
	return c == vocab.ConceptInvariant || c == vocab.ConceptAssertion || c == vocab.ConceptBusinessRule
}

func writeExplainText(w io.Writer, docs []vocab.ConceptDoc) error {
	p := &printer{w: w}
	for i, doc := range docs {
		if i > 0 {
			p.println()
		}
		p.println(doc.Title)
		p.println()
		p.println(doc.Meaning)
		p.println()
		p.println("Ask:")
		p.println()
		for _, q := range doc.Questions {
			p.printf("  %s\n", q)
		}
		p.println()
		p.println(doc.Supplies)
		p.printf("ArcLint supplies the meaning of %s.\n", doc.Title)
	}
	return p.err
}

func writeDefineText(w io.Writer, result application.DomainDefineResult, req application.DefineDomainRequest) error {
	p := &printer{w: w}
	typeSpelling := string(result.Concept)
	switch result.Outcome {
	case vocab.OutcomeCreated:
		p.printf("Defined %s %s.\n", typeSpelling, result.Name)
	case vocab.OutcomeUpdated:
		p.printf("Updated %s %s.\n", typeSpelling, result.Name)
		for _, field := range result.Changed {
			switch field {
			case "definition":
				if req.Definition != nil && *req.Definition == "" {
					p.println("  definition: cleared")
				} else {
					p.println("  definition: changed")
				}
			case "aliases":
				if req.ClearAliases || len(result.Aliases) == 0 {
					p.println("  aliases: cleared")
				} else {
					p.printf("  aliases: %s\n", strings.Join(result.Aliases, ", "))
				}
			case "aggregate":
				p.println("  aggregate: designated")
			case "owner":
				p.println("  owner: changed")
			}
		}
	default:
		p.printf("Unchanged %s %s.\n", typeSpelling, result.Name)
	}
	return p.err
}

func writeRemoveText(w io.Writer, result application.DomainRemoveResult) error {
	p := &printer{w: w}
	if result.Concept == vocab.ConceptAggregate || result.Concept == vocab.ConceptAggregateRoot || result.EntityPreserved {
		p.printf("Removed the Aggregate designation from %s.\n", result.Name)
		p.printf("The %s Entity remains defined.\n", result.Name)
		return p.err
	}
	p.printf("Removed %s %s from the project domain model.\n", result.Concept, result.Name)
	p.println("Source files were not changed.")
	return p.err
}

func writeTwoLineEntities(p *printer, entities []vocab.Entity, indent string) {
	for i, e := range entities {
		if i > 0 {
			p.println()
		}
		p.printf("%s  %s\n", indent, e.Name)
		if e.Definition.Definition != "" {
			p.printf("%s  %s\n", indent, e.Definition.Definition)
		}
	}
}

func writePaddedOneLiners(p *printer, defs []vocab.Definition, indent string) {
	width := 0
	for _, d := range defs {
		if d.Definition != "" && len(d.Name) > width {
			width = len(d.Name)
		}
	}
	for _, d := range defs {
		if d.Definition == "" {
			p.printf("%s  %s\n", indent, d.Name)
			continue
		}
		p.printf("%s  %-*s — %s\n", indent, width, d.Name, d.Definition)
	}
}

func writePaddedEntityOneLiners(p *printer, entities []vocab.Entity, indent string) {
	width := 0
	for _, e := range entities {
		if e.Definition.Definition != "" && len(e.Name) > width {
			width = len(e.Name)
		}
	}
	for _, e := range entities {
		if e.Definition.Definition == "" {
			p.printf("%s  %s\n", indent, e.Name)
			continue
		}
		p.printf("%s  %-*s — %s\n", indent, width, e.Name, e.Definition.Definition)
	}
}

func countPhrase(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func sortedDefs(defs []vocab.Definition) []vocab.Definition {
	out := append([]vocab.Definition(nil), defs...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedEntities(entities []vocab.Entity) []vocab.Entity {
	out := append([]vocab.Entity(nil), entities...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func listGroupHeader(c vocab.Concept) string {
	switch c {
	case vocab.ConceptEntity:
		return "Entities"
	case vocab.ConceptAggregate:
		return "Aggregates"
	case vocab.ConceptAggregateRoot:
		return "Aggregate roots"
	case vocab.ConceptValueObject:
		return "Value objects"
	case vocab.ConceptInvariant:
		return "Invariants"
	case vocab.ConceptAssertion:
		return "Assertions"
	case vocab.ConceptBusinessRule:
		return "Business rules"
	case vocab.ConceptDomainEvent:
		return "Domain events"
	case vocab.ConceptBoundedContext:
		return "Bounded contexts"
	default:
		return vocab.Listing(c)
	}
}

// --- JSON docs ------------------------------------------------------------

type domainCountsJSON struct {
	Contexts     int `json:"contexts"`
	Entities     int `json:"entities"`
	Aggregates   int `json:"aggregates"`
	ValueObjects int `json:"valueObjects"`
	Invariants   int `json:"invariants"`
	Events       int `json:"events"`
	Relations    int `json:"relations"`
}

type domainDefJSON struct {
	Name       string   `json:"name"`
	Definition string   `json:"definition,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	Aggregate  bool     `json:"aggregate,omitempty"`
}

type domainInvariantJSON struct {
	Statement string `json:"statement"`
	Owner     string `json:"owner"`
}

type domainContextJSON struct {
	Name         string                `json:"name"`
	Entities     []domainDefJSON       `json:"entities,omitempty"`
	ValueObjects []domainDefJSON       `json:"valueObjects,omitempty"`
	Invariants   []domainInvariantJSON `json:"invariants,omitempty"`
	Events       []domainDefJSON       `json:"events,omitempty"`
	// Filtered listing keys when only one concept is requested.
	Aggregates []domainDefJSON `json:"aggregates,omitempty"`
}

type domainRelationJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type domainOverviewJSON struct {
	Source    string               `json:"source"`
	Found     bool                 `json:"found"`
	Counts    domainCountsJSON     `json:"counts"`
	Contexts  []domainContextJSON  `json:"contexts,omitempty"`
	Relations []domainRelationJSON `json:"relations,omitempty"`
}

func overviewJSONDoc(result application.DomainOverview) domainOverviewJSON {
	counts := result.Language.Counts()
	doc := domainOverviewJSON{
		Source: result.Source,
		Found:  result.Found,
		Counts: domainCountsJSON{
			Contexts:     counts.Contexts,
			Entities:     counts.Entities,
			Aggregates:   counts.Aggregates,
			ValueObjects: counts.ValueObjects,
			Invariants:   counts.Invariants,
			Events:       counts.Events,
			Relations:    counts.Relations,
		},
	}
	if !result.Found {
		return doc
	}
	doc.Contexts = contextsToJSON(result.Language.ListContexts(), false)
	doc.Relations = relationsToJSON(result.Language.Relations)
	return doc
}

func contextsToJSON(contexts []vocab.BoundedContext, namesOnly bool) []domainContextJSON {
	if len(contexts) == 0 {
		return nil
	}
	out := make([]domainContextJSON, 0, len(contexts))
	for _, ctx := range contexts {
		item := domainContextJSON{Name: ctx.Name}
		if namesOnly {
			item.Entities = entityNamesJSON(ctx.Entities)
			item.ValueObjects = defNamesJSON(ctx.ValueObjects)
			item.Invariants = invariantJSON(ctx.Invariants)
			item.Events = defNamesJSON(ctx.Events)
		} else {
			item.Entities = entitiesToJSON(ctx.Entities, true)
			item.ValueObjects = defsToJSON(ctx.ValueObjects)
			item.Invariants = invariantJSON(ctx.Invariants)
			item.Events = defsToJSON(ctx.Events)
		}
		out = append(out, item)
	}
	return out
}

func relationsToJSON(rels []vocab.ContextRelation) []domainRelationJSON {
	if len(rels) == 0 {
		return nil
	}
	out := make([]domainRelationJSON, 0, len(rels))
	for _, r := range rels {
		out = append(out, domainRelationJSON{From: r.From, To: r.To, Kind: string(r.Kind)})
	}
	return out
}

func defsToJSON(defs []vocab.Definition) []domainDefJSON {
	if len(defs) == 0 {
		return nil
	}
	out := make([]domainDefJSON, 0, len(defs))
	for _, d := range defs {
		out = append(out, domainDefJSON{
			Name:       d.Name,
			Definition: d.Definition,
			Aliases:    d.Aliases,
		})
	}
	return out
}

func defNamesJSON(defs []vocab.Definition) []domainDefJSON {
	if len(defs) == 0 {
		return nil
	}
	out := make([]domainDefJSON, 0, len(defs))
	for _, d := range sortedDefs(defs) {
		out = append(out, domainDefJSON{Name: d.Name})
	}
	return out
}

func entitiesToJSON(entities []vocab.Entity, withAggregate bool) []domainDefJSON {
	if len(entities) == 0 {
		return nil
	}
	out := make([]domainDefJSON, 0, len(entities))
	for _, e := range entities {
		item := domainDefJSON{
			Name:       e.Name,
			Definition: e.Definition.Definition,
			Aliases:    e.Aliases,
		}
		if withAggregate {
			item.Aggregate = e.Aggregate
		}
		out = append(out, item)
	}
	return out
}

func entityNamesJSON(entities []vocab.Entity) []domainDefJSON {
	if len(entities) == 0 {
		return nil
	}
	out := make([]domainDefJSON, 0, len(entities))
	for _, e := range sortedEntities(entities) {
		item := domainDefJSON{Name: e.Name}
		if e.Aggregate {
			item.Aggregate = true
		}
		out = append(out, item)
	}
	return out
}

func invariantJSON(invs []vocab.Invariant) []domainInvariantJSON {
	if len(invs) == 0 {
		return nil
	}
	out := make([]domainInvariantJSON, 0, len(invs))
	for _, inv := range invs {
		out = append(out, domainInvariantJSON{Statement: inv.Statement, Owner: inv.Owner})
	}
	return out
}

func listJSONDoc(result application.DomainListing) map[string]any {
	contexts := selectedContexts(result)
	out := map[string]any{}
	items := make([]map[string]any, 0, len(contexts))
	for _, ctx := range contexts {
		entry := map[string]any{"name": ctx.Name}
		if result.Filtered {
			addFilteredListEntry(entry, ctx, result.Concept)
		} else {
			addFullListEntry(entry, ctx)
		}
		items = append(items, entry)
	}
	if len(items) > 0 {
		out["contexts"] = items
	}
	if !result.Filtered && result.Context == "" {
		if rels := relationsToJSON(result.Language.Relations); len(rels) > 0 {
			out["relations"] = rels
		}
	}
	return out
}

// addFilteredListEntry records only the listed concept's section.
func addFilteredListEntry(entry map[string]any, ctx vocab.BoundedContext, c vocab.Concept) {
	switch c {
	case vocab.ConceptEntity:
		if v := entityNamesJSON(ctx.Entities); len(v) > 0 {
			entry["entities"] = v
		}
	case vocab.ConceptAggregate, vocab.ConceptAggregateRoot:
		var aggs []vocab.Entity
		for _, e := range ctx.Entities {
			if e.Aggregate {
				aggs = append(aggs, e)
			}
		}
		if v := entityNamesJSON(aggs); len(v) > 0 {
			entry[listJSONKey(c)] = v
		}
	case vocab.ConceptValueObject:
		if v := defNamesJSON(ctx.ValueObjects); len(v) > 0 {
			entry["valueObjects"] = v
		}
	case vocab.ConceptInvariant, vocab.ConceptAssertion, vocab.ConceptBusinessRule:
		if v := invariantJSON(ctx.Invariants); len(v) > 0 {
			entry[listJSONKey(c)] = v
		}
	case vocab.ConceptDomainEvent:
		if v := defNamesJSON(ctx.Events); len(v) > 0 {
			entry["events"] = v
		}
	case vocab.ConceptBoundedContext:
		// name already present
	}
}

// addFullListEntry records every recorded section of the context.
func addFullListEntry(entry map[string]any, ctx vocab.BoundedContext) {
	if v := entityNamesJSON(ctx.Entities); len(v) > 0 {
		entry["entities"] = v
	}
	if v := defNamesJSON(ctx.ValueObjects); len(v) > 0 {
		entry["valueObjects"] = v
	}
	if v := invariantJSON(ctx.Invariants); len(v) > 0 {
		entry["invariants"] = v
	}
	if v := defNamesJSON(ctx.Events); len(v) > 0 {
		entry["events"] = v
	}
}

func listJSONKey(c vocab.Concept) string {
	switch c {
	case vocab.ConceptEntity:
		return "entities"
	case vocab.ConceptAggregate:
		return "aggregates"
	case vocab.ConceptAggregateRoot:
		return "aggregateRoots"
	case vocab.ConceptValueObject:
		return "valueObjects"
	case vocab.ConceptInvariant:
		return "invariants"
	case vocab.ConceptAssertion:
		return "assertions"
	case vocab.ConceptBusinessRule:
		return "businessRules"
	case vocab.ConceptDomainEvent:
		return "events"
	case vocab.ConceptBoundedContext:
		return "boundedContexts"
	default:
		return string(c)
	}
}

func showJSONDoc(result application.DomainDefinitionView) map[string]any {
	doc := map[string]any{
		jsonKeyType: string(result.Concept),
		jsonKeyName: result.Definition.Name,
	}
	if result.Context != "" {
		doc["context"] = result.Context
	}
	if isEntityShow(result.Concept) {
		doc["aggregate"] = result.Aggregate || result.Concept == vocab.ConceptAggregate || result.Concept == vocab.ConceptAggregateRoot
	}
	if isInvariantShow(result.Concept) {
		if result.Owner != "" {
			doc["owner"] = result.Owner
		}
		return doc
	}
	if result.Definition.Definition != "" {
		doc["definition"] = result.Definition.Definition
	}
	if len(result.Definition.Aliases) > 0 {
		doc["aliases"] = result.Definition.Aliases
	}
	return doc
}

func explainJSONDoc(doc vocab.ConceptDoc) map[string]any {
	return map[string]any{
		jsonKeyType: string(doc.Concept),
		"title":     doc.Title,
		"meaning":   doc.Meaning,
		"questions": doc.Questions,
	}
}

func defineJSONDoc(result application.DomainDefineResult, req application.DefineDomainRequest) map[string]any {
	doc := map[string]any{
		"result":    string(result.Outcome),
		jsonKeyType: string(result.Concept),
		jsonKeyName: result.Name,
	}
	if result.Context != "" {
		doc["context"] = result.Context
	}
	if len(result.Changed) > 0 {
		doc["changed"] = result.Changed
	}
	if len(result.Aliases) > 0 {
		doc["aliases"] = result.Aliases
	}
	if req.Definition != nil && *req.Definition != "" {
		doc["definition"] = *req.Definition
	}
	if req.Owner != "" {
		doc["owner"] = req.Owner
	}
	return doc
}

func removeJSONDoc(result application.DomainRemoveResult) map[string]any {
	doc := map[string]any{
		jsonKeyType: string(result.Concept),
		jsonKeyName: result.Name,
		"result":    "removed",
	}
	if result.Context != "" {
		doc["context"] = result.Context
	}
	if result.EntityPreserved || result.Concept == vocab.ConceptAggregate || result.Concept == vocab.ConceptAggregateRoot {
		doc["entityPreserved"] = true
		return doc
	}
	doc["sourceFilesChanged"] = false
	return doc
}

func writeDomainJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(data)); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// --- guided authoring -----------------------------------------------------

func runGuidedDefine(ctx Context, define application.DefineDomainDefinition, format string) error {
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
	if format == formatJSON {
		return writeDomainJSON(ctx.Stdout, defineJSONDoc(result, req))
	}
	return writeDefineText(ctx.Stdout, result, req)
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
		case vocab.ConceptInvariant, vocab.ConceptAssertion, vocab.ConceptBusinessRule:
			for _, inv := range ctx.Invariants {
				names = append(names, inv.Statement)
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
