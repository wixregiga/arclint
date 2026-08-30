// Package reportfactory selects a sealed report Renderer by ArcLint-owned
// identity. Together with the composition root it is the only code
// permitted to reach the concrete report adapters.
package reportfactory

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/delivery/cli"
	jsonreport "github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/json"
	lipglossreport "github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/lipgloss"
	plainreport "github.com/wixregiga/arclint/internal/delivery/cli/adapters/report/plain"
)

// Select returns the sealed Renderer for one ArcLint-owned name.
func Select(name cli.RendererName) (cli.Renderer, error) {
	switch name {
	case cli.RendererPlain:
		return plainreport.New(), nil
	case cli.RendererJSON:
		return jsonreport.New(), nil
	case cli.RendererLipgloss:
		return lipglossreport.New(), nil
	}
	return nil, fmt.Errorf("report adapter %q: unknown", name)
}
