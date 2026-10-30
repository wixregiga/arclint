import { storageStudioMemory } from "../../shared/lib/studio-memory/index";
import { mountStudio } from "./mount-web";

/**
 * Standalone startup: the page served by `arclint map serve`, or the
 * same build under a nested path. The API base is the page's own
 * directory, so a nested host keeps every request under it, feature
 * drafts included; unsaved edits live in session storage under that
 * base. No service worker is registered: the build makes no offline
 * promise.
 */
const base = new URL("./", document.baseURI).toString();
const container = document.getElementById("studio");
if (container === null) {
  throw new Error("ArcLint Studio: the page has no #studio element to mount into.");
}
document.documentElement.classList.add("st-standalone");
mountStudio(container, {
  apiBase: base,
  memory: storageStudioMemory(window.sessionStorage, `arclint-studio:${base}`),
});
