// Package app is the FSD app layer of the server: the chi router,
// the handlers and DTOs, the organizer gate, and request logging.
// Technology lives here so features and entities stay clean.
package app

import (
	"boxoffice/internal/entities/capacity"
	"boxoffice/internal/entities/event"
	"boxoffice/internal/entities/order"
	"boxoffice/internal/features/cancelevent"
	"boxoffice/internal/features/editevent"
	"boxoffice/internal/features/holdseats"
	"boxoffice/internal/features/placeorder"
	"boxoffice/internal/features/publishevent"
	"boxoffice/internal/features/refundticket"
	"boxoffice/internal/shared/httpkit"
	"crypto/subtle"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// Config carries the app's choices.
type Config struct {
	// OrganizerToken unlocks the organizer's side of the counter;
	// empty keeps it locked.
	OrganizerToken string
	// HoldFor is how long a hold lasts; zero means five minutes.
	HoldFor time.Duration
	// Now is the clock; nil means time.Now.
	Now func() time.Time
	// WebFS, when set, is served as the web app: the embedded build
	// or any directory the composition root chose.
	WebFS fs.FS
}

// Deps are the repositories the app wires the features with.
type Deps struct {
	Events     event.Repository
	Orders     order.Repository
	Capacities capacity.Repository
}

type server struct {
	cfg     Config
	deps    Deps
	edit    editevent.Feature
	publish publishevent.Feature
	cancel  cancelevent.Feature
	hold    holdseats.Feature
	place   placeorder.Feature
	refund  refundticket.Feature
}

// New assembles the router.
func New(cfg Config, deps Deps) http.Handler {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.HoldFor <= 0 {
		cfg.HoldFor = 5 * time.Minute
	}
	s := &server{
		cfg:     cfg,
		deps:    deps,
		edit:    editevent.Feature{Events: deps.Events, Capacities: deps.Capacities},
		publish: publishevent.Feature{Events: deps.Events},
		cancel:  cancelevent.Feature{Events: deps.Events, Orders: deps.Orders, Capacities: deps.Capacities},
		hold:    holdseats.Feature{Events: deps.Events, Capacities: deps.Capacities, HoldFor: cfg.HoldFor},
		place:   placeorder.Feature{Events: deps.Events, Orders: deps.Orders, Capacities: deps.Capacities},
		refund:  refundticket.Feature{Orders: deps.Orders, Capacities: deps.Capacities},
	}

	r := chi.NewRouter()
	r.Use(requestLog)
	r.Route("/api", func(r chi.Router) {
		r.Get("/events", s.listPublished)
		r.Get("/events/{eventID}", s.getEvent)
		r.Post("/events/{eventID}/holds", s.createHold)
		r.Post("/orders", s.placeOrder)
		r.Get("/orders/{orderID}", s.getOrder)
		r.Route("/organizer", func(r chi.Router) {
			r.Use(s.requireOrganizer)
			r.Get("/events", s.listAll)
			r.Post("/events", s.createEvent)
			r.Get("/events/{eventID}", s.organizerEvent)
			r.Put("/events/{eventID}", s.editEvent)
			r.Post("/events/{eventID}/publish", s.publishEvent)
			r.Post("/events/{eventID}/cancel", s.cancelEvent)
			r.Post("/orders/{orderID}/refunds", s.refundTicket)
		})
	})
	if cfg.WebFS != nil {
		r.Handle("/*", spa(cfg.WebFS))
	} else {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			httpkit.Respond(w, http.StatusOK, map[string]string{"service": "boxoffice"})
		})
	}
	return r
}

// spa serves the built web app: real files as they are, and the
// index shell for client-routed paths like /events/jazz-trio.
func spa(dist fs.FS) http.Handler {
	files := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := dist.Open(path); err == nil {
				if cerr := f.Close(); cerr != nil {
					slog.Error("closing served file", "error", cerr)
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFileFS(w, r, dist, "index.html")
	})
}

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "took", time.Since(start))
	})
}

// isOrganizer checks the bearer token; an empty configured token
// matches nothing, so the organizer side stays locked.
func (s *server) isOrganizer(r *http.Request) bool {
	if s.cfg.OrganizerToken == "" {
		return false
	}
	got := r.Header.Get("Authorization")
	want := "Bearer " + s.cfg.OrganizerToken
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (s *server) requireOrganizer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.isOrganizer(r) {
			httpkit.Error(w, http.StatusUnauthorized, "the organizer's side of the counter needs the organizer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}
