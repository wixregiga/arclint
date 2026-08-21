package sobekextension

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"

	"github.com/wixregiga/arclint/internal/domain/rule"
)

//go:embed sdk/arclint.ts
var sdkSource string

// ExtensionsDir is the discovery root, relative to the repo root.
const ExtensionsDir = ".arclint/extensions"

// Options tunes loading and execution.
type Options struct {
	// CheckTimeout bounds one check() invocation (interrupt-based).
	// Zero means the 5s default.
	CheckTimeout time.Duration
	// RegisterTimeout bounds one extension's registration phase.
	// Zero means the 5s default.
	RegisterTimeout time.Duration
	// CacheDir persists transpile results ("" disables).
	CacheDir string
	// Version participates in the transpile cache key.
	Version string
}

const (
	defaultTimeout = 5 * time.Second
	sdkNamespace   = "arclint-sdk"
)

func (o *Options) fill() {
	if o.CheckTimeout == 0 {
		o.CheckTimeout = defaultTimeout
	}
	if o.RegisterTimeout == 0 {
		o.RegisterTimeout = defaultTimeout
	}
	if o.Version == "" {
		// Build info salts the transpile cache: a new arclint (or a new
		// esbuild dependency) invalidates cached bundles.
		if bi, ok := debug.ReadBuildInfo(); ok {
			o.Version = bi.String()
		}
	}
}

// Registry holds the rule types registered by every discovered extension.
type Registry struct {
	types map[string]*RuleType
	order []string
}

// Empty reports whether no rule types are registered.
func (r *Registry) Empty() bool { return r == nil || len(r.order) == 0 }

// Get returns a rule type by name, or nil.
func (r *Registry) Get(name string) *RuleType {
	if r == nil {
		return nil
	}
	return r.types[name]
}

// Types returns every rule type in registration order.
func (r *Registry) Types() []*RuleType {
	if r == nil {
		return nil
	}
	out := make([]*RuleType, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.types[name])
	}
	return out
}

// LoadDir discovers .arclint/extensions/*.ts|*.js under repoRoot,
// deduplicates by real path, transpiles each in-process, and runs the
// registration phase. A missing directory yields an empty registry.
func LoadDir(repoRoot string, opts Options) (*Registry, error) {
	opts.fill()
	reg := &Registry{types: map[string]*RuleType{}}

	dir := filepath.Join(repoRoot, filepath.FromSlash(ExtensionsDir))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return reg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("extensions: %w", err)
	}

	// Discovery is top-level only: every *.ts/*.js directly under the
	// extensions directory is one extension entry. Shared helpers live in
	// subdirectories and are pulled in via relative imports.
	var files []string
	seenReal := map[string]bool{}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		if !rule.InstallableExtensionFileName(name) {
			continue
		}
		abs := filepath.Join(dir, name)
		realPath, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return nil, fmt.Errorf("extensions: %s: %w", name, err)
		}
		if seenReal[realPath] {
			continue
		}
		seenReal[realPath] = true
		files = append(files, abs)
	}

	dirHash := hashDir(dir, opts.Version)
	for _, file := range files {
		bundle, err := transpile(file, dirHash, opts.CacheDir)
		if err != nil {
			return nil, err
		}
		rel := filepath.ToSlash(filepath.Join(ExtensionsDir, filepath.Base(file)))
		if err := reg.register(rel, bundle, opts); err != nil {
			return nil, err
		}
	}
	return reg, nil
}

// hashDir fingerprints every file under the extensions directory
// (recursively — helpers in subdirectories are bundle inputs too), plus
// the build version and the embedded SDK, so any change invalidates
// cached bundles.
func hashDir(dir, version string) string {
	h := sha256.New()
	h.Write([]byte(version))
	h.Write([]byte(sdkSource))
	var paths []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			paths = append(paths, p)
		}
		return nil
	})
	sort.Strings(paths)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		h.Write([]byte(filepath.ToSlash(strings.TrimPrefix(p, dir))))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// transpile bundles one extension entry: relative imports are bundled,
// "arclint" resolves to the embedded SDK, bare npm imports are rejected
// with a clear error. Results are cached by directory fingerprint.
func transpile(entry, dirHash, cacheDir string) (string, error) {
	var cachePath string
	if cacheDir != "" {
		stem := strings.TrimSuffix(filepath.Base(entry), filepath.Ext(entry))
		cachePath = filepath.Join(cacheDir, "extensions", stem+"-"+dirHash+".js")
		if data, err := os.ReadFile(cachePath); err == nil {
			return string(data), nil
		}
	}

	sdkPlugin := esbuild.Plugin{
		Name: sdkNamespace,
		Setup: func(pb esbuild.PluginBuild) {
			pb.OnResolve(esbuild.OnResolveOptions{Filter: `^arclint$`},
				func(_ esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
					return esbuild.OnResolveResult{Path: "arclint", Namespace: sdkNamespace}, nil
				})
			pb.OnLoad(esbuild.OnLoadOptions{Filter: `.*`, Namespace: sdkNamespace},
				func(_ esbuild.OnLoadArgs) (esbuild.OnLoadResult, error) {
					contents := sdkSource
					return esbuild.OnLoadResult{Contents: &contents, Loader: esbuild.LoaderTS}, nil
				})
			// Anything else that is not relative or absolute is a bare npm
			// specifier: v1 extensions are single-file (plus relative
			// imports); reject with a clear error instead of a resolve
			// failure.
			pb.OnResolve(esbuild.OnResolveOptions{Filter: `^[^./]`},
				func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
					return esbuild.OnResolveResult{}, fmt.Errorf(
						"bare import %q is not supported: extensions bundle relative imports only, and the SDK is available as \"arclint\"", args.Path)
				})
		},
	}

	result := esbuild.Build(esbuild.BuildOptions{
		EntryPoints: []string{entry},
		Bundle:      true,
		Write:       false,
		Format:      esbuild.FormatCommonJS,
		Platform:    esbuild.PlatformNeutral,
		Target:      esbuild.ES2017,
		Plugins:     []esbuild.Plugin{sdkPlugin},
		LogLevel:    esbuild.LogLevelSilent,
	})
	if len(result.Errors) > 0 {
		var msgs []string
		for _, m := range result.Errors {
			loc := ""
			if m.Location != nil {
				loc = fmt.Sprintf("%s:%d: ", m.Location.File, m.Location.Line)
			}
			msgs = append(msgs, loc+m.Text)
		}
		return "", fmt.Errorf("extension %s: %s", filepath.Base(entry), strings.Join(msgs, "; "))
	}
	if len(result.OutputFiles) != 1 {
		return "", fmt.Errorf("extension %s: expected one bundle, got %d", filepath.Base(entry), len(result.OutputFiles))
	}
	js := string(result.OutputFiles[0].Contents)

	if cachePath != "" {
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o750); err == nil {
			_ = os.WriteFile(cachePath, []byte(js), 0o600)
		}
	}
	return js, nil
}
