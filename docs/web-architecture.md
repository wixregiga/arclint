# Web architecture contract

The repository rules reserve `web/` for the Vite frontend. At the time this
contract was introduced, the repository contained no Web source.
These are target constraints, not a description of an existing implementation.
`web/launch-surfaces-present` is temporarily disabled until the package,
Vite configuration, HTML entry, manifest, standalone startup, and mount entry
exist. Remove its `disable` field when those launch surfaces are implemented,
move the two `web-launch-surfaces-*` fixtures from `.arclint/tests/disabled/`
back to `.arclint/tests/`, and regenerate the agent guidance. The fixture runner
loads only that directory's immediate YAML files and cannot express the
pathless disabled-rule notice as an expectation, so these fixtures retain
their original enabled-rule assertions while parked. The other Web rules
remain enabled.

## Responsibilities visible in the files

Web uses Feature-Sliced Design with this dependency direction:

```text
app -> pages -> widgets (optional) -> features -> entities -> shared
```

Any layer may skip intermediate layers when importing downward. Add a layer or
slice when it owns a useful responsibility; there is no requirement to fill
every layer. Keep behavior used by one screen with that screen, extract reused
user actions into Features, and keep portable UI and libraries in Shared. The
current official guidance discourages Widgets as a default because their
responsibility can overlap Features; this contract permits them when their
responsibility is explicit. [FSD layers](https://fsd.how/docs/reference/layers/)

| Location | Responsibility |
| --- | --- |
| `web/src/app/entrypoints/start-web.ts` | Start the standalone application and choose its mount target and PWA policy. |
| `web/src/app/entrypoints/mount-web.ts` | Expose host-controlled mounting and disposal without starting the application on import. |
| `web/src/app/` | Compose dependencies, routing, providers, and adapters for the selected host. |
| `web/src/app/pwa/` | Own worker registration and updates for standalone use. Only standalone startup may import this code. |
| `web/src/pages/<screen>/` | Own one screen's cohesive presentation and behavior. |
| `web/src/widgets/<composition>/` | Optionally own a reused screen composition that has a clear responsibility. |
| `web/src/features/<action>/` | Own a reusable user interaction. |
| `web/src/entities/<concept>/` | Own a reusable concept in Web's model. This directory does not establish that the concept is a DDD Entity. |
| `web/src/shared/<purpose>/` | Own domain-independent transport contracts, configuration, browser libraries, or UI primitives. |

Name the actual concept or action: `project-description.ts`, `open-project.ts`,
`ProjectScreen.tsx`. The rules forbid listed generic container names such as
`types.ts`, `utils.ts`, and `components/`. That is a finite filename check; it
does not prove that a better-spelled file has one responsibility. A `model/`
segment is permitted; a generic `model.ts` file is forbidden.

Pages, Widgets, Features, and Entities contain slices. Each slice exposes
`index.ts`; its implementation belongs under `ui/`, `api/`, `model/`, `lib/`,
or `config/`. A component and the state, styles, and tests needed only by that
component stay within its owning slice. Siblings on a sliced layer cannot
import one another. Compose them above that layer. This applies to Entities
too: the optional FSD `@x` exception is not adopted without a concrete need.
The standard describes slice independence and purpose-oriented segments.
[FSD slices and segments](https://fsd.how/docs/reference/slices-segments/)

App and Shared have purpose-named segments rather than business slices.
App composes the application through its explicit launch interfaces. Shared
segments expose `index.ts`, except `shared/ui` and `shared/lib`, which expose
individual modules: a direct file such as `shared/ui/Button.tsx`, or a focused
directory such as `shared/lib/dates/index.ts`. This avoids a required barrel
for every UI component in the application. Directory modules hide their
internal files. A module's implementation must not import its own entry.
Public entries name their exports explicitly; wildcard exports are checked
as source lines. These choices follow the public-interface guidance on
encapsulation and the costs of large Shared barrels.
[FSD public API](https://fsd.how/docs/reference/public-api/)

## Project and host flexibility

The project being inspected supplies its vocabulary and data. Web's screens
must not require one particular customer domain, fixed aggregate list, or
hardcoded example project. Keep domain-independent transport contracts in a
purpose-named Shared segment; inject host-specific implementations through App
composition. Reusable screens and actions consume those contracts and receive
data through their declared interfaces.

These are product design requirements. Import direction prevents Shared from
depending on screens or features, and browser dependency checks prevent imports
of engine or build implementation. Neither check recognizes hardcoded project
assumptions inside a permitted file. Review those assumptions using two
unrelated project vocabularies and meaningful empty or unsupported inputs.

The mount entry must accept a host-owned container and dependencies and provide
cleanup. Standalone startup selects its own container and opts into PWA
lifecycle. Do not implement a mount that takes over the host's document,
navigation, storage, or global styles. The rules enforce the import boundaries
around startup and `app/pwa`; they cannot prove that browser effects are absent
elsewhere.

## What ArcLint checks

| Rule | Evidence and limit |
| --- | --- |
| `web/launch-surfaces-present` | Temporarily disabled until Web exists. When enabled, checks exact presence of six specified files. Does not validate package scripts, the Vite build, HTML wiring, manifest semantics, or mounting behavior. |
| `web/layers-point-downward` | Native layer check over resolved imports. |
| `web/browser-dependencies` | Native import check: source may import itself and declared third-party packages, but no Node builtins or code outside the source Zone. A declared package is not thereby proven browser-compatible. |
| `web/standalone-is-an-entrypoint` | Native importer restriction: no other source imports standalone startup. |
| `web/pwa-lifecycle-is-opt-in` | Native importer restriction: only standalone startup imports the PWA Zone, whose own files may import each other. |
| `web/files-speak-their-purpose` | Exact forbidden filenames and directories; semantic naming and cohesion still require review. |
| `web/typescript-source` | Forbids `.js`, `.jsx`, `.mjs`, `.cjs`, `.mts`, and `.cts` source under `src` so those files cannot evade the configured TypeScript observer. |
| `web/explicit-public-exports` | Checks `index.ts`, the mount entry, and direct `.ts`/`.tsx` modules under `shared/ui` and `shared/lib` for wildcard exports. This is a line check; multiline syntax and comments remain limitations. |
| `web/source-layout` | Scoped extension checks layer placement and implementation segments. |
| `web/slices-are-independent` | Scoped extension checks resolved imports between sibling slices. |
| `web/public-interfaces` | Scoped extension checks populated module entries, deep imports, and imports of an implementation's own barrel. |

The three extension checks live in
`.arclint/extensions/web_boundaries.ts`. They retain `ctx.imports(path)`, which
uses the same scoped import facts as `ctx.facts(path).imports` after #69.
The two import checks need outgoing exact-file targets; the incoming dependency projection
is not needed for these checks. The native `independent` constraint
deliberately removes folders owned by declared Zones, so it cannot check slices
inside these layer Zones. Native `structure.require` cannot quantify over every
populated directory without a recorded domain collection. FSD technical slices
are not a new collection of domain aggregates; the extension checks observed
files instead of changing the domain model to satisfy a folder convention.

All extension reads are scoped to `web_source`; no repository-wide access
has been added. ArcLint reports extension enforcement as **heuristic**, even
when an extension declares structural capability. No extension findings means
undetermined, not proven conformance.

The current TypeScript observer resolves relative imports and index files, and
observes static imports, re-exports, literal dynamic imports, and literal
`require()` calls. Use relative source imports in this contract.
Vite/tsconfig aliases are not resolved by this
observer; unknown imports remain errors. Workspace package specifiers resolve
to package directories rather than precise entry files. Relative imports also
fall back to a directory when no matching source or index file is found.
The slice and interface import checks inspect only exact-file targets; neither
directory-only nor unresolved targets prove a slice interface was used.
Computed dynamic imports, imports originating in declaration-only `.d.ts`
files, CSS dependency graphs, package internals, and runtime loading are
outside these import guarantees. Unknown package specifiers are errors, but
missing relative targets are classified as internal; these rules do not
replace TypeScript and Vite resolution checks. `web/dist/` is excluded as
build output. All checks also inherit the repository's `scan.exclude` entries;
excluded source is outside their coverage.

## Browser acceptance criteria

These require the real built application and browser tests. They are not
replaced by architecture fixtures:

1. Build with Vite and load the application from both the origin root and a
   nested deployment path. Assets, API URLs, routes, worker scope, and manifest
   links must remain within their intended base. Vite rewrites built asset URLs
   using its configured base; application URL construction still needs tests.
   [Vite public base path](https://vite.dev/guide/build#public-base-path)
2. Verify the manifest and install flow in the supported browsers over HTTPS
   or a supported local development origin. Confirm installed launch uses the
   expected start URL, scope, display, and icons. A service worker is not itself
   a universal installability requirement.
   [MDN installability](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Making_PWAs_installable)
3. Exercise the explicitly supported offline flows after an online visit, cold
   offline startup, reconnect, and an application update. State which project
   operations remain available, which show an unavailable state, and how edits
   recover without data loss. Cache files or a worker filename do not establish
   those properties.
   [MDN offline operation](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Offline_and_background_operation)
4. Import the mount entry in a host page and verify no automatic DOM mutation
   or worker registration occurs. Mount, dispose, and remount Web in a
   host-owned container; verify listener cleanup, focus behavior, style
   containment, host routing, and independent instances. Test the actual host
   integration surface, including an iframe or webview when one is chosen.
5. Run the same screens against two unrelated project domains, an empty project,
   unavailable capabilities, and host-supplied adapters. Check that changes in
   project vocabulary require data/configuration changes rather than screen
   forks or hardcoded domain switches.

Run `arclint rules test` for the rule fixtures, `arclint check --only 'web/*'`
for current Web findings, and the repository finish gate `make check`.
