package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
)

func TestComposedFeatureHTTP(t *testing.T) {
	root := t.TempDir()
	recording, _, repo, err := composeKnowledge(root, "rules.arclint.yaml")
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
	server, err := composeStudio(root, recording, overview, define, remove)
	if err != nil {
		t.Fatal(err)
	}
	h, err := server.Handler()
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body, rev string, want int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, "http://localhost"+path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("If-Match", rev)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	request("GET", "/api/features", "", "", 200)
	b := request("POST", "/api/feature", `{"title":"","body":"😀 日本語","references":[]}`, "", 200)
	var saved struct {
		Feature application.FeatureDraft `json:"feature"`
	}
	if err = json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	d := saved.Feature
	request("GET", "/api/feature?id="+d.ID, "", "", 200)
	request("GET", "/api/features", "", "", 200)
	d.Body += " edited"
	b, _ = json.Marshal(d)
	result := request("POST", "/api/feature", string(b), "", 200)
	request("POST", "/api/feature", string(b), "", 409)
	request("DELETE", "/api/feature?id="+d.ID, "", d.Revision, 409)
	if err = json.Unmarshal(result, &saved); err != nil {
		t.Fatal(err)
	}
	request("DELETE", "/api/feature?id="+d.ID, "", saved.Feature.Revision, 200)
	request("GET", "/api/feature?id="+d.ID, "", "", 404)
	request("POST", "/api/feature", `{"title":"","body":"","references":[]} {}`, "", 400)
	request("POST", "/api/feature", `{"title":"","body":""}`, "", 400)
	request("GET", "/api/feature?id=../escape", "", "", 400)
	if _, err = os.Stat(filepath.Join(root, "domain.arclint.yaml")); !os.IsNotExist(err) {
		t.Fatalf("drafts touched recording: %v", err)
	}
}
