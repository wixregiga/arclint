package distribution_test

import (
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

func mustRef(t *testing.T, s string) rule.PatternReference {
	t.Helper()
	ref, err := rule.ParsePatternReference(s)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func mustFile(t *testing.T, path, data string) distribution.PatternFile {
	t.Helper()
	f, err := distribution.NewPatternFile(path, []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDigestSpellingRoundTrips(t *testing.T) {
	d := distribution.DigestOf([]byte("hello"))
	if !strings.HasPrefix(d.String(), "sha256:") || len(d.String()) != len("sha256:")+64 {
		t.Fatalf("spelling = %q", d)
	}
	parsed, err := distribution.ParseDigest(d.String())
	if err != nil || !parsed.Equals(d) {
		t.Fatalf("ParseDigest(%q) = (%v, %v)", d, parsed, err)
	}
	for _, bad := range []string{"", "md5:abc", "sha256:abc", "sha256:" + strings.ToUpper(d.String()[7:]), "sha256:" + strings.Repeat("g", 64)} {
		if _, err := distribution.ParseDigest(bad); err == nil {
			t.Errorf("ParseDigest(%q) accepted", bad)
		}
	}
	if d.Short() != d.String()[7:19] {
		t.Errorf("Short() = %q", d.Short())
	}
}

func TestPatternFileRejectsEscapingPaths(t *testing.T) {
	for _, bad := range []string{"", "/pattern.yaml", "../pattern.yaml", "extensions/../x.ts", "./pattern.yaml", "a//b", `ext\x.ts`, "c:/x", "extensions/", "a/./b"} {
		if _, err := distribution.NewPatternFile(bad, nil); err == nil {
			t.Errorf("NewPatternFile(%q) accepted", bad)
		}
	}
	f := mustFile(t, "extensions/check.ts", "export {}")
	data := f.Data()
	data[0] = 'X'
	if string(f.Data()) != "export {}" {
		t.Error("Data() must return a copy")
	}
}

func TestManifestDigestIsDeterministicAndOrderFree(t *testing.T) {
	ref := mustRef(t, "acme/hex@1.2.3")
	a := mustFile(t, "pattern.yaml", "pattern: {}")
	b := mustFile(t, "extensions/check.ts", "export default 1")
	m1, err := distribution.ManifestOf(ref, []distribution.PatternFile{a, b})
	if err != nil {
		t.Fatal(err)
	}
	m2, err := distribution.ManifestOf(ref, []distribution.PatternFile{b, a})
	if err != nil {
		t.Fatal(err)
	}
	if !m1.Digest().Equals(m2.Digest()) {
		t.Fatalf("digest depends on file order: %s vs %s", m1.Digest(), m2.Digest())
	}
	entries := m1.Entries()
	if len(entries) != 2 || entries[0].Path() != "extensions/check.ts" || entries[1].Path() != "pattern.yaml" {
		t.Fatalf("entries not in path order: %+v", entries)
	}
	// The whole-Pattern Digest is the hash of the canonical listing, so
	// an independent implementation can reproduce it from the entries.
	var listing strings.Builder
	for _, e := range entries {
		listing.WriteString(e.Digest().String() + "  " + e.Path() + "\n")
	}
	if want := distribution.DigestOf([]byte(listing.String())); !want.Equals(m1.Digest()) {
		t.Errorf("digest %s is not the hash of the canonical listing %s", m1.Digest(), want)
	}
	changed := mustFile(t, "extensions/check.ts", "export default 2")
	m3, err := distribution.ManifestOf(ref, []distribution.PatternFile{a, changed})
	if err != nil {
		t.Fatal(err)
	}
	if m3.Digest().Equals(m1.Digest()) {
		t.Error("changed bytes must change the digest")
	}
}

func TestManifestConstructionRejectsInconsistency(t *testing.T) {
	ref := mustRef(t, "acme/hex@1.2.3")
	doc := mustFile(t, "pattern.yaml", "pattern: {}")
	m, err := distribution.ManifestOf(ref, []distribution.PatternFile{doc})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := distribution.NewManifestEntry("pattern.yaml", doc.Digest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := distribution.NewManifest(ref, m.Digest(), []distribution.ManifestEntry{entry}); err != nil {
		t.Fatalf("reconstruction from recorded parts failed: %v", err)
	}
	wrong := distribution.DigestOf([]byte("other"))
	if _, err := distribution.NewManifest(ref, wrong, []distribution.ManifestEntry{entry}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Errorf("recorded digest mismatch accepted: %v", err)
	}
	if _, err := distribution.NewManifest(ref, m.Digest(), []distribution.ManifestEntry{entry, entry}); err == nil || !strings.Contains(err.Error(), "listed twice") {
		t.Errorf("duplicate entry accepted: %v", err)
	}
	other, _ := distribution.NewManifestEntry("extensions/x.ts", wrong)
	if _, err := distribution.NewManifest(ref, wrong, []distribution.ManifestEntry{other}); err == nil || !strings.Contains(err.Error(), "pattern.yaml is not listed") {
		t.Errorf("manifest without pattern.yaml accepted: %v", err)
	}
	if _, err := distribution.ManifestOf(ref, nil); err == nil {
		t.Error("empty manifest accepted")
	}
	if _, err := distribution.NewManifestEntry("../x", doc.Digest()); err == nil {
		t.Error("escaping entry accepted")
	}
}

func TestManifestVerifyNamesTheDrift(t *testing.T) {
	ref := mustRef(t, "acme/hex@1.2.3")
	doc := mustFile(t, "pattern.yaml", "pattern: {}")
	ext := mustFile(t, "extensions/check.ts", "export default 1")
	m, err := distribution.ManifestOf(ref, []distribution.PatternFile{doc, ext})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]distribution.PatternFile{
		`listed file "extensions/check.ts" is missing`: {doc},
		`file "extensions/check.ts" has digest`:        {doc, mustFile(t, "extensions/check.ts", "tampered")},
		`unlisted file(s) README.md`:                   {doc, ext, mustFile(t, "README.md", "hi")},
	}
	for want, files := range cases {
		err := m.Verify(files)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Verify = %v, want %q", err, want)
		}
	}
	if err := m.Verify([]distribution.PatternFile{ext, doc}); err != nil {
		t.Errorf("exact files rejected: %v", err)
	}
}

func TestVendoredPatternPairsFilesWithManifest(t *testing.T) {
	ref := mustRef(t, "acme/hex@1.2.3")
	doc := mustFile(t, "pattern.yaml", "pattern: {}")
	ext := mustFile(t, "extensions/check.ts", "export default 1")
	v, err := distribution.Vendor(ref, []distribution.PatternFile{ext, doc})
	if err != nil {
		t.Fatal(err)
	}
	if v.Reference() != ref || !v.Digest().Equals(v.Manifest().Digest()) {
		t.Errorf("vendored identity drifted: %s %s", v.Reference(), v.Digest())
	}
	if files := v.Files(); len(files) != 2 || files[0].Path() != "extensions/check.ts" {
		t.Errorf("files not in path order: %+v", files)
	}
	if f, ok := v.File("pattern.yaml"); !ok || string(f.Data()) != "pattern: {}" {
		t.Errorf("File(pattern.yaml) = (%v, %v)", f, ok)
	}
	if _, err := distribution.NewVendoredPattern(v.Manifest(), []distribution.PatternFile{doc}); err == nil {
		t.Error("manifest and files disagree but were accepted")
	}
	if _, err := distribution.NewVendoredPattern(distribution.Manifest{}, nil); err == nil {
		t.Error("zero manifest accepted")
	}
}

func TestRegistryLocations(t *testing.T) {
	r, err := distribution.NewRegistry("https://example.com/registry/")
	if err != nil {
		t.Fatal(err)
	}
	ref := mustRef(t, "acme/hex@1.2.3")
	if r.Location() != "https://example.com/registry" {
		t.Errorf("Location = %q", r.Location())
	}
	if r.IndexURL() != "https://example.com/registry/index.json" {
		t.Errorf("IndexURL = %q", r.IndexURL())
	}
	if r.ManifestURL(ref) != "https://example.com/registry/acme/hex/1.2.3/manifest.json" {
		t.Errorf("ManifestURL = %q", r.ManifestURL(ref))
	}
	if r.FileURL(ref, "extensions/check.ts") != "https://example.com/registry/acme/hex/1.2.3/extensions/check.ts" {
		t.Errorf("FileURL = %q", r.FileURL(ref, "extensions/check.ts"))
	}
	if _, err := distribution.NewRegistry("file:///tmp/registry"); err != nil {
		t.Errorf("file registry rejected: %v", err)
	}
	for _, bad := range []string{"", "ftp://x", "https://", "file://", "https://example.com/r?x=1", "not a url"} {
		if _, err := distribution.NewRegistry(bad); err == nil {
			t.Errorf("NewRegistry(%q) accepted", bad)
		}
	}
	if _, err := distribution.NewRegistry(distribution.DefaultRegistryLocation); err != nil {
		t.Errorf("default registry invalid: %v", err)
	}
}

func available(t *testing.T, kind distribution.SourceKind, ref, doc string) distribution.Available {
	t.Helper()
	r := mustRef(t, ref)
	files := []distribution.PatternFile{mustFile(t, "pattern.yaml", doc)}
	v, err := distribution.Vendor(r, files)
	if err != nil {
		t.Fatal(err)
	}
	spec := rule.PatternSpec{Namespace: r.Namespace(), Name: r.Name(), Version: r.Version()}
	mod, err := rule.NewPatternModule("core", "the core", nil)
	if err != nil {
		t.Fatal(err)
	}
	spec.Modules = []rule.PatternModule{mod}
	internal, err := rule.NewAllowList()
	if err != nil {
		t.Fatal(err)
	}
	scope, err := rule.ModuleApplicability([]rule.ModuleName{"core"})
	if err != nil {
		t.Fatal(err)
	}
	rl, err := rule.New(rule.Spec{
		ID:            r.Namespace() + ":core/stdlib-only",
		Type:          rule.TypeConsumes,
		Params:        rule.ConsumesParams{Internal: &internal, External: rule.ImportForbid},
		Applicability: scope,
	})
	if err != nil {
		t.Fatal(err)
	}
	spec.Rules = []rule.Rule{rl}
	p, err := rule.NewPattern(spec)
	if err != nil {
		t.Fatal(err)
	}
	a, err := distribution.NewAvailable(kind, p, v, false)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestCatalogDeduplicatesByReferenceAndRejectsDrift(t *testing.T) {
	embedded := available(t, distribution.SourceEmbedded, "arclint/vertical@0.1.0", "a")
	sameLocal := available(t, distribution.SourceLocal, "arclint/vertical@0.1.0", "a")
	driftedLocal := available(t, distribution.SourceLocal, "arclint/vertical@0.1.0", "b")
	other := available(t, distribution.SourceLocal, "acme/hex@1.0.0", "c")

	c, err := distribution.NewCatalog([]distribution.Available{embedded}, []distribution.Available{sameLocal, other})
	if err != nil {
		t.Fatal(err)
	}
	entries := c.Entries()
	if len(entries) != 2 || entries[0].Kind != distribution.SourceEmbedded || entries[1].Reference().Name() != "hex" {
		t.Fatalf("entries = %+v", entries)
	}
	if got, ok := c.Lookup(mustRef(t, "acme/hex@1.0.0")); !ok || got.Kind != distribution.SourceLocal {
		t.Errorf("Lookup = (%+v, %v)", got, ok)
	}
	copies := c.Copies(mustRef(t, "arclint/vertical@0.1.0"))
	if len(copies) != 2 || copies[0].Kind != distribution.SourceEmbedded || copies[1].Kind != distribution.SourceLocal {
		t.Errorf("Copies = %+v, want the embedded copy first, then the agreeing local one", copies)
	}
	if copies := c.Copies(mustRef(t, "acme/hex@1.0.0")); len(copies) != 1 {
		t.Errorf("Copies of a single-source reference = %+v", copies)
	}
	if copies := c.Copies(mustRef(t, "acme/none@1.0.0")); copies != nil {
		t.Errorf("Copies of an unknown reference = %+v", copies)
	}
	if _, err := distribution.NewCatalog([]distribution.Available{embedded}, []distribution.Available{driftedLocal}); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Errorf("drifted duplicate accepted: %v", err)
	}
}

func TestSelectionSpellings(t *testing.T) {
	refs := []rule.PatternReference{
		mustRef(t, "arclint/vertical@0.1.0"),
		mustRef(t, "arclint/vertical@0.2.0-rc.1"),
		mustRef(t, "arclint/vertical@0.1.5"),
		mustRef(t, "acme/vertical@3.0.0"),
		mustRef(t, "acme/hex@1.0.0"),
	}
	exact, err := distribution.Selection("arclint/vertical@0.1.5", refs)
	if err != nil || len(exact) != 1 || exact[0].Version() != "0.1.5" {
		t.Fatalf("exact = (%v, %v)", exact, err)
	}
	missing, err := distribution.Selection("arclint/vertical@9.9.9", refs)
	if err != nil || missing != nil {
		t.Fatalf("missing exact = (%v, %v)", missing, err)
	}
	byName, err := distribution.Selection("arclint/vertical", refs)
	if err != nil || len(byName) != 1 || byName[0].Version() != "0.2.0-rc.1" {
		t.Fatalf("namespace/name = (%v, %v), want the highest version", byName, err)
	}
	bare, err := distribution.Selection("vertical", refs)
	if err != nil || len(bare) != 2 {
		t.Fatalf("bare name = (%v, %v), want one per namespace", bare, err)
	}
	unique, err := distribution.Selection("hex", refs)
	if err != nil || len(unique) != 1 || unique[0].Namespace() != "acme" {
		t.Fatalf("unique bare = (%v, %v)", unique, err)
	}
	for _, bad := range []string{"", "a/b/c", "/x", "x/", "arclint/vertical@nope"} {
		if _, err := distribution.Selection(bad, refs); err == nil {
			t.Errorf("Selection(%q) accepted", bad)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.10.0", "1.9.0", 1},
		{"2.0.0", "1.99.99", 1},
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},
		{"1.0.0-beta.2", "1.0.0-beta.11", -1},
		{"1.0.0-rc.1", "1.0.0-beta.11", 1},
		{"1.0.0+build.1", "1.0.0+build.2", 0},
	}
	for _, c := range cases {
		if got := distribution.CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := distribution.CompareVersions(c.b, c.a); got != -c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.b, c.a, got, -c.want)
		}
	}
}
