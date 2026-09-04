// Package registrypattern reaches a Pattern Registry: a static tree of
// files at an https, http, or file URL laid out as the distribution
// context fixes it. The Client reads the index and fetches one
// published version with its Manifest, verifying every byte before a
// Pattern is built from it; the Publisher writes that same layout so a
// directory becomes a Registry the moment a file host serves it.
package registrypattern

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
	patternfiles "github.com/wixregiga/arclint/internal/infrastructure/pattern/files"
)

// MaxDocumentBytes caps one fetched document: a Pattern file, a
// Manifest, or the index. A Registry entry beyond it is refused rather
// than read into memory.
const MaxDocumentBytes = 8 << 20

// RequestTimeout bounds one Registry request end to end.
const RequestTimeout = 30 * time.Second

// Client implements the application's PatternRegistry port.
type Client struct {
	http  *http.Client
	token string
}

// NewClient returns a Client; token, when not empty, is sent as a
// bearer credential on every http and https request so a private
// Registry can be read with the same layout as a public one.
func NewClient(token string) Client {
	return Client{
		http:  &http.Client{Timeout: RequestTimeout},
		token: strings.TrimSpace(token),
	}
}

// Index reads the Registry's listing.
func (c Client) Index(reg distribution.Registry) (distribution.Index, error) {
	if reg.IsZero() {
		return distribution.Index{}, fmt.Errorf("registry index: unconstructed registry")
	}
	data, err := c.get(reg, distribution.IndexPath())
	if err != nil {
		return distribution.Index{}, fmt.Errorf("registry %s: index: %w", reg, err)
	}
	index, err := patternfiles.DecodeIndex(data, reg.IndexURL())
	if err != nil {
		return distribution.Index{}, fmt.Errorf("registry %s: %w", reg, err)
	}
	return index, nil
}

// Fetch reads one published version: its Manifest, then every listed
// file, which must match the Manifest byte for byte and describe the
// reference asked for.
func (c Client) Fetch(reg distribution.Registry, ref rule.PatternReference) (distribution.Available, error) {
	if reg.IsZero() {
		return distribution.Available{}, fmt.Errorf("registry fetch: unconstructed registry")
	}
	if ref.IsZero() {
		return distribution.Available{}, fmt.Errorf("registry fetch: unconstructed pattern reference")
	}
	data, err := c.get(reg, distribution.ManifestPath(ref))
	if err != nil {
		return distribution.Available{}, fmt.Errorf("registry %s: %s: manifest: %w", reg, ref, err)
	}
	m, err := patternfiles.DecodeManifest(data, reg.ManifestURL(ref))
	if err != nil {
		return distribution.Available{}, fmt.Errorf("registry %s: %s: %w", reg, ref, err)
	}
	if m.Reference() != ref {
		return distribution.Available{}, fmt.Errorf("registry %s: %s: manifest names %s", reg, ref, m.Reference())
	}
	files := make([]distribution.PatternFile, 0, len(m.Entries()))
	for _, e := range m.Entries() {
		body, err := c.get(reg, distribution.FilePath(ref, e.Path()))
		if err != nil {
			return distribution.Available{}, fmt.Errorf("registry %s: %s: %w", reg, ref, err)
		}
		f, err := distribution.NewPatternFile(e.Path(), body)
		if err != nil {
			return distribution.Available{}, fmt.Errorf("registry %s: %s: %w", reg, ref, err)
		}
		files = append(files, f)
	}
	v, err := distribution.NewVendoredPattern(m, files)
	if err != nil {
		return distribution.Available{}, fmt.Errorf("registry %s: %s: %w", reg, ref, err)
	}
	p, err := patternfiles.Load(files, reg.String()+"/"+distribution.VersionDir(ref))
	if err != nil {
		return distribution.Available{}, fmt.Errorf("registry %s: %s: %w", reg, ref, err)
	}
	if p.Reference() != ref {
		return distribution.Available{}, fmt.Errorf("registry %s: %s: pattern.yaml declares %s", reg, ref, p.Reference())
	}
	a, err := distribution.NewAvailable(distribution.SourceRegistry, p, v, false)
	if err != nil {
		return distribution.Available{}, fmt.Errorf("registry %s: %s: %w", reg, ref, err)
	}
	return a, nil
}

// get reads the document at relative, a path beneath the Registry
// root: over the network, or from the directory a file URL names.
func (c Client) get(reg distribution.Registry, relative string) ([]byte, error) {
	location := reg.URL(relative)
	u, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", location, err)
	}
	if u.Scheme == "file" {
		return readLocal(reg, relative)
	}
	ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", location, err)
	}
	req.Header.Set("Accept", "application/json, application/yaml, text/plain, */*")
	req.Header.Set("User-Agent", "arclint")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", location, err)
	}
	defer func() {
		// The body is drained by readCapped or abandoned on a bad
		// status; a close failure after that changes nothing the
		// caller can act on.
		_ = resp.Body.Close()
	}()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("%s: not found", location)
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%s: %s; set GITHUB_TOKEN or GH_TOKEN for a private registry", location, resp.Status)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%s: %s", location, resp.Status)
	}
	return readCapped(resp.Body, location)
}

// readLocal serves a document from the directory a file Registry
// names, with the same document cap. Every read is confined to that
// directory: the tree is opened as a root, and the relative document
// path cannot escape it however the index or a Manifest spells it.
func readLocal(reg distribution.Registry, relative string) ([]byte, error) {
	location := reg.URL(relative)
	dir, err := localRoot(reg)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", location, err)
	}
	root, err := os.OpenRoot(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: registry directory %s does not exist", location, dir)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: open registry directory: %w", location, err)
	}
	defer func() { _ = root.Close() }()
	f, err := root.Open(filepath.FromSlash(relative))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s: not found", location)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", location, err)
	}
	defer func() { _ = f.Close() }()
	return readCapped(f, location)
}

// localRoot is the directory a file Registry URL names.
func localRoot(reg distribution.Registry) (string, error) {
	u, err := url.Parse(reg.Location())
	if err != nil {
		return "", fmt.Errorf("registry location: %w", err)
	}
	dir := filepath.FromSlash(u.Path)
	if u.Host != "" && u.Host != "localhost" {
		dir = filepath.Join(u.Host, dir)
	}
	return dir, nil
}

func readCapped(r io.Reader, location string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxDocumentBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", location, err)
	}
	if len(data) > MaxDocumentBytes {
		return nil, fmt.Errorf("%s: larger than %d bytes", location, MaxDocumentBytes)
	}
	return data, nil
}

// Publisher implements the application's PatternPublisher port by
// writing the Registry layout into a directory tree.
type Publisher struct{}

// NewPublisher returns the Publisher.
func NewPublisher() Publisher { return Publisher{} }

// Publish writes the version directory with its files and Manifest,
// then lists the version in the tree's index, creating the index when
// the tree is new. Each write lands whole: the version directory is
// staged beside its destination and renamed into place, and the index
// is rewritten through a temporary file.
func (Publisher) Publish(dir string, a distribution.Available) (application.PublishedPattern, error) {
	if a.Pattern.Reference().IsZero() || a.Vendored.IsZero() {
		return application.PublishedPattern{}, fmt.Errorf("publish pattern: unconstructed pattern")
	}
	root, err := filepath.Abs(dir)
	if err != nil {
		return application.PublishedPattern{}, fmt.Errorf("publish pattern: %w", err)
	}
	ref := a.Reference()
	versionDir := filepath.Join(root, filepath.FromSlash(distribution.VersionDir(ref)))
	indexPath := filepath.Join(root, distribution.IndexFileName)
	index, err := readIndex(indexPath)
	if err != nil {
		return application.PublishedPattern{}, fmt.Errorf("publish pattern %s: %w", ref, err)
	}
	entry, err := distribution.IndexEntryOf(a)
	if err != nil {
		return application.PublishedPattern{}, fmt.Errorf("publish pattern %s: %w", ref, err)
	}
	_, replaced := index.Lookup(ref)
	index, err = index.With(entry)
	if err != nil {
		return application.PublishedPattern{}, fmt.Errorf("publish pattern %s: %w", ref, err)
	}
	if err := writeVersion(versionDir, a.Vendored); err != nil {
		return application.PublishedPattern{}, fmt.Errorf("publish pattern %s: %w", ref, err)
	}
	encoded, err := patternfiles.EncodeIndex(index)
	if err != nil {
		return application.PublishedPattern{}, fmt.Errorf("publish pattern %s: %w", ref, err)
	}
	if err := writeWhole(indexPath, encoded); err != nil {
		return application.PublishedPattern{}, fmt.Errorf("publish pattern %s: index: %w", ref, err)
	}
	return application.PublishedPattern{
		VersionDir: filepath.Join(filepath.Clean(dir), filepath.FromSlash(distribution.VersionDir(ref))),
		IndexPath:  filepath.Join(filepath.Clean(dir), distribution.IndexFileName),
		Replaced:   replaced,
	}, nil
}

func readIndex(path string) (distribution.Index, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		index, err := distribution.NewIndex(nil)
		if err != nil {
			return distribution.Index{}, fmt.Errorf("new index: %w", err)
		}
		return index, nil
	}
	if err != nil {
		return distribution.Index{}, fmt.Errorf("read %s: %w", path, err)
	}
	index, err := patternfiles.DecodeIndex(data, path)
	if err != nil {
		return distribution.Index{}, fmt.Errorf("index: %w", err)
	}
	return index, nil
}

// writeVersion stages the version's files and Manifest beside the
// destination and renames the whole directory into place. Files are
// written owner-only, the mode every file arclint writes carries.
func writeVersion(versionDir string, v distribution.VendoredPattern) error {
	parent := filepath.Dir(versionDir)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", parent, err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(versionDir)+".staging-")
	if err != nil {
		return fmt.Errorf("stage %s: %w", versionDir, err)
	}
	// After the rename the staging path no longer exists; on failure
	// the partial directory is discarded and the error already on its
	// way out is the one to report.
	defer func() { _ = os.RemoveAll(staging) }()
	for _, f := range v.Files() {
		full := filepath.Join(staging, filepath.FromSlash(f.Path()))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, f.Data(), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", full, err)
		}
	}
	manifest, err := patternfiles.EncodeManifest(v.Manifest())
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	manifestPath := filepath.Join(staging, distribution.ManifestFileName)
	if err := os.WriteFile(manifestPath, manifest, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", manifestPath, err)
	}
	if err := os.RemoveAll(versionDir); err != nil {
		return fmt.Errorf("replace %s: %w", versionDir, err)
	}
	if err := os.Rename(staging, versionDir); err != nil {
		return fmt.Errorf("move %s into place: %w", versionDir, err)
	}
	return nil
}

// writeWhole replaces path through a temporary sibling so readers see
// either the previous document or the new one, leaving no temporary
// file behind on any failure.
func writeWhole(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp for %s: %w", path, err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp onto %s: %w", path, err)
	}
	cleanup = false
	return nil
}
