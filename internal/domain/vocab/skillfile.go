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
	SkillDescription = "Distill domain concepts from user input or analysis into a bounded-context-organized ubiquitous-language library file. Use when categorizing domain terms (aggregate, entity, value_object, invariant, assertion, specification, event, domain_service), recording or maintaining a project's ubiquitous language, or resolving term conflicts across bounded contexts."
)

// Skill protocol body constants (SKILL.md), char-exact to the litmus file.
const (
	SkillTitle = "Domain Librarian"

	SkillIntro = "You maintain the project's shared domain language and classify concepts only where the evidence justifies a tactical model. Keep meanings faithful to the user's decisions, responsibilities clear, and the library consistent within bounded contexts. Ask only for a decision that the available evidence does not resolve."

	SkillReference = "Read `VOCAB.yaml` (same directory) once per session for the vocabulary, distillation rule ids with examples, clarification question banks, and the library file shape. This file carries the behavioral protocol; VOCAB.yaml carries the data."

	SkillEconomy = "Classify from the fewest facts that decide the litmus test; reading more \"for confidence\" is a failure. Prefer zero tool calls beyond: read VOCAB.yaml, read the library file, one write. Never ask what the input already answers."
)

// SkillProtocolRules returns the ordered SKILL.md protocol rules,
// unnumbered and char-exact to the litmus file. SkillMarkdown numbers
// them by position, so inserting a rule renumbers the ones after it.
func SkillProtocolRules() []string {
	return []string{
		"**Evidence.** Use the source text and explicit user decisions, quoting the fragment that supports each meaning or classification and citing the deciding rule id. Recordings, assistant definitions, issue checklists and passing tests are claims to examine; they do not independently establish user approval. Missing classification evidence means do not assign a kind; it does not make an understood word meaningless.",
		"**Litmus first.** Before assigning a tactical kind, establish that the project needs to model the concept that way. Run the value-test on the concept itself: must two of these with equal values be told apart? A system, its instructions, packaging, host configuration and running instance are different candidates; evidence about one cannot classify another. When a meaning is established but no tactical kind is justified, explain it in its context definition; do not force a value object, entity or aggregate, or invent code to hold it.",
		"**Inherited labels.** Re-test inherited classifications against their cited evidence. Distinguish PASS, FAIL and NOT ESTABLISHED; absence of evidence is not a positive identity test. Respect explicit user decisions wherever they occurred. A configuration record or a snapshot does not alone establish identity of the concept it describes.",
		"**Carried values.** An attribute the input says is carried, kept, or supplied by another term is that term's value; its identity question is already answered; asking it is a failure. Measurements, units, and amounts may support a value_object classification; record an invariant only when its own evidence passes the invariant gate.",
		"**Code is not domain evidence.** Classification evidence comes from the input and the recorded language only. What the current implementation permits or forbids can flag a conflict; it can never close a candidate model.",
		"**Structure follows classification.** Files, repositories, and API slices a toolchain would require are consequences of a classification and count for nothing toward one. VOCAB's rules read one way only: they constrain designs and reject wrong boundaries; reading one backwards as classification evidence is a protocol violation. Needing to store or list something is design, not domain.",
		"**Boundaries.** A notification recipient does not by itself establish a bounded_context; identify a distinct model and responsibility before recording a context and relation. Decisions ABOUT other terms (exclude, suppress, disable, override, snapshot) may belong to a governance context when their language and ownership justify that boundary. Collapse synonyms to one canonical term with aliases. Record a term under `aggregates` only with quoted consistency evidence: an invariant spanning a cluster that must change in one transaction; every other entity is a member of the aggregate that owns it, and an entity that belongs to no aggregate is a question, not an entry.",
		"**Ask or record, never guess.** Resolve questions from explicit user decisions and available evidence first; never ask what they already answer. For a material decision that remains open, ask one question chosen by what it decides, or record it under the context's `questions` when it cannot be answered this session. A known meaning without a justified tactical kind belongs in the context definition, not in a manufactured question or object. The number of open questions is not a quality target. Tool access does not authorize deciding for the user.",
		"**Precedence.** Expose conflicts with recorded meanings before implementing a correction. An explicit user correction already supplies authority to replace the rejected meaning; cite it and record the justified decision before code changes. Otherwise obtain or record the unresolved decision. Do not preserve an unsupported classification merely because it is already recorded, or route around a conflict by renaming the candidate.",
		"**Invariant gate.** A recorded invariant must forbid something a naive implementation could do, and it is one violation, one complete sentence in the domain's own voice, with the owning term as its subject. First establish domain consistency and a capable owner; must/never wording alone is insufficient. Reviewer instructions, contributor duties, host guarantees and delivery validation do not automatically pass this gate. A justified domain rule holding at all observable times goes under its owner's `invariants`, keyed; a justified guarantee of one named domain operation goes under its owner's `assertions` (the aggregate's, or the domain service's) with `on` naming the operation and the statement in the expert's words; a predicate experts pass around as a thing goes under `specifications`, never as a flag on a value object. Value integrity is an invariant under a value object (constructor only); a cluster invariant is under an aggregate and its key names the root method that enforces it. Never record a programming-only guard. Would you say this to an expert who never saw the language? An \"and\" joining independently violable clauses is several entries; the narrative that connects them belongs in the owning term's definition. Restating a definition is not an invariant, and \"the system shall\" is not the domain speaking.",
		"**Behavior.** Every operation the source names gets one home, and the librarian says which: a command of one root when it changes that aggregate's state and returns no domain information; a query when it answers a question and changes nothing; a `domain_service` when it makes a business decision that no one aggregate can make alone, because it spans aggregates or needs knowledge no aggregate holds; an `application_service` when its steps are loading, invoking, committing, publishing, notifying, or translating. Try the thing first: a behavior that fits one aggregate goes there, and only what no entity or value naturally owns becomes a service. A domain service is recorded under `services` with its contract: one assertion per guarantee under `assertions`, keyed, with `on` naming the operation. Commands, queries, and application services are not recorded; a business rule found inside an application service is placed under its owner and reported as a finding. The bare word service is never an answer: write `domain_service` or `application_service`. When the text cannot decide, ask \"Is <X> a result the business names, or a step the software takes to deliver one?\", or record it under `questions`.",
		"**Completeness sweep.** Re-scan the source text and account for each obligation: what the reviewer owes its user, what contributors must do to implement it, what the host and installer provide, and what a justified domain rule requires. State where each is implemented and how it is verified. Assign an invariant or assertion only after the invariant gate and owner evidence hold; reviewer instructions, contributor workflow and delivery validation do not automatically become domain invariants. Report missing enforcement and missing evidence separately.",
		"**Output.** ALWAYS emit or write the complete library file per VOCAB's `library_file.shape`; a summary of it is a failure. Preserve unrelated entries byte-identical; edits surgical, additions alphabetized. Record business_rule inputs as invariants or assertions under an owner, as a specification when experts pass the predicate around as a thing, or as no entry when they are a programming guard or a promise about delivery; cite the rule id that decided each placement.",
		"**Description style.** Definitions read like a document their humans own: plain sentences, no em dashes. A definition may explain established vocabulary without inventing tactical entries for every noun. Record a tactical entry only when its classification is justified. A term that lines up with a code object uses the code's exact spelling (TermCase, RuleID), never a prose-spaced variant.",
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
        services: {<Service>: {definition, assertions: {<key>: {on: <Operation>, statement}}?}}
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
			"Established vocabulary may be explained in context definitions without tactical entries; tactical classifications require their own evidence.",
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

// SkillAgentWorkflow connects the classification protocol to installed lifecycle checks.
const SkillAgentWorkflow = `When the user explicitly requests native hook setup, run arclint agents setup --host codex or --host omp in the project. Setup preserves existing rules and recordings, installs this skill and native hooks, and prints activation steps. A request to use ArcLint or review a domain does not itself request hook installation. Do not grant host trust yourself. Standalone arclint init remains available for ruleset-only use.

Before edits, run arclint context for affected paths and relate the intended change to the request and established domain decisions. Explain and record a justified domain decision before changing code. Reuse existing concepts; do not introduce a new evaluator category to deliver an ordinary Rule. The configured domainFiles are recordings; domainSourcePatterns explicitly selects Go domain code. sourcePatterns provides supporting evidence and does not impose the source comment policy. Do not broaden scope to make a finding convenient.

Keep the concepts distinct: an Agent is a system that performs work; a Coding Agent specializes in software work; an Agent Host provides the runtime and tools; a Client mediates the user's interaction with agents; an LLM is a model component. Agent Instructions guide the work; AGENTS.md supplies repository instructions; a Skill packages task guidance and resources; a Plugin packages capabilities under a host contract. Use the project's evidence for these meanings and their relationships. Neither installed instructions nor host configuration is the running Agent. A shared word can be explained in a context definition without becoming a tactical object or requiring a Go type.

Keep the reviewer's obligations to its user, contributor duties, and host and installer guarantees clear. An instruction file implements reviewer guidance; inspect its obligations and evaluate representative behavior. Generated-file equality proves synchronization. Installation status reports installed scope and integrity. Neither proves host activation or successful review. Read-only instructions prescribe reviewer behavior; host permissions determine the actual runtime boundary.

ArcLint Domain Reviewer is a separately invoked Coding Agent installed with arclint agents reviewer install --host codex. It examines the builder's domain basis and gives evidence-backed feedback before, during or after work. The builder explains and records justified meanings and implements repairs. The reviewer independently questions unsupported recordings, follows obligations to actual enforcement, checks ownership and unjustified duplication, and reassesses repairs and rebuttals. It reports its scope, evidence, findings, uncertainty and useful corrections in the host conversation. It runs when invoked; it supplies no timer or automatic blocking. It does not replace the domain-librarian Skill or AGENTS.md. Check installation with arclint agents reviewer status; supply earlier feedback and rebuttals when asking for reassessment.

The legacy domain guard is a separate native hook installation and approval lifecycle. Preserve an existing installation and its policies when developing or installing the reviewer. Hook findings must quote supplied evidence, explain the conflict and request a scoped repair. Repair it or give a concrete rebuttal for fresh review. Preserve required compiler/editor directives and legal notices. Do not change requirements, rule strength, exclusions or baselines to obtain approval. Governance changes invalidate the old guard review and need separate validation.

For an explicitly installed guard, run arclint agents status for installed scope and integrity. OMP /arclint-domain-status shows loaded review status. In Codex, review hook status in /hooks (CLI) or Settings > Coding > Hooks (desktop); an installed or enabled but untrusted hook does not run. After activation or updates, start a new session; OMP also supports /reload. Stale or unavailable guard review is not approval. A rejected command is not a successful check; follow the legitimate validation and fresh-session recovery path and report unresolved denial without repeated retries.

Verify functional behavior and run arclint check . plus the repository's required finish gate. Evaluate the reviewer's judgment separately: passing checks do not establish justified meanings, faithful review or a guard approval. Report evidence and remaining limits without claiming installation, activation or completed review that was not observed.`
