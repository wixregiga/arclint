// Package cli is the ArcLint-owned CLI Interface: framework-neutral
// Invocation, Outcome, command descriptions, and the Adapter contract
// a concrete CLI framework implements behind the factory. Framework
// types never cross this Interface.
package cli

import "io"

// Exit codes are the CLI contract: 0 clean, 1 error-severity findings,
// 2 configuration or usage error.
const (
	ExitClean       = 0
	ExitViolations  = 1
	ExitConfigError = 2
)

// Invocation is one CLI run request.
type Invocation struct {
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Outcome is the complete result of one Invocation.
type Outcome struct {
	ExitCode int
}

// AdapterName is the ArcLint-owned identity a sealed CLI Adapter is
// selected by.
type AdapterName string

// AdapterCobra names the Cobra adapter.
const AdapterCobra AdapterName = "cobra"

// Adapter runs a neutral command tree for one Invocation.
type Adapter interface { //nolint:iface // Framework-neutral port is intentionally consumed across package boundaries.
	Run(root Command, invocation Invocation) Outcome
}
