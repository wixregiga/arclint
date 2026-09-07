package jsonreport

import (
	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

const (
	jsonKeyType    = "type"
	jsonKeyName    = "name"
	jsonKeyContext = "context"
	jsonKeyOwner   = "owner"
)

// domainCountsJSON tallies a recorded model; the same shape serves the
// overview and the domain block of `arclint context`.
type domainCountsJSON struct {
	Contexts       int `json:"contexts"`
	Aggregates     int `json:"aggregates"`
	Entities       int `json:"entities"`
	ValueObjects   int `json:"valueObjects"`
	Invariants     int `json:"invariants"`
	Assertions     int `json:"assertions"`
	Specifications int `json:"specifications"`
	Events         int `json:"events"`
	Services       int `json:"services"`
	Questions      int `json:"questions"`
	Relations      int `json:"relations"`
}

func countsDoc(c vocab.Counts) domainCountsJSON {
	return domainCountsJSON{
		Contexts:       c.Contexts,
		Aggregates:     c.Aggregates,
		Entities:       c.Entities,
		ValueObjects:   c.ValueObjects,
		Invariants:     c.Invariants,
		Assertions:     c.Assertions,
		Specifications: c.Specifications,
		Events:         c.Events,
		Services:       c.Services,
		Questions:      c.Questions,
		Relations:      c.Relations,
	}
}

type domainInvariantJSON struct {
	Key       string `json:"key"`
	Statement string `json:"statement"`
}

type domainAssertionJSON struct {
	Key       string `json:"key"`
	On        string `json:"on"`
	Statement string `json:"statement"`
}

type domainEntityJSON struct {
	Name       string   `json:"name"`
	Definition string   `json:"definition"`
	Identity   string   `json:"identity,omitempty"`
	Aliases    []string `json:"aliases,omitempty"`
}

type domainAggregateJSON struct {
	Name       string                `json:"name"`
	Definition string                `json:"definition"`
	Identity   string                `json:"identity"`
	Aliases    []string              `json:"aliases,omitempty"`
	Entities   []domainEntityJSON    `json:"entities,omitempty"`
	Invariants []domainInvariantJSON `json:"invariants,omitempty"`
	Assertions []domainAssertionJSON `json:"assertions,omitempty"`
	Repository string                `json:"repository,omitempty"`
	Factory    string                `json:"factory,omitempty"`
}

type domainValueObjectJSON struct {
	Name       string                `json:"name"`
	Definition string                `json:"definition"`
	Aliases    []string              `json:"aliases,omitempty"`
	Invariants []domainInvariantJSON `json:"invariants,omitempty"`
}

type domainEventJSON struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
	RaisedBy   string `json:"raisedBy,omitempty"`
}

type domainTermJSON struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
}

type domainQuestionJSON struct {
	Key  string `json:"key"`
	Text string `json:"text"`
}

type domainContextJSON struct {
	Name           string                  `json:"name"`
	Definition     string                  `json:"definition"`
	Aggregates     []domainAggregateJSON   `json:"aggregates,omitempty"`
	ValueObjects   []domainValueObjectJSON `json:"valueObjects,omitempty"`
	Events         []domainEventJSON       `json:"events,omitempty"`
	Services       []domainTermJSON        `json:"services,omitempty"`
	Specifications []domainTermJSON        `json:"specifications,omitempty"`
	Questions      []domainQuestionJSON    `json:"questions,omitempty"`
}

type domainRelationJSON struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
}

type domainOverviewJSON struct {
	Source      string               `json:"source"`
	Found       bool                 `json:"found"`
	Project     string               `json:"project,omitempty"`
	Description string               `json:"description,omitempty"`
	Counts      domainCountsJSON     `json:"counts"`
	Contexts    []domainContextJSON  `json:"contexts,omitempty"`
	Relations   []domainRelationJSON `json:"relations,omitempty"`
}

func overviewJSONDoc(result application.DomainOverview) domainOverviewJSON {
	doc := domainOverviewJSON{
		Source: result.Source,
		Found:  result.Found,
		Counts: countsDoc(result.Counts),
	}
	if !result.Found {
		return doc
	}
	doc.Project = result.Language.Project
	doc.Description = result.Language.Description
	for _, ctx := range result.Language.Contexts {
		doc.Contexts = append(doc.Contexts, contextJSONDoc(ctx))
	}
	doc.Relations = relationsToJSON(result.Language.Relations)
	return doc
}

func contextJSONDoc(ctx vocab.BoundedContext) domainContextJSON {
	doc := domainContextJSON{
		Name:       ctx.Name,
		Definition: ctx.Definition,
	}
	for _, a := range ctx.Aggregates {
		doc.Aggregates = append(doc.Aggregates, aggregateJSONDoc(a))
	}
	for _, v := range ctx.ValueObjects {
		doc.ValueObjects = append(doc.ValueObjects, valueObjectJSONDoc(v))
	}
	for _, e := range ctx.Events {
		doc.Events = append(doc.Events, eventJSONDoc(e))
	}
	for _, s := range ctx.Services {
		doc.Services = append(doc.Services, domainTermJSON{Name: s.Name, Definition: s.Definition})
	}
	for _, s := range ctx.Specifications {
		doc.Specifications = append(doc.Specifications, domainTermJSON{Name: s.Name, Definition: s.Definition})
	}
	for _, q := range ctx.Questions {
		doc.Questions = append(doc.Questions, domainQuestionJSON{Key: q.Key, Text: q.Text})
	}
	return doc
}

func aggregateJSONDoc(a vocab.Aggregate) domainAggregateJSON {
	doc := domainAggregateJSON{
		Name:       a.Name,
		Definition: a.Definition,
		Identity:   a.Identity,
		Aliases:    append([]string(nil), a.Aliases...),
		Invariants: invariantsJSON(a.Invariants),
		Assertions: assertionsJSON(a.Assertions),
		Repository: a.Repository,
		Factory:    a.Factory,
	}
	for _, e := range a.Entities {
		doc.Entities = append(doc.Entities, entityJSONDoc(e))
	}
	return doc
}

func entityJSONDoc(e vocab.Entity) domainEntityJSON {
	return domainEntityJSON{
		Name:       e.Name,
		Definition: e.Definition,
		Identity:   e.Identity,
		Aliases:    append([]string(nil), e.Aliases...),
	}
}

func valueObjectJSONDoc(v vocab.ValueObject) domainValueObjectJSON {
	return domainValueObjectJSON{
		Name:       v.Name,
		Definition: v.Definition,
		Aliases:    append([]string(nil), v.Aliases...),
		Invariants: invariantsJSON(v.Invariants),
	}
}

func eventJSONDoc(e vocab.DomainEvent) domainEventJSON {
	return domainEventJSON{Name: e.Name, Definition: e.Definition, RaisedBy: e.RaisedBy}
}

func invariantsJSON(invs []vocab.Invariant) []domainInvariantJSON {
	if len(invs) == 0 {
		return nil
	}
	out := make([]domainInvariantJSON, 0, len(invs))
	for _, inv := range invs {
		out = append(out, domainInvariantJSON{Key: inv.Key, Statement: inv.Statement})
	}
	return out
}

func assertionsJSON(as []vocab.Assertion) []domainAssertionJSON {
	if len(as) == 0 {
		return nil
	}
	out := make([]domainAssertionJSON, 0, len(as))
	for _, a := range as {
		out = append(out, domainAssertionJSON{Key: a.Key, On: a.On, Statement: a.Statement})
	}
	return out
}

func relationsToJSON(relations []vocab.ContextRelation) []domainRelationJSON {
	if len(relations) == 0 {
		return nil
	}
	out := make([]domainRelationJSON, 0, len(relations))
	for _, rel := range relations {
		out = append(out, domainRelationJSON{From: rel.From, To: rel.To, Kind: string(rel.Kind), Description: rel.Description})
	}
	return out
}

// --- list -------------------------------------------------------------

// An owned rule in a listing names the owner it sits under.
type domainOwnedInvariantJSON struct {
	Key       string `json:"key"`
	Owner     string `json:"owner"`
	Statement string `json:"statement"`
}

type domainOwnedAssertionJSON struct {
	Key       string `json:"key"`
	Owner     string `json:"owner"`
	On        string `json:"on"`
	Statement string `json:"statement"`
}

type domainOwnedEntityJSON struct {
	Name      string `json:"name"`
	Aggregate string `json:"aggregate"`
}

// listJSONDoc is the listing: every context with everything in it, or
// one section of each selected context when the listing is filtered.
func listJSONDoc(result application.DomainListing) map[string]any {
	contexts := selectedContexts(result)
	doc := map[string]any{
		"source":  result.Source,
		"found":   result.Found,
		"project": result.Language.Project,
	}
	if result.Context != "" {
		doc[jsonKeyContext] = result.Context
	}
	if !result.Filtered {
		docs := make([]domainContextJSON, 0, len(contexts))
		for _, ctx := range contexts {
			docs = append(docs, contextJSONDoc(ctx))
		}
		doc["contexts"] = docs
		if result.Context == "" {
			doc["relations"] = relationsToJSON(result.Language.Relations)
		}
		return doc
	}
	doc["listing"] = vocab.Listing(result.Concept)
	var entries []map[string]any
	for _, ctx := range contexts {
		entry := map[string]any{jsonKeyName: ctx.Name}
		if section, ok := listedSection(ctx, result.Concept); ok {
			entry[listJSONKey(result.Concept)] = section
		}
		entries = append(entries, entry)
	}
	if result.Concept == vocab.ConceptBoundedContext {
		var names []domainTermJSON
		for _, ctx := range contexts {
			names = append(names, domainTermJSON{Name: ctx.Name, Definition: ctx.Definition})
		}
		doc["contexts"] = names
		return doc
	}
	doc["contexts"] = entries
	return doc
}

// listedSection is the one section of a context a filtered listing
// shows, in the shape its concept has.
func listedSection(ctx vocab.BoundedContext, c vocab.Concept) (any, bool) {
	switch c {
	case vocab.ConceptAggregate, vocab.ConceptAggregateRoot:
		out := make([]domainAggregateJSON, 0, len(ctx.Aggregates))
		for _, a := range ctx.Aggregates {
			out = append(out, aggregateJSONDoc(a))
		}
		return out, true
	case vocab.ConceptEntity:
		var out []domainOwnedEntityJSON
		for _, a := range ctx.Aggregates {
			for _, e := range a.Entities {
				out = append(out, domainOwnedEntityJSON{Name: e.Name, Aggregate: a.Name})
			}
		}
		return out, true
	case vocab.ConceptValueObject:
		var out []domainValueObjectJSON
		for _, v := range ctx.ValueObjects {
			out = append(out, valueObjectJSONDoc(v))
		}
		return out, true
	case vocab.ConceptInvariant:
		return ownedInvariantsJSON(ctx), true
	case vocab.ConceptAssertion:
		return ownedAssertionsJSON(ctx), true
	case vocab.ConceptBusinessRule:
		return map[string]any{
			"invariants": ownedInvariantsJSON(ctx),
			"assertions": ownedAssertionsJSON(ctx),
		}, true
	case vocab.ConceptSpecification:
		var out []domainTermJSON
		for _, s := range ctx.Specifications {
			out = append(out, domainTermJSON{Name: s.Name, Definition: s.Definition})
		}
		return out, true
	case vocab.ConceptDomainEvent:
		var out []domainEventJSON
		for _, e := range ctx.Events {
			out = append(out, eventJSONDoc(e))
		}
		return out, true
	case vocab.ConceptDomainService:
		var out []domainTermJSON
		for _, s := range ctx.Services {
			out = append(out, domainTermJSON{Name: s.Name, Definition: s.Definition})
		}
		return out, true
	case vocab.ConceptQuestion:
		var out []domainQuestionJSON
		for _, q := range ctx.Questions {
			out = append(out, domainQuestionJSON{Key: q.Key, Text: q.Text})
		}
		return out, true
	default:
		return nil, false
	}
}

func ownedInvariantsJSON(ctx vocab.BoundedContext) []domainOwnedInvariantJSON {
	owned := ctx.Invariants()
	out := make([]domainOwnedInvariantJSON, 0, len(owned))
	for _, oi := range owned {
		out = append(out, domainOwnedInvariantJSON{Key: oi.Invariant.Key, Owner: oi.Owner, Statement: oi.Invariant.Statement})
	}
	return out
}

func ownedAssertionsJSON(ctx vocab.BoundedContext) []domainOwnedAssertionJSON {
	owned := ctx.Assertions()
	out := make([]domainOwnedAssertionJSON, 0, len(owned))
	for _, oa := range owned {
		out = append(out, domainOwnedAssertionJSON{Key: oa.Assertion.Key, Owner: oa.Owner, On: oa.Assertion.On, Statement: oa.Assertion.Statement})
	}
	return out
}

// listJSONKey is the section key a filtered listing uses for its
// concept, lowerCamel like every other key.
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
	case vocab.ConceptSpecification:
		return "specifications"
	case vocab.ConceptBusinessRule:
		return "businessRules"
	case vocab.ConceptDomainEvent:
		return "events"
	case vocab.ConceptDomainService:
		return "services"
	case vocab.ConceptQuestion:
		return "questions"
	case vocab.ConceptBoundedContext:
		return "contexts"
	default:
		return vocab.Listing(c)
	}
}

func selectedContexts(result application.DomainListing) []vocab.BoundedContext {
	if result.Context == "" {
		return result.Language.Contexts
	}
	ctx, ok := result.Language.Context(result.Context)
	if !ok {
		return nil
	}
	return []vocab.BoundedContext{ctx}
}

// --- show -------------------------------------------------------------

// showJSONDoc is one entry: its concept, name, where it sits, and the
// properties of its concept.
func showJSONDoc(v application.DomainEntryView) map[string]any {
	doc := map[string]any{
		jsonKeyType: string(v.Concept),
		jsonKeyName: v.Name,
	}
	if v.Concept != vocab.ConceptBoundedContext {
		doc[jsonKeyContext] = v.Context
	}
	if v.Owner != "" {
		doc[jsonKeyOwner] = v.Owner
	}
	switch v.Concept {
	case vocab.ConceptBoundedContext:
		ctx := v.BoundedContext
		doc["definition"] = ctx.Definition
		doc["aggregates"] = namesOf(len(ctx.Aggregates), func(i int) string { return ctx.Aggregates[i].Name })
		doc["valueObjects"] = namesOf(len(ctx.ValueObjects), func(i int) string { return ctx.ValueObjects[i].Name })
		doc["events"] = namesOf(len(ctx.Events), func(i int) string { return ctx.Events[i].Name })
		doc["services"] = namesOf(len(ctx.Services), func(i int) string { return ctx.Services[i].Name })
		doc["specifications"] = namesOf(len(ctx.Specifications), func(i int) string { return ctx.Specifications[i].Name })
		doc["questions"] = namesOf(len(ctx.Questions), func(i int) string { return ctx.Questions[i].Key })
		doc["relations"] = relationsToJSON(v.Relations)
	case vocab.ConceptAggregate:
		a := v.Aggregate
		doc["definition"] = a.Definition
		doc["identity"] = a.Identity
		if len(a.Aliases) > 0 {
			doc["aliases"] = a.Aliases
		}
		entities := make([]domainEntityJSON, 0, len(a.Entities))
		for _, e := range a.Entities {
			entities = append(entities, entityJSONDoc(e))
		}
		doc["entities"] = entities
		doc["invariants"] = presentList(invariantsJSON(a.Invariants))
		doc["assertions"] = presentList(assertionsJSON(a.Assertions))
		if a.Repository != "" {
			doc["repository"] = a.Repository
		}
		if a.Factory != "" {
			doc["factory"] = a.Factory
		}
	case vocab.ConceptEntity:
		doc["definition"] = v.Entity.Definition
		if v.Entity.Identity != "" {
			doc["identity"] = v.Entity.Identity
		}
		if len(v.Entity.Aliases) > 0 {
			doc["aliases"] = v.Entity.Aliases
		}
	case vocab.ConceptValueObject:
		doc["definition"] = v.ValueObject.Definition
		if len(v.ValueObject.Aliases) > 0 {
			doc["aliases"] = v.ValueObject.Aliases
		}
		doc["invariants"] = presentList(invariantsJSON(v.ValueObject.Invariants))
	case vocab.ConceptInvariant:
		doc["statement"] = v.Invariant.Statement
	case vocab.ConceptAssertion:
		doc["on"] = v.Assertion.On
		doc["statement"] = v.Assertion.Statement
	case vocab.ConceptDomainEvent:
		doc["definition"] = v.Event.Definition
		if v.Event.RaisedBy != "" {
			doc["raisedBy"] = v.Event.RaisedBy
		}
	case vocab.ConceptDomainService:
		doc["definition"] = v.Service.Definition
	case vocab.ConceptSpecification:
		doc["definition"] = v.Specification.Definition
	case vocab.ConceptQuestion:
		doc["text"] = v.Question.Text
	case vocab.ConceptAggregateRoot, vocab.ConceptBusinessRule:
		// Resolved to the aggregate, invariant, or assertion they name
		// before a view exists; no view carries them.
	}
	return doc
}

func namesOf(n int, at func(int) string) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, at(i))
	}
	return out
}

// presentList is a list a show document always carries: empty, never
// null, when the entry records nothing under it.
func presentList[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// --- explain ----------------------------------------------------------

func explainJSONDoc(doc vocab.ConceptDoc) map[string]any {
	sources := make([]map[string]any, 0, len(doc.Sources))
	for _, s := range doc.Sources {
		sources = append(sources, explainJSONSource(s))
	}
	return map[string]any{
		jsonKeyType: string(doc.Concept),
		"title":     doc.Title,
		"meaning":   doc.Meaning,
		"sources":   sources,
		"questions": doc.Questions,
		"supplies":  doc.Supplies,
	}
}

// explainJSONSource is one resolved citation: the work it names and the
// parts that narrow it, plus the readable spelling the text renderers
// print.
func explainJSONSource(s vocab.Reference) map[string]any {
	src := map[string]any{
		"work":     s.Work.Key,
		"title":    s.Work.Title,
		"citation": s.String(),
	}
	if len(s.Work.Authors) > 0 {
		src["authors"] = s.Work.Authors
	}
	if s.Work.Year != 0 {
		src["year"] = s.Work.Year
	}
	if s.Work.Date != "" {
		src["date"] = s.Work.Date
	}
	if s.Work.URL != "" {
		src["url"] = s.Work.URL
	}
	if s.Chapter != 0 {
		src["chapter"] = s.Chapter
	}
	if s.Page != "" {
		src["page"] = s.Page
	}
	if s.Section != "" {
		src["section"] = s.Section
	}
	return src
}

// --- define and remove ------------------------------------------------

// defineJSONDoc reports one define: the outcome, the entry, and the
// properties that changed with the values they now hold.
func defineJSONDoc(r cli.DomainDefineReport) map[string]any {
	result := r.Result
	doc := map[string]any{
		"result":    string(result.Outcome),
		jsonKeyType: string(result.Concept),
		jsonKeyName: result.Name,
	}
	if result.Context != "" {
		doc[jsonKeyContext] = result.Context
	}
	if result.Owner != "" {
		doc[jsonKeyOwner] = result.Owner
	}
	if len(result.Changed) > 0 {
		doc["changed"] = result.Changed
		values := map[string]any{}
		for _, property := range result.Changed {
			values[property] = changedValue(property, r.Change)
		}
		doc["values"] = values
	}
	return doc
}

// changedValue is what a changed property now holds, as the change
// recorded it.
func changedValue(property string, ch vocab.Change) any {
	switch property {
	case "definition":
		return ch.Definition
	case "statement":
		return ch.Statement
	case "text":
		return ch.Text
	case "on":
		return ch.On
	case "identity":
		return ch.Identity
	case "aliases":
		return emptyList(ch.Aliases)
	case "repository":
		return ch.Repository
	case "factory":
		return ch.Factory
	case "raised_by":
		return ch.RaisedBy
	default:
		return nil
	}
}

func emptyList(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func removeJSONDoc(result application.DomainRemoveResult) map[string]any {
	doc := map[string]any{
		jsonKeyType:          string(result.Concept),
		jsonKeyName:          result.Name,
		"result":             "removed",
		"sourceFilesChanged": false,
	}
	if result.Context != "" {
		doc[jsonKeyContext] = result.Context
	}
	if result.Owner != "" {
		doc[jsonKeyOwner] = result.Owner
	}
	if len(result.Also) > 0 {
		doc["also"] = result.Also
	}
	return doc
}
