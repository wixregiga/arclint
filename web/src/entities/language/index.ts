export {
  CONCEPTS,
  CONCEPT_MARK,
  CONCEPT_MEANING,
  CONCEPT_TITLE,
  CONTEXT_CONCEPTS,
  MEMBER_CONCEPTS,
  OWNER_CONCEPTS,
  isConcept,
  nameHint,
  nameLabel,
  propertiesOf,
  readingProperty,
} from "./model/concept";
export type { Concept, PropertyName, PropertyShape, PropertySpec } from "./model/concept";
export {
  addressOf,
  aliasesOf,
  contextNames,
  entriesOf,
  entriesOfContext,
  entryAt,
  entryKey,
  listOf,
  membersOf,
  missingRequired,
  ofConcept,
  ownerOf,
  readingOf,
  textOf,
  topLevelOf,
} from "./model/entry";
export type { Entry, EntryAddress, EntryProperties, PropertyValue } from "./model/entry";
export { differences, proposedKeys, relationDifferences, standingOf } from "./model/standing";
export type { Difference, DifferenceKind, FieldDifference, RelationDifference, Standing } from "./model/standing";
export { connectionsOf, relationWord } from "./model/connections";
export type { Connection, ConnectionKind } from "./model/connections";
export { searchEntries } from "./lib/search";
export type { MatchedBy, SearchHit } from "./lib/search";
export { fit, layoutContext } from "./lib/map-layout";
export type { MapLayout, MapLine, MapLineKind, MapNode, Viewport } from "./lib/map-layout";
export { layoutContextMap } from "./lib/context-map-layout";
export type { ContextMapLayout, ContextNode, RelationLine } from "./lib/context-map-layout";
export { cleanName, hasWordCharacter, normalizeName, recognize, RECOGNIZED_CONCEPTS } from "./lib/recognition";
export type { Recognition } from "./lib/recognition";
export { describeReading, describeTerm, entryOfTerm, readingWord, termOf, useAt, usesOf, usesWithin } from "./lib/uses";
export type { Use, UseKind } from "./lib/uses";
