// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.


package llamacpp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/llm/llamacpp/ociregistry"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// defaultContextWindow is reported on a ModelEntry when we cannot read
// the real training context from the GGUF header. 8k is small enough to
// be safe for essentially every model on the Hub.
const defaultContextWindow = 8192

// LLMProvider identifies locally-served GGUF models in the model registry.
// Entries carry this on ModelEntry.Provider so the router dispatches them to
// the managed llama-server backend.
const LLMProvider = "llamacpp"

// ErrNotGGUFRepo and ErrNotFound are re-exports of the underlying
// ociregistry errors so callers outside this package do not have to
// import ociregistry directly. Tests and the shell's error formatting
// match on these values.
var (
	ErrNotGGUFRepo = ociregistry.ErrNotGGUFRepo
	ErrNotFound    = ociregistry.ErrNotFound
)

// DefaultRegistryHost is used when a reference passed to Download /
// Delete has no explicit host component. We default to Hugging Face
// because that's the registry the local GGUF loader is tuned for.
const DefaultRegistryHost = "huggingface.co"

// Reference identifies a model in an OCI registry. It is a thin alias
// for ociregistry.Reference so callers of this package never import
// the transport layer directly.
type Reference = ociregistry.Reference

// ProgressFunc is invoked repeatedly while a Download is in progress.
// `downloaded` is the cumulative bytes received across all layers and
// `total` is the manifest total. Either may be zero early in a pull
// before the manifest has been fetched.
type ProgressFunc func(downloaded, total int64)

// DownloadResult is returned by Registry.Download once a model has been
// fully pulled. ModelPath is the absolute on-disk path of the weight
// blob so callers can serve it via the llama-server backend.
type DownloadResult struct {
	Reference Reference
	ModelPath string
	// ProjectorPath is the absolute on-disk path of the optional mmproj GGUF.
	ProjectorPath string
	// LayerSizes enumerates every non-model layer in the manifest along
	// with its on-disk path and size, ordered as in the manifest. The
	// shell uses this to render a post-download summary.
	LayerSizes []LayerInfo
}

// LayerInfo describes one pulled layer. MediaType matches the OCI
// layer's media type (e.g. ociregistry.MediaTypeOllamaTemplate).
type LayerInfo struct {
	MediaType string
	Path      string
	Size      int64
}

// Downloader is the subset of ociregistry.Client that Registry depends
// on. Tests substitute a stub; production always uses
// ociregistry.NewClient(). The indirection exists purely to keep
// transport-less unit tests cheap.
type Downloader interface {
	Pull(
		ctx context.Context,
		cache *ociregistry.Cache,
		ref ociregistry.Reference,
		progress ociregistry.ProgressFunc,
	) (*ociregistry.PullResult, error)
}

// Registry is the single entry point for managing locally downloaded
// GGUF models. It wraps an Ollama-compatible OCI cache so callers
// never have to touch ociregistry directly, and it owns a persistent
// context-window metadata cache (keyed by blob digest) backed by a
// storageapi.Service so values survive restarts.
//
// Consistency guarantees:
//   - Models() and Get() only surface manifests whose weight blob is
//     fully present on disk. Interrupted pulls are hidden.
//   - Delete() removes the manifest, prunes every layer blob that no
//     other manifest still references, and clears any persistent
//     context-window cache entry for the pruned blobs. On-disk and
//     persisted state stay in sync by construction.
type Registry struct {
	cache      *ociregistry.Cache
	downloader Downloader

	// storage persists the context-window metadata cache across runs.
	// Keyed by blob digest. May be nil — in that case the in-memory
	// cache alone is used and metadata is re-read from every GGUF on
	// startup. Writes to the persistent store are best-effort: a
	// backend error is ignored and the value is served from memory.
	storage   storageapi.Service
	storageMu sync.Mutex

	// contextWindowOverride, when > 0, replaces the GGUF-derived value
	// on every ModelEntry produced by Models/Get. Intended for tests
	// and for runtime-context override paths.
	contextWindowOverride int

	// memCache is a best-effort in-memory mirror of the persistent
	// store. A Get call prefers memCache; a miss falls through to
	// storage; a miss there falls through to parsing the GGUF header.
	memCacheMu sync.Mutex
	memCache   map[string]int // digest -> context length
}

// Option configures a Registry.
type Option func(*Registry)

// WithStorage wires a persistent storageapi.Service to cache
// GGUF-derived metadata (currently just context windows) across
// process restarts. Records are keyed by blob digest so reusing the
// same cache across models is a content-addressed lookup — no
// manifest-path juggling required.
//
// The service is typically obtained as
//
//	storage.Partition("llamacpp-registry")
//
// to keep the records isolated from other partitions. Registry does
// not call Partition itself so callers retain control of the
// namespace.
func WithStorage(svc storageapi.Service) Option {
	return func(r *Registry) { r.storage = svc }
}

// WithDownloader overrides the OCI client used to fetch manifests and
// layers. Production callers should not set this; it exists so that
// tests can inject a fake transport without spinning up an HTTP
// server.
func WithDownloader(d Downloader) Option {
	return func(r *Registry) { r.downloader = d }
}

// WithContextWindow forces the ContextWindow reported by Models/Get to
// a specific value regardless of what the GGUF metadata advertises.
// Intended for tests and for callers that want a smaller runtime
// context than the model was trained with.
func WithContextWindow(n int) Option {
	return func(r *Registry) { r.contextWindowOverride = n }
}

// NewRegistry opens (or creates) an Ollama-compatible OCI cache at
// root and returns a Registry that manages it. root must be
// non-empty; callers that want the legacy default path should call
// DefaultModelsDir().
func NewRegistry(root string, opts ...Option) (*Registry, error) {
	if root == "" {
		return nil, errors.New("llamacpp: registry root path is empty")
	}
	cache, err := ociregistry.OpenCache(root)
	if err != nil {
		return nil, fmt.Errorf("llamacpp: open cache: %w", err)
	}
	r := &Registry{
		cache:    cache,
		memCache: make(map[string]int),
	}
	for _, o := range opts {
		o(r)
	}
	if r.downloader == nil {
		r.downloader = ociregistry.NewClient()
	}
	return r, nil
}

// DefaultModelsDir returns the conventional local GGUF cache path
// ($XDG_DATA_HOME/llama/models or ~/.local/share/llama/models). The
// extension supplies its own DataDir-based path in production; this
// helper exists for standalone CLI usage.
func DefaultModelsDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "llama", "models")
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return "models"
	}
	return filepath.Join(home, ".local", "share", "llama", "models")
}

// Root returns the cache root path. Exposed so tests can seed
// manifests/blobs directly under a known location.
func (r *Registry) Root() string { return r.cache.Root() }

// ParseReference parses s into a Reference, defaulting the host to
// DefaultRegistryHost when none is present.
func ParseReference(s string) (Reference, error) {
	return ociregistry.ParseReferenceWithDefault(s, DefaultRegistryHost)
}

// Models returns every model for which a manifest and its weight blob
// are fully present on disk. Scans the cache on every call so newly
// downloaded models become visible immediately. Missing or partial
// pulls are filtered out.
func (r *Registry) Models() iterator.Iterator[llmapi.ModelEntry] {
	return iterator.FromSlice(r.scan())
}

// Get resolves a model by its registry-qualified name
// (host/repo:tag or host/repo@digest), matching the names emitted by
// Models. Returns false when the manifest is absent or the weight
// blob is missing.
func (r *Registry) Get(_ context.Context, model string) (llmapi.ModelEntry, bool) {
	for _, e := range r.scan() {
		if e.Name == model {
			return e, true
		}
	}
	return llmapi.ModelEntry{}, false
}

// CachedReferences returns every model reference fully present in the
// cache, sorted lexicographically. Partial or manifest-only entries
// are excluded — see the consistency guarantees on Registry.
func (r *Registry) CachedReferences() ([]Reference, error) {
	return r.cache.CachedReferences()
}

// Download pulls ref into the cache, reporting progress through the
// given callback. On success the returned DownloadResult has the
// absolute ModelPath of the weight blob; the registry's context-window
// cache is refreshed as a side effect so the first post-download call
// to Models()/Get() already reports the right value.
//
// If ref.Host is empty it is populated with DefaultRegistryHost.
func (r *Registry) Download(
	ctx context.Context, ref Reference, progress ProgressFunc,
) (*DownloadResult, error) {
	if ref.Host == "" {
		ref.Host = DefaultRegistryHost
	}
	var cb ociregistry.ProgressFunc
	if progress != nil {
		cb = func(_ string, downloaded, total int64) {
			progress(downloaded, total)
		}
	}
	pr, err := r.downloader.Pull(ctx, r.cache, ref, cb)
	if err != nil {
		return nil, wrapPullError(err, ref)
	}
	out := &DownloadResult{
		Reference:     pr.Reference,
		ModelPath:     pr.ModelPath,
		ProjectorPath: pr.MMProjPath,
	}
	if pr.Manifest != nil {
		for _, layer := range pr.Manifest.Layers {
			if layer.MediaType == ociregistry.MediaTypeOllamaModel {
				continue
			}
			path := pr.LayerPath(layer.MediaType)
			if path == "" {
				continue
			}
			out.LayerSizes = append(out.LayerSizes, LayerInfo{
				MediaType: layer.MediaType,
				Path:      path,
				Size:      layer.Size,
			})
		}
	}
	// Warm the context-window cache so the entry surfaced by the very
	// next scan already has an accurate value. Best-effort: failing to
	// read metadata here just means the value comes from the default
	// until the next manual Get.
	if pr.ModelPath != "" {
		if digest := modelDigest(pr); digest != "" {
			_ = r.refreshContextWindow(ctx, digest, pr.ModelPath)
		}
	}
	return out, nil
}

// Delete removes ref from the cache. The manifest file is deleted
// first, then every layer blob that no other manifest still references
// is pruned, and finally every persistent context-window cache entry
// for those pruned blobs is cleared. If ref is not present
// os.ErrNotExist is returned unchanged so callers can render a
// "not cached" error.
func (r *Registry) Delete(ctx context.Context, ref Reference) error {
	if ref.Host == "" {
		ref.Host = DefaultRegistryHost
	}
	// Capture layer digests *before* DeleteReference so we know which
	// persistent records to clear even for blobs that end up pruned.
	digests := r.manifestLayerDigests(ref)
	if err := r.cache.DeleteReference(ref.Host, ref.Repository, ref.Target()); err != nil {
		return err
	}
	for _, d := range digests {
		// If a blob is still referenced by some other manifest its
		// on-disk file (and therefore its context-window metadata)
		// is still valid — leave the cache entry in place.
		if _, err := os.Stat(r.blobPath(d)); err == nil {
			continue
		}
		r.forgetContextWindow(ctx, d)
	}
	return nil
}

// manifestLayerDigests reads the manifest on disk and returns every
// layer's digest. Missing manifests return an empty slice (Delete
// turns that into os.ErrNotExist downstream).
func (r *Registry) manifestLayerDigests(ref Reference) []string {
	data, err := r.cache.GetManifest(ref.Host, ref.Repository, ref.Target())
	if err != nil {
		return nil
	}
	m, err := ociregistry.UnmarshalManifest(data)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(m.Layers))
	for _, l := range m.Layers {
		out = append(out, l.Digest)
	}
	return out
}

func (r *Registry) blobPath(digest string) string {
	p, _ := r.cache.BlobPath(digest)
	return p
}

// scan walks the cache and returns one ModelEntry per complete
// manifest. Missing blobs, incompatible manifests, and malformed
// coordinates are silently filtered.
func (r *Registry) scan() []llmapi.ModelEntry {
	root := filepath.Join(r.cache.Root(), "manifests")
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}
	var out []llmapi.ModelEntry
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		host, repo, target, ok := manifestCoords(root, path)
		if !ok {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		m, err := ociregistry.UnmarshalManifest(data)
		if err != nil {
			return nil
		}
		model := m.FindLayer(ociregistry.MediaTypeOllamaModel)
		if model == nil {
			return nil
		}
		var projectorPath string
		if projector := m.FindLayer(ociregistry.MediaTypeOllamaProjector); projector != nil {
			if p, err := r.cache.BlobPath(projector.Digest); err == nil {
				if st, err := os.Stat(p); err == nil && !st.IsDir() {
					projectorPath = p
				}
			}
		}
		blobPath, err := r.cache.BlobPath(model.Digest)
		if err != nil {
			return nil
		}
		if st, err := os.Stat(blobPath); err != nil || st.IsDir() {
			return nil
		}
		out = append(out, llmapi.ModelEntry{
			Name:          formatModelName(host, repo, target),
			Provider:      LLMProvider,
			ContextWindow: r.contextWindowForBlob(model.Digest, blobPath),
			BaseURL:       blobPath,
			ProjectorPath: projectorPath,
		})
		return nil
	})
	slices.SortFunc(out, func(a, b llmapi.ModelEntry) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// contextWindowForBlob returns the GGUF-advertised training context
// for the blob at path. It consults the in-memory cache first, then
// the persistent store, then falls back to parsing the GGUF header
// and populates both caches on success. A hard override set via
// WithContextWindow short-circuits every step.
func (r *Registry) contextWindowForBlob(digest, path string) int {
	if r.contextWindowOverride > 0 {
		return r.contextWindowOverride
	}
	if digest == "" {
		return defaultContextWindow
	}
	r.memCacheMu.Lock()
	if n, ok := r.memCache[digest]; ok && n > 0 {
		r.memCacheMu.Unlock()
		return n
	}
	r.memCacheMu.Unlock()

	if n, ok := r.loadContextWindow(digest); ok && n > 0 {
		r.rememberContextWindow(digest, n)
		return n
	}

	n, err := readGGUFContextLength(path)
	if err != nil || n <= 0 {
		return defaultContextWindow
	}
	r.rememberContextWindow(digest, n)
	_ = r.storeContextWindow(digest, n)
	return n
}

// refreshContextWindow re-reads the GGUF header at path and replaces
// any cached entry for digest with the result. Used after a fresh
// pull so the next Models() call does not return a stale value.
func (r *Registry) refreshContextWindow(_ context.Context, digest, path string) error {
	n, err := readGGUFContextLength(path)
	if err != nil || n <= 0 {
		return err
	}
	r.rememberContextWindow(digest, n)
	return r.storeContextWindow(digest, n)
}

func (r *Registry) rememberContextWindow(digest string, n int) {
	r.memCacheMu.Lock()
	r.memCache[digest] = n
	r.memCacheMu.Unlock()
}

// forgetContextWindow clears every cache entry for digest. Called
// after a blob has been pruned so subsequent probes do not return a
// phantom value from before the eviction.
func (r *Registry) forgetContextWindow(ctx context.Context, digest string) {
	r.memCacheMu.Lock()
	delete(r.memCache, digest)
	r.memCacheMu.Unlock()
	r.storageMu.Lock()
	svc := r.storage
	r.storageMu.Unlock()
	if svc == nil {
		return
	}
	_ = svc.Delete(ctx, storageKey(digest))
}

// contextWindowRecord is the on-disk shape of a persisted entry.
// Only a single field today, but declared as a struct so new
// metadata fields (chat template hashes, etc.) can be added without
// breaking the storageapi document schema. No struct tags — storage
// backends disagree on tag semantics (see storageapi.Service docs).
type contextWindowRecord struct {
	ContextWindow int
}

func storageKey(digest string) string {
	// Digests already contain a colon (sha256:…) which some storage
	// backends might not tolerate in IDs. Normalise to the same
	// representation the on-disk blob filename uses.
	return strings.Replace(digest, ":", "-", 1)
}

func (r *Registry) loadContextWindow(digest string) (int, bool) {
	r.storageMu.Lock()
	svc := r.storage
	r.storageMu.Unlock()
	if svc == nil {
		return 0, false
	}
	var rec contextWindowRecord
	if err := svc.Get(context.Background(), storageKey(digest), &rec); err != nil {
		return 0, false
	}
	return rec.ContextWindow, rec.ContextWindow > 0
}

func (r *Registry) storeContextWindow(digest string, n int) error {
	r.storageMu.Lock()
	svc := r.storage
	r.storageMu.Unlock()
	if svc == nil {
		return nil
	}
	return svc.Set(context.Background(), storageKey(digest),
		contextWindowRecord{ContextWindow: n})
}

// modelDigest extracts the weight blob digest from a PullResult, or
// "" if the manifest contains no model layer (shouldn't happen in
// practice but Pull is permissive).
func modelDigest(pr *ociregistry.PullResult) string {
	if pr == nil || pr.Manifest == nil {
		return ""
	}
	if m := pr.Manifest.FindLayer(ociregistry.MediaTypeOllamaModel); m != nil {
		return m.Digest
	}
	return ""
}

// manifestCoords parses the on-disk manifest layout back into its
// (host, repo, target) triple. Layout is written by Cache.PutManifest
// and mirrored in Cache.CachedReferences.
func manifestCoords(root, path string) (host, repo, target string, ok bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", "", "", false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 3 {
		return "", "", "", false
	}
	host = parts[0]
	target = parts[len(parts)-1]
	repo = strings.Join(parts[1:len(parts)-1], "/")
	if alg, rest, cut := strings.Cut(target, "-"); cut && (alg == "sha256" || alg == "sha512") {
		target = alg + ":" + rest
	}
	return host, repo, target, true
}

// formatModelName assembles the registry-qualified name under which a
// model is listed. It matches Reference.String so that a user can
// paste a listed name back into Download and have it resolve.
func formatModelName(host, repo, target string) string {
	var b strings.Builder
	b.WriteString(host)
	b.WriteByte('/')
	b.WriteString(repo)
	if strings.HasPrefix(target, "sha256:") || strings.HasPrefix(target, "sha512:") {
		b.WriteByte('@')
	} else {
		b.WriteByte(':')
	}
	b.WriteString(target)
	return b.String()
}

// wrapPullError produces user-friendly errors for the two error
// shapes the shell surfaces explicitly. Other errors are returned
// unchanged so callers can still errors.Is() against them.
func wrapPullError(err error, ref Reference) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ociregistry.ErrNotGGUFRepo):
		return fmt.Errorf(
			"%s is not compatible with the llama.cpp OCI endpoint. "+
				"Hugging Face only exposes this endpoint for GGUF repositories; "+
				"look for a %q or %q variant instead",
			ref.Repository, ref.Repository+"-GGUF", ref.Repository+"-gguf")
	case errors.Is(err, ociregistry.ErrNotFound):
		return fmt.Errorf(
			"%s not found on %s — check the repository name and tag "+
				"(use `download %s:<tag>` to request a specific tag)",
			ref.Repository, ref.Host, ref.Repository)
	}
	return err
}

// The Registry exposes Models and Get over the local model cache; it is
// not itself an llmapi.Service. Callers wrap it through llmrouter.WithModels
// and pair it with the llamacpp service for the actual completion path.
