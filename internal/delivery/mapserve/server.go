// Package mapserve serves the project's domain overview to a browser:
// the page that draws the recorded Ubiquitous Language, embedded at
// build time, and the overview itself as the JSON document the JSON
// renderer spells. Display only: no request changes anything.
package mapserve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/wixregiga/arclint/internal/application"
)

// Routes of the page and its document.
const (
	PagePath     = "/"
	ScriptPath   = "/studio.js"
	OverviewPath = "/api/overview"
)

// Server implements the CLI's MapServer port over HTTP.
type Server struct {
	// Overview writes the domain overview the page loads, as JSON.
	// Composition lends the JSON renderer, so the page reads the same
	// document `arclint domain overview --format json` prints.
	Overview func(io.Writer, application.DomainOverview) error
}

// Serve binds address, tells listening the URL a browser opens, and
// answers requests until the listener fails. An error from listening
// stops the server before it answers anything.
func (s Server) Serve(address string, overview application.DomainOverview, listening func(url string) error) error {
	if s.Overview == nil {
		return fmt.Errorf("map server: not composed with an overview writer")
	}
	handler, err := s.Handler(overview)
	if err != nil {
		return err
	}
	var config net.ListenConfig
	listener, err := config.Listen(context.Background(), "tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	defer func() { _ = listener.Close() }()
	if listening != nil {
		if err := listening(URL(listener.Addr())); err != nil {
			return err
		}
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	if err := server.Serve(listener); err != nil {
		return fmt.Errorf("serve on %s: %w", listener.Addr(), err)
	}
	return nil
}

// Handler answers GET / with the page, GET /studio.js with its script,
// and GET /api/overview with the overview document; any other method
// is refused. The document is rendered once, when the handler is
// built, because the overview does not change while the server runs.
func (s Server) Handler(overview application.DomainOverview) (http.Handler, error) {
	if s.Overview == nil {
		return nil, fmt.Errorf("map server: not composed with an overview writer")
	}
	var document bytes.Buffer
	if err := s.Overview(&document, overview); err != nil {
		return nil, fmt.Errorf("render domain overview: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+OverviewPath, serveBytes("application/json; charset=utf-8", document.Bytes()))
	mux.HandleFunc("GET "+ScriptPath, serveBytes("text/javascript; charset=utf-8", studioScript))
	mux.HandleFunc("GET /{$}", serveBytes("text/html; charset=utf-8", studioPage))
	return mux, nil
}

func serveBytes(contentType string, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	}
}

// URL is the address a browser opens for a bound listener: the host it
// listens on with its port, and localhost when it listens on every
// interface, so the printed URL always reaches this machine.
func URL(bound net.Addr) string {
	tcp, ok := bound.(*net.TCPAddr)
	if !ok {
		return "http://" + bound.String()
	}
	host := "localhost"
	if tcp.IP != nil && !tcp.IP.IsUnspecified() {
		host = tcp.IP.String()
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(tcp.Port))
}
