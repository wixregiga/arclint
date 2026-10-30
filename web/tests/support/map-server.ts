import { spawn, type ChildProcess } from "node:child_process";

export interface MapServer {
  url: string;
  stop(): void;
}

/**
 * `arclint map serve` over one project, on a port the OS picks; the URL
 * comes from its JSON report once it listens.
 */
export function startMapServer(arclint: string, dir: string): Promise<MapServer> {
  return new Promise((resolve, reject) => {
    const child: ChildProcess = spawn(arclint, ["map", "serve", "--addr", "127.0.0.1:0", "--format", "json"], { cwd: dir, stdio: ["ignore", "pipe", "pipe"] });
    let out = "";
    let err = "";
    let settled = false;
    child.stdout!.on("data", (chunk: Buffer) => {
      out += chunk.toString();
      if (settled) return;
      try {
        const report = JSON.parse(out) as { url: string };
        settled = true;
        resolve({ url: report.url, stop: () => child.kill("SIGTERM") });
      } catch {
        // Not a whole document yet.
      }
    });
    child.stderr!.on("data", (chunk: Buffer) => {
      err += chunk.toString();
    });
    child.on("exit", (code) => {
      if (!settled) {
        settled = true;
        reject(new Error(`arclint map serve in ${dir} exited with ${code}\n${out}\n${err}`));
      }
    });
    child.on("error", (e) => {
      if (!settled) {
        settled = true;
        reject(e);
      }
    });
  });
}
