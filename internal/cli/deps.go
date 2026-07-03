package cli

// huh is part of the locked stack (docs/discovery.md §5): it renders the
// interactive prompt forms for `arclint new` and `arclint make`. The skeleton
// has no prompts yet, so it is blank-imported here to pin it in go.mod and
// go.sum now — command agents can use it without ever touching go.mod.
import _ "github.com/charmbracelet/huh"
