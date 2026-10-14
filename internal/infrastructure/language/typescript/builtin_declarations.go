package typescript

import "sync"

// The compiler-derived tables are immutable and loaded only for resolution.
// Imports and syntactic facts never allocate or consult them.
type builtinDeclarations struct {
	types        map[string]bool
	values       map[string]bool
	valueTypes   map[string]string
	memberOwners map[string]string
	globalNames  map[string]bool
	constructors map[string]string
	computations map[string]string
}

var (
	builtinOnce         sync.Once
	builtinTypes        map[string]bool
	builtinValues       map[string]bool
	builtinValueTypes   map[string]string
	builtinMemberOwners map[string]string
	builtinGlobalNames  map[string]bool
	builtinConstructors map[string]string
	builtinComputations map[string]string
)

func initializeBuiltins() {
	builtinOnce.Do(func() {
		tables := builtinDeclarationData()
		builtinTypes, builtinValues = tables.types, tables.values
		builtinValueTypes, builtinMemberOwners = tables.valueTypes, tables.memberOwners
		builtinGlobalNames = tables.globalNames
		builtinConstructors, builtinComputations = tables.constructors, tables.computations
	})
}
