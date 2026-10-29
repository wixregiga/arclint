package mapserve_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/reportfactory"
	"github.com/wixregiga/arclint/internal/delivery/mapserve"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	gitproject "github.com/wixregiga/arclint/internal/infrastructure/project/git"
	filesystemscaffold "github.com/wixregiga/arclint/internal/infrastructure/scaffold/filesystem"
	gitvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/git"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

// exampleRoot is the Go example's project, a real recording the
// documentation quotes.
func exampleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "docs", "examples", "go"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// scratchClone copies the Go example into a repository, commits it on
// main, and clones that, so the clone has a remote default branch
// (origin/HEAD) holding the example's domain file. Returns the clone.
func scratchClone(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	base := t.TempDir()
	upstream := filepath.Join(base, "upstream")
	if err := os.MkdirAll(upstream, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"domain.arclint.yaml", "rules.arclint.yaml", "go.mod"} {
		data, err := os.ReadFile(filepath.Join(exampleRoot(t), name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(upstream, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git(t, upstream, "init", "-q", "-b", "main")
	git(t, upstream, "add", ".")
	git(t, upstream, "commit", "-q", "-m", "the accepted recording")
	clone := filepath.Join(base, "clone")
	git(t, base, "clone", "-q", upstream, clone)
	return clone
}

type parser struct{}

func (parser) ParseUbiquitousLanguage(content []byte, recording vocab.DomainRecording) (vocab.UbiquitousLanguage, error) {
	return yamlvocab.Parse(content, recording)
}

// composed is the server as the composition root builds it over a
// project root: the YAML repository reads and writes the domain file,
// the JSON renderer spells every document, and git reads the branch.
func composed(t *testing.T, root string) mapserve.Server {
	t.Helper()
	recording, err := vocab.NewDomainRecording(vocab.DefaultDomainRecordingPath)
	if err != nil {
		t.Fatal(err)
	}
	knowledge, err := yamlvocab.NewRepository(root, recording)
	if err != nil {
		t.Fatal(err)
	}
	overview, err := application.NewGetDomainOverview(knowledge)
	if err != nil {
		t.Fatal(err)
	}
	define, err := application.NewDefineDomainDefinition(knowledge, filepath.Base(root))
	if err != nil {
		t.Fatal(err)
	}
	remove, err := application.NewRemoveDomainDefinition(knowledge)
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := reportfactory.Select(cli.RendererJSON)
	if err != nil {
		t.Fatal(err)
	}
	server := mapserve.Server{
		Reports: mapserve.Reports{
			Overview: func(w io.Writer, o application.DomainOverview) error { return renderer.Render(w, cli.DomainOverviewReport{Overview: o}) },
			Defined: func(w io.Writer, r application.DomainDefineResult, req application.DefineDomainRequest) error {
				return renderer.Render(w, cli.NewDomainDefineReport(r, req))
			},
			Removed: func(w io.Writer, r application.DomainRemoveResult) error { return renderer.Render(w, cli.DomainRemoveReport{Result: r}) },
		},
		Project: mapserve.Project{Directory: root, Overview: overview, Define: define, Remove: remove},
		Create: func(req application.CreateProjectRequest) (application.CreateProjectResult, mapserve.Project, error) {
			create, err := application.NewCreateProject(projectDirectories{})
			if err != nil {
				return application.CreateProjectResult{}, mapserve.Project{}, err
			}
			result, err := create.Execute(req)
			if err != nil {
				return application.CreateProjectResult{}, mapserve.Project{}, err
			}
			return result, composed(t, result.Directory).Project, nil
		},
	}
	accepted, err := gitvocab.NewRepository(root, recording, parser{})
	if err != nil {
		server.Project.Accepted = mapserve.Accepted{Unavailable: err.Error()}
		return server
	}
	acceptedOverview, err := application.NewGetDomainOverview(accepted)
	if err != nil {
		t.Fatal(err)
	}
	server.Project.Accepted = mapserve.Accepted{Branch: accepted.Branch(), Overview: acceptedOverview}
	return server
}

// projectDirectories makes a project's directory through git and
// writes its files as the composition root does, with the default
// domain file.
type projectDirectories struct{}

func (projectDirectories) Make(dir string) (bool, error) { return gitproject.Make(dir) }

func (projectDirectories) Discard(dir string, made bool) error { return gitproject.Discard(dir, made) }

func (projectDirectories) Ruleset(dir string) (application.RulesetScaffold, error) {
	return filesystemscaffold.NewWriter(dir)
}

func (projectDirectories) DomainFile(dir string) (vocab.Repository, error) {
	recording, err := vocab.NewDomainRecording(vocab.DefaultDomainRecordingPath)
	if err != nil {
		return nil, err
	}
	return yamlvocab.NewRepository(dir, recording)
}

// inProject is an edit's body as the page sends it: drafted in the
// project at root.
func inProject(root, body string) string {
	project, _ := json.Marshal(root)
	return `{"project":` + string(project) + `,` + strings.TrimPrefix(body, "{")
}

// pageHost is the host a browser names when it opens the URL the
// server prints.
const pageHost = "127.0.0.1:7380"

func handler(t *testing.T, root string) http.Handler {
	t.Helper()
	h, err := composed(t, root).Handler()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func get(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	req.Host = pageHost
	h.ServeHTTP(rec, req)
	return rec
}

func post(t *testing.T, h http.Handler, path, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Host = pageHost
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	h.ServeHTTP(rec, req)
	return rec
}

type overviewDoc struct {
	Found    bool   `json:"found"`
	Project  string `json:"project"`
	Source   string `json:"source"`
	Contexts []struct {
		Name       string `json:"name"`
		Definition string `json:"definition"`
		Aggregates []struct {
			Name     string `json:"name"`
			Identity string `json:"identity"`
		} `json:"aggregates"`
		ValueObjects []struct {
			Name       string `json:"name"`
			Definition string `json:"definition"`
		} `json:"valueObjects"`
	} `json:"contexts"`
	Counts map[string]int `json:"counts"`
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), into); err != nil {
		t.Fatalf("decode: %v\n%s", err, rec.Body.String())
	}
}

func TestOverviewIsTheJSONRenderersDocument(t *testing.T) {
	root := exampleRoot(t)
	rec := get(t, handler(t, root), http.MethodGet, mapserve.OverviewPath)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("cache-control = %q", cc)
	}
	var doc overviewDoc
	decode(t, rec, &doc)
	if !doc.Found || doc.Project != "identityaccess" || doc.Source != vocab.DefaultDomainRecordingPath {
		t.Fatalf("found/project/source = %v/%q/%q", doc.Found, doc.Project, doc.Source)
	}
	if len(doc.Contexts) != 1 || doc.Contexts[0].Name != "identityaccess" || doc.Contexts[0].Definition == "" {
		t.Fatalf("contexts = %+v", doc.Contexts)
	}
	if len(doc.Contexts[0].Aggregates) != doc.Counts["aggregates"] || doc.Contexts[0].Aggregates[0].Identity == "" {
		t.Fatalf("aggregates = %+v, counts %v", doc.Contexts[0].Aggregates, doc.Counts)
	}
}

func TestPageAndScriptAreServed(t *testing.T) {
	h := handler(t, exampleRoot(t))
	page := get(t, h, http.MethodGet, mapserve.PagePath)
	if page.Code != http.StatusOK || !strings.HasPrefix(page.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("page: status %d, content-type %q", page.Code, page.Header().Get("Content-Type"))
	}
	for _, want := range []string{"ArcLint Studio", "Navigator", "Inspector", `src="studio.js"`, mapserve.OverviewPath, mapserve.AcceptedPath, mapserve.DefinePath, mapserve.RemovePath, mapserve.ProjectPath, mapserve.ProjectsPath, "Publish", "Changes", "New project"} {
		if !strings.Contains(page.Body.String(), want) {
			t.Errorf("page lacks %q", want)
		}
	}
	script := get(t, h, http.MethodGet, mapserve.ScriptPath)
	if script.Code != http.StatusOK || !strings.HasPrefix(script.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("script: status %d, content-type %q", script.Code, script.Header().Get("Content-Type"))
	}
	for _, want := range []string{"conceptsOf", "navigator", "layout", "inspect", "transformation", "applyDrafts", "requestOf"} {
		if !strings.Contains(script.Body.String(), want) {
			t.Errorf("script lacks %q", want)
		}
	}
}

func TestOnlyTheRoutesOfThePageAreServed(t *testing.T) {
	h := handler(t, exampleRoot(t))
	if rec := get(t, h, http.MethodPost, mapserve.OverviewPath); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST overview: status = %d", rec.Code)
	}
	if rec := get(t, h, http.MethodGet, mapserve.DefinePath); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET define: status = %d", rec.Code)
	}
	if rec := get(t, h, http.MethodPut, mapserve.PagePath); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT page: status = %d", rec.Code)
	}
	for _, path := range []string{"/studio.test.js", "/static/index.html", "/other", "/api/model"} {
		if rec := get(t, h, http.MethodGet, path); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", path, rec.Code)
		}
	}
}

func TestAnEditMustBeAJSONBody(t *testing.T) {
	h := handler(t, exampleRoot(t))
	body := `{"concept":"value_object","name":"EmailAddress","change":{"definition":"x"}}`
	for _, ct := range []string{"", "text/plain", "application/x-www-form-urlencoded"} {
		if rec := post(t, h, mapserve.DefinePath, ct, body); rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("content-type %q: status = %d, want 415", ct, rec.Code)
		}
	}
	rec := post(t, h, mapserve.DefinePath, "application/json", `{"concept":"value_object","name":"EmailAddress","surprise":1}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "surprise") {
		t.Fatalf("unknown field: status = %d, body %s", rec.Code, rec.Body.String())
	}
	// Nothing above reached the example's domain file.
	data, err := os.ReadFile(filepath.Join(exampleRoot(t), vocab.DefaultDomainRecordingPath))
	if err != nil || !bytes.Contains(data, []byte("A well-formed")) && !bytes.Contains(data, []byte("EmailAddress")) {
		t.Fatalf("the example's domain file changed or cannot be read: %v", err)
	}
}

func TestDefineRecordsThroughTheUseCaseAndTheOverviewFollows(t *testing.T) {
	clone := scratchClone(t)
	h := handler(t, clone)
	var before overviewDoc
	decode(t, get(t, h, http.MethodGet, mapserve.OverviewPath), &before)
	var was string
	for _, v := range before.Contexts[0].ValueObjects {
		if v.Name == "EmailAddress" {
			was = v.Definition
		}
	}
	if was == "" {
		t.Fatal("the example records EmailAddress")
	}

	rec := post(t, h, mapserve.DefinePath, "application/json; charset=utf-8",
		inProject(clone, `{"concept":"value_object","context":"identityaccess","owner":"","name":"EmailAddress","change":{"definition":"A mailbox address the tenant can be reached at."}}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("define: status = %d, body %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Result  string            `json:"result"`
		Type    string            `json:"type"`
		Name    string            `json:"name"`
		Changed []string          `json:"changed"`
		Values  map[string]string `json:"values"`
	}
	decode(t, rec, &result)
	if result.Result != "updated" || result.Type != "value_object" || result.Name != "EmailAddress" || len(result.Changed) != 1 || result.Changed[0] != "definition" || result.Values["definition"] != "A mailbox address the tenant can be reached at." {
		t.Fatalf("define document = %+v", result)
	}

	var after overviewDoc
	decode(t, get(t, h, http.MethodGet, mapserve.OverviewPath), &after)
	var now string
	for _, v := range after.Contexts[0].ValueObjects {
		if v.Name == "EmailAddress" {
			now = v.Definition
		}
	}
	if now != "A mailbox address the tenant can be reached at." {
		t.Fatalf("the overview still records %q", now)
	}
	data, err := os.ReadFile(filepath.Join(clone, vocab.DefaultDomainRecordingPath))
	if err != nil || !bytes.Contains(data, []byte("A mailbox address the tenant can be reached at.")) {
		t.Fatalf("the domain file was not written: %v\n%s", err, data)
	}
	// The branch still records what it accepted.
	var accepted struct {
		Available bool        `json:"available"`
		Branch    string      `json:"branch"`
		Overview  overviewDoc `json:"overview"`
	}
	decode(t, get(t, h, http.MethodGet, mapserve.AcceptedPath), &accepted)
	if !accepted.Available || accepted.Branch != "origin/main" || !accepted.Overview.Found {
		t.Fatalf("accepted = %+v", accepted)
	}
	for _, v := range accepted.Overview.Contexts[0].ValueObjects {
		if v.Name == "EmailAddress" && v.Definition != was {
			t.Fatalf("the accepted recording changed: %q", v.Definition)
		}
	}

	// A creation, then its removal, both as the command line records them.
	rec = post(t, h, mapserve.DefinePath, "application/json",
		inProject(clone, `{"concept":"invariant","context":"identityaccess","owner":"Tenant","name":"named","change":{"statement":"A tenant has a name.","verifiedBy":[]}}`))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created"`) {
		t.Fatalf("create: status = %d, body %s", rec.Code, rec.Body.String())
	}
	rec = post(t, h, mapserve.RemovePath, "application/json", inProject(clone, `{"concept":"invariant","context":"identityaccess","owner":"Tenant","name":"named"}`))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"invariant"`) {
		t.Fatalf("remove: status = %d, body %s", rec.Code, rec.Body.String())
	}
	data, err = os.ReadFile(filepath.Join(clone, vocab.DefaultDomainRecordingPath))
	if err != nil || bytes.Contains(data, []byte("A tenant has a name.")) {
		t.Fatalf("the removal did not reach the domain file: %v", err)
	}
}

func TestARefusedEditSaysWhyAndChangesNothing(t *testing.T) {
	clone := scratchClone(t)
	h := handler(t, clone)
	before, err := os.ReadFile(filepath.Join(clone, vocab.DefaultDomainRecordingPath))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, path, body string
		status           int
		reason           string
	}{
		{"unknown concept", mapserve.DefinePath, `{"concept":"gadget","context":"identityaccess","name":"X","change":{"definition":"d"}}`, http.StatusBadRequest, "gadget"},
		{"no name", mapserve.DefinePath, `{"concept":"value_object","context":"identityaccess","name":"","change":{"definition":"d"}}`, http.StatusBadRequest, "a name is required"},
		{"property the concept does not record", mapserve.DefinePath, `{"concept":"value_object","context":"identityaccess","name":"EmailAddress","change":{"statement":"s"}}`, http.StatusBadRequest, "statement"},
		{"over the word limit", mapserve.DefinePath, `{"concept":"value_object","context":"identityaccess","name":"EmailAddress","change":{"definition":"` + strings.Repeat("word ", 41) + `"}}`, http.StatusUnprocessableEntity, "at most 40"},
		{"not recorded", mapserve.RemovePath, `{"concept":"value_object","context":"identityaccess","name":"Nothing"}`, http.StatusUnprocessableEntity, "Nothing"},
		{"unknown context", mapserve.DefinePath, `{"concept":"value_object","context":"billing","name":"X","change":{"definition":"d"}}`, http.StatusBadRequest, "billing"},
	}
	for _, c := range cases {
		rec := post(t, h, c.path, "application/json", inProject(clone, c.body))
		var body struct {
			Error string `json:"error"`
		}
		decode(t, rec, &body)
		if rec.Code != c.status || !strings.Contains(body.Error, c.reason) {
			t.Errorf("%s: status = %d, error %q; want %d containing %q", c.name, rec.Code, body.Error, c.status, c.reason)
		}
	}
	after, err := os.ReadFile(filepath.Join(clone, vocab.DefaultDomainRecordingPath))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("a refused edit changed the domain file: %v", err)
	}
}

func TestAcceptedWithoutARemoteDefaultBranchSaysSo(t *testing.T) {
	root := t.TempDir()
	data, err := os.ReadFile(filepath.Join(exampleRoot(t), vocab.DefaultDomainRecordingPath))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, vocab.DefaultDomainRecordingPath), data, 0o600); err != nil {
		t.Fatal(err)
	}
	// Not a git work tree at all.
	var accepted struct {
		Available bool   `json:"available"`
		Reason    string `json:"reason"`
	}
	decode(t, get(t, handler(t, root), http.MethodGet, mapserve.AcceptedPath), &accepted)
	if accepted.Available || !strings.Contains(accepted.Reason, "no remote default branch") {
		t.Fatalf("accepted = %+v", accepted)
	}
	// A repository without a remote.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	git(t, root, "init", "-q", "-b", "main")
	decode(t, get(t, handler(t, root), http.MethodGet, mapserve.AcceptedPath), &accepted)
	if accepted.Available || !strings.Contains(accepted.Reason, "refs/remotes/origin/HEAD") {
		t.Fatalf("accepted = %+v", accepted)
	}
}

func TestAcceptedIsEmptyWhenTheBranchDoesNotHoldTheFile(t *testing.T) {
	clone := scratchClone(t)
	// The branch as the remote holds it: the upstream drops the file and
	// the clone fetches, so origin/main no longer holds it while the
	// working tree still does.
	upstream := filepath.Join(filepath.Dir(clone), "upstream")
	git(t, upstream, "rm", "-q", vocab.DefaultDomainRecordingPath)
	git(t, upstream, "commit", "-q", "-m", "drop the recording")
	git(t, clone, "fetch", "-q", "origin")
	var accepted struct {
		Available bool        `json:"available"`
		Branch    string      `json:"branch"`
		Overview  overviewDoc `json:"overview"`
	}
	decode(t, get(t, handler(t, clone), http.MethodGet, mapserve.AcceptedPath), &accepted)
	if !accepted.Available || accepted.Branch != "origin/main" || accepted.Overview.Found {
		t.Fatalf("accepted = %+v", accepted)
	}
}

func TestARequestByAnotherNameOrFromAnotherPageIsRefused(t *testing.T) {
	h := handler(t, exampleRoot(t))
	for host, want := range map[string]int{
		"127.0.0.1:7380":      http.StatusOK,
		"localhost:7380":      http.StatusOK,
		"[::1]:7380":          http.StatusOK,
		"192.168.1.20:7380":   http.StatusOK,
		"attacker.example:80": http.StatusMisdirectedRequest,
		"attacker.example":    http.StatusMisdirectedRequest,
	} {
		req := httptest.NewRequest(http.MethodGet, mapserve.OverviewPath, nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("host %q: status = %d, want %d", host, rec.Code, want)
		}
	}
	body := `{"concept":"value_object","context":"identityaccess","name":"EmailAddress","change":{"definition":"x"}}`
	for _, origin := range []string{"http://attacker.example", "null", "http://localhost:7380"} {
		req := httptest.NewRequest(http.MethodPost, mapserve.DefinePath, strings.NewReader(body))
		req.Host = pageHost
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("origin %q: status = %d, want 403", origin, rec.Code)
		}
	}
	data, err := os.ReadFile(filepath.Join(exampleRoot(t), vocab.DefaultDomainRecordingPath))
	if err != nil || !bytes.Contains(data, []byte("A well-formed email address.")) {
		t.Fatalf("a refused request changed the example's domain file: %v", err)
	}
}

func TestEditsAtOnceAreAllRecorded(t *testing.T) {
	clone := scratchClone(t)
	h := handler(t, clone)
	const edits = 12
	var wg sync.WaitGroup
	codes := make([]int, edits)
	for i := range edits {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := fmt.Sprintf(`{"concept":"value_object","context":"identityaccess","name":"Probe%d","change":{"definition":"A probe value."}}`, i)
			codes[i] = post(t, h, mapserve.DefinePath, "application/json", inProject(clone, body)).Code
		}()
	}
	wg.Wait()
	var after overviewDoc
	decode(t, get(t, h, http.MethodGet, mapserve.OverviewPath), &after)
	recorded := map[string]bool{}
	for _, v := range after.Contexts[0].ValueObjects {
		recorded[v.Name] = true
	}
	for i := range edits {
		if codes[i] != http.StatusOK || !recorded[fmt.Sprintf("Probe%d", i)] {
			t.Errorf("Probe%d: status %d, recorded %v", i, codes[i], recorded[fmt.Sprintf("Probe%d", i)])
		}
	}
}

// served is the project the page is told it is in.
func served(t *testing.T, h http.Handler) string {
	t.Helper()
	var project struct {
		Directory string `json:"directory"`
		Beside    string `json:"beside"`
	}
	decode(t, get(t, h, http.MethodGet, mapserve.ProjectPath), &project)
	if want := filepath.Dir(project.Directory) + string(filepath.Separator); project.Beside != want {
		t.Fatalf("a new project is suggested in %q, want %q beside %q", project.Beside, want, project.Directory)
	}
	return project.Directory
}

func TestCreatingAProjectServesItToBuildItsDomain(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	clone := scratchClone(t)
	h := handler(t, clone)
	if got := served(t, h); got != clone {
		t.Fatalf("served %q, want %q", got, clone)
	}
	before, err := os.ReadFile(filepath.Join(clone, vocab.DefaultDomainRecordingPath))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "orders")
	body, _ := json.Marshal(map[string]any{"directory": dir, "name": "Orders", "languages": []string{"go", "ts"}})
	rec := post(t, h, mapserve.ProjectsPath, "application/json", string(body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Directory, Project, Ruleset, Source string
	}
	decode(t, rec, &created)
	if created.Directory != dir || created.Project != "Orders" || created.Ruleset != filepath.Join(dir, "rules.arclint.yaml") || created.Source != vocab.DefaultDomainRecordingPath {
		t.Fatalf("created = %+v", created)
	}
	if top := strings.TrimSpace(git(t, dir, "rev-parse", "--show-toplevel")); !sameDirectory(t, top, dir) {
		t.Fatalf("the project's git repository is %q, want %q", top, dir)
	}
	ruleset, err := os.ReadFile(created.Ruleset)
	if err != nil || !bytes.Contains(ruleset, []byte("runtime: [go, ts]")) {
		t.Fatalf("ruleset: %v\n%s", err, ruleset)
	}

	// The page is now in the new project, whose language is empty.
	if got := served(t, h); got != dir {
		t.Fatalf("served %q after creating %q", got, dir)
	}
	var doc overviewDoc
	decode(t, get(t, h, http.MethodGet, mapserve.OverviewPath), &doc)
	if !doc.Found || doc.Project != "Orders" || len(doc.Contexts) != 0 {
		t.Fatalf("new project's overview = %+v", doc)
	}
	var accepted struct {
		Available bool `json:"available"`
	}
	decode(t, get(t, h, http.MethodGet, mapserve.AcceptedPath), &accepted)
	if accepted.Available {
		t.Fatal("a new repository without a remote has no accepted language")
	}

	// Its domain is built through the same define.
	rec = post(t, h, mapserve.DefinePath, "application/json", inProject(dir, `{"concept":"bounded_context","name":"ordering","change":{"definition":"Taking and fulfilling customer orders."}}`))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created"`) {
		t.Fatalf("define in the new project: status = %d, body %s", rec.Code, rec.Body.String())
	}
	rec = post(t, h, mapserve.DefinePath, "application/json", inProject(dir, `{"concept":"aggregate","context":"ordering","name":"Order","change":{"definition":"A customer's request for goods.","identity":"OrderID"}}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("define an aggregate: status = %d, body %s", rec.Code, rec.Body.String())
	}
	decode(t, get(t, h, http.MethodGet, mapserve.OverviewPath), &doc)
	if len(doc.Contexts) != 1 || doc.Contexts[0].Name != "ordering" || len(doc.Contexts[0].Aggregates) != 1 || doc.Contexts[0].Aggregates[0].Identity != "OrderID" {
		t.Fatalf("overview after building = %+v", doc)
	}
	recorded, err := yamlvocab.NewRepository(dir, mustRecording(t))
	if err != nil {
		t.Fatal(err)
	}
	lang, found, err := recorded.RecordedLanguage()
	if err != nil || !found || lang.Project != "Orders" || len(lang.Contexts) != 1 {
		t.Fatalf("the new domain file: found %v, err %v, language %+v", found, err, lang)
	}

	// An edit drafted in the project served before is refused, and its
	// domain file stays as it was.
	rec = post(t, h, mapserve.DefinePath, "application/json", inProject(clone, `{"concept":"value_object","context":"identityaccess","name":"EmailAddress","change":{"definition":"Moved."}}`))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "reload") {
		t.Fatalf("stale edit: status = %d, body %s", rec.Code, rec.Body.String())
	}
	if rec = post(t, h, mapserve.RemovePath, "application/json", `{"concept":"aggregate","context":"ordering","name":"Order"}`); rec.Code != http.StatusConflict {
		t.Fatalf("an edit naming no project: status = %d", rec.Code)
	}
	after, err := os.ReadFile(filepath.Join(clone, vocab.DefaultDomainRecordingPath))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("the earlier project's domain file changed: %v", err)
	}
}

func TestARefusedProjectLeavesNothingAndKeepsTheProjectServed(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	clone := scratchClone(t)
	h := handler(t, clone)
	base := t.TempDir()
	occupied := filepath.Join(base, "occupied")
	if err := os.MkdirAll(occupied, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "notes.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, body, reason, absent string
	}{
		{"a directory holding files", `{"directory":` + quote(occupied) + `,"name":"x"}`, "already holds files", ""},
		{"a relative path", `{"directory":"orders","name":"x"}`, "not an absolute path", ""},
		{"no directory", `{"directory":"","name":"x"}`, "name the directory", ""},
		{"a missing parent", `{"directory":` + quote(filepath.Join(base, "missing", "orders")) + `}`, "does not exist", filepath.Join(base, "missing")},
		{"an unknown language", `{"directory":` + quote(filepath.Join(base, "cobol")) + `,"languages":["cobol"]}`, "cobol", filepath.Join(base, "cobol")},
	}
	for _, c := range cases {
		rec := post(t, h, mapserve.ProjectsPath, "application/json", c.body)
		var body struct {
			Error string `json:"error"`
		}
		decode(t, rec, &body)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(body.Error, c.reason) {
			t.Errorf("%s: status = %d, error %q; want 422 containing %q", c.name, rec.Code, body.Error, c.reason)
		}
		if c.absent != "" {
			if _, err := os.Stat(c.absent); !os.IsNotExist(err) {
				t.Errorf("%s: %s was created", c.name, c.absent)
			}
		}
	}
	if entries, err := os.ReadDir(occupied); err != nil || len(entries) != 1 {
		t.Fatalf("the occupied directory changed: %v %v", entries, err)
	}
	if got := served(t, h); got != clone {
		t.Fatalf("served %q after refusals, want %q", got, clone)
	}
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func mustRecording(t *testing.T) vocab.DomainRecording {
	t.Helper()
	recording, err := vocab.NewDomainRecording(vocab.DefaultDomainRecordingPath)
	if err != nil {
		t.Fatal(err)
	}
	return recording
}

// sameDirectory compares two paths after links, as git reports a
// temporary directory through any link in its path.
func sameDirectory(t *testing.T, a, b string) bool {
	t.Helper()
	ra, err := filepath.EvalSymlinks(a)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := filepath.EvalSymlinks(b)
	if err != nil {
		t.Fatal(err)
	}
	return ra == rb
}

func TestHandlerNeedsItsComposition(t *testing.T) {
	if _, err := (mapserve.Server{}).Handler(); err == nil || !strings.Contains(err.Error(), "report writers") {
		t.Fatalf("err = %v, want the report writers named", err)
	}
	server := composed(t, exampleRoot(t))
	server.Create = nil
	if _, err := server.Handler(); err == nil || !strings.Contains(err.Error(), "project creation") {
		t.Fatalf("err = %v, want project creation named", err)
	}
	server = composed(t, exampleRoot(t))
	server.Project.Directory = ""
	if _, err := server.Handler(); err == nil || !strings.Contains(err.Error(), "directory") {
		t.Fatalf("err = %v, want the directory named", err)
	}
	server = composed(t, exampleRoot(t))
	server.Project.Overview = application.GetDomainOverview{}
	if _, err := server.Handler(); err == nil || !strings.Contains(err.Error(), "overview") {
		t.Fatalf("err = %v, want the overview named", err)
	}
	server = composed(t, exampleRoot(t))
	server.Project.Accepted = mapserve.Accepted{Branch: "origin/main"}
	if _, err := server.Handler(); err == nil || !strings.Contains(err.Error(), "accepted branch") {
		t.Fatalf("err = %v, want the accepted overview named", err)
	}
}

func TestURLNamesTheBoundHost(t *testing.T) {
	cases := []struct {
		addr net.Addr
		want string
	}{
		{&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7380}, "http://127.0.0.1:7380"},
		{&net.TCPAddr{IP: net.IPv4(192, 168, 1, 20), Port: 8080}, "http://192.168.1.20:8080"},
		{&net.TCPAddr{IP: net.IPv6loopback, Port: 7380}, "http://[::1]:7380"},
		{&net.TCPAddr{IP: net.IPv4zero, Port: 7380}, "http://localhost:7380"},
		{&net.TCPAddr{IP: net.IPv6unspecified, Port: 7380}, "http://localhost:7380"},
		{&net.TCPAddr{Port: 7380}, "http://localhost:7380"},
	}
	for _, c := range cases {
		if got := mapserve.URL(c.addr); got != c.want {
			t.Errorf("URL(%s) = %q, want %q", c.addr, got, c.want)
		}
	}
}

// serveOn binds address on a port the system picks and returns the URL
// the server reported. Serve has no stop of its own, as the command
// stops with the process, so the server lives until the test binary
// exits.
func serveOn(t *testing.T, address string) string {
	t.Helper()
	server := composed(t, exampleRoot(t))
	urls := make(chan string, 1)
	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(address, func(url string) error {
			urls <- url
			return nil
		})
	}()
	select {
	case url := <-urls:
		return url
	case err := <-errs:
		t.Fatalf("serve: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("the server did not report its URL")
	}
	return ""
}

func TestServeReportsAURLThatAnswers(t *testing.T) {
	url := serveOn(t, "127.0.0.1:0")
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("url = %q", url)
	}
	resp, err := http.Get(url + mapserve.OverviewPath) //nolint:noctx // a test request to the server under test
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"project"`) {
		t.Fatalf("status %d, body %s", resp.StatusCode, body)
	}
}

func TestServeOnEveryInterfacePrintsLocalhost(t *testing.T) {
	url := serveOn(t, ":0")
	if !strings.HasPrefix(url, "http://localhost:") {
		t.Fatalf("url = %q", url)
	}
}

func TestServeStopsWhenListeningRefuses(t *testing.T) {
	refused := errors.New("stdout closed")
	err := composed(t, exampleRoot(t)).Serve("127.0.0.1:0", func(string) error { return refused })
	if !errors.Is(err, refused) {
		t.Fatalf("err = %v, want %v", err, refused)
	}
}

func TestServeRefusesAnAddressAlreadyInUse(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	address := taken.Addr().String()
	err = composed(t, exampleRoot(t)).Serve(address, nil)
	if err == nil || !strings.Contains(err.Error(), "listen on "+address) {
		t.Fatalf("err = %v, want a listen error on %s", err, address)
	}
}

// TestStudioScriptUnderNode runs the page's model tests with node,
// lending them the Go example's overview as the JSON renderer spells
// it, so the script is proved against a real recording and against
// the same document the server sends.
func TestStudioScriptUnderNode(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed; the studio.js tests run where it is")
	}
	rec := get(t, handler(t, exampleRoot(t)), http.MethodGet, mapserve.OverviewPath)
	overviewFile := filepath.Join(t.TempDir(), "overview.json")
	if err := os.WriteFile(overviewFile, rec.Body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(node, "--test", "studio.test.js")
	cmd.Dir = "static"
	cmd.Env = append(os.Environ(), "ARCLINT_OVERVIEW_JSON="+overviewFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node --test failed: %v\n%s", err, out)
	}
	for _, want := range []string{"fail 0", "skipped 0", "cancelled 0"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("node --test did not run every test (%q missing):\n%s", want, out)
		}
	}
}
