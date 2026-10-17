package sobekextension

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/sobek"
	sj "github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/wixregiga/arclint/internal/domain/conformance"
)

// RuleType is one extension-registered rule type with a compiled program
// and params schema. Check instantiates the program in a fresh runtime.
type RuleType struct {
	Name        string
	Description string // one-line summary from defineRule
	Capability  string // exact | structural | heuristic | advisory
	SourcePath  string // repo-relative extension file
	RawSchema   map[string]any

	schema          *sj.Schema
	vm              *sobek.Runtime
	check           sobek.Callable
	program         *sobek.Program
	registerTimeout time.Duration
	// declarations are the definition errors the SDK handed to this
	// runtime, keyed by the thrown value.
	declarations map[*sobek.Object]string
}

// Host is the read-only surface the engine lends to a rule during one
// check() invocation. Extensions get exactly this and nothing else: no
// filesystem, no network, no Node shims.
type Host struct {
	Files   func(glob string) ([]FileInfo, error)
	Read    func(path string) (string, error)
	Imports func(path string) []ImportInfo
	Zones   func() map[string][]string
	// Facts returns already-observed declarations and imports for a selected
	// subject, including incident dependencies; nil when outside Scope.
	Facts func(path string) *FactsInfo
	// ZoneOf returns the sorted zones a file belongs to.
	ZoneOf func(path string) []string
	// Domain returns the project's recorded domain model; nil means
	// the host supplies empty collections.
	Domain func() DomainInfo
	// CaseTerm renders a recorded term in one published TermCase,
	// the same implementation yaml expansion uses, so extensions never
	// reimplement casing. Errors on unknown cases and wordless terms.
	CaseTerm func(term, termCase string) (string, error)
}

// sandboxJS closes the engine's documented determinism gap: Date.now and
// Math.random are host-controlled. __arclint exists solely to turn runtime
// calls during the registration phase into a designed error (the two-phase
// lifecycle: register, then run).
const sandboxJS = `
(function () {
	"use strict";
	var fixedNow = %d;
	Date.now = function () { return fixedNow; };
	var seed = %d >>> 0;
	Math.random = function () {
		seed = (seed * 1103515245 + 12345) >>> 0;
		return seed / 4294967296;
	};
	var guard = function () {
		throw new Error("arclint: the runtime API is unavailable during the registration phase; use the ctx passed to check()");
	};
	globalThis.__arclint = Object.freeze({
		files: guard, read: guard, imports: guard, zones: guard, report: guard
	});
})();
`

// register runs one bundled extension through the registration phase and
// records every rule definition its default export declares.
func (r *Registry) register(sourcePath, bundleJS string, opts Options) error {
	prog, err := sobek.Compile(sourcePath, "(function(module, exports){\n"+bundleJS+"\n})", true)
	if err != nil {
		return fmt.Errorf("extension %s: compile: %w", sourcePath, err)
	}
	return r.registerProgram(sourcePath, prog, opts)
}

// registerProgram instantiates compiled code without reloading extension files.
// Only the immutable program is shared; JavaScript state belongs to one run.
func (r *Registry) registerProgram(sourcePath string, prog *sobek.Program, opts Options) error {
	vm := sobek.New()
	vm.SetFieldNameMapper(sobek.TagFieldNameMapper("json", true))

	// The SDK hands every definition error it creates to the host, which
	// then recognizes the thrown value by identity and never runs
	// extension code to classify it.
	declarations := map[*sobek.Object]string{}
	if err := vm.Set("__arclintDefinitionError", func(call sobek.FunctionCall) sobek.Value {
		thrown, isObject := call.Argument(0).(*sobek.Object)
		message, isText := call.Argument(1).Export().(string)
		if isObject && isText {
			declarations[thrown] = message
		}
		return sobek.Undefined()
	}); err != nil {
		return fmt.Errorf("extension %s: sandbox init: %w", sourcePath, err)
	}

	// Deterministic, host-controlled values: a fixed epoch and seed. The
	// point is that no wall clock or entropy reaches rules.
	if _, err := vm.RunString(fmt.Sprintf(sandboxJS, int64(1_600_000_000_000), 42)); err != nil {
		return fmt.Errorf("extension %s: sandbox init: %w", sourcePath, err)
	}

	started := time.Now()
	stopped := func(context string, err error) error {
		if message, declared := definitionError(declarations, err); declared {
			return fmt.Errorf("extension %s: %s", sourcePath, message)
		}
		return newCodeFailure(vm, CodeFailure{
			Source: sourcePath, Phase: conformance.PhaseRegistration,
			Limit: opts.RegisterTimeout, Elapsed: time.Since(started),
		}, context, err)
	}
	timer := time.AfterFunc(opts.RegisterTimeout, func() {
		vm.Interrupt(timeLimit{phase: conformance.PhaseRegistration, limit: opts.RegisterTimeout})
	})
	defer timer.Stop()
	defer vm.ClearInterrupt()

	wrapper, err := vm.RunProgram(prog)
	if err != nil {
		return stopped("", err)
	}
	call, ok := sobek.AssertFunction(wrapper)
	if !ok {
		return fmt.Errorf("extension %s: internal wrapper is not callable", sourcePath)
	}
	module := vm.NewObject()
	exports := vm.NewObject()
	if err := module.Set("exports", exports); err != nil {
		return fmt.Errorf("extension %s: %w", sourcePath, err)
	}
	if _, err := call(sobek.Undefined(), module, exports); err != nil {
		return stopped("registration phase: ", err)
	}

	moduleExports := module.Get("exports")
	if moduleExports == nil || sobek.IsUndefined(moduleExports) || sobek.IsNull(moduleExports) {
		return fmt.Errorf("extension %s: no exports; default-export defineRule(...)", sourcePath)
	}
	obj := moduleExports.ToObject(vm)
	def := obj.Get("default")
	if def == nil || sobek.IsUndefined(def) || sobek.IsNull(def) {
		return fmt.Errorf("extension %s: missing default export; default-export defineRule(...) or an array of them", sourcePath)
	}

	var defs []*sobek.Object
	defObj := def.ToObject(vm)
	if defObj.ClassName() == "Array" {
		length := int(defObj.Get("length").ToInteger())
		for i := 0; i < length; i++ {
			item := defObj.Get(fmt.Sprint(i))
			if item == nil || sobek.IsUndefined(item) {
				continue
			}
			defs = append(defs, item.ToObject(vm))
		}
	} else {
		defs = append(defs, defObj)
	}
	if len(defs) == 0 {
		return fmt.Errorf("extension %s: default export declares no rules", sourcePath)
	}

	for _, d := range defs {
		if marker := d.Get("__arclintRule"); marker == nil || !marker.ToBoolean() {
			return fmt.Errorf("extension %s: default export is not a defineRule(...) result", sourcePath)
		}
		name := d.Get("type").String()
		if prev, exists := r.types[name]; exists {
			return fmt.Errorf("extension %s: rule type %q already registered by %s", sourcePath, name, prev.SourcePath)
		}
		checkFn, ok := sobek.AssertFunction(d.Get("check"))
		if !ok {
			return fmt.Errorf("extension %s: rule %q: check is not a function", sourcePath, name)
		}
		rawSchema, ok := d.Get("paramsSchema").Export().(map[string]any)
		if !ok {
			return fmt.Errorf("extension %s: rule %q: params schema is not an object", sourcePath, name)
		}
		schema, err := compileParamsSchema(name, rawSchema)
		if err != nil {
			return fmt.Errorf("extension %s: rule %q: %w", sourcePath, name, err)
		}
		rt := &RuleType{
			Name:            name,
			Description:     d.Get("description").String(),
			Capability:      d.Get("capability").String(),
			SourcePath:      sourcePath,
			RawSchema:       rawSchema,
			schema:          schema,
			vm:              vm,
			declarations:    declarations,
			check:           checkFn,
			program:         prog,
			registerTimeout: opts.RegisterTimeout,
		}
		r.types[name] = rt
		r.order = append(r.order, name)
	}
	return nil
}

func compileParamsSchema(name string, raw map[string]any) (*sj.Schema, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("params schema not JSON-representable: %w", err)
	}
	doc, err := sj.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("params schema: %w", err)
	}
	c := sj.NewCompiler()
	url := "ext:///" + name + "/params.json"
	if err := c.AddResource(url, doc); err != nil {
		return nil, fmt.Errorf("params schema: %w", err)
	}
	schema, err := c.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("params schema: %w", err)
	}
	return schema, nil
}

// ValidateParams checks a rules.arclint.yaml instance's params against the rule
// type's declared schema, BEFORE any extension code runs, and returns the
// params with top-level schema defaults applied.
func (rt *RuleType) ValidateParams(params map[string]any) (map[string]any, error) {
	if params == nil {
		params = map[string]any{}
	}
	// Apply top-level property defaults (JSON Schema validators do not).
	merged := map[string]any{}
	for k, v := range params {
		merged[k] = v
	}
	if props, ok := rt.RawSchema["properties"].(map[string]any); ok {
		for key, sub := range props {
			subMap, ok := sub.(map[string]any)
			if !ok {
				continue
			}
			if dflt, has := subMap["default"]; has {
				if _, present := merged[key]; !present {
					merged[key] = dflt
				}
			}
		}
	}
	// Round-trip through JSON so the validator sees the JSON data model.
	data, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("rule %q: params not JSON-representable: %w", rt.Name, err)
	}
	instance, err := sj.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("rule %q: params: %w", rt.Name, err)
	}
	if err := rt.schema.Validate(instance); err != nil {
		return nil, fmt.Errorf("rule %q: params do not match the extension's schema:\n%v", rt.Name, err)
	}
	// Return the detached JSON value, never the configuration's nested maps
	// or the shared schema's default values, to the JavaScript runtime.
	return instance.(map[string]any), nil
}

// Check runs one evaluation-phase invocation: check(ctx, params) with the
// host-lent read-only ctx, interrupted at limit. It returns the
// violations the rule reported and how long the check ran.
func (rt *RuleType) Check(host Host, params map[string]any, limit time.Duration) ([]ViolationInput, time.Duration, error) {
	if limit <= 0 {
		return nil, 0, fmt.Errorf("extension %s: rule %q: time limit %s is not positive", rt.SourcePath, rt.Name, limit)
	}
	registry := &Registry{types: map[string]*RuleType{}}
	if err := registry.registerProgram(rt.SourcePath, rt.program, Options{
		RegisterTimeout: rt.registerTimeout,
	}); err != nil {
		return nil, 0, err
	}
	fresh := registry.Get(rt.Name)
	if fresh == nil {
		return nil, 0, fmt.Errorf("extension %s: rule %q was not registered in the fresh runtime", rt.SourcePath, rt.Name)
	}
	return fresh.checkWithHost(host, params, limit)
}

func (rt *RuleType) checkWithHost(host Host, params map[string]any, limit time.Duration) ([]ViolationInput, time.Duration, error) {
	vm := rt.vm
	var reported []ViolationInput
	// lastFile is the subject the check last asked the host about: the
	// file it was most likely evaluating when it stopped.
	var lastFile string
	reading := func(call sobek.FunctionCall) string {
		lastFile = call.Arguments[0].String()
		return lastFile
	}

	ctx := vm.NewObject()
	fail := func(format string, args ...any) sobek.Value {
		panic(vm.NewGoError(fmt.Errorf(format, args...)))
	}
	mustSet := func(name string, fn func(call sobek.FunctionCall) sobek.Value) {
		if err := ctx.Set(name, fn); err != nil {
			panic(err)
		}
	}
	mustSet("files", func(call sobek.FunctionCall) sobek.Value {
		glob := ""
		if len(call.Arguments) > 0 && !sobek.IsUndefined(call.Arguments[0]) {
			glob = call.Arguments[0].String()
		}
		files, err := host.Files(glob)
		if err != nil {
			return fail("ctx.files: %v", err)
		}
		return vm.ToValue(files)
	})
	mustSet("read", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 1 {
			return fail("ctx.read: path is required")
		}
		content, err := host.Read(reading(call))
		if err != nil {
			return fail("ctx.read: %v", err)
		}
		return vm.ToValue(content)
	})
	mustSet("imports", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 1 {
			return fail("ctx.imports: path is required")
		}
		return vm.ToValue(host.Imports(reading(call)))
	})
	mustSet("zones", func(_ sobek.FunctionCall) sobek.Value {
		return vm.ToValue(host.Zones())
	})
	mustSet("facts", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 1 {
			return fail("ctx.facts: path is required")
		}
		if host.Facts == nil {
			return sobek.Null()
		}
		facts := host.Facts(reading(call))
		if facts == nil {
			return sobek.Null()
		}
		return vm.ToValue(facts)
	})
	mustSet("zoneOf", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 1 {
			return fail("ctx.zoneOf: path is required")
		}
		zones := host.ZoneOf(reading(call))
		if zones == nil {
			zones = []string{}
		}
		return vm.ToValue(zones)
	})
	mustSet("domain", func(_ sobek.FunctionCall) sobek.Value {
		if host.Domain == nil {
			return vm.ToValue(emptyDomainInfo(""))
		}
		return vm.ToValue(host.Domain())
	})
	mustSet("caseTerm", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 2 {
			return fail("ctx.caseTerm: term and case are required")
		}
		if host.CaseTerm == nil {
			return fail("ctx.caseTerm: no term-case capability on this host")
		}
		segment, err := host.CaseTerm(call.Arguments[0].String(), call.Arguments[1].String())
		if err != nil {
			return fail("ctx.caseTerm: %v", err)
		}
		return vm.ToValue(segment)
	})
	mustSet("report", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 1 {
			return fail("ctx.report: violation object is required")
		}
		raw, ok := call.Arguments[0].Export().(map[string]any)
		if !ok {
			return fail("ctx.report: argument must be an object")
		}
		v := ViolationInput{}
		if s, ok := raw["subjectPath"].(string); ok {
			v.SubjectPath = s
		}
		if s, ok := raw["path"].(string); ok {
			v.Path = s
		}
		if s, ok := raw["message"].(string); ok {
			v.Message = s
		}
		if n, ok := raw["line"].(int64); ok {
			v.Line = int(n)
		} else if f, ok := raw["line"].(float64); ok {
			v.Line = int(f)
		}
		if s, ok := raw["fixHint"].(string); ok {
			v.FixHint = s
		}
		if v.Path == "" || v.Message == "" {
			return fail("ctx.report: path and message are required")
		}
		reported = append(reported, v)
		return sobek.Undefined()
	})

	started := time.Now()
	timer := time.AfterFunc(limit, func() {
		vm.Interrupt(timeLimit{phase: conformance.PhaseCheck, limit: limit})
	})
	defer timer.Stop()
	defer vm.ClearInterrupt()

	if _, err := rt.check(sobek.Undefined(), ctx, vm.ToValue(params)); err != nil {
		if message, declared := definitionError(rt.declarations, err); declared {
			return reported, time.Since(started), fmt.Errorf("extension %s: rule %q: %s", rt.SourcePath, rt.Name, message)
		}
		elapsed := time.Since(started)
		return reported, elapsed, newCodeFailure(vm, CodeFailure{
			Source: rt.SourcePath, Phase: conformance.PhaseCheck, Limit: limit,
			Elapsed: elapsed, File: lastFile,
		}, fmt.Sprintf("rule %q: ", rt.Name), err)
	}
	return reported, time.Since(started), nil
}

// timeLimit is the value a runtime is interrupted with when extension
// code runs past its limit, so the interruption is told apart from an
// error the code raised.
type timeLimit struct {
	phase conformance.ExtensionPhase
	limit time.Duration
}

func (t timeLimit) String() string {
	return fmt.Sprintf("%s exceeded the %s timeout", t.phase, t.limit)
}

// definitionError returns the message of a definition error the SDK
// handed to the host, recognized by the thrown value's identity. It runs
// no extension code: errors.As would call the exception's Unwrap, which
// walks the thrown object's prototype chain, so the error's own type is
// asserted instead.
func definitionError(declarations map[*sobek.Object]string, err error) (string, bool) {
	exception, ok := err.(*sobek.Exception)
	if !ok {
		return "", false
	}
	thrown, isObject := exception.Value().(*sobek.Object)
	if !isObject {
		return "", false
	}
	message, declared := declarations[thrown]
	return message, declared
}

// CodeFailure is extension code that did not complete: it ran past its
// time limit or raised an error while running, in one phase. Errors
// outside running code (bundling, a malformed definition, invalid
// params) are not CodeFailures.
type CodeFailure struct {
	// Source is the extension the code came from.
	Source string
	Phase  conformance.ExtensionPhase
	// Limit is the time limit in force; Elapsed is how long the code ran.
	Limit   time.Duration
	Elapsed time.Duration
	// File is the subject the check last asked the host about, "" when
	// none is known.
	File     string
	timedOut bool
	raised   string
}

// newCodeFailure completes a failure from what the code raised. Its
// text is read once, here, while the runtime's interrupt is still armed.
// The failure keeps no reference to the raised value: unwrapping a
// sobek exception may run extension code.
func newCodeFailure(vm *sobek.Runtime, failure CodeFailure, context string, err error) *CodeFailure {
	if interrupted, ok := err.(*sobek.InterruptedError); ok {
		if _, ok := interrupted.Value().(timeLimit); ok {
			failure.timedOut = true
			return &failure
		}
	}
	text := describe(vm, err)
	if text == "" {
		text = "a thrown value with no message"
	}
	failure.raised = context + text
	return &failure
}

// unreadable stands for a thrown value whose text cannot be read.
const unreadable = "a thrown value whose message could not be read"

// describe renders what extension code raised on one line. A thrown
// object's text comes from its own toString, which may throw or never
// return; either leaves a placeholder instead of reaching the host.
func describe(vm *sobek.Runtime, err error) (text string) {
	exception, ok := err.(*sobek.Exception)
	if !ok {
		return strings.Join(strings.Fields(err.Error()), " ")
	}
	if _, isObject := exception.Value().(*sobek.Object); !isObject {
		return strings.Join(strings.Fields(exception.Error()), " ")
	}
	// An interrupt inside toString is not a JS exception: Try re-panics
	// it, so the recover below is what keeps it from the host.
	defer func() {
		if recover() != nil {
			text = unreadable
		}
	}()
	if thrown := vm.Try(func() { text = exception.Error() }); thrown != nil {
		return unreadable
	}
	return strings.Join(strings.Fields(text), " ")
}

func (f *CodeFailure) Error() string {
	if f.timedOut {
		return fmt.Sprintf("extension %s: %s", f.Source, timeLimit{phase: f.Phase, limit: f.Limit})
	}
	return fmt.Sprintf("extension %s: %s", f.Source, f.raised)
}

// TimedOut reports whether the code was stopped at its time limit.
func (f *CodeFailure) TimedOut() bool { return f.timedOut }

// Raised is the error the code raised, on one line; "" for a timeout.
func (f *CodeFailure) Raised() string { return f.raised }

// ExtensionFailure translates the failure into the domain's terms.
func (f *CodeFailure) ExtensionFailure() (conformance.ExtensionFailure, error) {
	failure, err := conformance.NewExtensionFailure(conformance.ExtensionFailureSpec{
		Source: f.Source, Phase: f.Phase, Limit: f.Limit, Elapsed: f.Elapsed,
		TimedOut: f.timedOut, Raised: f.raised, File: f.File,
	})
	if err != nil {
		return conformance.ExtensionFailure{}, fmt.Errorf("extension %s: %w", f.Source, err)
	}
	return failure, nil
}
