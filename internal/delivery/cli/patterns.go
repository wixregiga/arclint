package cli

import (
	"fmt"
	"strings"

	"github.com/wixregiga/arclint/internal/application"
)

// NewPatternsCommand adapts the Pattern listing use case into the
// patterns command.
func NewPatternsCommand(list application.ListPatterns) Command {
	return Command{
		Name:  "patterns",
		Short: "list available Pattern distribution packages",
		Run: func(ctx Context) error {
			rows, err := list.Execute()
			if err != nil {
				return ConfigError(err)
			}
			p := &printer{w: ctx.Stdout}
			if len(rows) == 0 {
				p.println("no patterns available")
			}
			for _, row := range rows {
				coverage := ""
				if len(row.Coverage) > 0 {
					coverage = "  coverage [" + strings.Join(row.Coverage, ", ") + "]"
				}
				p.printf("%s/%s@%s  %d rule(s)%s\n",
					row.Namespace, row.Name, row.Version, row.Rules, coverage)
			}
			if p.err != nil {
				return fmt.Errorf("write output: %w", p.err)
			}
			return nil
		},
	}
}
