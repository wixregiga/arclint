package application_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/wixregiga/arclint/internal/application"
	"github.com/wixregiga/arclint/internal/domain/distribution"
	"github.com/wixregiga/arclint/internal/domain/rule"
)

// fakeRegistry publishes a fixed set of Patterns and records what was
// read, so tests prove the network is consulted only when needed.
type fakeRegistry struct {
	published []rule.Pattern
	indexed   int
	fetched   []string
	// drift serves files on fetch that differ from what the index
	// recorded, the way a tampered or half-updated registry would.
	drift bool
}

func (f *fakeRegistry) Index(reg distribution.Registry) (distribution.Index, error) {
	f.indexed++
	entries := make([]distribution.IndexEntry, 0, len(f.published))
	for _, p := range f.published {
		a, err := availableFixture(distribution.SourceRegistry, false, p, "published: "+p.Reference().String()+"\n")
		if err != nil {
			return distribution.Index{}, err
		}
		e, err := distribution.IndexEntryOf(a)
		if err != nil {
			return distribution.Index{}, err
		}
		entries = append(entries, e)
	}
	return distribution.NewIndex(entries)
}

func (f *fakeRegistry) Fetch(reg distribution.Registry, ref rule.PatternReference) (distribution.Available, error) {
	f.fetched = append(f.fetched, ref.String())
	for _, p := range f.published {
		if p.Reference() == ref {
			doc := "published: " + ref.String() + "\n"
			if f.drift {
				doc += "# drifted\n"
			}
			return availableFixture(distribution.SourceRegistry, false, p, doc)
		}
	}
	return distribution.Available{}, fmt.Errorf("%s not published", ref)
}

type fakeStore struct {
	written  []distribution.VendoredPattern
	replaced string
}

func (f *fakeStore) Write(v distribution.VendoredPattern) (application.StoredPattern, error) {
	f.written = append(f.written, v)
	return application.StoredPattern{
		Path:     ".arclint/patterns/" + v.Reference().Namespace() + "/" + v.Reference().Name(),
		Replaced: f.replaced,
	}, nil
}

type fakeEditor struct {
	exists   bool
	replaced string
	extended []rule.Installation
}

func (f *fakeEditor) Exists() (bool, error) { return f.exists, nil }

func (f *fakeEditor) Extend(inst rule.Installation) (application.RulesetChange, error) {
	f.extended = append(f.extended, inst)
	return application.RulesetChange{Path: "rules.arclint.yaml", Replaced: f.replaced}, nil
}

type fakeRegistryPublisher struct {
	dirs []string
	refs []string
}

func (f *fakeRegistryPublisher) Publish(dir string, a distribution.Available) (application.PublishedPattern, error) {
	f.dirs = append(f.dirs, dir)
	f.refs = append(f.refs, a.Reference().String())
	return application.PublishedPattern{
		VersionDir: dir + "/" + distribution.VersionDir(a.Reference()),
		IndexPath:  dir + "/" + distribution.IndexFileName,
	}, nil
}

func TestVendorPatternPrefersOfflineSources(t *testing.T) {
	embedded := patternFixture(t, "1.0.0")
	published := patternFixture(t, "2.0.0")
	registry := &fakeRegistry{published: []rule.Pattern{published}}
	store := &fakeStore{replaced: "0.9.0"}
	uc, err := application.NewVendorPattern(store, registry, fakePatternSource{patterns: []rule.Pattern{embedded}})
	if err != nil {
		t.Fatalf("NewVendorPattern: %v", err)
	}
	result, err := uc.Execute(application.VendorPatternRequest{Selection: "ddd-flat", Registry: "https://example.test/registry"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Reference != "arclint/ddd-flat@1.0.0" || result.Source != distribution.SourceEmbedded ||
		result.Path != ".arclint/patterns/arclint/ddd-flat" || result.Replaced != "0.9.0" || result.Unchanged {
		t.Errorf("result = %+v", result)
	}
	if registry.indexed != 0 || len(registry.fetched) != 0 {
		t.Errorf("an embedded pattern must vendor without touching the registry: %+v", registry)
	}
	if len(store.written) != 1 || store.written[0].Reference() != embedded.Reference() {
		t.Errorf("store received %+v", store.written)
	}

	result, err = uc.Execute(application.VendorPatternRequest{Selection: "arclint/ddd-flat@2.0.0", Registry: "https://example.test/registry"})
	if err != nil {
		t.Fatalf("Execute from registry: %v", err)
	}
	if result.Source != distribution.SourceRegistry || registry.indexed != 1 || len(registry.fetched) != 1 {
		t.Errorf("registry vendoring: result %+v, registry %+v", result, registry)
	}
	if _, err := uc.Execute(application.VendorPatternRequest{Selection: "arclint/ddd-flat@2.0.0"}); err == nil ||
		!strings.Contains(err.Error(), "not under .arclint/patterns") {
		t.Errorf("offline vendoring of an unpublished version must fail with guidance, got %v", err)
	}
	if _, err := uc.Execute(application.VendorPatternRequest{Selection: "nope", Registry: "https://example.test/registry"}); err == nil ||
		!strings.Contains(err.Error(), "patterns --remote") {
		t.Errorf("unknown selection must point at the remote listing, got %v", err)
	}
	registry.drift = true
	if _, err := uc.Execute(application.VendorPatternRequest{Selection: "arclint/ddd-flat@2.0.0", Registry: "https://example.test/registry"}); err == nil ||
		!strings.Contains(err.Error(), "index records") {
		t.Errorf("a fetch whose digest disagrees with the index must be refused, got %v", err)
	}
}

func TestVendorPatternLeavesAnIdenticalCopyAlone(t *testing.T) {
	p := patternFixture(t, "1.0.0")
	store := &fakeStore{}
	vendored := fakePatternSource{kind: distribution.SourceLocal, patterns: []rule.Pattern{p}}
	uc, err := application.NewVendorPattern(store, nil, vendored)
	if err != nil {
		t.Fatalf("NewVendorPattern: %v", err)
	}
	result, err := uc.Execute(application.VendorPatternRequest{Selection: "arclint/ddd-flat"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Unchanged || result.Path != "" || len(store.written) != 0 {
		t.Errorf("a vendored copy must be left alone; result %+v, written %d", result, len(store.written))
	}
	authored := fakePatternSource{kind: distribution.SourceLocal, authored: true, patterns: []rule.Pattern{p}}
	uc, err = application.NewVendorPattern(store, nil, authored)
	if err != nil {
		t.Fatalf("NewVendorPattern: %v", err)
	}
	result, err = uc.Execute(application.VendorPatternRequest{Selection: "arclint/ddd-flat"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Unchanged || len(store.written) != 1 {
		t.Errorf("an authored local pattern is pinned by writing its manifest; result %+v, written %d", result, len(store.written))
	}

	store = &fakeStore{}
	embedded := fakePatternSource{kind: distribution.SourceEmbedded, patterns: []rule.Pattern{p}}
	uc, err = application.NewVendorPattern(store, nil, embedded, vendored)
	if err != nil {
		t.Fatalf("NewVendorPattern: %v", err)
	}
	result, err = uc.Execute(application.VendorPatternRequest{Selection: "ddd-flat"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Unchanged || result.Source != distribution.SourceEmbedded || len(store.written) != 0 {
		t.Errorf("an embedded pattern already vendored under .arclint/patterns is left alone; result %+v, written %d", result, len(store.written))
	}
	list, err := application.NewListPatterns(nil, embedded, vendored)
	if err != nil {
		t.Fatalf("NewListPatterns: %v", err)
	}
	rows, err := list.Execute()
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rows) != 1 || rows[0].Source != distribution.SourceEmbedded || !rows[0].Vendored || rows[0].Authored {
		t.Errorf("the listing must say the embedded pattern is also vendored: %+v", rows)
	}
}

func TestInstallPatternExtendsAnExistingRuleset(t *testing.T) {
	published := patternFixture(t, "2.0.0")
	registry := &fakeRegistry{published: []rule.Pattern{published}}
	store := &fakeStore{}
	editor := &fakeEditor{exists: true, replaced: "1.0.0"}
	scaffold := &recordingScaffold{}
	uc, err := application.NewInstallPattern(store, editor, scaffold, registry, fakePatternSource{})
	if err != nil {
		t.Fatalf("NewInstallPattern: %v", err)
	}
	result, err := uc.Execute(application.InstallPatternRequest{Selection: "arclint/ddd-flat", Registry: "file:///registry"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Reference != "arclint/ddd-flat@2.0.0" || result.Source != distribution.SourceRegistry ||
		result.VendoredPath != ".arclint/patterns/arclint/ddd-flat" || result.RulesetPath != "rules.arclint.yaml" ||
		result.RulesetCreated || result.RulesetReplaced != "1.0.0" {
		t.Errorf("result = %+v", result)
	}
	if len(result.Bound) != 1 || result.Bound[0].Module != "m" || result.Bound[0].Paths[0] != "src/m/**" {
		t.Errorf("bound = %+v", result.Bound)
	}
	if len(result.Unbound) != 1 || result.Unbound[0] != "unbound" {
		t.Errorf("unbound = %+v", result.Unbound)
	}
	if len(store.written) != 1 || len(editor.extended) != 1 || editor.extended[0].Reference() != published.Reference() {
		t.Errorf("store %+v editor %+v", store.written, editor.extended)
	}
	if scaffold.content != "" {
		t.Errorf("an existing ruleset is edited, never rewritten")
	}
}

func TestInstallPatternDraftsARulesetWhenNoneExists(t *testing.T) {
	embedded := patternFixture(t, "1.0.0")
	store := &fakeStore{}
	editor := &fakeEditor{exists: false}
	scaffold := &recordingScaffold{}
	uc, err := application.NewInstallPattern(store, editor, scaffold, nil, fakePatternSource{patterns: []rule.Pattern{embedded}})
	if err != nil {
		t.Fatalf("NewInstallPattern: %v", err)
	}
	result, err := uc.Execute(application.InstallPatternRequest{Selection: "ddd-flat"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.RulesetCreated || result.VendoredPath != "" || len(store.written) != 0 || len(editor.extended) != 0 {
		t.Errorf("result = %+v store %+v editor %+v", result, store.written, editor.extended)
	}
	for _, want := range []string{"runtime: [go]\n", "extends:\n  - pattern: arclint/ddd-flat@1.0.0\n", "      m: \"src/m/**\"\n", "      # unbound: <glob>\n"} {
		if !strings.Contains(scaffold.content, want) {
			t.Errorf("drafted ruleset lacks %q:\n%s", want, scaffold.content)
		}
	}
	if _, err := uc.Execute(application.InstallPatternRequest{Selection: "ddd-flat", Languages: []string{"cobol"}}); err == nil {
		t.Error("unsupported language accepted")
	}
	if _, err := application.NewInstallPattern(nil, editor, scaffold, nil, fakePatternSource{}); err == nil {
		t.Error("missing store accepted")
	}
}

func TestExportPatternPublishesOfflinePatterns(t *testing.T) {
	embedded := patternFixture(t, "1.0.0")
	publisher := &fakeRegistryPublisher{}
	uc, err := application.NewExportPattern(publisher, fakePatternSource{patterns: []rule.Pattern{embedded}})
	if err != nil {
		t.Fatalf("NewExportPattern: %v", err)
	}
	result, err := uc.Execute(application.ExportPatternRequest{Selection: "ddd-flat", Dir: "/tmp/registry"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.VersionDir != "/tmp/registry/arclint/ddd-flat/1.0.0" || result.IndexPath != "/tmp/registry/index.json" ||
		result.Reference != "arclint/ddd-flat@1.0.0" {
		t.Errorf("result = %+v", result)
	}
	if _, err := uc.Execute(application.ExportPatternRequest{Selection: "ddd-flat"}); err == nil {
		t.Error("missing directory accepted")
	}
	if _, err := uc.Execute(application.ExportPatternRequest{Selection: "nope", Dir: "/tmp/registry"}); err == nil {
		t.Error("unknown pattern accepted")
	}
}
