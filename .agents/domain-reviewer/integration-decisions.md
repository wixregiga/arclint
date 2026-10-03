# Finishing the integration — 2026-10-03

Recorded before implementation, following the corrective meanings in decisions.md.
The user requests the whole integration, including inherited guard code, actual
repair through a running hook, restoration of missing scoped files, native reviewer
invocation and protection in the active checkout and a fresh worktree. The latest
direction is to keep agents on the requested work and within their domain.

## Justified behavior and ownership

The guard reviews the original request and explicitly selected evidence. Its
purpose requires a usable repair path: a finding must not prevent the agent from
proposing the repair it requests. Missing material remains part of the configured
scope and prevents a completion pass; it does not silently disappear. A directly
inspectable proposal restoring missing scoped material may receive permission,
but that permission does not accept the resulting contents. Post-tool and final
review must use fresh evidence. Mixed, unrelated or protected edits do not obtain
a restoration exemption. Invalid configuration, traversal and escaping symlinks
remain errors, not missing material.

Dedicated inspection and status operations may report the unavailable state
without renewing approval. Arbitrary shell commands and execution wrappers cannot
be declared read-only from their descriptions. Governance changes still invalidate
the session: restore the protected bytes, or separately validate the intended
change and begin a fresh session. The separate installed guard in the main checkout
is preserved; fixing shipped assets is not a claim that it was repaired or updated.

Host adapters own hook protocol, paths, containment and tool-shape recognition.
Resolved containment must use the target platform's path semantics. The application
owns setup ordering: validate the requested installation before writing setup
artifacts, including scope, supported host, managed-file conflicts and AGENTS
markers. Only the default recording that setup itself creates may be planned as
absent. Adapters must recheck at write time. This is validation-before-write, not
a claim of crash-atomic multi-file installation.

The CLI returns installation and integrity results through its existing report
contract. JSON output is one parseable structured result, human renderers explain
the same facts, and neither installer nor status reports claim host activation.
The application returns facts; adapters do not manufacture presentation prose.

These are delivery and workflow obligations under the librarian's
`not-a-domain-rule`, `language-not-guards` and `application-service-holds-no-rule`.
They justify no new entity, aggregate, value object or consistency invariant.
The agent context records only the guard's meaning and repair relationship.
Existing inward-dependency and renderer-boundary rules still apply; no rule or
baseline relaxation is justified by these corrections.

## Observed worktree host behavior

A real Codex 0.159.2 native invocation selected the named reviewer from the local
worktree, but its Stop hook identified the main checkout's hooks.json. The host
maps linked-worktree hook folders to the main checkout. This does not authorize
changing the preserved main installation or treating a worktree-local hook file
as active. New hook definitions still require the host's ordinary trust approval.

The shipped guard must select the session checkout from the host event's cwd and
that checkout's own explicit guard configuration. It must not silently review the
installation checkout when the agent is working elsewhere. Search stops at the
session repository boundary; missing target configuration means unavailable review.
An absent cwd retains the installed-root fallback for protocol compatibility.
This is adapter routing, not a new domain concept. Validate Linux and WSL cwd
translation, separate checkout state, and rejection instead of cross-checkout
fallback. Preserve the separately installed main script byte for byte.

## Corrections from actual named-reviewer samples

The corrected named reviewer ran in both target checkouts with its authored
instructions loaded. Independent assessment found two partial cases in each
initial fourteen-case response. This is evidence of instruction gaps, not a new
domain model: a completed snapshot audit was given an unsupported chronology
obligation, and an invented invariant was rejected without clearly removing
the implementation that existed only to enforce it.

Record the repair before changing instructions. Missing chronology is a coverage
limit unless changed meaning or a process claim makes that history part of the
requested review. Do not turn unavailable history into additional work merely to
complete a format. When a rule has no justified basis, the correction must cover
both its recording and dependent ceremonial implementation, preserving separately
justified behavior. Do not invent a replacement rule to rescue that implementation.
These clarify DR-4, DR-5, DR-6 and DR-17; no new invariant or host guarantee follows.

## Independent installed guards must not overwrite each other's review state

Before enabling the prepared additive hooks, independent fixture execution found
that the preserved main provider and corrected local provider wrote the same
session JSON in the target checkout. Each SessionStart replaced the other's
protection snapshot, so the next event falsely reported governance changes in
either start order. The immutable main script was copied for this reproduction;
the installed original was not changed.

The corrected adapter will namespace its cache by the resolved executing script's
installation path, while retaining the existing per-session key and lock. The
namespace is stable across content updates, so code/governance hashes still
invalidate prior approval rather than escaping it in a new cache. Keep the legacy
provider's files untouched. This is ownership of adapter state for independent
installations, not a new Agent identity, domain aggregate or policy exception.

## Acceptance evidence required

- Exercise permission before mutation, then post-tool and final review. Include
  ordinary findings, missing recording, missing literal source and an empty glob.
  A supplied test verdict proves protocol behavior only; use a real reviewer
  process separately to prove live repair and a fresh pass.
- Attempt mixed and protected edits, path escapes and stale-result reuse. Verify
  inspection leaves an invalidated approval invalidated.
- Execute Windows containment tests on Windows, not merely cross-compile them.
- Compare the complete setup tree before and after rejected installation requests.
  Parse the actual complete stdout of setup/status/hooks/reviewer JSON commands.
- Identify the exact source commit, authored and installed asset hashes, host
  version, target worktree and session when reporting runtime evidence. Distinguish
  files installed, host loading, actual invocation, observed protection and gaps.
- Run repository gates and inspect outstanding baseline findings. Tests cannot
  establish the correctness of a meaning or substitute for native invocation.
## Correct the repair exercise's contradictory fixture

The production hook probe on `5db6d0b` passed comment repair and missing-file
restoration, then rejected its final fresh-session review: the fictional recording
called Room both a bookable space and the name of that space. Independent review
confirmed that rejection. The failed activity remains evidence, not a successful
recovery result.

The fixture author intends Room to mean a named bookable space. This protocol
exercise needs no equality, identity or transaction-boundary decision: it tests
scoped comment repair, restoration and governance invalidation. Remove the
unjustified `value_objects` fixture entry and retain the context definition and
existing implementation. This changes test material, not guard policy, scope or
expected protocol outcomes. A fresh semantic review must assess that material;
the correction itself does not establish a pass.
