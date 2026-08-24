package vocab

import (
	"fmt"
	"strings"
)

// RelationKind is one context-map edge kind between bounded contexts.
// Values match library.schema.json's kind enum order exactly.
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
// downstream, Kind is the relationship pattern.
type ContextRelation struct {
	From string
	To   string
	Kind RelationKind
}

// RelationKindDoc is the ArcLint-owned meaning of one RelationKind.
// Meaning is the VOCAB.yaml one-liner; SchemaMeaning is the phrase used
// in library.schema.json's kind description. They match for every kind
// except shared_kernel (VOCAB omits "model"; schema includes it).
type RelationKindDoc struct {
	Kind          RelationKind
	Meaning       string
	SchemaMeaning string
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

// RelationKindDocs returns documentation for every kind in enum order.
func RelationKindDocs() []RelationKindDoc {
	return []RelationKindDoc{
		{Kind: RelationPartnership, Meaning: "succeed/fail together", SchemaMeaning: "succeed/fail together"},
		// VOCAB.yaml: "small jointly-owned subset"
		// library.schema.json: "small jointly-owned model subset"
		{Kind: RelationSharedKernel, Meaning: "small jointly-owned subset", SchemaMeaning: "small jointly-owned model subset"},
		{Kind: RelationCustomerSupplier, Meaning: "upstream plans for downstream", SchemaMeaning: "upstream plans for downstream"},
		{Kind: RelationConformist, Meaning: "downstream adopts upstream model", SchemaMeaning: "downstream adopts upstream model"},
		{Kind: RelationAnticorruptionLayer, Meaning: "downstream translates defensively", SchemaMeaning: "downstream translates defensively"},
		{Kind: RelationOpenHostService, Meaning: "upstream exposes one protocol", SchemaMeaning: "upstream exposes one protocol"},
		{Kind: RelationPublishedLanguage, Meaning: "shared interchange language", SchemaMeaning: "shared interchange language"},
		{Kind: RelationSeparateWays, Meaning: "no integration", SchemaMeaning: "no integration"},
	}
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

// SchemaKindDescription builds the library.schema.json kind.description
// text from RelationKindDocs so schema and VOCAB cannot drift on the
// seven identical meanings (shared_kernel uses SchemaMeaning).
func SchemaKindDescription() string {
	docs := RelationKindDocs()
	parts := make([]string, len(docs))
	for i, d := range docs {
		parts[i] = string(d.Kind) + " = " + d.SchemaMeaning
	}
	return "Relationship kind: " + strings.Join(parts, "; ") + "."
}

// ContextRelationFlowYAML builds the VOCAB.yaml context_relation flow
// mapping from RelationKindDocs (uses Meaning, not SchemaMeaning).
func ContextRelationFlowYAML() string {
	docs := RelationKindDocs()
	parts := make([]string, len(docs))
	for i, d := range docs {
		parts[i] = string(d.Kind) + ": " + d.Meaning
	}
	return "{" + strings.Join(parts, ", ") + "}"
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
