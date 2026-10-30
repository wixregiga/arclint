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
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/reportfactory"
	"github.com/wixregiga/arclint/internal/delivery/mapserve"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	cmlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/cml"
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
	define, err := application.NewDefineDomainDefinition(knowledge, root)
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
			Overview: func(w io.Writer, o application.DomainOverview) error {
				return renderer.Render(w, cli.DomainOverviewReport{Overview: o})
			},
			Defined: func(w io.Writer, r application.DomainDefineResult, req application.DefineDomainRequest) error {
				return renderer.Render(w, cli.NewDomainDefineReport(r, req))
			},
			Removed: func(w io.Writer, r application.DomainRemoveResult) error {
				return renderer.Render(w, cli.DomainRemoveReport{Result: r})
			},
		},
		Overview: overview,
		Define:   define,
		Remove:   remove,
	}
	accepted, err := gitvocab.NewRepository(root, recording, parser{})
	if err != nil {
		server.Accepted = mapserve.Accepted{Unavailable: err.Error()}
		return server
	}
	acceptedOverview, err := application.NewGetDomainOverview(accepted)
	if err != nil {
		t.Fatal(err)
	}
	server.Accepted = mapserve.Accepted{Branch: accepted.Branch(), Overview: acceptedOverview}
	return server
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

func TestBuiltStudioIsServed(t *testing.T) {
	h := handler(t, exampleRoot(t))
	page := get(t, h, http.MethodGet, mapserve.PagePath)
	if page.Code != http.StatusOK || !strings.HasPrefix(page.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("page: status %d, content-type %q", page.Code, page.Header().Get("Content-Type"))
	}
	for _, want := range []string{"ArcLint Studio", `id="studio"`, `rel="manifest"`} {
		if !strings.Contains(page.Body.String(), want) {
			t.Errorf("page lacks %q", want)
		}
	}
	scriptURL := regexp.MustCompile(`src="(\./assets/[^" ]+\.js)"`).FindStringSubmatch(page.Body.String())
	if len(scriptURL) != 2 {
		t.Fatalf("page has no built module: %s", page.Body.String())
	}
	script := get(t, h, http.MethodGet, strings.TrimPrefix(scriptURL[1], "."))
	if script.Code != http.StatusOK || !strings.Contains(script.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("script: status %d, content-type %q", script.Code, script.Header().Get("Content-Type"))
	}
	for _, want := range []string{"api/overview", "api/accepted", "api/project", "api/define", "api/remove", "Name the project"} {
		if !strings.Contains(script.Body.String(), want) {
			t.Errorf("built Studio lacks %q", want)
		}
	}
	manifest := get(t, h, http.MethodGet, "/manifest.webmanifest")
	if manifest.Code != http.StatusOK || manifest.Header().Get("Content-Type") != "application/manifest+json" {
		t.Fatalf("manifest: %d %s", manifest.Code, manifest.Body.String())
	}
	var app map[string]any
	if err := json.Unmarshal(manifest.Body.Bytes(), &app); err != nil {
		t.Fatal(err)
	}
	if app["name"] != "ArcLint Studio" {
		t.Fatalf("manifest: %v", app)
	}
	icon := get(t, h, http.MethodGet, "/favicon.svg")
	if icon.Code != http.StatusOK || !strings.Contains(icon.Header().Get("Content-Type"), "image/svg+xml") {
		t.Fatalf("icon: %d", icon.Code)
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
	for _, path := range []string{"/studio.js", "/assets/missing.js", "/studio.test.js", "/static/index.html", "/other", "/api/model"} {
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
		`{"concept":"value_object","context":"identityaccess","owner":"","name":"EmailAddress","change":{"definition":"A mailbox address the tenant can be reached at."}}`)
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
		`{"concept":"invariant","context":"identityaccess","owner":"Tenant","name":"named","change":{"statement":"A tenant has a name.","verifiedBy":[]}}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"created"`) {
		t.Fatalf("create: status = %d, body %s", rec.Code, rec.Body.String())
	}
	rec = post(t, h, mapserve.RemovePath, "application/json", `{"concept":"invariant","context":"identityaccess","owner":"Tenant","name":"named"}`)
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
		rec := post(t, h, c.path, "application/json", c.body)
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
	projectCode := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		projectCode = post(t, h, mapserve.ProjectPath, "application/json", `{"description":"Concurrent project edit."}`).Code
	}()

	for i := range edits {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := fmt.Sprintf(`{"concept":"value_object","context":"identityaccess","name":"Probe%d","change":{"definition":"A probe value."}}`, i)
			codes[i] = post(t, h, mapserve.DefinePath, "application/json", body).Code
		}()
	}
	wg.Wait()
	current, err := composed(t, clone).Overview.Execute()
	if err != nil || projectCode != http.StatusOK || current.Language.Description != "Concurrent project edit." {
		t.Fatalf("concurrent project edit: status %d, overview %+v, error %v", projectCode, current, err)
	}

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

func TestHandlerNeedsItsComposition(t *testing.T) {
	if _, err := (mapserve.Server{}).Handler(); err == nil || !strings.Contains(err.Error(), "report writers") {
		t.Fatalf("err = %v, want the report writers named", err)
	}
	server := composed(t, exampleRoot(t))
	server.Overview = application.GetDomainOverview{}
	if _, err := server.Handler(); err == nil || !strings.Contains(err.Error(), "overview") {
		t.Fatalf("err = %v, want the overview named", err)
	}
	server = composed(t, exampleRoot(t))
	server.Accepted = mapserve.Accepted{Branch: "origin/main"}
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

func TestProjectEditsRecordingFormats(t *testing.T) {
	for _, format := range []string{"yaml", "cml"} {
		t.Run(format, func(t *testing.T) {
			root := t.TempDir()
			recording, err := vocab.NewDomainRecording("domain.arclint." + format)
			if err != nil {
				t.Fatal(err)
			}
			var knowledge vocab.Repository
			if format == "yaml" {
				knowledge, err = yamlvocab.NewRepository(root, recording)
			} else {
				knowledge, err = cmlvocab.NewRepository(root, recording)
			}
			if err != nil {
				t.Fatal(err)
			}
			server := composed(t, root)
			server.Overview, err = application.NewGetDomainOverview(knowledge)
			if err != nil {
				t.Fatal(err)
			}
			server.Define, err = application.NewDefineDomainDefinition(knowledge, root)
			if err != nil {
				t.Fatal(err)
			}
			server.Remove, err = application.NewRemoveDomainDefinition(knowledge)
			if err != nil {
				t.Fatal(err)
			}
			h, err := server.Handler()
			if err != nil {
				t.Fatal(err)
			}
			initial, err := server.Overview.Execute()
			if err != nil || initial.Found || initial.Source != recording.Path() {
				t.Fatalf("initial = %+v, %v", initial, err)
			}
			steps := []struct{ body, name, description, outcome string }{
				{`{"name":"clinic","description":"Booking visits."}`, "clinic", "Booking visits.", "created"},
				{`{"name":"practice"}`, "practice", "Booking visits.", "updated"},
				{`{"description":"Arranging care."}`, "practice", "Arranging care.", "updated"},
				{`{"description":""}`, "practice", "", "updated"},
				{`{"name":"practice"}`, "practice", "", "unchanged"},
			}
			for _, step := range steps {
				rec := post(t, h, mapserve.ProjectPath, "application/json", step.body)
				var result struct {
					Result, Type, Name string
					Changed            []string
					Values             map[string]string
				}
				decode(t, rec, &result)
				if rec.Code != 200 || result.Result != step.outcome || result.Type != "project" || result.Name != step.name {
					t.Fatalf("response: %d %s", rec.Code, rec.Body.String())
				}
				if step.outcome != "unchanged" && len(result.Changed) == 0 {
					t.Fatal("missing changes")
				}
				overview, err := server.Overview.Execute()
				if err != nil || !overview.Found || overview.Language.Project != step.name || overview.Language.Description != step.description {
					t.Fatalf("overview = %+v, %v", overview, err)
				}
			}
			path := filepath.Join(root, recording.Path())
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, refusal := range []struct {
				body   string
				status int
			}{
				{`{}`, 400}, {`{"name":""}`, 400}, {`{"name":"   "}`, 400},
				{`{"name":"changed","description":"` + strings.Repeat("word ", 101) + `"}`, 422},
				{`{"unknown":true}`, 400},
			} {
				rec := post(t, h, mapserve.ProjectPath, "application/json", refusal.body)
				if rec.Code != refusal.status || !strings.Contains(rec.Body.String(), `"error"`) {
					t.Fatalf("refusal: %d %s", rec.Code, rec.Body.String())
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("refusal wrote recording: %v", err)
				}
			}
		})
	}
}
