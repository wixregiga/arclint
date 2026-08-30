package jsonreport

import (
	"sort"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const (
	jsonKeyType = "type"
	jsonKeyName = "name"
)

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

func defineJSONDoc(r cli.DomainDefineReport) map[string]any {
	result := r.Result
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
	if r.Definition != nil && *r.Definition != "" {
		doc["definition"] = *r.Definition
	}
	if r.Owner != "" {
		doc["owner"] = r.Owner
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

func isEntityShow(c vocab.Concept) bool {
	return c == vocab.ConceptEntity || c == vocab.ConceptAggregate || c == vocab.ConceptAggregateRoot
}

func isInvariantShow(c vocab.Concept) bool {
	return c == vocab.ConceptInvariant || c == vocab.ConceptAssertion || c == vocab.ConceptBusinessRule
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
