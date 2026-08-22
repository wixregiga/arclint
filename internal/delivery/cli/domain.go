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

const (
	formatText = "text"
	// flagFormat names the shared output-format flag; jsonKeyType and
	// jsonKeyName are the recurring JSON document keys.
	flagFormat  = "format"
	jsonKeyType = "type"
	jsonKeyName = "name"
)

// NewDomainCommand adapts the project domain-model use cases into the
// domain command family: overview (also the bare `domain` default),
// list, show, explain, define (including --guided), remove/rm, and
// schema.
func NewDomainCommand(
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
				return ConfigError(fmt.Errorf("unknown command %q for \"arclint domain\"", ctx.Args[0]))
			}
			return runOverview(ctx)
		},
		Subcommands: []Command{
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
				Flags:        []Flag{formatFlag()},
				CompleteArgs: completeListings(),
				Run:          listRunner(list),
			},
			{
				Name:         "show",
				Short:        "show one domain definition",
				Long:         showLong,
				Example:      showExample,
				MaxArgs:      2,
				Flags:        []Flag{formatFlag()},
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
				Flags:        []Flag{formatFlag()},
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
		result, err := list.Execute(listing)
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
			return ConfigError(fmt.Errorf("show requires <entity|aggregate|value-object|business-rule|event> <name>"))
		}
		result, err := show.Execute(ctx.Args[0], ctx.Args[1])
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
		{Name: "definition", Doc: "project-specific meaning of the definition"},
		{Name: "alias", Repeat: true, Doc: "recognized alternative name; may be repeated"},
		{Name: "clear-aliases", Bool: true, Doc: "remove every alias from the definition"},
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
			if len(ctx.Args) > 0 || ctx.Changed("definition") || ctx.Changed("alias") || ctx.Changed("clear-aliases") {
				return ConfigError(fmt.Errorf("--guided cannot be combined with a type, name, or mutation flags"))
			}
			return runGuidedDefine(ctx, define, format)
		}
		if len(ctx.Args) != 2 {
			return ConfigError(fmt.Errorf("define requires <entity|aggregate|value-object|business-rule|event> <name>, or --guided"))
		}
		req := application.DefineDomainRequest{
			Concept:      ctx.Args[0],
			Name:         ctx.Args[1],
			Aliases:      ctx.Strings("alias"),
			ClearAliases: ctx.Bool("clear-aliases"),
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
			return ConfigError(fmt.Errorf("remove requires <entity|aggregate|value-object|business-rule|event> <name>"))
		}
		result, err := remove.Execute(ctx.Args[0], ctx.Args[1])
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
	p.println("Define one item:")
	p.println("  arclint domain define entity <name> --definition <text>")
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
	p.printf("%s · %s · %s · %s · %s\n",
		countPhrase(counts.Entities, "Entity", "Entities"),
		countPhrase(counts.Aggregates, "Aggregate", "Aggregates"),
		countPhrase(counts.ValueObjects, "Value Object", "Value Objects"),
		countPhrase(counts.BusinessRules, "Business Rule", "Business Rules"),
		countPhrase(counts.Events, "Event", "Events"),
	)

	aggregates := lang.ListAggregates()
	if len(aggregates) > 0 {
		p.println()
		if len(aggregates) == 1 {
			p.println("Aggregate")
		} else {
			p.println("Aggregates")
		}
		p.println()
		writeTwoLineEntities(p, aggregates)
	}

	entities := nonAggregateEntities(lang)
	if len(entities) > 0 {
		p.println()
		p.println("Entities")
		p.println()
		writePaddedEntityOneLiners(p, entities)
	}

	rules := lang.List(vocab.ConceptBusinessRule)
	if len(rules) > 0 {
		p.println()
		p.println("Business rules")
		p.println()
		writeTwoLineEntries(p, rules)
	}

	values := lang.List(vocab.ConceptValueObject)
	if len(values) > 0 {
		p.println()
		p.println("Value objects")
		p.println()
		writePaddedOneLiners(p, values)
	}

	events := lang.List(vocab.ConceptEvent)
	if len(events) > 0 {
		p.println()
		p.println("Domain events")
		p.println()
		writePaddedOneLiners(p, events)
	}
	return p.err
}

func writeListText(w io.Writer, result application.DomainListing) error {
	p := &printer{w: w}
	p.println("Project domain")
	if result.Filtered {
		switch result.Concept {
		case vocab.ConceptEntity:
			writeEntityListGroup(p, listGroupHeader(result.Concept), sortedEntities(result.Language.ListEntities()), true)
		case vocab.ConceptAggregate:
			writeEntityListGroup(p, listGroupHeader(result.Concept), sortedEntities(result.Language.ListAggregates()), false)
		default:
			writeListGroup(p, listGroupHeader(result.Concept), sortedDefs(result.Language.List(result.Concept)))
		}
		return p.err
	}
	lang := result.Language
	writeEntityListGroup(p, "Entities", sortedEntities(lang.ListEntities()), true)
	writeListGroup(p, "Value objects", sortedDefs(lang.List(vocab.ConceptValueObject)))
	writeListGroup(p, "Business rules", sortedDefs(lang.List(vocab.ConceptBusinessRule)))
	writeListGroup(p, "Domain events", sortedDefs(lang.List(vocab.ConceptEvent)))
	return p.err
}

func writeListGroup(p *printer, header string, defs []vocab.Definition) {
	if len(defs) == 0 {
		return
	}
	p.println()
	p.println(header)
	for _, d := range defs {
		p.printf("  %s\n", d.Name)
	}
}

func writeEntityListGroup(p *printer, header string, entities []vocab.Entity, markAggregate bool) {
	if len(entities) == 0 {
		return
	}
	p.println()
	p.println(header)
	for _, e := range entities {
		if markAggregate && e.Aggregate {
			p.printf("  %s [aggregate]\n", e.Name)
			continue
		}
		p.printf("  %s\n", e.Name)
	}
}

func writeShowText(w io.Writer, result application.DomainDefinitionView) error {
	p := &printer{w: w}
	doc := result.Concept.Doc()
	p.printf("%s: %s\n", doc.Title, result.Definition.Name)
	if result.Concept == vocab.ConceptEntity {
		if result.Aggregate {
			p.println("Aggregate: yes")
		} else {
			p.println("Aggregate: no")
		}
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
			}
		}
	default:
		p.printf("Unchanged %s %s.\n", typeSpelling, result.Name)
	}
	return p.err
}

func writeRemoveText(w io.Writer, result application.DomainRemoveResult) error {
	p := &printer{w: w}
	if result.Concept == vocab.ConceptAggregate || result.EntityPreserved {
		p.printf("Removed the Aggregate designation from %s.\n", result.Name)
		p.printf("The %s Entity remains defined.\n", result.Name)
		return p.err
	}
	p.printf("Removed %s %s from the project domain model.\n", result.Concept, result.Name)
	p.println("Source files were not changed.")
	return p.err
}

func writeTwoLineEntries(p *printer, defs []vocab.Definition) {
	for i, d := range defs {
		if i > 0 {
			p.println()
		}
		p.printf("  %s\n", d.Name)
		if d.Definition != "" {
			p.printf("  %s\n", d.Definition)
		}
	}
}

func writeTwoLineEntities(p *printer, entities []vocab.Entity) {
	for i, e := range entities {
		if i > 0 {
			p.println()
		}
		p.printf("  %s\n", e.Name)
		if e.Definition.Definition != "" {
			p.printf("  %s\n", e.Definition.Definition)
		}
	}
}

func writePaddedOneLiners(p *printer, defs []vocab.Definition) {
	width := 0
	for _, d := range defs {
		if d.Definition != "" && len(d.Name) > width {
			width = len(d.Name)
		}
	}
	for _, d := range defs {
		if d.Definition == "" {
			p.printf("  %s\n", d.Name)
			continue
		}
		p.printf("  %-*s — %s\n", width, d.Name, d.Definition)
	}
}

func writePaddedEntityOneLiners(p *printer, entities []vocab.Entity) {
	width := 0
	for _, e := range entities {
		if e.Definition.Definition != "" && len(e.Name) > width {
			width = len(e.Name)
		}
	}
	for _, e := range entities {
		if e.Definition.Definition == "" {
			p.printf("  %s\n", e.Name)
			continue
		}
		p.printf("  %-*s — %s\n", width, e.Name, e.Definition.Definition)
	}
}

func countPhrase(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func nonAggregateEntities(lang vocab.UbiquitousLanguage) []vocab.Entity {
	all := lang.ListEntities()
	out := make([]vocab.Entity, 0, len(all))
	for _, e := range all {
		if !e.Aggregate {
			out = append(out, e)
		}
	}
	return out
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
	case vocab.ConceptValueObject:
		return "Value objects"
	case vocab.ConceptBusinessRule:
		return "Business rules"
	case vocab.ConceptEvent:
		return "Domain events"
	default:
		return vocab.Listing(c)
	}
}

// --- JSON docs ------------------------------------------------------------

type domainCountsJSON struct {
	Entities      int `json:"entities"`
	Aggregates    int `json:"aggregates"`
	ValueObjects  int `json:"valueObjects"`
	BusinessRules int `json:"businessRules"`
	Events        int `json:"events"`
}

type domainDefJSON struct {
	Name       string   `json:"name"`
	Definition string   `json:"definition,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
	Aggregate  bool     `json:"aggregate,omitempty"`
}

type domainOverviewJSON struct {
	Source        string           `json:"source"`
	Found         bool             `json:"found"`
	Counts        domainCountsJSON `json:"counts"`
	Entities      []domainDefJSON  `json:"entities,omitempty"`
	ValueObjects  []domainDefJSON  `json:"valueObjects,omitempty"`
	BusinessRules []domainDefJSON  `json:"businessRules,omitempty"`
	Events        []domainDefJSON  `json:"events,omitempty"`
}

func overviewJSONDoc(result application.DomainOverview) domainOverviewJSON {
	counts := result.Language.Counts()
	doc := domainOverviewJSON{
		Source: result.Source,
		Found:  result.Found,
		Counts: domainCountsJSON{
			Entities:      counts.Entities,
			Aggregates:    counts.Aggregates,
			ValueObjects:  counts.ValueObjects,
			BusinessRules: counts.BusinessRules,
			Events:        counts.Events,
		},
	}
	if !result.Found {
		return doc
	}
	lang := result.Language
	doc.Entities = entitiesToJSON(lang.ListEntities(), true)
	doc.ValueObjects = defsToJSON(lang.List(vocab.ConceptValueObject))
	doc.BusinessRules = defsToJSON(lang.List(vocab.ConceptBusinessRule))
	doc.Events = defsToJSON(lang.List(vocab.ConceptEvent))
	return doc
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

func listJSONDoc(result application.DomainListing) map[string]any {
	lang := result.Language
	if result.Filtered {
		key := listJSONKey(result.Concept)
		switch result.Concept {
		case vocab.ConceptEntity:
			return map[string]any{key: listEntitiesJSON(lang.ListEntities(), result.Concept)}
		case vocab.ConceptAggregate:
			return map[string]any{key: listEntitiesJSON(lang.ListAggregates(), result.Concept)}
		default:
			return map[string]any{key: listDefsJSON(lang.List(result.Concept))}
		}
	}
	out := map[string]any{}
	if items := listEntitiesJSON(lang.ListEntities(), vocab.ConceptEntity); len(items) > 0 {
		out["entities"] = items
	}
	if items := listDefsJSON(lang.List(vocab.ConceptValueObject)); len(items) > 0 {
		out["valueObjects"] = items
	}
	if items := listDefsJSON(lang.List(vocab.ConceptBusinessRule)); len(items) > 0 {
		out["businessRules"] = items
	}
	if items := listDefsJSON(lang.List(vocab.ConceptEvent)); len(items) > 0 {
		out["events"] = items
	}
	return out
}

func listJSONKey(c vocab.Concept) string {
	switch c {
	case vocab.ConceptEntity:
		return "entities"
	case vocab.ConceptAggregate:
		return "aggregates"
	case vocab.ConceptValueObject:
		return "valueObjects"
	case vocab.ConceptBusinessRule:
		return "businessRules"
	case vocab.ConceptEvent:
		return "events"
	default:
		return string(c)
	}
}

func listDefsJSON(defs []vocab.Definition) []map[string]any {
	sorted := sortedDefs(defs)
	out := make([]map[string]any, 0, len(sorted))
	for _, d := range sorted {
		out = append(out, map[string]any{jsonKeyName: d.Name})
	}
	return out
}

func listEntitiesJSON(entities []vocab.Entity, concept vocab.Concept) []map[string]any {
	sorted := sortedEntities(entities)
	out := make([]map[string]any, 0, len(sorted))
	for _, e := range sorted {
		item := map[string]any{jsonKeyName: e.Name}
		if (concept == vocab.ConceptEntity || concept == vocab.ConceptAggregate) && e.Aggregate {
			item["aggregate"] = true
		}
		out = append(out, item)
	}
	return out
}

func showJSONDoc(result application.DomainDefinitionView) map[string]any {
	doc := map[string]any{
		jsonKeyType: string(result.Concept),
		jsonKeyName: result.Definition.Name,
	}
	if result.Concept == vocab.ConceptEntity || result.Concept == vocab.ConceptAggregate {
		doc["aggregate"] = result.Aggregate || result.Concept == vocab.ConceptAggregate
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
	if len(result.Changed) > 0 {
		doc["changed"] = result.Changed
	}
	if len(result.Aliases) > 0 {
		doc["aliases"] = result.Aliases
	}
	if req.Definition != nil && *req.Definition != "" {
		doc["definition"] = *req.Definition
	}
	return doc
}

func removeJSONDoc(result application.DomainRemoveResult) map[string]any {
	doc := map[string]any{
		jsonKeyType: string(result.Concept),
		jsonKeyName: result.Name,
		"result":    "removed",
	}
	if result.EntityPreserved || result.Concept == vocab.ConceptAggregate {
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
				return "", fmt.Errorf("read guided input: %w", err)
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
	if err := promptBlank(); err != nil {
		return err
	}
	options := []struct {
		title   string
		concept vocab.Concept
	}{
		{"Entity", vocab.ConceptEntity},
		{"Value Object", vocab.ConceptValueObject},
		{"Business Rule", vocab.ConceptBusinessRule},
		{"Domain Event", vocab.ConceptEvent},
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
		if err := prompt("Please choose 1-4 or the concept title."); err != nil {
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

	var term string
	for {
		if err := prompt("Project term:"); err != nil {
			return err
		}
		line, err := readLine()
		if err != nil {
			return guidedAborted(err)
		}
		if line != "" {
			term = line
			break
		}
	}

	if err := prompt(fmt.Sprintf("Define %s in the project's Ubiquitous Language:", term)); err != nil {
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
	if err := promptBlank(); err != nil {
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
	}
	if len(aliases) > 0 {
		if err := prompt(fmt.Sprintf("  Aliases: %s", strings.Join(aliases, ", "))); err != nil {
			return err
		}
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

	req := application.DefineDomainRequest{
		Concept:   string(chosen),
		Name:      term,
		Aliases:   aliases,
		Aggregate: aggregate,
	}
	if definition != "" {
		def := definition
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
		if p != "" {
			out = append(out, p)
		}
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
	return &ExitError{Code: ExitViolations, Message: "guided authoring aborted"}
}

// --- completion -----------------------------------------------------------

func completeListings() func(args []string, toComplete string) []AutoCompleteCandidate {
	return func(args []string, toComplete string) []AutoCompleteCandidate {
		if len(args) > 0 {
			return nil
		}
		return filterCandidates(toComplete, []string{
			"entities", "aggregates", "value-objects", "business-rules", "events",
		})
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
	result, err := list.Execute("")
	if err != nil {
		return nil
	}
	defs := result.Language.List(concept)
	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
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

// --- help text (verbatim from docs/domain-cli-recommendation.md) ----------

const domainGroupLong = `Inspect and maintain the project's ubiquitous language and domain model.

A project's Ubiquitous Language is the shared language used by its developers,
domain experts, documentation, tests, and code.

ArcLint records that language as a structured Project Domain Model containing
Entities, Aggregates, Value Objects, Business Rules, and Domain Events.

ArcLint makes this knowledge available to context, enabled lint rules, patterns,
extensions, and agent integrations.

Running arclint domain without a subcommand is the same as:

  arclint domain overview`

const overviewLong = `Summarize the project's domain model for understanding.

The overview presents stored definitions by their domain role. It does not
infer source boundaries, relationships, or path relevance.

Running arclint domain without a subcommand runs this command.`

const overviewExample = `  arclint domain
  arclint domain overview
  arclint domain overview --format json`

const listLong = `List the project's domain definitions.

Without a type, definitions are grouped by entities, aggregates, value objects,
business rules, and domain events.`

const listExample = `  arclint domain list
  arclint domain list entities
  arclint domain list aggregates
  arclint domain list business-rules
  arclint domain list events
  arclint domain list --format json`

const showLong = `Show one domain definition.

Names are matched exactly. Quote names containing spaces.`

const showExample = `  arclint domain show entity Order
  arclint domain show aggregate Order
  arclint domain show value-object Money
  arclint domain show business-rule OrderMustHaveCustomer
  arclint domain show event OrderPlaced
  arclint domain show entity "Purchase Order"
  arclint domain show entity Order --format json`

const explainLong = `Explain ArcLint's supported domain concepts.

Without a type, this command summarizes every supported concept. With a type,
it explains that concept and gives questions that help authors recognize it in
their project.`

const explainExample = `  arclint domain explain
  arclint domain explain entity
  arclint domain explain aggregate
  arclint domain explain business-rule
  arclint domain explain --format json`

const defineLong = `Create or update a domain definition.

If the named definition does not exist, ArcLint creates it. If it already
exists, ArcLint updates only the fields supplied by this command.

Running the same command repeatedly produces no additional change.

Defining an Aggregate designates the named Entity as an Aggregate. If the
Entity does not exist yet, ArcLint creates it as part of the same operation.

ArcLint validates the structure of the definition but does not decide whether
the project's real-world language is correct. Enabled lint rules may apply
additional project requirements.`

const defineExample = `  arclint domain define entity Order
  arclint domain define aggregate Order

  arclint domain define entity Order \
    --definition "A customer's request to purchase products"

  arclint domain define value-object Money \
    --definition "A monetary amount expressed in a particular currency"

  arclint domain define business-rule OrderMustHaveCustomer \
    --definition "Every Order must identify its Customer."

  arclint domain define event OrderPlaced \
    --definition "An Order has been accepted for processing."

  arclint domain define entity Order \
    --alias "Purchase Order" \
    --alias "Customer Order"

  arclint domain define --guided`

const removeLong = `Remove a domain definition.

This command changes only the project domain model. It never deletes or modifies
source files.

Removing an Aggregate removes the Aggregate designation while preserving the
Entity. Removing an Entity also removes its Aggregate designation.`

const removeExample = `  arclint domain remove aggregate Order
  arclint domain remove entity LegacyOrder
  arclint domain remove value-object LegacyOrderID
  arclint domain remove business-rule ObsoleteOrderRule
  arclint domain rm event LegacyOrderCreated
  arclint domain remove value-object LegacyOrderID --format json`

const schemaLong = `Print the JSON Schema accepted for ubiquitous-language.yaml.

The schema is the machine-readable contract for direct YAML authoring and
editor completion.`

const schemaExample = `  arclint domain schema
  arclint domain schema > ubiquitous-language.schema.json`
