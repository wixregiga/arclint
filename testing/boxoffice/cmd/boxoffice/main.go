// Command boxoffice runs the box office: a chi router over
// in-memory repositories, optionally seeded with Priya's shows.
package main

import (
	"boxoffice/internal/app"
	"boxoffice/internal/app/memory"
	"boxoffice/web"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"time"
)

// version is the product version; the build stamps the rest.
var version = "0.1.0"

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	seed := flag.Bool("seed", false, "start with Priya's sample shows")
	token := flag.String("organizer-token", "", "the organizer's token; empty keeps the organizer side locked")
	webDir := flag.String("web", "", "serve this directory as the web app instead of the embedded build")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("boxoffice version " + formatVersion(version))
		return
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	events := memory.NewEvents()
	orders := memory.NewOrders()
	capacities := memory.NewCapacities()
	if *seed {
		if err := seedShows(events, capacities); err != nil {
			slog.Error("seeding", "error", err)
			os.Exit(1)
		}
		if err := seedOrders(orders, capacities, time.Now()); err != nil {
			slog.Error("seeding orders", "error", err)
			os.Exit(1)
		}
		slog.Info("seeded Priya's shows and the deals struck for them")
	}
	if *token == "" {
		slog.Warn("no -organizer-token given: the organizer side stays locked")
	}

	var webFS fs.FS
	switch dist, embedded := web.Dist(); {
	case *webDir != "":
		webFS = os.DirFS(*webDir)
		slog.Info("serving web app from directory", "dir", *webDir)
	case embedded:
		webFS = dist
		slog.Info("serving the embedded web app")
	default:
		slog.Info("no web app in this build; API only")
	}

	handler := app.New(app.Config{
		OrganizerToken: *token,
		HoldFor:        5 * time.Minute,
		Now:            time.Now,
		WebFS:          webFS,
	}, app.Deps{Events: events, Orders: orders, Capacities: capacities})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	slog.Info("boxoffice listening", "addr", *addr)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("listening", "error", err)
		os.Exit(1)
	}
}

// formatVersion appends what the build stamped: commit, dirty, date.
func formatVersion(product string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return product
	}
	var rev, ts string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.time":
			ts = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	return format(product, rev, ts, dirty)
}

// format is the pure rendering, tested directly; unstamped builds
// fall back to the plain product version.
func format(product, revision, timestamp string, dirty bool) string {
	if revision == "" {
		return product
	}
	short := revision
	if len(short) > 7 {
		short = short[:7]
	}
	out := product + " (" + short
	if dirty {
		out += ", dirty"
	}
	if len(timestamp) >= 10 {
		out += ", " + timestamp[:10]
	}
	return out + ")"
}
