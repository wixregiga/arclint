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

// SuppliedSource is one extension entry handed to the host as bytes
// rather than discovered on disk: the Extension sources an extended
// Pattern distributes. Name is the attribution every diagnostic and
// inventory line carries; it is not a filesystem path.
type SuppliedSource struct {
	Name   string
	Source string
}

// LoadDir discovers .arclint/extensions/*.ts|*.js under repoRoot,
// deduplicates by real path, transpiles each in-process, and runs the
// registration phase. A missing directory yields an empty registry.
func LoadDir(repoRoot string, opts Options) (*Registry, error) {
	return Load(repoRoot, nil, opts)
}

// Load registers the supplied sources first, in the given order, then
// the repository's own extensions under .arclint/extensions. A rule
// type registered twice is an error naming both sources: a local
// extension never silently shadows one a Pattern distributes.
func Load(repoRoot string, supplied []SuppliedSource, opts Options) (*Registry, error) {
	opts.fill()
	reg := &Registry{types: map[string]*RuleType{}}
	for _, s := range supplied {
		bundle, err := transpileSource(s, opts.Version, opts.CacheDir)
		if err != nil {
			return nil, err
		}
		if err := reg.register(s.Name, bundle, opts); err != nil {
			return nil, err
		}
	}
	return loadDir(repoRoot, reg, opts)
}

func loadDir(repoRoot string, reg *Registry, opts Options) (*Registry, error) {
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
// (recursively; helpers in subdirectories are bundle inputs too), plus
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
	stem := strings.TrimSuffix(filepath.Base(entry), filepath.Ext(entry))
	return cachedBundle(cacheDir, stem+"-"+dirHash, func() (string, error) {
		return bundle(filepath.Base(entry), esbuild.BuildOptions{
			EntryPoints: []string{entry},
			Plugins:     []esbuild.Plugin{sdkPlugin(true)},
		})
	})
}

// transpileSource bundles one supplied source held in memory. A
// supplied source is a single file: it may import the SDK, nothing
// else. Results are cached by a fingerprint of the bytes, the build
// version, and the SDK.
func transpileSource(s SuppliedSource, version, cacheDir string) (string, error) {
	h := sha256.New()
	h.Write([]byte(version))
	h.Write([]byte(sdkSource))
	h.Write([]byte(s.Name))
	h.Write([]byte(s.Source))
	key := "supplied-" + hex.EncodeToString(h.Sum(nil))[:16]
	loader := esbuild.LoaderTS
	if strings.HasSuffix(s.Name, ".js") {
		loader = esbuild.LoaderJS
	}
	return cachedBundle(cacheDir, key, func() (string, error) {
		return bundle(s.Name, esbuild.BuildOptions{
			Stdin: &esbuild.StdinOptions{
				Contents:   s.Source,
				Sourcefile: s.Name,
				Loader:     loader,
			},
			Plugins: []esbuild.Plugin{sdkPlugin(false)},
		})
	})
}

// cachedBundle returns the cached bundle under key, or produces and
// stores it. A missing cache directory disables caching.
func cachedBundle(cacheDir, key string, produce func() (string, error)) (string, error) {
	var cachePath string
	if cacheDir != "" {
		cachePath = filepath.Join(cacheDir, "extensions", key+".js")
		if data, err := os.ReadFile(cachePath); err == nil {
			return string(data), nil
		}
	}
	js, err := produce()
	if err != nil {
		return "", err
	}
	if cachePath != "" {
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o750); err == nil {
			_ = os.WriteFile(cachePath, []byte(js), 0o600)
		}
	}
	return js, nil
}

// sdkPlugin resolves "arclint" to the embedded SDK and rejects bare
// npm specifiers. Relative imports bundle from disk for discovered
// entries; a supplied (in-memory) source has no directory to resolve
// them against and rejects them with a clear message.
func sdkPlugin(allowRelative bool) esbuild.Plugin {
	return esbuild.Plugin{
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
			// specifier: extensions are single-file (plus relative
			// imports); reject with a clear error instead of a resolve
			// failure.
			pb.OnResolve(esbuild.OnResolveOptions{Filter: `^[^./]`},
				func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
					return esbuild.OnResolveResult{}, fmt.Errorf(
						"bare import %q is not supported: extensions bundle relative imports only, and the SDK is available as \"arclint\"", args.Path)
				})
			if !allowRelative {
				pb.OnResolve(esbuild.OnResolveOptions{Filter: `^[./]`},
					func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
						return esbuild.OnResolveResult{}, fmt.Errorf(
							"import %q is not supported: a pattern extension is one self-contained file, and the SDK is available as \"arclint\"", args.Path)
					})
			}
		},
	}
}

// bundle runs esbuild with the shared output settings and returns the
// single CommonJS bundle.
func bundle(name string, options esbuild.BuildOptions) (string, error) {
	options.Bundle = true
	options.Write = false
	options.Format = esbuild.FormatCommonJS
	options.Platform = esbuild.PlatformNeutral
	options.Target = esbuild.ES2017
	options.LogLevel = esbuild.LogLevelSilent
	result := esbuild.Build(options)
	if len(result.Errors) > 0 {
		msgs := make([]string, 0, len(result.Errors))
		for _, m := range result.Errors {
			loc := ""
			if m.Location != nil {
				loc = fmt.Sprintf("%s:%d: ", m.Location.File, m.Location.Line)
			}
			msgs = append(msgs, loc+m.Text)
		}
		return "", fmt.Errorf("extension %s: %s", name, strings.Join(msgs, "; "))
	}
	if len(result.OutputFiles) != 1 {
		return "", fmt.Errorf("extension %s: expected one bundle, got %d", name, len(result.OutputFiles))
	}
	return string(result.OutputFiles[0].Contents), nil
}
