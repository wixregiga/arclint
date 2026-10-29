// Package mapserve serves the project's domain overview to a browser:
// the page that draws the recorded Ubiquitous Language, embedded at
// build time, the overview as the JSON renderer spells it, the overview
// the default branch records, and the define and remove use cases the
// page publishes its drafts through.
package mapserve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// Routes of the page, its documents and its edits.
const (
	PagePath     = "/"
	ScriptPath   = "/studio.js"
	OverviewPath = "/api/overview"
	AcceptedPath = "/api/accepted"
	DefinePath   = "/api/define"
	RemovePath   = "/api/remove"
	ProjectPath  = "/api/project"
)

// requestLimit bounds one edit's body; a recorded text is at most a few
// hundred words.
const requestLimit = 1 << 20

// Reports writes the use cases' results as the JSON renderer spells
// them. Composition lends the renderer, so the page reads the same
// documents `arclint domain overview --format json`, `domain define
// --format json` and `domain remove --format json` print.
type Reports struct {
	Overview func(io.Writer, application.DomainOverview) error
	Defined  func(io.Writer, application.DomainDefineResult, application.DefineDomainRequest) error
	Removed  func(io.Writer, application.DomainRemoveResult) error
}

// Accepted is the recording the remote's default branch holds: the
// language the project has agreed on. Branch names it, such as
// origin/main; Unavailable says why there is none when Branch is empty.
type Accepted struct {
	Branch      string
	Overview    application.GetDomainOverview
	Unavailable string
}

// Server implements the CLI's MapServer port over HTTP.
type Server struct {
	Reports  Reports
	Overview application.GetDomainOverview
	Define   application.DefineDomainDefinition
	Remove   application.RemoveDomainDefinition
	Accepted Accepted
}

// Serve binds address, tells listening the URL a browser opens, and
// answers requests until the listener fails. An error from listening
// stops the server before it answers anything.
func (s Server) Serve(address string, listening func(url string) error) error {
	handler, err := s.Handler()
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
// GET /api/overview with the overview the domain file records now, GET
// /api/accepted with the one the default branch records, POST
// /api/project, POST /api/define and POST /api/remove with one edit each through the
// define and remove use cases, one edit at a time; any other request,
// and any request addressed by another name or sent by another page, is
// refused.
func (s Server) Handler() (http.Handler, error) {
	switch {
	case s.Reports.Overview == nil || s.Reports.Defined == nil || s.Reports.Removed == nil:
		return nil, errors.New("map server: not composed with the report writers")
	case s.Overview == (application.GetDomainOverview{}):
		return nil, errors.New("map server: not composed with the domain overview")
	case s.Define == (application.DefineDomainDefinition{}) || s.Remove == (application.RemoveDomainDefinition{}):
		return nil, errors.New("map server: not composed with the define and remove use cases")
	case s.Accepted.Branch != "" && s.Accepted.Overview == (application.GetDomainOverview{}):
		return nil, errors.New("map server: the accepted branch is named without its overview")
	}
	var edits sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+OverviewPath, s.overview)
	mux.HandleFunc("GET "+AcceptedPath, s.accepted)
	mux.HandleFunc("POST "+ProjectPath, oneAtATime(&edits, s.project))
	mux.HandleFunc("POST "+DefinePath, oneAtATime(&edits, s.define))
	mux.HandleFunc("POST "+RemovePath, oneAtATime(&edits, s.remove))
	mux.HandleFunc("GET "+ScriptPath, serveBytes("text/javascript; charset=utf-8", studioScript))
	mux.HandleFunc("GET /{$}", serveBytes("text/html; charset=utf-8", studioPage))
	return fromThePage(mux), nil
}

// oneAtATime runs each edit alone. Define and remove each read the
// domain file, change it and write it whole, so two edits at once would
// both succeed and the later write would drop the earlier one.
func oneAtATime(edits *sync.Mutex, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		edits.Lock()
		defer edits.Unlock()
		next(w, r)
	}
}

// fromThePage answers only requests addressed to this server directly,
// by an IP address or localhost, and sent by its own page. A page that
// points its own DNS name at this machine reaches the server under that
// name, so a request naming any other host is refused; a request whose
// Origin is another host is refused too, so no other page publishes an
// edit.
func fromThePage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !addressedDirectly(r.Host) {
			writeError(w, http.StatusMisdirectedRequest, fmt.Errorf("the map answers at an IP address or localhost, not at %q", r.Host))
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
				writeError(w, http.StatusForbidden, fmt.Errorf("a request from %s is not from the map's own page", origin))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func addressedDirectly(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	return strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil
}

// overview is the document the domain file records at this request;
// it changes with every published draft.
func (s Server) overview(w http.ResponseWriter, _ *http.Request) {
	result, err := s.Overview.Execute()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var document bytes.Buffer
	if err := s.Reports.Overview(&document, result); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("render domain overview: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, document.Bytes())
}

// accepted is the document the default branch records, wrapped with the
// branch it was read from, or the reason there is none.
func (s Server) accepted(w http.ResponseWriter, _ *http.Request) {
	if s.Accepted.Branch == "" {
		body, _ := json.Marshal(map[string]any{"available": false, "reason": s.Accepted.Unavailable})
		writeJSON(w, http.StatusOK, body)
		return
	}
	result, err := s.Accepted.Overview.Execute()
	if err != nil {
		body, _ := json.Marshal(map[string]any{"available": false, "branch": s.Accepted.Branch, "reason": err.Error()})
		writeJSON(w, http.StatusOK, body)
		return
	}
	var document bytes.Buffer
	if err := s.Reports.Overview(&document, result); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("render accepted domain overview: %w", err))
		return
	}
	branch, _ := json.Marshal(s.Accepted.Branch)
	var body bytes.Buffer
	body.WriteString(`{"available":true,"branch":`)
	body.Write(branch)
	body.WriteString(`,"overview":`)
	body.Write(bytes.TrimSpace(document.Bytes()))
	body.WriteString("}\n")
	writeJSON(w, http.StatusOK, body.Bytes())
}

// defineRequest is one edit the page publishes: the entry as `arclint
// domain define <concept> <name>` names it, and the properties to set.
// A property present in change is set, even to an empty value; one
// absent stays as recorded, as the define flags behave.
type defineRequest struct {
	Concept string        `json:"concept"`
	Context string        `json:"context"`
	Owner   string        `json:"owner"`
	Name    string        `json:"name"`
	Change  requestChange `json:"change"`
}

type requestChange struct {
	Definition *string   `json:"definition"`
	Statement  *string   `json:"statement"`
	On         *string   `json:"on"`
	VerifiedBy *[]string `json:"verifiedBy"`
	Text       *string   `json:"text"`
	Identity   *string   `json:"identity"`
	Aliases    *[]string `json:"aliases"`
	Repository *string   `json:"repository"`
	Factory    *string   `json:"factory"`
	RaisedBy   *string   `json:"raisedBy"`
}

func (c requestChange) change() vocab.Change {
	ch := vocab.Change{}
	if c.Definition != nil {
		ch.SetDefinition, ch.Definition = true, *c.Definition
	}
	if c.Statement != nil {
		ch.SetStatement, ch.Statement = true, *c.Statement
	}
	if c.On != nil {
		ch.SetOn, ch.On = true, *c.On
	}
	if c.VerifiedBy != nil {
		ch.SetVerifiedBy, ch.VerifiedBy = true, *c.VerifiedBy
	}
	if c.Text != nil {
		ch.SetText, ch.Text = true, *c.Text
	}
	if c.Identity != nil {
		ch.SetIdentity, ch.Identity = true, *c.Identity
	}
	if c.Aliases != nil {
		ch.SetAliases, ch.Aliases = true, *c.Aliases
	}
	if c.Repository != nil {
		ch.SetRepository, ch.Repository = true, *c.Repository
	}
	if c.Factory != nil {
		ch.SetFactory, ch.Factory = true, *c.Factory
	}
	if c.RaisedBy != nil {
		ch.SetRaisedBy, ch.RaisedBy = true, *c.RaisedBy
	}
	return ch
}

// removeRequest names the entry to remove as `arclint domain remove
// <concept> <name>` names it.
type removeRequest struct {
	Concept string `json:"concept"`
	Context string `json:"context"`
	Owner   string `json:"owner"`
	Name    string `json:"name"`
}

func (s Server) define(w http.ResponseWriter, r *http.Request) {
	var body defineRequest
	if !readJSON(w, r, &body) {
		return
	}
	req := application.DefineDomainRequest{Concept: body.Concept, Context: body.Context, Owner: body.Owner, Name: body.Name, Change: body.Change.change()}
	result, err := s.Define.Execute(req)
	if err != nil {
		writeError(w, refusalStatus(err), err)
		return
	}
	var document bytes.Buffer
	if err := s.Reports.Defined(&document, result, req); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("render define result: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, document.Bytes())
}

// projectRequest preserves the distinction between omitted and cleared fields.
type projectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (s Server) project(w http.ResponseWriter, r *http.Request) {
	var body projectRequest
	if !readJSON(w, r, &body) {
		return
	}
	req := application.DefineProjectRequest{}
	if body.Name != nil {
		req.SetName, req.Name = true, *body.Name
	}
	if body.Description != nil {
		req.SetDescription, req.Description = true, *body.Description
	}
	result, err := s.Define.ExecuteProject(req)
	if err != nil {
		writeError(w, refusalStatus(err), err)
		return
	}
	var document bytes.Buffer
	if err := s.Reports.Defined(&document, result, application.DefineDomainRequest{Concept: "project", Name: result.Name}); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("render define result: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, document.Bytes())
}

func (s Server) remove(w http.ResponseWriter, r *http.Request) {
	var body removeRequest
	if !readJSON(w, r, &body) {
		return
	}
	result, err := s.Remove.Execute(body.Concept, body.Context, body.Owner, body.Name)
	if err != nil {
		writeError(w, refusalStatus(err), err)
		return
	}
	var document bytes.Buffer
	if err := s.Reports.Removed(&document, result); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("render remove result: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, document.Bytes())
}

// refusalStatus maps a refused edit as the CLI maps it: a request that
// cannot be carried out as spelled is the client's error; a change the
// language refuses, or an entry that is not recorded, is unprocessable.
func refusalStatus(err error) int {
	if errors.Is(err, application.ErrDomainUsage) {
		return http.StatusBadRequest
	}
	return http.StatusUnprocessableEntity
}

// readJSON decodes one edit. The body must declare itself JSON, which a
// cross-site form cannot, and must spell only the fields an edit has.
func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, errors.New("an edit is a JSON body: Content-Type application/json"))
		return false
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, requestLimit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("read the edit: %w", err))
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, status int, err error) {
	body, _ := json.Marshal(map[string]string{"error": err.Error()})
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
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
