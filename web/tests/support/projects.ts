import { execFileSync } from "node:child_process";
import { cpSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";
import { REPO_ROOT } from "./arclint-binary.ts";

export interface FixtureProject {
  name: string;
  dir: string;
}

const gitEnv = {
  ...process.env,
  GIT_AUTHOR_NAME: "studio-e2e",
  GIT_AUTHOR_EMAIL: "studio-e2e@example.com",
  GIT_COMMITTER_NAME: "studio-e2e",
  GIT_COMMITTER_EMAIL: "studio-e2e@example.com",
  GIT_CONFIG_GLOBAL: "/dev/null",
  GIT_CONFIG_SYSTEM: "/dev/null",
};

function git(dir: string, ...args: string[]): void {
  execFileSync("git", args, { cwd: dir, env: gitEnv, stdio: "pipe" });
}

/**
 * Three real recordings in fresh folders under scratch, written again on
 * every run so the suite starts from the committed language:
 *
 * - identity: the Go example's YAML recording (Vernon's identity and
 *   access), committed upstream and cloned, so the clone has a remote
 *   default branch and every entry starts Accepted.
 * - boxoffice: the proving ground's CML recording, with no remote, so
 *   the default branch is unavailable.
 * - empty: a project whose recording holds nothing yet, from `arclint
 *   domain init`.
 */
export function prepareProjects(scratch: string, arclint: string): Record<"identity" | "boxoffice" | "empty" | "newProject", FixtureProject> {
  const base = path.join(scratch, "projects");
  rmSync(base, { recursive: true, force: true });
  mkdirSync(base, { recursive: true });

  const upstream = path.join(base, "identity-upstream");
  mkdirSync(upstream);
  for (const name of ["domain.arclint.yaml", "rules.arclint.yaml", "go.mod"]) {
    cpSync(path.join(REPO_ROOT, "docs/examples/go", name), path.join(upstream, name));
  }
  git(upstream, "init", "-q", "-b", "main");
  git(upstream, "add", ".");
  git(upstream, "commit", "-q", "-m", "the accepted recording");
  const identity = path.join(base, "identity");
  git(base, "clone", "-q", upstream, identity);

  const boxoffice = path.join(base, "boxoffice");
  cpSync(path.join(REPO_ROOT, "testing/boxoffice"), boxoffice, {
    recursive: true,
    filter: (source) => !source.includes(`${path.sep}node_modules`) && !source.includes(`${path.sep}.arclint${path.sep}cache`),
  });

  const empty = path.join(base, "empty");
  mkdirSync(empty);
  writeFileSync(path.join(empty, "rules.arclint.yaml"), "project: fresh\nruntime: [go]\n");
  execFileSync(arclint, ["domain", "init"], { cwd: empty, stdio: "pipe" });

  const newProject = path.join(base, "new-project");
  mkdirSync(newProject);
  writeFileSync(path.join(newProject, "rules.arclint.yaml"), "project: fresh\nruntime: [go]\n");

  return {
    newProject: { name: "new-project", dir: newProject },
    identity: { name: "identity", dir: identity },
    boxoffice: { name: "boxoffice", dir: boxoffice },
    empty: { name: "empty", dir: empty },
  };
}
