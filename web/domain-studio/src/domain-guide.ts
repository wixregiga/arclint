import type { ConceptKind } from './contracts';

export type GuidedKind = ConceptKind | 'bounded_context' | 'invariant' | 'assertion';
type QuestionId = 'start' | 'subject' | 'identity' | 'consistency' | 'operation' | 'construction' | 'condition' | 'promise';
export type GuideStepId = QuestionId | GuidedKind;
type GuideStep = { question: string; hint: string; choices: { label: string; detail: string; next: GuideStepId }[] }
  | { kind: GuidedKind; reason: string };

/** Suggestions follow explicit answers, never a keyword guess about the draft. */
export const domainGuide: Record<GuideStepId, GuideStep> = {
  start: {
    question: 'What are you trying to describe?',
    hint: 'Choose the closest fit. You can change your answer.',
    choices: [
      { label: 'A place where words have a particular meaning', detail: 'For example, “account” means something different in billing and sign-in.', next: 'bounded_context' },
      { label: 'Something people work with, or something that happens', detail: 'An order, an address, a delivery, or approving a request.', next: 'subject' },
      { label: 'A condition that needs to hold', detail: 'Something that must remain true, or a way to decide if something qualifies.', next: 'condition' },
    ],
  },
  subject: {
    question: 'Which part are you describing?', hint: 'Think about the meaning of your words, before how the code works.',
    choices: [
      { label: 'A thing people name and describe', detail: 'An order, a customer, or a delivery address.', next: 'identity' },
      { label: 'Something that has happened', detail: 'An order was placed or a payment was received.', next: 'domain_event' },
      { label: 'Work the domain needs to do', detail: 'Create an order, find it later, or calculate a quote.', next: 'operation' },
    ],
  },
  identity: {
    question: 'What makes it the same thing over time?', hint: 'Imagine its details change tomorrow.',
    choices: [
      { label: 'Its identity stays the same', detail: 'An order is still that order when its delivery address changes.', next: 'consistency' },
      { label: 'Only its values matter', detail: 'Two equal addresses mean the same thing. A change gives you a new value.', next: 'value_object' },
    ],
  },
  consistency: {
    question: 'Does it control a boundary of changes?', hint: 'A boundary keeps its promises true whenever something inside it changes.',
    choices: [
      { label: 'It controls the whole group', detail: 'An order controls its lines and keeps the whole order consistent.', next: 'aggregate' },
      { label: 'It is a member within that boundary', detail: 'An order line has its own identity, but the order controls changes to it.', next: 'entity' },
    ],
  },
  operation: {
    question: 'What is this work responsible for?', hint: 'Pick the responsibility you want to explain.',
    choices: [
      { label: 'Creating or finding domain objects', detail: 'Bring a valid object into existence, or retrieve one that already exists.', next: 'construction' },
      { label: 'An operation that belongs to no single object', detail: 'For example, calculating an exchange between two currencies.', next: 'domain_service' },
      { label: 'Deciding whether something qualifies', detail: 'For example, whether an order is eligible for a discount.', next: 'specification' },
    ],
  },
  construction: {
    question: 'Are you creating something or retrieving it?', hint: 'These responsibilities have different names in the domain.',
    choices: [
      { label: 'Creating a valid domain object', detail: 'Own the steps needed to construct it correctly.', next: 'factory' },
      { label: 'Finding or storing aggregates', detail: 'Offer access to aggregates as a collection.', next: 'repository' },
    ],
  },
  condition: {
    question: 'Is this a promise or a decision?', hint: 'A promise protects validity. A decision says whether something qualifies.',
    choices: [
      { label: 'A promise the domain must uphold', detail: 'An order must never have a negative total.', next: 'promise' },
      { label: 'A condition used to make a decision', detail: 'An order qualifies for free delivery when it meets the threshold.', next: 'specification' },
    ],
  },
  promise: {
    question: 'When must it hold?', hint: 'You will choose the owner when you record the statement.',
    choices: [
      { label: 'Whenever the owner is in a valid state', detail: 'A quantity is always positive.', next: 'invariant' },
      { label: 'After a particular aggregate operation', detail: 'After placing an order, its lines are frozen.', next: 'assertion' },
    ],
  },
  bounded_context: { kind: 'bounded_context', reason: 'You described a boundary within which a particular language and model apply. Give it a name and explain what its words mean here.' },
  aggregate: { kind: 'aggregate', reason: 'You described a root that controls a consistency boundary. Record what it owns and the promises it protects.' },
  entity: { kind: 'entity', reason: 'You described an object whose identity survives changes to its details. Record that identity and the boundary it belongs to.' },
  value_object: { kind: 'value_object', reason: 'You described something defined by its values, with no separate identity. Record its meaning and what makes a value valid.' },
  domain_event: { kind: 'domain_event', reason: 'You described a domain-significant occurrence. Name what happened and explain why it matters.' },
  domain_service: { kind: 'domain_service', reason: 'You described domain work that belongs to no single entity or value object. Record the operation and its responsibility.' },
  specification: { kind: 'specification', reason: 'You described a predicate that decides whether something meets a business condition. Record what qualifies and why.' },
  factory: { kind: 'factory', reason: 'You described the responsibility for constructing a valid domain object. Record what it creates.' },
  repository: { kind: 'repository', reason: 'You described collection-like access to aggregates. Record which aggregates it retrieves and stores.' },
  invariant: { kind: 'invariant', reason: 'You described a condition that must always hold for its owner. Attach it to the aggregate or value object that protects it.' },
  assertion: { kind: 'assertion', reason: 'You described what must hold after an aggregate operation. Record its owner, operation, and checking statement. Recording it does not prove the code checks it.' },
  unclassified: { kind: 'unclassified', reason: 'You can record the uncertainty as an open question. Keep the original wording and return to it when the meaning is clearer.' },
};
