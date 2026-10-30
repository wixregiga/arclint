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

// MapServer shows the project's domain overview in a browser and
// saves language edits into the domain file and feature drafts separately. Serve binds
// address, tells listening the URL a browser opens once the listener is
// bound, and answers requests until the listener fails. Composition
// selects the adapter that implements it and the use cases it serves.
type MapServer interface {
	Serve(address string, listening func(url string) error) error
}

// NewMapCommand adapts the domain overview use case into the map
// command family: serve shows the recorded Ubiquitous Language in a
// browser, where it is edited as drafts and published. Presentation of
// the command's own report is closed over the injected Renderer; the
// page is the server's.
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
		if err := server.Serve(ctx.String(flagListen), listening); err != nil {
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
entry. Entries are edited, added and removed in the page as drafts;
Saving records an entry in the domain file through the same define and
remove the command line runs, so every edit is validated as arclint
domain define validates it. Features describes requested behavior with that same
language at hand and saves the prose as project files.`

const mapServeLong = `Serve a map of the project's recorded Ubiquitous Language to a browser.

The page shows the project's domain file: its bounded contexts, and in
each context the aggregates with their entities, invariants and
assertions, the value objects with their invariants, the domain events,
the domain services with their assertions, the specifications and the
open questions, with the relations of the context map. The page loads
the same document arclint domain overview --format json prints.

Each entry is shown with its standing: Accepted when the remote's
default branch (origin/HEAD) records it the same, Proposed when the
domain file in the working tree records it differently or the branch
does not record it yet, Draft when it is edited in the page and not
saved. Saving records an entry in the domain file through define or remove
and shows what changed; a draft the language
refuses stays a draft with the reason. What happens to the branch after
that is git's: commit and propose the change as the project does.
Without a remote default branch the page shows the recording alone.

Features holds freeform prose alongside the current language. Saved feature
drafts live under .arclint/studio/features in the selected project; they do
not change the domain recording. Draft saves refuse a stale revision instead
of overwriting another editor.

If the domain recording is missing, the page opens so you can begin the
language. An existing malformed recording must be corrected before serving.

With no path, the project is the one whose rules.arclint.yaml is found
upward from the working directory, as every other command finds it;
pass a path inside another project to serve that project's domain file.
The server listens on 127.0.0.1 unless --addr names another host, and
prints the URL to open. It answers only requests addressed to an IP
address or localhost and sent by its own page. Ctrl+C stops it.`

const mapServeExample = `  # Map the current project's domain file
  arclint map serve

  # Listen on another port
  arclint map serve --addr 127.0.0.1:8080

  # Map an example project's domain file
  arclint map serve docs/examples/go`
