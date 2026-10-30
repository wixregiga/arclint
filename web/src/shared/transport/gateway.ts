import type {
  AcceptedDocument,
  DefineReport,
  DefineRequest,
  OverviewDocument,
  ProjectRequest,
  RemoveReport,
  RemoveRequest,
} from "./documents";

/**
 * The map server as Studio sees it. App composition injects one; the
 * screens never build a URL or call fetch themselves.
 */
export interface StudioGateway {
  overview(): Promise<OverviewDocument>;
  accepted(): Promise<AcceptedDocument>;
  define(request: DefineRequest): Promise<DefineReport>;
  remove(request: RemoveRequest): Promise<RemoveReport>;
  describeProject(request: ProjectRequest): Promise<DefineReport>;
}

/**
 * How a call failed. `refused` carries the server's own reason from its
 * {error} body (the 400 and 422 the CLI maps); `unreachable` means no
 * answer came at all; `unexpected` is an answer Studio cannot read, such
 * as a route the running server does not have.
 */
export type GatewayFailureKind = "refused" | "unreachable" | "unexpected";

export class GatewayFailure extends Error {
  readonly kind: GatewayFailureKind;
  readonly status: number | null;

  constructor(kind: GatewayFailureKind, message: string, status: number | null = null) {
    super(message);
    this.name = "GatewayFailure";
    this.kind = kind;
    this.status = status;
  }
}

export function isGatewayFailure(error: unknown): error is GatewayFailure {
  return error instanceof GatewayFailure;
}
