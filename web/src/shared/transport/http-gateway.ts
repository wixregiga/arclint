import type {
  AcceptedDocument,
  DefineReport,
  DefineRequest,
  OverviewDocument,
  ProjectRequest,
  RemoveReport,
  RemoveRequest,
} from "./documents";
import type { StudioGateway } from "./gateway";
import { httpCaller } from "./http-call";

export interface HttpGatewayOptions {
  /** The absolute URL every API path is relative to; it ends with a slash. */
  base: string;
  fetch?: typeof fetch;
}

/** The API paths the map server answers for the language, relative to the injected base. */
export const API_PATHS = {
  overview: "api/overview",
  accepted: "api/accepted",
  define: "api/define",
  remove: "api/remove",
  project: "api/project",
} as const;

/** The gateway over HTTP, as the Go map server serves it. */
export function createHttpGateway(options: HttpGatewayOptions): StudioGateway {
  const { call, post } = httpCaller(options.base, options.fetch);
  return {
    overview: () => call<OverviewDocument>(API_PATHS.overview),
    accepted: () => call<AcceptedDocument>(API_PATHS.accepted),
    define: (request: DefineRequest) => post<DefineReport>(API_PATHS.define, request),
    remove: (request: RemoveRequest) => post<RemoveReport>(API_PATHS.remove, request),
    describeProject: (request: ProjectRequest) => post<DefineReport>(API_PATHS.project, request),
  };
}
