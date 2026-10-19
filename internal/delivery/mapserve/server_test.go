package mapserve_test

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/cli"
	"github.com/wixregiga/arclint/internal/delivery/cli/reportfactory"
	"github.com/wixregiga/arclint/internal/delivery/mapserve"
	"github.com/wixregiga/arclint/internal/domain/vocab"
	yamlvocab "github.com/wixregiga/arclint/internal/infrastructure/vocab/yaml"
)

// exampleOverview loads the Go example's domain file, a real recording
// the documentation quotes, through the repository that reads it.
func exampleOverview(t *testing.T) application.DomainOverview {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "docs", "examples", "go"))
	if err != nil {
		t.Fatal(err)
	}
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
	result, err := overview.Execute()
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatalf("no domain file under %s", root)
	}
	return result
}

// composed is the server as the composition root builds it: the JSON
// renderer spells the overview.
func composed(t *testing.T) mapserve.Server {
	t.Helper()
	renderer, err := reportfactory.Select(cli.RendererJSON)
	if err != nil {
		t.Fatal(err)
	}
	return mapserve.Server{Overview: func(w io.Writer, overview application.DomainOverview) error {
		return renderer.Render(w, cli.DomainOverviewReport{Overview: overview})
	}}
}

func handler(t *testing.T, overview application.DomainOverview) http.Handler {
	t.Helper()
	h, err := composed(t).Handler(overview)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func get(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestOverviewIsTheJSONRenderersDocument(t *testing.T) {
	overview := exampleOverview(t)
	rec := get(t, handler(t, overview), http.MethodGet, mapserve.OverviewPath)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("cache-control = %q", cc)
	}
	var doc struct {
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
				Name string `json:"name"`
			} `json:"valueObjects"`
		} `json:"contexts"`
		Counts map[string]int `json:"counts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v\n%s", err, rec.Body.String())
	}
	if !doc.Found || doc.Project != overview.Language.Project || doc.Source != overview.Source {
		t.Fatalf("found/project/source = %v/%q/%q, want true/%q/%q", doc.Found, doc.Project, doc.Source, overview.Language.Project, overview.Source)
	}
	if len(doc.Contexts) != len(overview.Language.Contexts) {
		t.Fatalf("contexts = %d, want %d", len(doc.Contexts), len(overview.Language.Contexts))
	}
	ctx := doc.Contexts[0]
	if ctx.Name != overview.Language.Contexts[0].Name || ctx.Definition == "" {
		t.Fatalf("context = %+v", ctx)
	}
	if len(ctx.Aggregates) != len(overview.Language.Contexts[0].Aggregates) || ctx.Aggregates[0].Identity == "" {
		t.Fatalf("aggregates = %+v", ctx.Aggregates)
	}
	if len(ctx.ValueObjects) != len(overview.Language.Contexts[0].ValueObjects) {
		t.Fatalf("value objects = %d, want %d", len(ctx.ValueObjects), len(overview.Language.Contexts[0].ValueObjects))
	}
	if doc.Counts["aggregates"] != overview.Counts.Aggregates {
		t.Fatalf("counts = %v, want %+v", doc.Counts, overview.Counts)
	}
}

func TestPageAndScriptAreServed(t *testing.T) {
	h := handler(t, exampleOverview(t))
	page := get(t, h, http.MethodGet, mapserve.PagePath)
	if page.Code != http.StatusOK || !strings.HasPrefix(page.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("page: status %d, content-type %q", page.Code, page.Header().Get("Content-Type"))
	}
	for _, want := range []string{"ArcLint Studio", "Navigator", "Inspector", `src="studio.js"`, mapserve.OverviewPath} {
		if !strings.Contains(page.Body.String(), want) {
			t.Errorf("page lacks %q", want)
		}
	}
	script := get(t, h, http.MethodGet, mapserve.ScriptPath)
	if script.Code != http.StatusOK || !strings.HasPrefix(script.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("script: status %d, content-type %q", script.Code, script.Header().Get("Content-Type"))
	}
	for _, want := range []string{"conceptsOf", "navigator", "layout", "inspect"} {
		if !strings.Contains(script.Body.String(), want) {
			t.Errorf("script lacks %q", want)
		}
	}
}

func TestOnlyGetIsAnsweredAndNothingElseIsServed(t *testing.T) {
	h := handler(t, exampleOverview(t))
	if rec := get(t, h, http.MethodPost, mapserve.OverviewPath); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST overview: status = %d", rec.Code)
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

func TestHandlerNeedsAnOverviewWriter(t *testing.T) {
	if _, err := (mapserve.Server{}).Handler(application.DomainOverview{}); err == nil {
		t.Fatal("expected an error without an overview writer")
	}
	failing := mapserve.Server{Overview: func(io.Writer, application.DomainOverview) error { return errors.New("boom") }}
	if _, err := failing.Handler(application.DomainOverview{}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want the writer's error", err)
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
	server, overview := composed(t), exampleOverview(t)
	urls := make(chan string, 1)
	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(address, overview, func(url string) error {
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
	err := composed(t).Serve("127.0.0.1:0", exampleOverview(t), func(string) error { return refused })
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
	err = composed(t).Serve(address, exampleOverview(t), nil)
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
	rec := get(t, handler(t, exampleOverview(t)), http.MethodGet, mapserve.OverviewPath)
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
