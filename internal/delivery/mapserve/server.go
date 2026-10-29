// Package mapserve serves the project's domain overview to a browser:
// the page that draws the recorded Ubiquitous Language, embedded at
// build time, the overview as the JSON renderer spells it, the overview
// the default branch records, the define and remove use cases the page
// publishes its drafts through, and the projects the page lists,
// creates, opens, updates and removes.
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
	"path/filepath"
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
	ProjectsPath = "/api/projects"
	OpenPath     = "/api/projects/open"
	UpdatePath   = "/api/projects/update"
	ForgetPath   = "/api/projects/remove"
)

// requestLimit bounds one request's body; a recorded text is at most a
// few hundred words.
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

// Project is one project the page serves: the directory that holds its
// ruleset, and the use cases over its domain file.
type Project struct {
	Directory string
	Overview  application.GetDomainOverview
	Define    application.DefineDomainDefinition
	Remove    application.RemoveDomainDefinition
	Accepted  Accepted
}

// Projects are the use cases over the projects the page lists, and
// Serve, which composes the use cases a project is served with.
type Projects struct {
	List   application.ListProjects
	Create application.CreateProject
	Open   application.OpenProject
	Update application.UpdateProject
	Remove application.RemoveProject
	Serve  func(dir string) (Project, error)
}

// Server implements the CLI's MapServer port over HTTP. It serves
// Project until the page creates or opens another project, and serves
// that one from then on.
type Server struct {
	Reports  Reports
	Project  Project
	Projects Projects
}

// Serve binds address, tells listening the URL a browser opens, and
// answers requests until the listener fails. An error from listening
// stops the server before it answers anything. The project served at
// start is remembered among the listed projects; when it cannot be,
// the list says why.
func (s Server) Serve(address string, listening func(url string) error) error {
	served, err := s.studio()
	if err != nil {
		return err
	}
	if _, err := s.Projects.Open.Execute(served.project.Directory); err != nil {
		served.unremembered = err.Error()
	}
	handler := served.routes()
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
// GET /api/project with the directory of the project served, GET
// /api/overview with the overview its domain file records now, GET
// /api/accepted with the one the default branch records, POST
// /api/define and POST /api/remove with one edit each through the
// define and remove use cases, GET /api/projects with the listed
// projects, and POST /api/projects, /api/projects/open,
// /api/projects/update and /api/projects/remove to create, open, update
// and remove one; any other request, and any request addressed by
// another name or sent by another page, is refused.
func (s Server) Handler() (http.Handler, error) {
	served, err := s.studio()
	if err != nil {
		return nil, err
	}
	return served.routes(), nil
}

func (s Server) studio() (*studio, error) {
	switch {
	case s.Reports.Overview == nil || s.Reports.Defined == nil || s.Reports.Removed == nil:
		return nil, errors.New("map server: not composed with the report writers")
	case s.Projects.List == (application.ListProjects{}) || s.Projects.Create == (application.CreateProject{}) ||
		s.Projects.Open == (application.OpenProject{}) || s.Projects.Update == (application.UpdateProject{}) ||
		s.Projects.Remove == (application.RemoveProject{}) || s.Projects.Serve == nil:
		return nil, errors.New("map server: not composed with the project use cases")
	}
	if err := s.Project.composed(); err != nil {
		return nil, err
	}
	return &studio{reports: s.Reports, projects: s.Projects, project: s.Project}, nil
}

func (s *studio) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+ProjectPath, s.alone(s.directory))
	mux.HandleFunc("GET "+OverviewPath, s.alone(s.overview))
	mux.HandleFunc("GET "+AcceptedPath, s.alone(s.accepted))
	mux.HandleFunc("POST "+DefinePath, s.alone(s.define))
	mux.HandleFunc("POST "+RemovePath, s.alone(s.remove))
	mux.HandleFunc("GET "+ProjectsPath, s.alone(s.listProjects))
	mux.HandleFunc("POST "+ProjectsPath, s.alone(s.createProject))
	mux.HandleFunc("POST "+OpenPath, s.alone(s.openProject))
	mux.HandleFunc("POST "+UpdatePath, s.alone(s.updateProject))
	mux.HandleFunc("POST "+ForgetPath, s.alone(s.removeProject))
	mux.HandleFunc("GET "+ScriptPath, serveBytes("text/javascript; charset=utf-8", studioScript))
	mux.HandleFunc("GET /{$}", serveBytes("text/html; charset=utf-8", studioPage))
	return fromThePage(mux)
}

func (p Project) composed() error {
	switch {
	case p.Directory == "":
		return errors.New("map server: the project is served without its directory")
	case p.Overview == (application.GetDomainOverview{}):
		return errors.New("map server: not composed with the domain overview")
	case p.Define == (application.DefineDomainDefinition{}) || p.Remove == (application.RemoveDomainDefinition{}):
		return errors.New("map server: not composed with the define and remove use cases")
	case p.Accepted.Branch != "" && p.Accepted.Overview == (application.GetDomainOverview{}):
		return errors.New("map server: the accepted branch is named without its overview")
	}
	return nil
}

// studio is the state the page changes: the project served, which a
// created or opened project replaces, and why the project served at
// start is not remembered, when it is not.
type studio struct {
	reports      Reports
	projects     Projects
	mu           sync.Mutex
	project      Project
	unremembered string
}

// alone answers each request by itself. Define and remove each read the
// domain file, change it and write it whole, so two edits at once would
// both succeed and the later write would drop the earlier one; and no
// request reads a project while a created one replaces it.
func (s *studio) alone(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
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

// projectPlace is where the project served is, and where a new one is
// suggested: beside it. A project opened or removed is named by its
// directory alone, and Deleted says whether its files went too.
type projectPlace struct {
	Directory string `json:"directory"`
	Beside    string `json:"beside,omitempty"`
	Deleted   bool   `json:"deleted,omitempty"`
}

// projectCreated is a created project: where it is, the name its domain
// file records, its ruleset and its domain file.
type projectCreated struct {
	Directory string `json:"directory"`
	Project   string `json:"project"`
	Ruleset   string `json:"ruleset"`
	Source    string `json:"source"`
}

// directory names the project served and the directory a new project
// is suggested in: beside it.
func (s *studio) directory(w http.ResponseWriter, _ *http.Request) {
	beside := filepath.Dir(s.project.Directory)
	if !strings.HasSuffix(beside, string(filepath.Separator)) {
		beside += string(filepath.Separator)
	}
	body, _ := json.Marshal(projectPlace{Directory: s.project.Directory, Beside: beside})
	writeJSON(w, http.StatusOK, body)
}

// overview is the document the domain file records at this request;
// it changes with every published draft.
func (s *studio) overview(w http.ResponseWriter, _ *http.Request) {
	result, err := s.project.Overview.Execute()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	var document bytes.Buffer
	if err := s.reports.Overview(&document, result); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("render domain overview: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, document.Bytes())
}

// accepted is the document the default branch records, wrapped with the
// branch it was read from, or the reason there is none.
func (s *studio) accepted(w http.ResponseWriter, _ *http.Request) {
	accepted := s.project.Accepted
	if accepted.Branch == "" {
		body, _ := json.Marshal(map[string]any{"available": false, "reason": accepted.Unavailable})
		writeJSON(w, http.StatusOK, body)
		return
	}
	result, err := accepted.Overview.Execute()
	if err != nil {
		body, _ := json.Marshal(map[string]any{"available": false, "branch": accepted.Branch, "reason": err.Error()})
		writeJSON(w, http.StatusOK, body)
		return
	}
	var document bytes.Buffer
	if err := s.reports.Overview(&document, result); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("render accepted domain overview: %w", err))
		return
	}
	branch, _ := json.Marshal(accepted.Branch)
	var body bytes.Buffer
	body.WriteString(`{"available":true,"branch":`)
	body.Write(branch)
	body.WriteString(`,"overview":`)
	body.Write(bytes.TrimSpace(document.Bytes()))
	body.WriteString("}\n")
	writeJSON(w, http.StatusOK, body.Bytes())
}

// defineRequest is one edit the page publishes: the project it was
// drafted in, the entry as `arclint domain define <concept> <name>`
// names it, and the properties to set. A property present in change is
// set, even to an empty value; one absent stays as recorded, as the
// define flags behave.
type defineRequest struct {
	Project string        `json:"project"`
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

// removeRequest names the project and the entry to remove as `arclint
// domain remove <concept> <name>` names it.
type removeRequest struct {
	Project string `json:"project"`
	Concept string `json:"concept"`
	Context string `json:"context"`
	Owner   string `json:"owner"`
	Name    string `json:"name"`
}

func (s *studio) define(w http.ResponseWriter, r *http.Request) {
	var body defineRequest
	if !readJSON(w, r, &body) || !s.serves(w, body.Project) {
		return
	}
	req := application.DefineDomainRequest{Concept: body.Concept, Context: body.Context, Owner: body.Owner, Name: body.Name, Change: body.Change.change()}
	result, err := s.project.Define.Execute(req)
	if err != nil {
		writeError(w, refusalStatus(err), err)
		return
	}
	var document bytes.Buffer
	if err := s.reports.Defined(&document, result, req); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("render define result: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, document.Bytes())
}

func (s *studio) remove(w http.ResponseWriter, r *http.Request) {
	var body removeRequest
	if !readJSON(w, r, &body) || !s.serves(w, body.Project) {
		return
	}
	result, err := s.project.Remove.Execute(body.Concept, body.Context, body.Owner, body.Name)
	if err != nil {
		writeError(w, refusalStatus(err), err)
		return
	}
	var document bytes.Buffer
	if err := s.reports.Removed(&document, result); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("render remove result: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, document.Bytes())
}

// serves refuses an edit drafted in another project than the one served
// now: a page loaded before a project was created publishes nothing into
// the new one.
func (s *studio) serves(w http.ResponseWriter, project string) bool {
	if project == s.project.Directory {
		return true
	}
	writeError(w, http.StatusConflict, fmt.Errorf("this edit was drafted in %q, and the map now serves %s; reload the page", project, s.project.Directory))
	return false
}

// projectRequest is a new project as the page asks for it.
type projectRequest struct {
	Directory string   `json:"directory"`
	Name      string   `json:"name"`
	Languages []string `json:"languages"`
}

// createProject creates the project and serves it from then on; a
// refused project leaves the one served as it was.
func (s *studio) createProject(w http.ResponseWriter, r *http.Request) {
	var body projectRequest
	if !readJSON(w, r, &body) {
		return
	}
	result, err := s.projects.Create.Execute(application.CreateProjectRequest{Directory: body.Directory, Name: body.Name, Languages: body.Languages})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if err := s.serve(result.Directory); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	created, _ := json.Marshal(projectCreated{Directory: result.Directory, Project: result.Project, Ruleset: result.Ruleset, Source: result.Source})
	writeJSON(w, http.StatusCreated, created)
}

// serve composes the project at dir and serves it from then on; a
// project that cannot be composed leaves the one served as it was.
func (s *studio) serve(dir string) error {
	project, err := s.projects.Serve(dir)
	if err == nil {
		err = project.composed()
	}
	if err != nil {
		return fmt.Errorf("serve %s: %w", dir, err)
	}
	s.project = project
	return nil
}

// projectSummary is one listed project as the page reads it.
type projectSummary struct {
	Directory   string `json:"directory"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Problem     string `json:"problem,omitempty"`
	Served      bool   `json:"served"`
}

func (s *studio) summaryOf(p application.ProjectSummary) projectSummary {
	return projectSummary{Directory: p.Directory, Name: p.Name, Description: p.Description, Problem: p.Problem, Served: p.Directory == s.project.Directory}
}

// listProjects lists the project served, then the remembered ones; a
// list that cannot be read in full says why beside what it could read.
func (s *studio) listProjects(w http.ResponseWriter, _ *http.Request) {
	listed, err := s.projects.List.Execute(s.project.Directory)
	projects := make([]projectSummary, 0, len(listed))
	for _, p := range listed {
		projects = append(projects, s.summaryOf(p))
	}
	problem := s.unremembered
	if err != nil {
		problem = err.Error()
	}
	body, _ := json.Marshal(map[string]any{"projects": projects, "problem": problem})
	writeJSON(w, http.StatusOK, body)
}

// openRequest names a path inside the project to open.
type openRequest struct {
	Directory string `json:"directory"`
}

// openProject serves the project a path lies in from then on.
func (s *studio) openProject(w http.ResponseWriter, r *http.Request) {
	var body openRequest
	if !readJSON(w, r, &body) {
		return
	}
	dir, err := s.projects.Open.Execute(body.Directory)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if err := s.serve(dir); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	opened, _ := json.Marshal(projectPlace{Directory: dir})
	writeJSON(w, http.StatusOK, opened)
}

// updateRequest is a project's name and description as they should be
// recorded.
type updateRequest struct {
	Directory   string `json:"directory"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// updateProject records a listed project's name and description in its
// domain file.
func (s *studio) updateProject(w http.ResponseWriter, r *http.Request) {
	var body updateRequest
	if !readJSON(w, r, &body) {
		return
	}
	if !s.lists(w, body.Directory) {
		return
	}
	updated, err := s.projects.Update.Execute(application.UpdateProjectRequest{Directory: body.Directory, Name: body.Name, Description: body.Description})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	summary, _ := json.Marshal(s.summaryOf(updated))
	writeJSON(w, http.StatusOK, summary)
}

// removeProjectRequest names the project to take off the list and whether to
// delete its files, confirmed by its directory's name.
type removeProjectRequest struct {
	Directory    string `json:"directory"`
	DeleteFiles  bool   `json:"deleteFiles"`
	Confirmation string `json:"confirmation"`
}

// removeProject takes a project off the list, deleting its files when
// asked and confirmed. The project served is never removed: the page
// opens another first.
func (s *studio) removeProject(w http.ResponseWriter, r *http.Request) {
	var body removeProjectRequest
	if !readJSON(w, r, &body) {
		return
	}
	if body.Directory == s.project.Directory {
		writeError(w, http.StatusConflict, fmt.Errorf("%s is the project the map serves; open another project before removing it", body.Directory))
		return
	}
	if err := s.projects.Remove.Execute(application.RemoveProjectRequest{Directory: body.Directory, DeleteFiles: body.DeleteFiles, Confirmation: body.Confirmation}); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
		return
	}
	removed, _ := json.Marshal(projectPlace{Directory: body.Directory, Deleted: body.DeleteFiles})
	writeJSON(w, http.StatusOK, removed)
}

// lists refuses a request naming a project the page does not list: the
// one served or a remembered one.
func (s *studio) lists(w http.ResponseWriter, dir string) bool {
	if dir == s.project.Directory {
		return true
	}
	listed, err := s.projects.List.Execute(s.project.Directory)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return false
	}
	for _, p := range listed {
		if p.Directory == dir {
			return true
		}
	}
	writeError(w, http.StatusUnprocessableEntity, fmt.Errorf("%s is not on the list of projects", dir))
	return false
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

// readJSON decodes one request. The body must declare itself JSON,
// which a cross-site form cannot, and must spell only the fields the
// request has.
func readJSON(w http.ResponseWriter, r *http.Request, into any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, errors.New("a request is a JSON body: Content-Type application/json"))
		return false
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, requestLimit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("read the request: %w", err))
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
