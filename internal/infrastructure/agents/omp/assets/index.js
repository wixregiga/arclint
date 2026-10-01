import { createGuard } from "./guard.mjs";

export default function (omp) {
  createGuard(omp, import.meta.url);
}
