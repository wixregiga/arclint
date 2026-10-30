package mapserve

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWebAssetsServePageAndReferencedFiles(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":           {Data: []byte(`<script src="./assets/studio.js"></script>`)},
		"assets/studio.js":     {Data: []byte(`document.title = "ArcLint Studio";`)},
		"assets/studio.css":    {Data: []byte(`body { color: teal; }`)},
		"manifest.webmanifest": {Data: []byte(`{"name":"ArcLint Studio"}`)},
	}
	for _, test := range []struct {
		url, contentType, content string
	}{
		{"/", "text/html", `./assets/studio.js`},
		{"/assets/studio.js", "javascript", "ArcLint Studio"},
		{"/assets/studio.css", "text/css", "teal"},
		{"/manifest.webmanifest", "application/manifest+json", "ArcLint Studio"},
	} {
		t.Run(test.url, func(t *testing.T) {
			response := httptest.NewRecorder()
			serveWebAssets(assets)(response, httptest.NewRequest(http.MethodGet, test.url, nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), test.contentType) || !strings.Contains(response.Body.String(), test.content) {
				t.Fatalf("GET %s: status %d, headers %v, body %s", test.url, response.Code, response.Header(), response.Body)
			}
			if response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("built assets must retain their declared content type")
			}
		})
	}
}

func TestWebAssetsDoNotTurnMissingFilesIntoPages(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("the page")}}
	for _, url := range []string{"/assets/missing.js", "/api/unknown", "/../domain.arclint.yaml", "/" + "."} {
		response := httptest.NewRecorder()
		serveWebAssets(assets)(response, httptest.NewRequest(http.MethodGet, url, nil))
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "the page") {
			t.Errorf("GET %s: status %d, body %s", url, response.Code, response.Body)
		}
	}
	response := httptest.NewRecorder()
	serveWebAssets(assets)(response, httptest.NewRequest(http.MethodPost, "/", nil))
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST asset: status %d", response.Code)
	}
}

func TestWebAssetHeadRetainsHeadersWithoutAResponseBody(t *testing.T) {
	assets := fstest.MapFS{"index.html": {Data: []byte("the page")}}
	response := httptest.NewRecorder()
	serveWebAssets(assets)(response, httptest.NewRequest(http.MethodHead, "/", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Length") != "8" || response.Body.Len() != 0 {
		t.Fatalf("HEAD page: status %d, headers %v, body %s", response.Code, response.Header(), response.Body)
	}
}
