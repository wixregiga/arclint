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
export { GatewayFailure, isGatewayFailure } from "./gateway";
export type { GatewayFailureKind, StudioGateway } from "./gateway";
export { API_PATHS, apiUrl, createHttpGateway, normalizedBase } from "./http-gateway";
export type { HttpGatewayOptions } from "./http-gateway";
