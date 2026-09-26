import type { UnitOfWork } from "../../provision-tenant/index.js";

/** Runs work directly; the in-memory adapters have nothing to commit or roll back. */
export class DirectUnitOfWork implements UnitOfWork {
  run<T>(work: () => Promise<T>): Promise<T> {
    return work();
  }
}
