/*
 * The browser suite's host: the built Studio served at several bases,
 * each proxying its API to a real `arclint map serve` over a real
 * recording, so the page under test talks to the Go server exactly as
 * the embedded build will.
 *
 *   /                 identity (YAML, remote default branch available)
 *   /nested/studio/   identity again, under a nested path
 *   /boxoffice/       boxoffice (CML, no remote)
 *   /empty/           an empty recording
 */
import { createReadStream, existsSync, mkdirSync, statSync } from "node:fs";
import http from "node:http";
import path from "node:path";
import { arclintBinary } from "./arclint-binary.ts";
import { startMapServer, type MapServer } from "./map-server.ts";
import { prepareProjects } from "./projects.ts";

const PORT = Number(process.env.STUDIO_E2E_PORT ?? 4421);
const SCRATCH = process.env.STUDIO_E2E_SCRATCH ?? path.join(process.cwd(), ".e2e-scratch");
const DIST = path.resolve(import.meta.dirname, "../../dist");

const TYPES: Record<string, string> = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".svg": "image/svg+xml",
  ".webmanifest": "application/manifest+json",
  ".json": "application/json",
};

async function main() {
  if (!existsSync(path.join(DIST, "index.html"))) {
    throw new Error(`No build at ${DIST}; run npm run build first.`);
  }
  mkdirSync(SCRATCH, { recursive: true });
  const arclint = arclintBinary(SCRATCH);
  const projects = prepareProjects(SCRATCH, arclint);
  const identity = await startMapServer(arclint, projects.identity.dir);
  const boxoffice = await startMapServer(arclint, projects.boxoffice.dir);
  const empty = await startMapServer(arclint, projects.empty.dir);
  const mounts: { base: string; server: MapServer }[] = [
    { base: "/nested/studio/", server: identity },
    { base: "/boxoffice/", server: boxoffice },
    { base: "/empty/", server: empty },
    { base: "/", server: identity },
  ];

  const server = http.createServer((req, res) => {
    const url = new URL(req.url ?? "/", `http://${req.headers.host ?? "localhost"}`);
    if (url.pathname === "/healthz") {
      res.writeHead(200, { "Content-Type": "text/plain" }).end("ok");
      return;
    }
    const mount = mounts.find((m) => url.pathname === m.base || url.pathname.startsWith(m.base));
    if (!mount) {
      res.writeHead(404).end("no mount");
      return;
    }
    const rest = url.pathname.slice(mount.base.length);
    if (rest.startsWith("api/")) {
      proxy(req, res, mount.server.url, "/" + rest + url.search);
      return;
    }
    const file = rest === "" ? "index.html" : rest;
    const target = path.join(DIST, file);
    if (!target.startsWith(DIST) || !existsSync(target) || !statSync(target).isFile()) {
      res.writeHead(404).end("not found");
      return;
    }
    res.writeHead(200, { "Content-Type": TYPES[path.extname(target)] ?? "application/octet-stream", "Cache-Control": "no-store" });
    createReadStream(target).pipe(res);
  });

  // Requests without the trailing slash would resolve relative URLs one
  // level up; a real host serves the page under its directory.
  server.listen(PORT, "127.0.0.1", () => {
    console.log(`Studio host at http://localhost:${PORT}/ (identity), /nested/studio/, /boxoffice/, /empty/`);
  });

  const stop = () => {
    server.close();
    for (const m of [identity, boxoffice, empty]) m.stop();
    process.exit(0);
  };
  process.on("SIGTERM", stop);
  process.on("SIGINT", stop);
}

/** Forwards one API request to a map server, addressed as its own page would address it. */
function proxy(req: http.IncomingMessage, res: http.ServerResponse, serverUrl: string, pathAndQuery: string) {
  const target = new URL(serverUrl);
  const headers: http.OutgoingHttpHeaders = { ...req.headers, host: target.host };
  if (req.headers.origin) headers.origin = target.origin;
  const upstream = http.request({ hostname: target.hostname, port: target.port, path: pathAndQuery, method: req.method, headers }, (answer) => {
    res.writeHead(answer.statusCode ?? 502, answer.headers);
    answer.pipe(res);
  });
  upstream.on("error", (e) => {
    res.writeHead(502, { "Content-Type": "application/json" }).end(JSON.stringify({ error: `map server unreachable: ${e.message}` }));
  });
  req.pipe(upstream);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
