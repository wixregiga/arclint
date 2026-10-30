# Studio absorption

## Scope and order

Absorb BWrite into ArcLint Studio, leaving /home/jofyi/ai/bwrite untouched.
The parent worktree was fast-forwarded to origin/main at 5a01c469c2b16946f1a338b629ecd9e4b767f6d0.
One unit completes before the next starts:

1. Domain authoring and map: Web loading/editing the configured language through
   the existing Go host, including an empty recording. Preserve CLI operations.
2. Feature writing: adapt BWrite's editor recognition, term insertion and deliberate
   associations over the same language; durable drafts and copy-as-ticket.

The parent coordinates integration and gates. Astra owns Go HTTP/CLI support for
unit 1. Fable owns unit 1's web package. Their changes are integrated and verified
before unit 2 begins. No push, PR, squash or BWrite change is part of this step.

## Existing extension points

Use internal/delivery/mapserve and the existing application domain overview,
define and remove operations. The new frontend is a client of those operations,
not another domain interpreter or a Node service. Browser-host dependencies are
injected at App composition; source layout follows docs/web-architecture.md.

GET api/overview and GET api/accepted retain their existing JSON shapes.
POST api/define and POST api/remove retain their existing request shapes.
POST api/project accepts optional name and description strings and invokes
DefineDomainDefinition.ExecuteProject; absence leaves a property unchanged.
Errors retain the existing error JSON shape and HTTP status mapping.
All paths are relative to an injected API base for nested hosting.

Web builds to web/dist. The parent integrates built assets into the existing Go
embedding mechanism so installed users still run one Go binary without Node.
The old asset integration is replaced only when the new build works end to end.

## DDD design review

Decision: existing vocabulary objects decide validity; application use cases
coordinate; mapserve translates HTTP and Web renders recorded meaning.
Home: vocabulary/rule domain (decides), application (coordinates), delivery/Web
(carries out). Verdict: conforms. Basis: application_service guidance; Evans
2015 p.10 (tier 1, read through the verified meta-model). Opposite: copy BWrite's
canonical language model for reuse; declined because the owner chose absorption
into the same recording and existing validity rules must keep one home.

Decision: FSD slices and browser transport contracts describe UI responsibilities,
not new domain aggregates. Home: Web presentation. Verdict: conforms. Basis:
aggregate consistency definition, Evans 2015 p.16 (tier 1, read through verified
meta-model), and the existing Web contract. Opposite: import BWrite's aggregates
unchanged; their classification belongs to its model, not automatically ArcLint's.

Decision: deliberate meaning edits use define/remove; recognition never silently
rewrites prose or creates a recorded contract. Home: vocabulary application ports
and presentation interactions. Verdict: conforms. Basis: recorded language and
declined normative-wording-is-a-rule. Explicit human editing is permitted.

This first unit changes delivery and presentation, not domain meaning. It records
no invented Feature aggregate or authoring context. If implementation discovers
a new policy, stop before encoding it and use the domain-librarian workflow.

## Verification

Exercise actual HTTP writes against temporary copies of real YAML and CML
recordings, plus an empty project. Verify reload, refusal without losing edits,
creation/removal and default-branch comparison. Verify host-controlled mounting
and disposal and all shipped Web entrypoints. Parent runs architecture fixtures,
Web build/tests, make check and make ci. Rule changes only activate the reserved
Web launch rule once implemented; never weaken existing checks or baselines.


## Unit 1 integration

The Web source is integrated with Astra's project/empty-recording HTTP path.
The Go host embeds the Vite build under `static/web`, serves only its page,
assets, manifest and icon, and retains the existing origin and edit-serialization
checks. `make generate ONLY=web` owns the committed build; the repository gate
compares it against a fresh temporary build. Installed binaries need no Node.

The reserved Web launch rule is active and its two parked fixtures are restored.
The existing legacy pure map-model regression tests remain during the absorption;
the legacy page and script are no longer served. Actual browser flows load the
embedded Web build and use real Go servers for YAML, CML, empty and absent
recordings, including nested hosting. Browser coverage currently uses Chromium.
