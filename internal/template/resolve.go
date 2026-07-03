package template

import "fmt"

// ResolveInput carries the tiered value sources for Resolve, in priority
// order: explicit flags beat saved answers beat manifest defaults
// (docs/design/cli.md, input resolution order). Builtins are only used to
// interpolate defaults; they never satisfy a declared variable.
type ResolveInput struct {
	Flags    map[string]string
	Saved    map[string]string
	Builtins map[string]string
}

// Resolution is the outcome of one resolve pass over the manifest's ordered
// variable list.
type Resolution struct {
	// Values maps resolved variable names to normalized values. Variables
	// skipped by when are absent (they persist as absent in answers files).
	Values map[string]string
	// Skipped is the set of variables whose when condition was false.
	Skipped map[string]bool
	// Missing lists required variables still unresolved after flags, saved
	// answers, and defaults — the exact set a prompt may ask for.
	Missing []*Variable
}

// Resolve performs a single pass over the variables in declaration order.
// Callers prompt for Missing and re-resolve with the prompted values merged
// into Flags; when conditions referencing a still-missing variable see it as
// the empty string until the re-resolve.
func (m *Manifest) Resolve(in ResolveInput) (*Resolution, error) {
	res := &Resolution{
		Values:  map[string]string{},
		Skipped: map[string]bool{},
	}
	for i := range m.Variables {
		v := &m.Variables[i]
		if v.When != "" && !evalWhen(v.whenClauses, res.Values) {
			res.Skipped[v.Name] = true
			if _, has := in.Flags[v.Name]; has {
				return nil, fmt.Errorf("variable %q is skipped by its when condition (%s) — drop --var %s=...", v.Name, v.When, v.Name)
			}
			continue
		}
		if raw, ok := in.Flags[v.Name]; ok {
			val, err := v.Check(raw)
			if err != nil {
				return nil, err
			}
			res.Values[v.Name] = val
			continue
		}
		if raw, ok := in.Saved[v.Name]; ok {
			// Saved answers were validated when recorded; re-check so a
			// template that tightened its constraints falls through to
			// default/prompt instead of rendering an invalid value.
			if val, err := v.Check(raw); err == nil {
				res.Values[v.Name] = val
				continue
			}
		}
		if def, ok := v.DefaultString(); ok {
			r := NewRenderer(overlay(in.Builtins, res.Values))
			out, err := r.Render([]byte(def), "default of variable "+v.Name)
			if err != nil {
				if len(res.Missing) > 0 {
					// The default references an earlier variable that is
					// itself still unresolved; leave this one unresolved too
					// and let the post-prompt re-resolve fill it in.
					continue
				}
				return nil, err
			}
			val, err := v.Check(string(out))
			if err != nil {
				return nil, err
			}
			res.Values[v.Name] = val
			continue
		}
		res.Missing = append(res.Missing, v)
	}
	return res, nil
}

// RenderVars builds the renderer variable map: built-ins first, skipped
// variables as the empty string, then resolved values (manifest variables
// shadow built-ins of the same name).
func (res *Resolution) RenderVars(builtins map[string]string) map[string]string {
	out := make(map[string]string, len(builtins)+len(res.Values)+len(res.Skipped))
	for k, v := range builtins {
		out[k] = v
	}
	for name := range res.Skipped {
		out[name] = ""
	}
	for k, v := range res.Values {
		out[k] = v
	}
	return out
}

func overlay(base, over map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(over))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}
