package cli

import (
	"fmt"

	"github.com/wixregiga/arclint/internal/application"
)

const (
	mapCommand = "map"
	flagListen = "addr"
	// defaultListen binds the loopback interface alone: the page shows
	// the project's recorded language to the person at this machine,
	// never to the network, unless they name another host.
	defaultListen = "127.0.0.1:7380"
)

// MapServer shows the project's domain overview in a browser. Serve
// binds address, tells listening the URL a browser opens once the
// listener is bound, and answers requests until the listener fails.
// Composition selects the adapter that implements it.
type MapServer interface {
	Serve(address string, overview application.DomainOverview, listening func(url string) error) error
}

// NewMapCommand adapts the domain overview use case into the map
// command family: serve shows the recorded Ubiquitous Language in a
// browser. Presentation of the command's own report is closed over the
// injected Renderer; the page is the server's.
func NewMapCommand(overview application.GetDomainOverview, server MapServer, render Renderer) Command {
	return Command{
		Name:  mapCommand,
		Short: "show the project's ubiquitous language as a map",
		Long:  mapGroupLong,
		Subcommands: []Command{
			{
				Name:    "serve",
				Short:   "serve a map of the project's ubiquitous language to a browser",
				Long:    mapServeLong,
				Example: mapServeExample,
				// The optional path selects the repository; the composition
				// root resolves it before any adapter exists, as `check
				// [path]` does. The path keeps the adapter's default file
				// completion, so no CompleteArgs here.
				MaxArgs: 1,
				Flags: []Flag{{
					Name:    flagListen,
					Default: defaultListen,
					Doc:     "address to listen on, host:port; the default serves this machine alone",
				}},
				Run: mapServeRunner(overview, server, render),
			},
		},
	}
}

func mapServeRunner(overview application.GetDomainOverview, server MapServer, render Renderer) func(Context) error {
	return func(ctx Context) error {
		result, err := overview.Execute()
		if err != nil {
			return domainError(err)
		}
		if !result.Found {
			if err := render.Render(ctx.Stdout, DomainMissingReport{Source: result.Source}); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return &ExitError{Code: ExitViolations, Message: "map serve: no recorded Ubiquitous Language at " + result.Source}
		}
		// A report that cannot be written is the command's own failure,
		// kept apart from the server's, which is a configuration error.
		var writeErr error
		listening := func(url string) error {
			report := MapServeReport{URL: url, Project: result.Language.Project, Source: result.Source}
			if err := render.Render(ctx.Stdout, report); err != nil {
				writeErr = fmt.Errorf("write output: %w", err)
			}
			return writeErr
		}
		if err := server.Serve(ctx.String(flagListen), result, listening); err != nil {
			if writeErr != nil {
				return writeErr
			}
			return ConfigError(fmt.Errorf("map serve: %w", err))
		}
		return nil
	}
}

const mapGroupLong = `Show the project's recorded Ubiquitous Language as a map.

The map command serves a browser page over HTTP: a navigator of the
recorded bounded contexts, a concept map of one context at a time, and
an inspector that shows what the domain file records for the selected
entry. Display only: it does not edit the domain file. The language
shown is the one the project's domain file records; record meanings with
arclint domain define.`

const mapServeLong = `Serve a map of the project's recorded Ubiquitous Language to a browser.

The page shows the project's domain file: its bounded contexts, and in
each context the aggregates with their entities, invariants and
assertions, the value objects with their invariants, the domain events,
the domain services with their assertions, the specifications and the
open questions, with the relations of the context map. The page loads
the same document arclint domain overview --format json prints.

With no path, the project is the one whose rules.arclint.yaml is found
upward from the working directory, as every other command finds it;
pass a path inside another project to serve that project's domain file.
The server listens on 127.0.0.1 unless --addr names another host, and
prints the URL to open. Ctrl+C stops it.`

const mapServeExample = `  # Map the current project's domain file
  arclint map serve

  # Listen on another port
  arclint map serve --addr 127.0.0.1:8080

  # Map an example project's domain file
  arclint map serve docs/examples/go`
