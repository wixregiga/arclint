package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/delivery/mapserve"
	"github.com/wixregiga/arclint/internal/domain/vocab"
)

// freshCheck composes the Conformance Check for root through the one
// composition `arclint check` and Studio share.
func freshCheck(t *testing.T, root string, knowledge vocab.Repository) func() (application.AssessConformance, error) {
	t.Helper()
	rulesPath := filepath.Join(root, "rules.arclint.yaml")
	if _, err := composeConformance(root, rulesPath, knowledge); err != nil {
		t.Fatal(err)
	}
	return func() (application.AssessConformance, error) {
		parts, err := composeConformance(root, rulesPath, knowledge)
		return parts.assess, err
	}
}

// evidenceStudio serves Studio over a copy of the Go example, composed
// as `arclint map serve` composes it.
func evidenceStudio(t *testing.T) (string, http.Handler) {
	return evidenceStudioWith(t, nil)
}

// evidenceStudioWith is evidenceStudio with the composition of each
// assessment wrapped, so a test can hold an assessment while it runs.
func evidenceStudioWith(t *testing.T, wrap func(application.FreshCheck) application.FreshCheck) (string, http.Handler) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join("..", "..", "docs", "examples", "go"))); err != nil {
		t.Fatal(err)
	}
	// An analyzed source file named for secrets, one holding key
	// material, and a link leading out of the project.
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("identityaccess/tenant/secrets.go", "package tenant\n\nconst plain = 1\n")
	write("identityaccess/tenant/vault.go", "package tenant\n\nconst pem = `\n-----BEGIN RSA PRIVATE KEY-----\nMIIBOgIBAAJBAK\n-----END RSA PRIVATE KEY-----\n`\n")
	write("identityaccess/tenant/config.go", "package tenant\n\nvar apiKey = \"live-0123456789abcdefghij\"\n")
	write("identityaccess/tenant/typed.go", "package tenant\n\nvar dbPassword string = \"correct-horse-battery\"\n")
	write(".env", "DB_PASSWORD=correct-horse-battery\n")
	outside := filepath.Join(t.TempDir(), "outside.go")
	if err := os.WriteFile(outside, []byte("package tenant\n\nconst escaped = \"outside the project\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "identityaccess/tenant/escape.go")); err != nil {
		t.Fatal(err)
	}
	recording, _, repo, err := composeKnowledge(root, filepath.Join(root, "rules.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	overview, err := application.NewGetDomainOverview(repo)
	if err != nil {
		t.Fatal(err)
	}
	define, err := application.NewDefineDomainDefinition(repo, root)
	if err != nil {
		t.Fatal(err)
	}
	remove, err := application.NewRemoveDomainDefinition(repo)
	if err != nil {
		t.Fatal(err)
	}
	check := application.FreshCheck(freshCheck(t, root, repo))
	if wrap != nil {
		check = wrap(check)
	}
	server, err := composeStudio(root, recording, overview, define, remove, check)
	if err != nil {
		t.Fatal(err)
	}
	h, err := server.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return root, h
}

// serve sends a request as the map's own page does, with the Studio
// header; a header given empty is left out.
func serve(h http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost"+target, bytes.NewBufferString(body))
	r.Header.Set(mapserve.StudioRequestHeader, "1")
	for k, v := range headers {
		if v == "" {
			r.Header.Del(k)
			continue
		}
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

type evidenceAnswer struct {
	Snapshot string `json:"snapshot"`
	Complete bool   `json:"complete"`
	Entries  []struct {
		Concept      string `json:"concept"`
		Name         string `json:"name"`
		Reading      string `json:"reading"`
		Declarations struct {
			State     string `json:"state"`
			Locations []struct {
				Path     string `json:"path"`
				Line     int    `json:"line"`
				Readable bool   `json:"readable"`
			} `json:"locations"`
		} `json:"declarations"`
		Contracts []struct {
			Key     string `json:"key"`
			Carrier struct {
				State string `json:"state"`
			} `json:"carrier"`
			Coverage string `json:"coverage"`
		} `json:"contracts"`
	} `json:"entries"`
}

func assessed(t *testing.T, h http.Handler) evidenceAnswer {
	t.Helper()
	w := serve(h, "GET", "/api/evidence", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/evidence: %d %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control %q, want no-store", got)
	}
	var doc evidenceAnswer
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func excerpt(h http.Handler, snapshot, path string, line int) *httptest.ResponseRecorder {
	q := url.Values{"snapshot": {snapshot}, "path": {path}, "line": {fmt.Sprint(line)}}
	return serve(h, "GET", "/api/source?"+q.Encode(), "", nil)
}

// treeDigest hashes every file of the project but ArcLint's cache, so a
// change to source or to the domain recording shows.
func treeDigest(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() && rel == filepath.Join(".arclint", "cache") {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(b)
			out[rel] = hex.EncodeToString(sum[:])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestComposedEvidenceAttributesTheExampleAndChangesNothing(t *testing.T) {
	root, h := evidenceStudio(t)
	before := treeDigest(t, root)
	doc := assessed(t, h)
	var tenant bool
	for _, e := range doc.Entries {
		if e.Concept == "aggregate" && e.Name == "Tenant" {
			tenant = true
			if e.Declarations.State != "located" || len(e.Declarations.Locations) != 1 || e.Declarations.Locations[0].Path != "identityaccess/tenant/tenant.go" || !e.Declarations.Locations[0].Readable {
				t.Errorf("Tenant declarations %+v", e.Declarations)
			}
			if len(e.Contracts) != 2 || e.Contracts[0].Carrier.State != "found" || e.Contracts[0].Coverage != "structurally_checked" {
				t.Errorf("Tenant contracts %+v", e.Contracts)
			}
			if e.Reading == "no_attributed_findings" {
				t.Errorf("Tenant reads without findings although a context check did not decide")
			}
		}
	}
	if !tenant {
		t.Fatal("no evidence for aggregate Tenant")
	}
	if w := excerpt(h, doc.Snapshot, "identityaccess/tenant/tenant.go", 16); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "type Tenant struct") {
		t.Errorf("excerpt of the Tenant root: %d %s", w.Code, w.Body.String())
	}
	if after := treeDigest(t, root); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("assessing and reading excerpts changed the project:\nbefore %v\nafter  %v", before, after)
	}
}

func TestComposedExcerptsReadOnlyTheSnapshotsSafeSource(t *testing.T) {
	_, h := evidenceStudio(t)
	doc := assessed(t, h)
	cases := []struct {
		name, path string
		line, want int
	}{
		{"traversal", "../../../../etc/passwd", 1, http.StatusBadRequest},
		{"absolute", "/etc/passwd", 1, http.StatusBadRequest},
		{"unclean", "identityaccess/./tenant/tenant.go", 1, http.StatusBadRequest},
		{"backslash", `identityaccess\tenant\tenant.go`, 1, http.StatusBadRequest},
		{"line zero", "identityaccess/tenant/tenant.go", 0, http.StatusBadRequest},
		{"manifest", "go.mod", 1, http.StatusNotFound},
		{"ruleset", "rules.arclint.yaml", 1, http.StatusNotFound},
		{"secret by name", "identityaccess/tenant/secrets.go", 1, http.StatusForbidden},
		{"key material", "identityaccess/tenant/vault.go", 1, http.StatusForbidden},
		{"credential literal", "identityaccess/tenant/config.go", 1, http.StatusForbidden},
		{"typed credential literal", "identityaccess/tenant/typed.go", 1, http.StatusForbidden},
		{"beyond the end", "identityaccess/tenant/tenant.go", 100000, http.StatusUnprocessableEntity},
		{"recording", "domain.arclint.yaml", 27, http.StatusOK},
	}
	for _, c := range cases {
		if w := excerpt(h, doc.Snapshot, c.path, c.line); w.Code != c.want {
			t.Errorf("%s: %d %s, want %d", c.name, w.Code, w.Body.String(), c.want)
		}
	}
	// A link leading out of the project is never read, whether or not
	// the scan analyzed it.
	w := excerpt(h, doc.Snapshot, "identityaccess/tenant/escape.go", 1)
	if w.Code == http.StatusOK || strings.Contains(w.Body.String(), "outside the project") {
		t.Errorf("escaping link: %d %s", w.Code, w.Body.String())
	}
	if w := excerpt(h, strings.Repeat("0", 32), "identityaccess/tenant/tenant.go", 16); w.Code != http.StatusConflict {
		t.Errorf("unknown snapshot: %d, want 409", w.Code)
	}
	if w := serve(h, "GET", "/api/source?snapshot="+doc.Snapshot+"&path=identityaccess/tenant/tenant.go&line=x", "", nil); w.Code != http.StatusBadRequest {
		t.Errorf("line not a number: %d", w.Code)
	}
	for _, path := range []string{"/api/evidence", "/api/source?snapshot=" + doc.Snapshot + "&path=identityaccess/tenant/tenant.go&line=16"} {
		if w := serve(h, "GET", path, "", map[string]string{mapserve.StudioRequestHeader: ""}); w.Code != http.StatusForbidden {
			t.Errorf("GET %s without the Studio header, as an image of another site sends it: %d, want 403", path, w.Code)
		}
	}
	if w := serve(h, "GET", "/api/evidence", "", map[string]string{"Origin": "http://evil.example"}); w.Code != http.StatusForbidden {
		t.Errorf("another page's assessment: %d, want 403", w.Code)
	}
	for _, site := range []string{"cross-site", "same-site"} {
		if w := serve(h, "GET", "/api/evidence", "", map[string]string{"Sec-Fetch-Site": site}); w.Code != http.StatusForbidden {
			t.Errorf("an assessment a %s page asks for: %d, want 403", site, w.Code)
		}
		if w := serve(h, "GET", "/api/source?snapshot="+doc.Snapshot+"&path=identityaccess/tenant/tenant.go&line=16", "", map[string]string{"Sec-Fetch-Site": site}); w.Code != http.StatusForbidden {
			t.Errorf("an excerpt a %s page asks for: %d, want 403", site, w.Code)
		}
	}
	if w := excerpt(h, doc.Snapshot, "identityaccess/tenant/tenant.go", 16); w.Code != http.StatusOK {
		t.Errorf("excerpt without Sec-Fetch-Site: %d", w.Code)
	}
	if w := serve(h, "POST", "/api/evidence", "", nil); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST evidence: %d, want 405", w.Code)
	}
}

// A newer assessment keeps a few older snapshots readable, then the
// oldest is stale; an assessment and an edit at once both complete.
func TestComposedSnapshotsAgeAndAssessmentsWaitForEdits(t *testing.T) {
	_, h := evidenceStudio(t)
	first := assessed(t, h)
	for range 4 {
		assessed(t, h)
	}
	if w := excerpt(h, first.Snapshot, "identityaccess/tenant/tenant.go", 16); w.Code != http.StatusConflict {
		t.Errorf("a snapshot four assessments old: %d, want 409", w.Code)
	}
	var wg sync.WaitGroup
	codes := make([]int, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		codes[0] = serve(h, "GET", "/api/evidence", "", nil).Code
	}()
	go func() {
		defer wg.Done()
		codes[1] = serve(h, "POST", "/api/define", `{"concept":"value_object","context":"identityaccess","name":"Seat","change":{"definition":"A place."}}`, map[string]string{"Content-Type": "application/json"}).Code
	}()
	wg.Wait()
	if codes[0] != http.StatusOK || codes[1] != http.StatusOK {
		t.Fatalf("assessment %d and edit %d at once", codes[0], codes[1])
	}
	doc := assessed(t, h)
	for _, e := range doc.Entries {
		if e.Name == "Seat" {
			return
		}
	}
	t.Error("the assessment after the edit does not hold the new entry")
}

// A source file of an older snapshot swapped for a link to a file the
// assessment never analyzed, or for a pipe, is not read: the link is
// not followed and the pipe neither blocks nor answers.
func TestComposedExcerptsRefuseFilesSwappedAfterTheAssessment(t *testing.T) {
	root, h := evidenceStudio(t)
	doc := assessed(t, h)
	tenant := filepath.Join(root, "identityaccess/tenant/tenant.go")
	if err := os.Remove(tenant); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../.env", tenant); err != nil {
		t.Fatal(err)
	}
	w := excerpt(h, doc.Snapshot, "identityaccess/tenant/tenant.go", 1)
	if w.Code == http.StatusOK || strings.Contains(w.Body.String(), "correct-horse-battery") {
		t.Fatalf("a source file swapped for a link to .env: %d %s", w.Code, w.Body.String())
	}
	role := filepath.Join(root, "identityaccess/role/role.go")
	if err := os.Remove(role); err != nil {
		t.Fatal(err)
	}
	if err := mkfifo(role); err != nil {
		t.Skipf("no named pipes here: %v", err)
	}
	answered := make(chan int, 1)
	go func() { answered <- excerpt(h, doc.Snapshot, "identityaccess/role/role.go", 1).Code }()
	select {
	case code := <-answered:
		if code == http.StatusOK {
			t.Errorf("a source file swapped for a pipe answered 200")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("an excerpt of a source file swapped for a pipe blocked")
	}
}

// A request that has already left starts no assessment, so it neither
// costs a run nor pushes out a snapshot a page still reads.
func TestComposedAssessmentsSkipRequestsAlreadyGone(t *testing.T) {
	_, h := evidenceStudio(t)
	first := assessed(t, h)
	for range 6 {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r := httptest.NewRequest("GET", "http://localhost/api/evidence", nil).WithContext(ctx)
		r.Header.Set(mapserve.StudioRequestHeader, "1")
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	if w := excerpt(h, first.Snapshot, "identityaccess/tenant/tenant.go", 16); w.Code != http.StatusOK {
		t.Fatalf("after six requests that left at once, the first snapshot: %d %s", w.Code, w.Body.String())
	}
}

// Requests that arrive while an assessment runs wait for it and share
// its result. The server, the barrier and the release are all made
// inside one synctest bubble, so every wait below is a wait the bubble
// observes: the first request's composition signals that it started and
// is held; the other requests join the run under way; synctest.Wait
// returns once all of them are blocked; only then is the run released.
func TestComposedAssessmentsShareTheRunUnderWay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var composed atomic.Int32
		started := make(chan struct{})
		release := make(chan struct{})
		_, h := evidenceStudioWith(t, func(check application.FreshCheck) application.FreshCheck {
			return func() (application.AssessConformance, error) {
				if composed.Add(1) == 1 {
					close(started)
				}
				<-release
				return check()
			}
		})
		snapshots := make([]string, 4)
		var wg sync.WaitGroup
		ask := func(i int) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				w := serve(h, "GET", "/api/evidence", "", nil)
				var doc evidenceAnswer
				if w.Code == http.StatusOK && json.Unmarshal(w.Body.Bytes(), &doc) == nil {
					snapshots[i] = doc.Snapshot
				}
			}()
		}
		ask(0)
		<-started
		for i := 1; i < len(snapshots); i++ {
			ask(i)
		}
		synctest.Wait()
		if n := composed.Load(); n != 1 {
			t.Fatalf("four overlapping requests composed %d assessments, want 1", n)
		}
		close(release)
		wg.Wait()
		for _, s := range snapshots {
			if s == "" || s != snapshots[0] {
				t.Fatalf("the requests did not share one snapshot: %v", snapshots)
			}
		}
	})
}

// The peek reads the file on disk now. A change of size after the
// assessment is reported; an edit that keeps the size is not, which is
// why the page always says the lines may differ from the assessed code.
func TestComposedExcerptsReportASizeChangeSinceTheAssessment(t *testing.T) {
	root, h := evidenceStudio(t)
	doc := assessed(t, h)
	changed := func() bool {
		t.Helper()
		w := excerpt(h, doc.Snapshot, "identityaccess/tenant/tenant.go", 16)
		if w.Code != http.StatusOK {
			t.Fatalf("excerpt: %d %s", w.Code, w.Body.String())
		}
		var got struct {
			Changed bool `json:"changedSinceAssessment"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got.Changed
	}
	if changed() {
		t.Fatal("an untouched file reads as changed")
	}
	path := filepath.Join(root, "identityaccess/tenant/tenant.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sameSize := strings.Replace(string(content), "Tenant is", "TENANT is", 1)
	if err := os.WriteFile(path, []byte(sameSize), 0o600); err != nil {
		t.Fatal(err)
	}
	if changed() {
		t.Error("an edit of the same size cannot be told from no change, and must not be claimed as one")
	}
	if err := os.WriteFile(path, append(content, []byte("// one more line\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if !changed() {
		t.Error("a file whose size changed since the assessment reads as unchanged")
	}
}

// findingSet is every finding of an evidence document, attributed or
// not, as rule, place and message.
func findingSet(t *testing.T, body []byte) map[string]bool {
	t.Helper()
	type finding struct {
		Rule     string `json:"rule"`
		Message  string `json:"message"`
		Location *struct {
			Path string `json:"path"`
			Line int    `json:"line"`
		} `json:"location"`
	}
	var doc struct {
		Entries []struct {
			Findings []finding `json:"findings"`
		} `json:"entries"`
		Unattributed struct {
			Findings []finding `json:"findings"`
		} `json:"unattributed"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	add := func(f finding) {
		key := f.Rule + "|" + f.Message
		if f.Location != nil {
			key += fmt.Sprintf("|%s:%d", f.Location.Path, f.Location.Line)
		}
		out[key] = true
	}
	for _, e := range doc.Entries {
		for _, f := range e.Findings {
			add(f)
		}
	}
	for _, f := range doc.Unattributed.Findings {
		add(f)
	}
	return out
}

// An assessment after the ruleset, the recording and an Extension changed
// on disk finds what `arclint check` composed afresh finds then: the
// server keeps no rules, vocabulary or Extension from an earlier run.
func TestComposedReassessmentReadsChangedRulesRecordingAndExtensions(t *testing.T) {
	root, h := evidenceStudio(t)
	before := serve(h, "GET", "/api/evidence", "", nil)
	if before.Code != http.StatusOK {
		t.Fatalf("first assessment: %d %s", before.Code, before.Body.String())
	}
	if strings.Contains(before.Body.String(), "marker one") {
		t.Fatal("the marker extension reported before it existed")
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	marker := func(message string) string {
		return `import { defineRule } from "arclint";
export default defineRule({
  type: "marker",
  check(ctx) {
    for (const f of ctx.files()) ctx.report({ path: f.path, line: 1, message: "` + message + `" });
  },
});
`
	}
	// A new Rule using a new Extension, and a new aggregate the code does not declare.
	write(".arclint/extensions/marker.ts", marker("marker one"))
	rules, err := os.ReadFile(filepath.Join(root, "rules.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	write("rules.arclint.yaml", string(rules)+`
  identityaccess/marker:
    rationale: "Marks each file of the tenant package."
    on: identityaccess
    files: "identityaccess/tenant/tenant.go"
    uses: marker
`)
	recording, err := os.ReadFile(filepath.Join(root, "domain.arclint.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	write("domain.arclint.yaml", strings.Replace(string(recording), "    aggregates:\n", "    aggregates:\n      Ledger:\n        definition: A record of every charge to a tenant.\n        identity: LedgerID\n", 1))

	recordingRepo := func() vocab.Repository {
		_, _, repo, err := composeKnowledge(root, filepath.Join(root, "rules.arclint.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		return repo
	}
	cliFindings := func() map[string]bool {
		t.Helper()
		parts, err := composeConformance(root, filepath.Join(root, "rules.arclint.yaml"), recordingRepo())
		if err != nil {
			t.Fatal(err)
		}
		a, err := parts.assess.Execute(application.AssessConformanceRequest{})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, v := range a.Violations() {
			key := v.Rule().Qualified() + "|" + v.Message()
			if v.Path() != "" {
				key += fmt.Sprintf("|%s:%d", v.Path(), v.Line())
			}
			out[key] = true
		}
		return out
	}
	assertMatches := func(step string) []byte {
		t.Helper()
		w := serve(h, "GET", "/api/evidence", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", step, w.Code, w.Body.String())
		}
		got, want := findingSet(t, w.Body.Bytes()), cliFindings()
		for k := range want {
			if !got[k] {
				t.Errorf("%s: the check composed now finds %q; the evidence does not", step, k)
			}
		}
		for k := range got {
			if !want[k] {
				t.Errorf("%s: the evidence holds %q; the check composed now does not", step, k)
			}
		}
		return w.Body.Bytes()
	}
	after := assertMatches("after the ruleset, recording and extension changed")
	if !strings.Contains(string(after), "marker one") || !strings.Contains(string(after), `"name":"Ledger"`) {
		t.Fatalf("the reassessment missed the new Rule or the new entry")
	}
	write(".arclint/extensions/marker.ts", marker("marker two"))
	again := assertMatches("after the extension changed again")
	if strings.Contains(string(again), "marker one") || !strings.Contains(string(again), "marker two") {
		t.Fatal("the reassessment kept the earlier extension")
	}
}
