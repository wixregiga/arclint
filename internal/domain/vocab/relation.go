package vocab

import (
	"fmt"
	"strings"
)

// RelationKind is one context-map edge kind between bounded contexts.
// Values match domain.arclint.schema.json's kind enum order exactly.
type RelationKind string

// The eight context_relation kinds in schema-enum order.
const (
	RelationPartnership         RelationKind = "partnership"
	RelationSharedKernel        RelationKind = "shared_kernel"
	RelationCustomerSupplier    RelationKind = "customer_supplier"
	RelationConformist          RelationKind = "conformist"
	RelationAnticorruptionLayer RelationKind = "anticorruption_layer"
	RelationOpenHostService     RelationKind = "open_host_service"
	RelationPublishedLanguage   RelationKind = "published_language"
	RelationSeparateWays        RelationKind = "separate_ways"
)

// ContextRelation is one context-map edge: From is upstream, To is
// downstream, Kind is the relationship pattern, and Description says
// what passes between the two and which way, in the ubiquitous
// language. Line is where the relation is written down in the recorded
// Ubiquitous Language file, 0 for a relation that is not written down
// yet.
type ContextRelation struct {
	From        string
	To          string
	Kind        RelationKind
	Description string
	Line        int
}

// Influences reports whether code of context a may import code of
// context b under this relation: under a one-way kind only the
// downstream imports the upstream; under partnership or shared_kernel
// both may; under separate_ways neither does.
func (r ContextRelation) Influences(a, b string) bool {
	switch r.Kind {
	case RelationPartnership, RelationSharedKernel:
		return (r.From == a && r.To == b) || (r.From == b && r.To == a)
	case RelationSeparateWays:
		return false
	default:
		return r.To == a && r.From == b
	}
}

// RelationKindDoc is the meaning of one RelationKind: the kind's
// definition in the meta-model's context_relation block, with its
// sources.
type RelationKindDoc struct {
	Kind    RelationKind
	Meaning string
	Sources []Reference
}

// RelationKinds returns the eight kinds in schema-enum order.
func RelationKinds() []RelationKind {
	return []RelationKind{
		RelationPartnership,
		RelationSharedKernel,
		RelationCustomerSupplier,
		RelationConformist,
		RelationAnticorruptionLayer,
		RelationOpenHostService,
		RelationPublishedLanguage,
		RelationSeparateWays,
	}
}

// RelationKindDocs returns documentation for every kind in enum order,
// read from the meta-model; a kind the meta-model does not define is
// documented by its spelling alone.
func RelationKindDocs() []RelationKindDoc {
	model := DDD()
	block, _ := model.Block("context_relation")
	docs := make([]RelationKindDoc, 0, len(RelationKinds()))
	for _, k := range RelationKinds() {
		doc := RelationKindDoc{Kind: k}
		for _, kind := range block.Records.Kinds {
			if kind.Name == string(k) {
				doc.Meaning = kind.Definition
				doc.Sources = model.References(kind.Sources)
			}
		}
		docs = append(docs, doc)
	}
	return docs
}

// ParseRelationKind accepts one kind spelling.
func ParseRelationKind(s string) (RelationKind, error) {
	for _, k := range RelationKinds() {
		if RelationKind(s) == k {
			return k, nil
		}
	}
	return "", fmt.Errorf("context relation kind %q: not one of %s", s, joinRelationKinds(RelationKinds()))
}

// Doc returns the ArcLint-owned documentation for this RelationKind.
func (k RelationKind) Doc() RelationKindDoc {
	for _, d := range RelationKindDocs() {
		if d.Kind == k {
			return d
		}
	}
	return RelationKindDoc{Kind: k}
}

// SchemaKindDescription builds the domain.arclint.schema.json
// kind.description text from RelationKindDocs, one line per kind, so
// the schema and the vocabulary cannot drift.
func SchemaKindDescription() string {
	var b strings.Builder
	b.WriteString("Relationship kind, one of:")
	for _, d := range RelationKindDocs() {
		b.WriteString("\n")
		b.WriteString(string(d.Kind))
		b.WriteString(": ")
		b.WriteString(d.Meaning)
	}
	return b.String()
}

func joinRelationKinds(ks []RelationKind) string {
	parts := make([]string, len(ks))
	for i, k := range ks {
		parts[i] = string(k)
	}
	return strings.Join(parts, ", ")
}

func cloneRelations(in []ContextRelation) []ContextRelation {
	if in == nil {
		return nil
	}
	out := make([]ContextRelation, len(in))
	copy(out, in)
	return out
}
