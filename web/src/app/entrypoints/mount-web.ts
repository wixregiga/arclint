import { createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { App } from "../composition/App";
import { inMemoryStudioMemory, type StudioMemory } from "../../shared/lib/studio-memory/index";
import { createHttpGateway, type StudioGateway } from "../../shared/transport/index";
import { studioStylesheet } from "../styles/studio-stylesheet";

/**
 * What a host lends Studio. A gateway, or the base its API is under
 * (relative to the host document when relative); memory for unsaved
 * edits, in-memory for the mount's lifetime when none is given.
 */
export interface StudioHostDependencies {
  gateway?: StudioGateway;
  apiBase?: string;
  memory?: StudioMemory;
  fetch?: typeof fetch;
}

export interface StudioMount {
  /** The element Studio renders into, inside the container's shadow root. */
  readonly root: HTMLElement;
  dispose(): void;
}

/**
 * Mounts Studio into a host-owned container and returns its disposal.
 * Importing this module changes nothing; everything happens in the call.
 * Studio renders inside a shadow root on the container with its own
 * stylesheet, so its styles stay inside and the host's stay outside, and
 * it registers no worker, takes no route and writes to no storage the
 * host did not lend. Disposing unmounts and empties the shadow root; the
 * container can be mounted again.
 */
export function mountStudio(container: HTMLElement, dependencies: StudioHostDependencies = {}): StudioMount {
  const document = container.ownerDocument;
  const shadow = container.shadowRoot ?? container.attachShadow({ mode: "open" });
  shadow.replaceChildren();
  const style = document.createElement("style");
  style.textContent = studioStylesheet;
  const root = document.createElement("div");
  root.className = "st-root";
  shadow.append(style, root);

  const gateway =
    dependencies.gateway ??
    createHttpGateway({ base: new URL(dependencies.apiBase ?? "./", document.baseURI).toString(), fetch: dependencies.fetch });
  const memory = dependencies.memory ?? inMemoryStudioMemory();
  const reactRoot: Root = createRoot(root);
  reactRoot.render(createElement(App, { gateway, memory }));

  let disposed = false;
  return {
    root,
    dispose() {
      if (disposed) return;
      disposed = true;
      reactRoot.unmount();
      shadow.replaceChildren();
    },
  };
}
