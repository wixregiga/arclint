# Embedded Extension Runtimes: Executing User TypeScript from a Go Single Binary

Research report, 2026-08-10, for the arclint multi-language design. Every claim carries its source link inline; [INFERENCE] labels mark statements not directly verified. Binary sizes and the pipeline proof were measured by the researching agent (Go 1.26.4, linux/amd64).

## Option A — esbuild transpile + goja/sobek execute

**The k6 pattern is confirmed, and the researching agent reproduced it.** k6 documents: "k6 uses [esbuild](https://esbuild.github.io/) to transpile TypeScript (TS) code for all files that have the `.ts` extension" and "Starting on k6 v0.57, TypeScript support is enabled by default" ([k6 docs](https://grafana.com/docs/k6/latest/using-k6/javascript-typescript-compatibility-mode/)). k6pack states "Under the hood, k6pack uses the esbuild library" ([k6pack](https://github.com/grafana/k6pack)). esbuild is written in Go and exposes "A straightforward API for CLI, JS, and Go" with "JavaScript, CSS, TypeScript, and JSX built-in" ([esbuild](https://github.com/evanw/esbuild)).

Reproduction: a Go binary importing `esbuild/pkg/api` + `grafana/sobek`, fed `let x: number = 1; x+1`, printed `2`. The pipeline runs fully in-process — no Node, no npm, nothing user-side.

**Critical caveat:** esbuild strips types, it does not check them. k6's docs say so explicitly: "TypeScript support is partial as it strips the type information but doesn't provide type safety" ([same](https://grafana.com/docs/k6/latest/using-k6/javascript-typescript-compatibility-mode/)). Type safety is an author-time concern (their editor), not a host-side guarantee.

**goja:** "ECMAScript 5.1(+) in pure Go", "no cgo dependencies", "Most of ES6 functionality, still work in progress", "6-7 times faster than otto", but "not a replacement for V8" ([goja](https://github.com/dop251/goja)). Since esbuild targets whatever ES level you set, the engine's ES ceiling is largely a non-issue.

**sobek:** Grafana forked goja "To accelerate the development speed and bring ECMAScript Modules (ESM) support to k6 earlier"; since v0.52.0 "k6 (and its extensions) now use `sobek` instead of the original `goja`" ([k6 v0.52.0 release notes](https://github.com/grafana/k6/blob/master/release%20notes/v0.52.0.md)).

**Maintenance, both alive (GitHub API, queried 2026-08-10):** goja `pushed_at` 2026-08-06, `archived: false`, 7041 stars ([API](https://api.github.com/repos/dop251/goja)); sobek `pushed_at` 2026-07-27, `archived: false` ([API](https://api.github.com/repos/grafana/sobek)). The fork did not kill upstream.

**Sandboxing:** a bare `sobek.New()` runtime exposes only ES built-ins; filesystem/network reach lives in the separate `goja_nodejs` package, so a host that injects nothing grants nothing ([goja](https://github.com/dop251/goja)). **Determinism is the weak spot** — `Date.now()` and `Math.random()` exist by default and would need host overrides. [INFERENCE] goja's documented `Interrupt` mechanism supplies runaway-loop timeouts; the agent did not execute a timeout test.

## Option B — wazero + Extism

wazero is a "zero dependency WebAssembly runtime for Go developers" that "doesn't rely on CGO" ([wazero](https://github.com/tetratelabs/wazero)). Core 1.0/2.0 conformant, WASI Preview1 partial; Preview2 closed "not planned" pending Component Model ([specs](https://wazero.io/specs/), [issue 2289](https://github.com/tetratelabs/wazero/issues/2289)). No fuel/gas metering — cancellation goes through `context.Context`, memory through `WithMemoryLimitPages` ([RATIONALE.md](https://github.com/tetratelabs/wazero/blob/main/RATIONALE.md)). A 2026 benchmark puts wazero at ~4.72x native vs. wasmtime ~2.41x ([00f.net](https://00f.net/2026/06/23/webassembly-runtimes-2026/)).

Extism's Go SDK uses wazero, confirmed in its [go.mod](https://raw.githubusercontent.com/extism/go-sdk/main/go.mod). Host functions bind guest `extern` imports to Go closures ([docs](https://extism.org/docs/concepts/host-functions/)).

**This is where Option B breaks for the stated use case.** The JS PDK "compiles [JS] to a Wasm module using QuickJS-ng (via rquickjs) and Wizer" and requires the plugin author to have the `extism-js` binary plus Binaryen on PATH; TypeScript is handled only by pre-transpiling with esbuild, with `.d.ts` for IDE support ([js-pdk](https://github.com/extism/js-pdk)). So rules ship as prebuilt `.wasm`, not as readable `.ts` — you trade a zero-toolchain user for a heavy-toolchain author and opaque artifacts.

Real WASM plugin systems verified: dprint (Wasmtime since [0.55.0](https://github.com/dprint/dprint/releases/tag/0.55.0), Rust host), Shopify Functions ([Javy](https://shopify.engineering/javascript-in-webassembly-for-shopify-functions)), Envoy/Istio [proxy-wasm](https://github.com/proxy-wasm/spec), Zellij (wasmi). **Correction to the research brief:** risor is not WASM — it is a pure-Go bytecode VM ([risor](https://github.com/risor-io/risor)).

## Option C — subprocess

go-plugin uses gRPC or net/rpc over local transport and is "in use by Terraform, Nomad, Vault, Boundary, and Waypoint" ([go-plugin](https://github.com/hashicorp/go-plugin)). It offers **no sandboxing** — documented isolation is crash containment and optional TLS, with no seccomp/namespace restriction in the README or [internals doc](https://github.com/hashicorp/go-plugin/blob/main/docs/internals.md). Terraform solves distribution via the registry protocol's per-platform download endpoint `GET :namespace/:type/:version/download/:os/:arch` ([spec](https://developer.hashicorp.com/terraform/internals/provider-registry-protocol)) — that is, it builds and hosts N binaries per plugin.

The lightweight variant is a plain stdin/stdout JSON protocol: the host writes the targeted file list to the child's stdin and reads a JSON violation array from stdout, with a sanitized environment and a timeout. It costs nothing in binary size but requires the user to already have the rule's executable and its runtime — which fails a zero-toolchain requirement. For output interop, SARIF 2.1.0 is an OASIS Standard ([OASIS](https://docs.oasis-open.org/sarif/sarif/v2.1.0/os/sarif-v2.1.0-os.html)) and reviewdog ingests errorformat, rdjson, checkstyle, and SARIF ([reviewdog](https://github.com/reviewdog/reviewdog)).

## Option D — pure-Go scripting languages

**starlark-go** — Go implementation of Starlark, "a dialect of Python intended for use as a configuration language". The spec guarantees determinism ("no sources of random numbers, clocks, or unspecified iterators"), hermeticity ("no external side effects"), and globals frozen after load. It is an "untyped dynamic language" with no annotations ([repo](https://github.com/google/starlark-go), [spec.md](https://github.com/google/starlark-go/blob/master/doc/spec.md)). Verified embedders via go.mod: [Tilt](https://github.com/tilt-dev/tilt/blob/master/go.mod), [Cirrus CI](https://github.com/cirruslabs/cirrus-cli/blob/master/go.mod), [qri/startf](https://github.com/qri-io/startf), and Isopod (dormant, last push 2023-11-17). Best sandbox/determinism story of any option; worst familiarity story for TypeScript authors.

**risor** — pure Go, Go/Python-hybrid syntax, "type declarations are not supported"; active (v2.1.0, push 2026-08-09) ([repo](https://github.com/risor-io/risor), [syntax](https://risor.io/docs/syntax)). Small ecosystem.

**gopher-lua** — pure-Go Lua 5.1 VM, dynamically typed, active (v1.1.2, push 2026-04-01) ([repo](https://github.com/yuin/gopher-lua), [API](https://api.github.com/repos/yuin/gopher-lua)). Familiar to game/Neovim/nginx developers, not to TS developers.

**tengo** — pure Go, no cgo, dynamically typed, not archived, but latest tag v3.0.0 dates to 2025-05-24 — no release in ~14 months ([repo](https://github.com/d5/tengo), [API](https://api.github.com/repos/d5/tengo/releases/latest)). Weakest maintenance signal.

## Option E — typing bridge (schemas and generated types)

- **Zod v4 ships built-in `z.toJSONSchema()`** ([zod.dev](https://zod.dev/json-schema)). The third-party `zod-to-json-schema` is **archived**: "As of November 2025, this project will no longer be actively maintained" ([repo](https://github.com/StefanTerdell/zod-to-json-schema)). Do not adopt it.
- **json-schema-to-typescript** — "Compile JSON Schema to TypeScript type declarations"; commits current (2026-08-06) but npm stuck at 15.0.4 from 2025-01-14 ([repo](https://github.com/bcherny/json-schema-to-typescript), [npm](https://registry.npmjs.org/json-schema-to-typescript)).
- **quicktype** — JSON/JSON Schema/TS/GraphQL to 20+ languages including both Go and TypeScript; actively maintained, v26.0.0 released 2026-07-20 ([repo](https://github.com/glideapps/quicktype), [API](https://api.github.com/repos/glideapps/quicktype)).
- **Go publishing schemas:** `invopop/jsonschema` generates JSON Schema from Go types, draft 2020-12, v0.14.0 ([repo](https://github.com/invopop/jsonschema)). `santhosh-tekuri/jsonschema` is a validator, very active (v6.0.3, 2026-08-06) ([repo](https://github.com/santhosh-tekuri/jsonschema)).
- **Direct Go to TS:** `gzuidhof/tygo` generates TS typings from Go source, honoring `json` tags, v0.2.21 ([repo](https://github.com/gzuidhof/tygo)). This skips JSON Schema entirely and is the shortest path to a `.d.ts` for rule authors.
- SchemaStore and the `$schema` key give config-file autocomplete, though VS Code notes the syntax "is VS Code-specific" ([VS Code docs](https://code.visualstudio.com/docs/languages/json)).

## Measured binary cost

Built locally by the researching agent, Go 1.26.4, linux/amd64, `-ldflags="-s -w"`, minimal real usage of each package. Baseline hello-world = 1.60 MB.

| Import | Binary | Delta |
|---|---|---|
| baseline | 1.60 MB | — |
| wazero v1.12.0 | 2.58 MB | **+0.98 MB** |
| starlark-go | 2.76 MB | **+1.16 MB** |
| esbuild v0.28.2 | 8.17 MB | **+6.57 MB** |
| goja | 10.00 MB | **+8.40 MB** |
| sobek | 10.20 MB | **+8.60 MB** |
| **esbuild + sobek** | **16.23 MB** | **+14.63 MB** |
| extism/go-sdk v1.7.1 | 12.76 MB | **+11.16 MB** |

Extism's SDK costs ~11x bare wazero — it drags in OTLP protobuf and pprof. If you go WASM, use wazero directly.

## Ranked recommendation

| # | Approach | TS authoring | Zero user toolchain | Sandbox | Determinism | Size | Verdict |
|---|---|---|---|---|---|---|---|
| 1 | **esbuild + sobek** | Native `.ts` source | Yes | Good (inject nothing) | Needs `Date`/`Math.random` overrides | +14.6 MB | Proven by k6; only option meeting the TS requirement |
| 2 | **starlark-go** | No (Python-ish) | Yes | Best (spec-guaranteed) | Best (spec-guaranteed) | +1.16 MB | Ideal on every axis except the stated TS requirement |
| 3 | **wazero direct** | Prebuilt `.wasm` only | Yes at runtime, no at authoring | Strong | Good (no ambient clock) | +0.98 MB | Cheap host, but rules stop being readable source |
| 4 | **Extism + JS PDK** | TS via esbuild pre-step | No — author needs `extism-js` + Binaryen | Strong | Good | +11.2 MB | Heavy author toolchain, worst size-per-benefit |
| 5 | **go-plugin / stdin-stdout subprocess** | Any language | No — user must have the executable and its runtime | None | None | ~0 | Escape hatch, not an extension SDK |

**Recommendation:** Option A. It is the only architecture that satisfies "TS-authored rules with zero user-side toolchain," and k6 is a large-scale production proof. Budget ~15 MB. Close the determinism gap by overriding `Date`/`Math.random` in the runtime and injecting a read-only file API instead of any Node shim.

**Strong secondary suggestion:** ship rule-author types via `tygo` (or `invopop/jsonschema` + `quicktype`) so authors get a `.d.ts` generated from the host's Go structs — that recovers author-side type safety that esbuild deliberately does not provide.

**If the TS requirement is negotiable, reconsider starlark-go.** It gives spec-level determinism and hermeticity for 1/13th the binary cost — otherwise exactly the listed property set.
