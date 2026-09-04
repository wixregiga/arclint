package sobekextension

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/grafana/sobek"
	sj "github.com/santhosh-tekuri/jsonschema/v6"
)

// RuleType is one extension-registered rule type: a sobek runtime holding
// the compiled check function, plus the host-compiled params schema.
type RuleType struct {
	Name        string
	Description string // one-line summary from defineRule
	Capability  string // exact | structural | heuristic | advisory
	SourcePath  string // repo-relative extension file
	RawSchema   map[string]any

	schema  *sj.Schema
	vm      *sobek.Runtime
	check   sobek.Callable
	timeout time.Duration
}

// Host is the read-only surface the engine lends to a rule during one
// check() invocation. Extensions get exactly this and nothing else: no
// filesystem, no network, no Node shims.
type Host struct {
	Files   func(glob string) ([]FileInfo, error)
	Read    func(path string) (string, error)
	Imports func(path string) []ImportInfo
	Modules func() map[string][]string
	// Facts returns declaration facts for one file, nil when the
	// file's language does not supply declarations.
	Facts func(path string) *FactsInfo
	// ModuleOf returns the sorted modules a file belongs to.
	ModuleOf func(path string) []string
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
		files: guard, read: guard, imports: guard, modules: guard, report: guard
	});
})();
`

// register runs one bundled extension through the registration phase and
// records every rule definition its default export declares.
func (r *Registry) register(sourcePath, bundleJS string, opts Options) error {
	vm := sobek.New()
	vm.SetFieldNameMapper(sobek.TagFieldNameMapper("json", true))

	// Deterministic, host-controlled values: a fixed epoch and seed. The
	// point is that no wall clock or entropy reaches rules.
	if _, err := vm.RunString(fmt.Sprintf(sandboxJS, int64(1_600_000_000_000), 42)); err != nil {
		return fmt.Errorf("extension %s: sandbox init: %w", sourcePath, err)
	}

	prog, err := sobek.Compile(sourcePath, "(function(module, exports){\n"+bundleJS+"\n})", true)
	if err != nil {
		return fmt.Errorf("extension %s: compile: %w", sourcePath, err)
	}

	timer := time.AfterFunc(opts.RegisterTimeout, func() {
		vm.Interrupt(fmt.Sprintf("registration exceeded %s", opts.RegisterTimeout))
	})
	defer timer.Stop()
	defer vm.ClearInterrupt()

	wrapper, err := vm.RunProgram(prog)
	if err != nil {
		return fmt.Errorf("extension %s: %w", sourcePath, err)
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
		return fmt.Errorf("extension %s: registration phase: %w", sourcePath, err)
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
			Name:        name,
			Description: d.Get("description").String(),
			Capability:  d.Get("capability").String(),
			SourcePath:  sourcePath,
			RawSchema:   rawSchema,
			schema:      schema,
			vm:          vm,
			check:       checkFn,
			timeout:     opts.CheckTimeout,
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

// ValidateParams checks a rules.yaml instance's params against the rule
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
	return merged, nil
}

// Check runs one evaluation-phase invocation: check(ctx, params) with the
// host-lent read-only ctx, under an interrupt-based timeout. It returns
// the violations the rule reported.
func (rt *RuleType) Check(host Host, params map[string]any) ([]ViolationInput, error) {
	vm := rt.vm
	var reported []ViolationInput

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
		content, err := host.Read(call.Arguments[0].String())
		if err != nil {
			return fail("ctx.read: %v", err)
		}
		return vm.ToValue(content)
	})
	mustSet("imports", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 1 {
			return fail("ctx.imports: path is required")
		}
		return vm.ToValue(host.Imports(call.Arguments[0].String()))
	})
	mustSet("modules", func(_ sobek.FunctionCall) sobek.Value {
		return vm.ToValue(host.Modules())
	})
	mustSet("facts", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 1 {
			return fail("ctx.facts: path is required")
		}
		if host.Facts == nil {
			return sobek.Null()
		}
		facts := host.Facts(call.Arguments[0].String())
		if facts == nil {
			return sobek.Null()
		}
		return vm.ToValue(facts)
	})
	mustSet("moduleOf", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < 1 {
			return fail("ctx.moduleOf: path is required")
		}
		mods := host.ModuleOf(call.Arguments[0].String())
		if mods == nil {
			mods = []string{}
		}
		return vm.ToValue(mods)
	})
	mustSet("domain", func(_ sobek.FunctionCall) sobek.Value {
		if host.Domain == nil {
			return vm.ToValue(emptyDomainInfo())
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

	timer := time.AfterFunc(rt.timeout, func() {
		vm.Interrupt(fmt.Sprintf("rule %q exceeded the %s timeout", rt.Name, rt.timeout))
	})
	defer timer.Stop()
	defer vm.ClearInterrupt()

	if _, err := rt.check(sobek.Undefined(), ctx, vm.ToValue(params)); err != nil {
		return reported, fmt.Errorf("rule %q: %w", rt.Name, err)
	}
	return reported, nil
}
