package vocab

// Domain-librarian skill artifact names and default install directory.
const (
	// SkillDirectory is the default relative directory for skill artifacts.
	SkillDirectory = ".agents/skills/domain-librarian"
	// SkillProtocolFile is the skill entrypoint filename (SKILL.md).
	SkillProtocolFile = "SKILL.md"
	// SkillVocabularyFile is the VOCAB.yaml filename.
	SkillVocabularyFile = "VOCAB.yaml"
	// SkillLibrarySchemaFile is the library.schema.json filename.
	SkillLibrarySchemaFile = "library.schema.json"
)

// Skill frontmatter (SKILL.md).
const (
	SkillName        = "domain-librarian"
	SkillDescription = "Distill domain concepts from user input or analysis into a bounded-context-organized ubiquitous-language library file. Use when categorizing domain terms (entity, value_object, invariant, event), recording or maintaining a project's ubiquitous language, or resolving term conflicts across bounded contexts."
)

// Skill protocol body constants (SKILL.md), char-exact to the litmus file.
const (
	SkillTitle = "Domain Librarian"

	SkillIntro = "You classify domain concepts and maintain one library file per project. Success is measured on: correct categorization, smallest context consumed, fewest tools used, asking the right follow-up question when evidence is insufficient, and keeping the library consistent within bounded contexts."

	SkillReference = "Read `VOCAB.yaml` (same directory) once per session for the vocabulary, distillation rule ids with examples, clarification question banks, and the library file shape. This file carries the behavioral protocol; VOCAB.yaml carries the data."

	SkillEconomy = "Classify from the fewest facts that decide the litmus test; reading more \"for confidence\" is a failure. Prefer zero tool calls beyond: read VOCAB.yaml, read the library file, one write. Never ask what the input already answers."
)

// SkillProtocolRules returns the ordered SKILL.md protocol rules,
// unnumbered and char-exact to the litmus file. SkillMarkdown numbers
// them by position, so inserting a rule renumbers the ones after it.
func SkillProtocolRules() []string {
	return []string{
		"**Evidence.** The input is the source text verbatim; a paraphrase is not input. Every classification quotes the source fragment satisfying the litmus test and cites the deciding rule id. No quotable evidence = UNRESOLVED: ask, never classify.",
		"**Litmus first.** Before any kind is assigned, run the value-test on the new concept itself and quote the evidence: must two of these with equal values be told apart? Test the carried form explicitly — a concept that reads as a status, marking, or behavior OF a recorded term is that term's value or behavior unless evidence shows independent identity.",
		"**Inherited labels.** Pre-labeled terms are claims, not facts; re-run the litmus test and record PASS/FAIL. So is any modeling settled before this protocol loaded: re-derive the candidates under these rules — a pre-invocation conclusion is never a decision. A definition describing a record OF an occurrence, a snapshot at a moment, or telling one instance apart from later ones implies identity and FAILS the value-test.",
		"**Carried values.** An attribute the input says is carried, kept, or supplied by another term is that term's value; its identity question is already answered — asking it is a failure. Measurements, units, and amounts are value_object evidence and usually carry an invariant.",
		"**Code is not domain evidence.** Classification evidence comes from the input and the recorded language only. What the current implementation permits or forbids can flag a conflict; it can never close a candidate model.",
		"**Structure follows classification.** Files, repositories, and API slices a toolchain would require are consequences of a classification and count for nothing toward one. VOCAB's rules read one way only: they constrain designs and reject wrong boundaries; reading one backwards as classification evidence is a protocol violation. Needing to store or list something is design, not domain.",
		"**Boundaries.** A party that must be informed or notified is a second bounded_context: record it and its relation even when empty. Decisions ABOUT other terms (exclude, suppress, disable, override, snapshot) form their own governance context. Collapse synonyms to one canonical term with aliases. Mark `aggregate: true` only with quoted consistency evidence.",
		"**Ask or record — never guess.** One question per blocked concept, chosen from VOCAB's question banks by what it decides. A structural fork among surviving candidates — new aggregate versus value on an existing term, a new context, bypassing existing machinery — is a question for the domain expert regardless of partial evidence. Asking is always possible: the harness's question tool reaches the domain expert, and a subagent's question routes through its parent session — never assume nobody can answer. Zero unresolved terms is a red flag: re-scan definition-only evidence and skipped re-tests before finalizing. Having tools changes nothing — a write tool does not authorize resolving what the evidence cannot.",
		"**Precedence.** The conflict protocol outranks every other rule; routing around a conflict is a conflict. The conflict question fires when any CANDIDATE model would change a recorded entry's meaning, not only when an edit is made; choosing a different candidate to avoid the change while the question is unanswered is a protocol violation. No recorded entry's name, kind, or definition changes without an answered conflict question; inherited re-tests and language-fidelity renames PROPOSE changes, never authorize them.",
		"**Invariant gate.** A recorded invariant must forbid something a naive implementation could do; restating a definition is not an invariant. Name the concrete violation it prevents.",
		"**Output.** ALWAYS emit or write the complete library file per VOCAB's `library_file.shape`; a summary of it is a failure. Preserve unrelated entries byte-identical; edits surgical, additions alphabetized. Record business_rule inputs as resolved invariants/assertions with an owner.",
		"**Description style.** Definitions read like a document their humans own: plain sentences, no em dashes. Anything object-level a definition names must itself be recorded, or the mention is reworded into plain language. A term that lines up with a code object uses the code's exact spelling (TermCase, RuleID), never a prose-spaced variant.",
	}
}

// LibraryFile holds the library_file section of VOCAB.yaml.
type LibraryFile struct {
	Purpose    string
	JSONSchema string
	// JSONSchemaComment is the inline comment on the json_schema line.
	JSONSchemaComment string
	Header            string
	// HeaderComment is the inline comment on the header line.
	HeaderComment string
	// Shape is the human-readable shape block (without trailing newline
	// on the last content line beyond what the litmus stores).
	Shape string
	// ShapeComment is the inline comment on the shape: | line.
	ShapeComment string
	// RelationsShapeComment is the trailing comment on the relations shape line.
	RelationsShapeComment string
	Rules                 []string
}

// LibraryFileSpec returns the library_file section data char-exact to VOCAB.yaml.
func LibraryFileSpec() LibraryFile {
	return LibraryFile{
		Purpose:           "One YAML file per project, sole custody of the librarian; humans review and edit, the librarian writes.",
		JSONSchema:        SkillLibrarySchemaFile,
		JSONSchemaComment: "ships beside this file; gives human editors descriptions and validation",
		Header:            `# yaml-language-server: $schema=<relative path to library.schema.json, or its canonical $id URL when the schema file is not nearby>`,
		HeaderComment:     "first line of every written library file",
		ShapeComment:      "human-readable summary; library.schema.json is authoritative — on any divergence, the schema wins",
		Shape: `    version: 1
    contexts:
      - name: <context>
        entities: [{name, definition, aggregate: true?, aliases: []?}]
        value_objects: [{name, definition, aliases: []?}]
        invariants: [{statement, owner}]
        events: [{name, definition}]
    relations: [{from, to, kind}]   # kind = one context_relation key; omit when single context
`,
		RelationsShapeComment: "kind = one context_relation key; omit when single context",
		Rules: []string{
			"Every term carries a definition; no definition, no entry.",
			"A term lives in exactly one context; same word elsewhere is a second term.",
			"Aliases point at the canonical term; never duplicate definitions.",
			"Unresolved classifications are never written; ask first, record after.",
			"Definitions read as plain human sentences, without em dashes; humans own the document.",
			"Anything object-level a definition names is itself recorded, or the mention is reworded in plain language.",
			"A term matching a code object uses the code's exact spelling (TermCase, RuleID), never a prose-spaced variant.",
		},
	}
}

// VOCABHeaderComment is line 1 of VOCAB.yaml.
const VOCABHeaderComment = "# domain-librarian core reference: DDD vocabulary, distillation rules, clarification protocol."

// Schema document scaffolding (library.schema.json). Descriptions that
// embed taxonomy data are built in schema.go; these are fixed prose.
const (
	SchemaID    = "https://raw.githubusercontent.com/wixregiga/arclint/main/.agents/skills/domain-librarian/library.schema.json"
	SchemaDraft = "https://json-schema.org/draft/2020-12/schema"
	SchemaTitle = "domain-librarian ubiquitous-language library"

	SchemaDescription = "The project's recorded Ubiquitous Language, organized by bounded context. The domain-librarian has sole write custody; humans review and edit under the same rules: every term carries a definition, a term lives in exactly one context, aliases point at the canonical term, and unresolved classifications are never written."

	SchemaVersionDescription = "Document version. This library accepts version 1 only."

	SchemaContextsDescription = "One entry per bounded context: an explicit boundary within which one model applies and every term has exactly one meaning."

	SchemaContextNameDescription = "The bounded context's name. A party that must be informed or notified is its own context, recorded even while empty."

	SchemaEntitiesDescription = "Domain concepts defined by identity that persists across attribute change (identity-test: it stays the same thing when its attributes change)."

	SchemaValueObjectsDescription = "Immutable, identity-less concepts described entirely by their values (value-test: two with identical values are interchangeable). Measurements, units, and amounts belong here."

	SchemaInvariantsDescription = "Must-always/must-never rules that hold at all times within this context. A valid invariant forbids something a naive implementation could do; restating a definition is not an invariant."

	SchemaEventsDescription = "Domain events: things that happened that experts care about, named in past-tense expert language (event-detection). Technical changes are not events."

	SchemaRelationsDescription = "Context-map edges between bounded contexts. Omit when a single context exists."

	SchemaCanonicalNameDescription = "Canonical term, exactly as domain experts say it (language-fidelity). A term that lines up with a code object uses the code's exact spelling (TermCase, RuleID), never a prose-spaced variant."

	SchemaEntityDefinitionDescription = "What the term means, grounded in expert language. A term without a definition is rejected, not stored. Written for humans first: plain sentences, no em dashes; anything object-level it names must be recorded in this library or reworded in plain language."

	SchemaValueDefinitionDescription = "What the value describes. A term without a definition is rejected, not stored. Written for humans first: plain sentences, no em dashes; anything object-level it names must be recorded in this library or reworded in plain language."

	SchemaEntityAliasesDescription = "Synonyms collapsed onto this canonical term (synonym-collapse). Aliases never carry their own definitions."

	SchemaValueAliasesDescription = "Synonyms collapsed onto this canonical term. Aliases never carry their own definitions."

	SchemaAggregateFlagDescription = "True only with quoted consistency evidence: an invariant spanning a cluster that must change in one transaction. Never true by default."

	SchemaStatementDescription = "The rule, phrased so the concrete forbidden violation is clear."

	SchemaOwnerDescription = "Exactly one enforcing term in this context: an entity/aggregate root (enforced in behavior) or a value_object (enforced at construction)."

	SchemaEventNameDescription = "Past-tense event name, e.g. OrderConfirmed."

	SchemaEventDefinitionDescription = "What happened and who cares, including any party that must be informed. Written for humans first: plain sentences, no em dashes; anything object-level it names must be recorded in this library or reworded in plain language."

	SchemaFromDescription = "Upstream context name (must match a contexts[].name)."

	SchemaToDescription = "Downstream context name (must match a contexts[].name)."
)
