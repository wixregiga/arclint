export type {
  AcceptedDocument,
  AggregateDocument,
  AssertionDocument,
  ChangeRequest,
  ContextDocument,
  CountsDocument,
  DefineReport,
  DefineRequest,
  EntityDocument,
  EventDocument,
  InvariantDocument,
  OverviewDocument,
  ProjectRequest,
  QuestionDocument,
  RelationDocument,
  RemoveReport,
  RemoveRequest,
  ServiceDocument,
  TermDocument,
  ValueObjectDocument,
} from "./documents";
export { FEATURE_READINGS, isFeatureReading } from "./feature-documents";
export type { FeatureDocument, FeatureReading, FeatureReferenceDocument, FeatureSummaryDocument, FeatureTermDocument, SaveFeatureRequest } from "./feature-documents";
export { isMissingFeature, isStaleRevision } from "./feature-store";
export type { FeatureStore } from "./feature-store";
export { GatewayFailure, isGatewayFailure } from "./gateway";
export type { GatewayFailureKind, StudioGateway } from "./gateway";
export { apiUrl, normalizedBase } from "./http-call";
export { createHttpFeatureStore, FEATURE_API_PATHS } from "./http-feature-store";
export type { HttpFeatureStoreOptions } from "./http-feature-store";
export { API_PATHS, createHttpGateway } from "./http-gateway";
export type { HttpGatewayOptions } from "./http-gateway";
