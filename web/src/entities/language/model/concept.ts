/*
 * The concepts the domain file records, spelled as `arclint domain define
 * <concept>` spells them, and the properties each one records, as the
 * define flags and the define request name them. Studio shows exactly
 * these; it invents no classification of its own.
 */

export const CONCEPTS = [
  "bounded_context",
  "aggregate",
  "entity",
  "value_object",
  "domain_event",
  "domain_service",
  "invariant",
  "assertion",
  "specification",
  "question",
] as const;

export type Concept = (typeof CONCEPTS)[number];

export function isConcept(value: string): value is Concept {
  return (CONCEPTS as readonly string[]).includes(value);
}

export const CONCEPT_TITLE: Readonly<Record<Concept, string>> = {
  bounded_context: "Bounded context",
  aggregate: "Aggregate",
  entity: "Entity",
  value_object: "Value object",
  domain_event: "Domain event",
  domain_service: "Domain service",
  invariant: "Invariant",
  assertion: "Assertion",
  specification: "Specification",
  question: "Open question",
};

/**
 * The short mark inside a concept's badge: at most two characters, unique
 * per concept, never the full word. BWrite's marks for the kinds it
 * classified; "I" for an invariant, "G" for an assertion (a guarantee of
 * one operation) and "?" for an open question.
 */
export const CONCEPT_MARK: Readonly<Record<Concept, string>> = {
  bounded_context: "C",
  aggregate: "A",
  entity: "E",
  value_object: "V",
  domain_event: "Ev",
  domain_service: "S",
  invariant: "I",
  assertion: "G",
  specification: "Sp",
  question: "?",
};

/** One sentence a stakeholder reads before choosing a concept. */
export const CONCEPT_MEANING: Readonly<Record<Concept, string>> = {
  bounded_context: "A part of the business with its own language, where each word has one meaning.",
  aggregate: "A thing the business keeps consistent as a whole, told apart by its identity.",
  entity: "A thing inside an aggregate, told apart by what it is rather than by its values.",
  value_object: "A value described by what it holds, with no identity of its own.",
  domain_event: "Something that happened that the business cares about.",
  domain_service: "An operation of the business that belongs to no single thing.",
  invariant: "A rule that always holds for the thing it belongs to.",
  assertion: "A guarantee of one named operation.",
  specification: "A named business condition that can decide whether something satisfies it.",
  question: "Something not yet settled about the language.",
};

/** The concept an entry of a nested concept is recorded under. */
export const OWNER_CONCEPTS: Readonly<Partial<Record<Concept, readonly Concept[]>>> = {
  entity: ["aggregate"],
  invariant: ["aggregate", "value_object"],
  assertion: ["aggregate", "domain_service"],
};

/** What an owner records under itself, in the recording's order. */
export const MEMBER_CONCEPTS: Readonly<Partial<Record<Concept, readonly Concept[]>>> = {
  aggregate: ["entity", "invariant", "assertion"],
  value_object: ["invariant"],
  domain_service: ["assertion"],
};

/** The concepts a bounded context records directly, in the recording's order. */
export const CONTEXT_CONCEPTS: readonly Concept[] = ["aggregate", "value_object", "domain_event", "domain_service", "specification", "question"];

export type PropertyName =
  | "definition"
  | "identity"
  | "aliases"
  | "repository"
  | "factory"
  | "raisedBy"
  | "statement"
  | "verifiedBy"
  | "on"
  | "text";

export type PropertyShape = "prose" | "text" | "list" | "aggregate";

export interface PropertySpec {
  name: PropertyName;
  label: string;
  /** Required when the entry is first recorded; the language refuses without it. */
  required: boolean;
  shape: PropertyShape;
  /** An engineering detail a stakeholder does not need: shown only on request or when recorded. */
  advanced: boolean;
  hint: string;
}

const definition: PropertySpec = { name: "definition", label: "Definition", required: true, shape: "prose", advanced: false, hint: "What it means, in the project's words. At most 40 words." };
const aliases: PropertySpec = { name: "aliases", label: "Aliases", required: false, shape: "list", advanced: false, hint: "Other names it goes by, one per line." };
const verifiedBy: PropertySpec = { name: "verifiedBy", label: "Verified by", required: false, shape: "list", advanced: true, hint: "Automated tests that verify it, one per line, as path:TestName." };

const PROPERTIES: Readonly<Record<Concept, readonly PropertySpec[]>> = {
  bounded_context: [{ ...definition, hint: "What this part of the business is about. At most 60 words." }],
  aggregate: [
    definition,
    { name: "identity", label: "Identity", required: true, shape: "text", advanced: false, hint: "The name that tells one apart from every other, such as OrderID." },
    aliases,
    { name: "repository", label: "Repository", required: false, shape: "text", advanced: true, hint: "The repository that keeps it, when the code names one." },
    { name: "factory", label: "Factory", required: false, shape: "text", advanced: true, hint: "The factory that builds it, when the code names one." },
  ],
  entity: [
    definition,
    { name: "identity", label: "Identity", required: false, shape: "text", advanced: false, hint: "The name that tells one apart within its aggregate, when it has one." },
    aliases,
  ],
  value_object: [definition, aliases],
  domain_event: [
    definition,
    { name: "raisedBy", label: "Raised by", required: false, shape: "aggregate", advanced: false, hint: "The aggregate whose change raises it, when one does." },
  ],
  domain_service: [definition],
  invariant: [
    { name: "statement", label: "Statement", required: true, shape: "prose", advanced: false, hint: "The rule, as one sentence that always holds. At most 40 words." },
    verifiedBy,
  ],
  assertion: [
    { name: "on", label: "On", required: true, shape: "text", advanced: false, hint: "The operation that makes the guarantee." },
    { name: "statement", label: "Statement", required: true, shape: "prose", advanced: false, hint: "The guarantee that applies when the operation runs. At most 40 words." },
    verifiedBy,
  ],
  specification: [definition],
  question: [{ name: "text", label: "Question", required: true, shape: "prose", advanced: false, hint: "What is not yet settled." }],
};

export function propertiesOf(concept: Concept): readonly PropertySpec[] {
  return PROPERTIES[concept];
}

/** The property that carries an entry's reading: its definition, statement or text. */
export function readingProperty(concept: Concept): PropertyName {
  switch (concept) {
    case "invariant":
    case "assertion":
      return "statement";
    case "question":
      return "text";
    default:
      return "definition";
  }
}

/** What the recording calls an entry's name: a key for a rule or a question. */
export function nameLabel(concept: Concept): "Name" | "Key" {
  return concept === "invariant" || concept === "assertion" || concept === "question" ? "Key" : "Name";
}

export function nameHint(concept: Concept): string {
  switch (concept) {
    case "invariant":
    case "assertion":
      return "Short and lowercase-kebab-case, such as lines-frozen; the code names its check after it.";
    case "question":
      return "Short and lowercase-kebab-case, such as refund-window.";
    case "bounded_context":
      return "One word the team uses for this part of the business, such as ordering.";
    default:
      return "The name the team uses, such as Order or Money.";
  }
}
