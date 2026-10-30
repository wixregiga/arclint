package mapserve

import (
	"bytes"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// serveWebAssets serves only files in the built frontend. It does not fall
// back to the page for missing scripts, expose directories, or read the project.
func serveWebAssets(files fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) {
			http.NotFound(w, r)
			return
		}
		body, err := fs.ReadFile(files, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		contentType := mime.TypeByExtension(path.Ext(name))
		if path.Ext(name) == ".webmanifest" {
			contentType = "application/manifest+json"
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(body))
	}
}
