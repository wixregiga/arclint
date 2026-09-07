package vocab

// Domain-librarian skill artifact names and default install directory.
const (
	// SkillDirectory is the default relative directory for skill artifacts.
	SkillDirectory = ".agents/skills/domain-librarian"
	// SkillProtocolFile is the skill entrypoint filename (SKILL.md).
	SkillProtocolFile = "SKILL.md"
	// SkillVocabularyFile is the VOCAB.yaml filename.
	SkillVocabularyFile = "VOCAB.yaml"
)

// Where the published schemas live. SchemaDirectory is the
// project-relative directory arclint writes every generated JSON
// Schema into; SchemaFileName is the Ubiquitous Language schema under
// it, and SchemaPath is the two joined, the path a library file's
// modeline names when the schema is nearby.
const (
	SchemaDirectory = ".arclint/schemas"
	SchemaFileName  = "domain.arclint.schema.json"
	SchemaPath      = SchemaDirectory + "/" + SchemaFileName
)

// Skill frontmatter (SKILL.md).
const (
	SkillName        = "domain-librarian"
	SkillDescription = "Distill domain concepts from user input or analysis into a bounded-context-organized ubiquitous-language library file. Use when categorizing domain terms (aggregate, entity, value_object, invariant, assertion, specification, event, service), recording or maintaining a project's ubiquitous language, or resolving term conflicts across bounded contexts."
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
		"**Litmus first.** Before any kind is assigned, run the value-test on the new concept itself and quote the evidence: must two of these with equal values be told apart? Test the carried form explicitly: a concept that reads as a status, marking, or behavior OF a recorded term is that term's value or behavior unless evidence shows independent identity.",
		"**Inherited labels.** Pre-labeled terms are claims, not facts; re-run the litmus test and record PASS/FAIL. So is any modeling settled before this protocol loaded: re-derive the candidates under these rules; a pre-invocation conclusion is never a decision. A definition describing a record OF an occurrence, a snapshot at a moment, or telling one instance apart from later ones implies identity and FAILS the value-test.",
		"**Carried values.** An attribute the input says is carried, kept, or supplied by another term is that term's value; its identity question is already answered; asking it is a failure. Measurements, units, and amounts are value_object evidence and usually carry an invariant.",
		"**Code is not domain evidence.** Classification evidence comes from the input and the recorded language only. What the current implementation permits or forbids can flag a conflict; it can never close a candidate model.",
		"**Structure follows classification.** Files, repositories, and API slices a toolchain would require are consequences of a classification and count for nothing toward one. VOCAB's rules read one way only: they constrain designs and reject wrong boundaries; reading one backwards as classification evidence is a protocol violation. Needing to store or list something is design, not domain.",
		"**Boundaries.** A party that must be informed or notified is a second bounded_context: record it and its relation even when empty. Decisions ABOUT other terms (exclude, suppress, disable, override, snapshot) form their own governance context. Collapse synonyms to one canonical term with aliases. Record a term under `aggregates` only with quoted consistency evidence: an invariant spanning a cluster that must change in one transaction; every other entity is a member of the aggregate that owns it, and an entity that belongs to no aggregate is a question, not an entry.",
		"**Ask or record, never guess.** One question per blocked concept, chosen from VOCAB's question banks by what it decides. A structural fork among surviving candidates (new aggregate versus value on an existing term, a new context, bypassing existing machinery) is a question for the domain expert regardless of partial evidence. Asking is always possible: the harness's question tool reaches the domain expert, and a subagent's question routes through its parent session; never assume nobody can answer. A question nobody can answer this session is written under the context's `questions`, keyed, so the library shows what is undecided. Zero unresolved terms is a red flag: re-scan definition-only evidence and skipped re-tests before finalizing. Having tools changes nothing; a write tool does not authorize resolving what the evidence cannot.",
		"**Precedence.** The conflict protocol outranks every other rule; routing around a conflict is a conflict. The conflict question fires when any CANDIDATE model would change a recorded entry's meaning, not only when an edit is made; choosing a different candidate to avoid the change while the question is unanswered is a protocol violation. No recorded entry's name, kind, or definition changes without an answered conflict question; inherited re-tests and language-fidelity renames PROPOSE changes, never authorize them.",
		"**Invariant gate.** A recorded invariant must forbid something a naive implementation could do, and it is one violation, one complete sentence in the domain's own voice, with the owning term as its subject. Always-true rules go under their owner's `invariants`, keyed; a post-condition of a named operation goes under the aggregate's `assertions` with `on`; a predicate experts pass around as a thing goes under `specifications`, never as a flag on a value object. Value integrity is an invariant under a value object (constructor only); a cluster invariant is under an aggregate and its key names the root method that enforces it. Never record a programming-only guard. Would you say this to an expert who never saw the language? An \"and\" joining independently violable clauses is several entries; the narrative that connects them belongs in the owning term's definition. Restating a definition is not an invariant, and \"the system shall\" is not the domain speaking.",
		"**Completeness sweep.** Before finishing, re-scan the source text for must/never promises no recorded invariant carries; record each under its owner or raise it as a question. A promise living only in tests or views is unrecorded.",
		"**Output.** ALWAYS emit or write the complete library file per VOCAB's `library_file.shape`; a summary of it is a failure. Preserve unrelated entries byte-identical; edits surgical, additions alphabetized. Record business_rule inputs as resolved invariants or assertions under an owner; never as a specification.",
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
	Rules        []string
}

// LibraryFileSpec returns the library_file section data char-exact to VOCAB.yaml.
func LibraryFileSpec() LibraryFile {
	return LibraryFile{
		Purpose:           "One YAML file per project, sole custody of the librarian; humans review and edit, the librarian writes.",
		JSONSchema:        SchemaPath,
		JSONSchemaComment: "written by arclint domain schema --write; gives human editors descriptions and validation",
		Header:            `# yaml-language-server: $schema=<relative path to ` + SchemaPath + `, or its canonical $id URL when the schema file is not nearby>`,
		HeaderComment:     "first line of every written library file",
		ShapeComment:      "human-readable summary; " + SchemaFileName + " is authoritative; on any divergence, the schema wins",
		Shape: `    version: 1
    project: <name>
    description: <the domain in a few sentences>?
    contexts:
      <context>:
        definition: <what the context is responsible for>
        aggregates:
          <Aggregate>:
            definition: <text>
            identity: <ValueObject>
            aliases: [<name>]?
            entities: {<Entity>: {definition, identity?, aliases?}}?
            invariants: {<key>: <statement>}?    # key names the root method that enforces it
            assertions: {<key>: {on: <Operation>, statement}}?
            repository: <Name>?
            factory: <Name>?
        value_objects: {<ValueObject>: {definition, aliases?, invariants: {<key>: <statement>}?}}
        events: {<Event>: {definition, raised_by: <Aggregate>?}}
        services: {<Service>: {definition}}
        specifications: {<Specification>: {definition}}
        questions: {<key>: <text>}
    relations: [{from, to, kind, description?}]   # kind = one context_relation key; omit when single context
`,
		Rules: []string{
			"Every term carries a definition; no definition, no entry.",
			"A term lives in exactly one context; same word elsewhere is a second term.",
			"Aliases point at the canonical term; never duplicate definitions.",
			"Every entity lives under the aggregate that owns it; an aggregate names its identity; an invariant lives under the one aggregate or value object that enforces it.",
			"Unresolved classifications are never written; an open question is written under `questions`, keyed, and nothing is enforced from it.",
			"Definitions read as plain human sentences, without em dashes; humans own the document.",
			"Anything object-level a definition names is itself recorded, or the mention is reworded in plain language.",
			"A term matching a code object uses the code's exact spelling (TermCase, RuleID), never a prose-spaced variant.",
		},
	}
}

// VOCABHeaderComment is line 1 of VOCAB.yaml.
const VOCABHeaderComment = "# domain-librarian core reference: DDD vocabulary, distillation rules, clarification protocol."

// Schema document scaffolding (domain.arclint.schema.json). Every
// property description comes from the meta-model's recordings; these
// are the fixed prose of the document itself.
const (
	SchemaID    = "https://raw.githubusercontent.com/wixregiga/arclint/main/docs/schemas/" + SchemaFileName
	SchemaDraft = "https://json-schema.org/draft/2020-12/schema"
	SchemaTitle = "domain-librarian ubiquitous-language library"

	SchemaDescription = "The project's recorded Ubiquitous Language, organized by bounded context. The domain-librarian has sole write custody; humans review and edit under the same rules. Every term carries a definition, and a term lives in exactly one context. Every entity lives under its aggregate, and every invariant lives under the term that enforces it. Aliases point at the canonical term. Unresolved classifications are written as questions, never guessed."

	SchemaVersionDescription = "Document version. This library accepts version 1 only."
)
