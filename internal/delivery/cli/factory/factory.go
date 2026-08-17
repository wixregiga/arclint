// Package clifactory selects a sealed CLI Adapter by ArcLint-owned
// identity. Together with the composition root it is the only code
// permitted to reach the Cobra adapter.
package clifactory

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/delivery/cli"
	cobraadapter "github.com/wixregiga/arclint/internal/delivery/cli/adapters/cobra"
)

// Select returns the sealed Adapter for one ArcLint-owned name.
func Select(name cli.AdapterName) (cli.Adapter, error) {
	switch name {
	case cli.AdapterCobra:
		return cobraadapter.New(), nil
	}
	return nil, fmt.Errorf("cli adapter %q: unknown", name)
}
