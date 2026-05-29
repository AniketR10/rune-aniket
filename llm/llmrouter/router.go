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

// Package llmrouter provides a Router that implements llmapi.Service by
// dispatching CreateCompletion / CountTokens calls to one of the
// known provider clients. The router owns every client it dispatches
// to; there is no external Register API. Callers configure the router
// once with an llm.Config and treat it as an opaque llmapi.Service.
package llmrouter

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/llm"
	"unstable.build/go-tui/llm/anthropic"
	"unstable.build/go-tui/llm/codex"
	"unstable.build/go-tui/llm/gemini"
	"unstable.build/go-tui/llm/llamacpp"
	"unstable.build/go-tui/llm/ollama"
	"unstable.build/go-tui/llm/openai"
)

// Provider identifiers as they appear on llmapi.ModelEntry.Provider.
// The router uses these to dispatch CreateCompletion / CountTokens.
const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"
	ProviderCodex     = "codex"
	ProviderGemini    = "gemini"
	ProviderCustom    = "custom"
	ProviderOllama    = "ollama"
	ProviderLocal     = llamacpp.LLMProvider // "llamacpp"
)

// Router is the rune-side LLM dispatcher. It is constructed once from
// an llm.Config and a data directory and then implements
// llmapi.Service for the lifetime of the IDE. The router owns one
// llmapi.Service per provider, plus the local llama.cpp registry used
// by the `models local` REPL command.
type Router struct {
	cfg llm.Config

	openai    llmapi.Service
	anthropic llmapi.Service
	gemini    llmapi.Service
	custom    llmapi.Service
	ollama    llmapi.Service

	// customCatalog is materialised once from cfg.Custom.AvailableModels
	// so Models()/GetModel() can return a stable slice on each call.
	customCatalog []llmapi.ModelEntry
	// ollamaRegistry is the dynamic Ollama-tag registry; its Models()
	// queries the local Ollama daemon on demand.
	ollamaRegistry *ollama.Registry

	// localRegistry is the llama.cpp on-disk model cache. Exposed via
	// LocalRegistry so the `models local` REPL command can manage it
	// without taking a separate dependency.
	localRegistry *llamacpp.Registry
	localCfg      llamacpp.Config

	// mu guards localServices and closed. The host no longer serialises
	// Router calls on the event-loop locker: llmrpc.Server dispatches each
	// gRPC request on grpc-go's goroutine pool, so the Router must be
	// internally goroutine-safe for the I/O-bound provider paths.
	mu sync.Mutex
	// localServices caches one llama.cpp Service per distinct
	// (name, model path, projector path, context window) tuple. Each
	// Service mmaps a multi-GiB GGUF and allocates a KV cache;
	// constructing one per request (the original behaviour) leaked those
	// resources every turn. Guarded by mu because gRPC handlers may
	// dispatch concurrently. Close drains it.
	localServices map[localCacheKey]localService
	// newLocal builds a llama.cpp Service. Tests swap in a fake to avoid
	// loading a real GGUF.
	newLocal func(llamacpp.Config) (localService, error)
	// closed flips to true after Close so late dispatches that try to
	// resurrect a torn-down service are rejected instead.
	closed bool

	// storage is the rune-side persistent storage used by providers
	// that own their auth state (today: codex). The router never reads
	// from storage itself; it only forwards it to per-provider
	// credential loaders that resolve the access token lazily, on
	// the request that needs it.
	storage storageapi.Service
}

// localService is the minimal interface a cached llama.cpp service must
// satisfy. It exists so tests can substitute a fake that records Close
// calls without paying the GGUF-load cost.
type localService interface {
	llmapi.Service
	Close()
}

type localCacheKey struct {
	Name          string
	ModelPath     string
	ProjectorPath string
	ContextWindow uint32
}

// ErrRouterClosed is returned by dispatches issued after Router.Close.
var ErrRouterClosed = errors.New("llmrouter: router closed")

// New constructs a Router from cfg. dataDir is used as the parent
// directory for the llama.cpp model cache when cfg.Local.ModelsCacheDir
// is empty. storage is forwarded to providers that own their own
// auth state (currently codex); pass an in-memory stub in tests that
// don't exercise the codex path. Returns an error only when the local
// registry cannot be initialised — stateless provider clients are
// constructed eagerly and never fail.
func New(cfg llm.Config, dataDir string, storage storageapi.Service) (*Router, error) {
	if storage == nil {
		panic("llmrouter: New: storage must not be nil")
	}
	r := &Router{
		cfg:           cfg,
		storage:       storage,
		localServices: make(map[localCacheKey]localService),
	}
	r.newLocal = func(c llamacpp.Config) (localService, error) {
		return llamacpp.NewService(c)
	}

	r.openai = openai.NewClient(cfg.OpenAI.APIKey, cfg.OpenAIClientConfig())
	r.anthropic = anthropic.NewClient(cfg.Anthropic.APIKey, cfg.AnthropicClientConfig())
	// Codex piggybacks on the OpenAI client but always forces the
	// Responses API and carries per-credential headers + client
	// metadata. The credential is loaded from storage at request time
	// (see resolveCodex) so login/logout takes effect without
	// restarting the IDE.
	// Gemini and custom use the OpenAI-compatible API; the base URL
	// is filled per request from the model entry / shared config.
	r.gemini = openai.NewClient(cfg.Gemini.APIKey, cfg.GeminiClientConfig(gemini.OpenAICompatibleURL))
	if cfg.Custom.URL != "" {
		r.custom = openai.NewClient(cfg.Custom.APIKey, cfg.CustomClientConfig())
		r.customCatalog = customModelEntries(cfg.Custom)
	}

	r.ollamaRegistry = ollama.NewRegistry("")
	r.ollama = openai.NewClient("ollama", cfg.OllamaClientConfig(""))

	cacheDir := cfg.Local.ModelsCacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(dataDir, "models")
	}
	reg, err := llamacpp.NewRegistry(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("llmrouter: init local registry: %w", err)
	}
	r.localRegistry = reg
	r.localCfg = cfg.Local.Service

	return r, nil
}

// LocalRegistry returns the llama.cpp on-disk model cache. Used by the
// `models local` REPL command and by callers that need to download or
// inspect cached models.
func (r *Router) LocalRegistry() *llamacpp.Registry { return r.localRegistry }

// Close releases provider clients that own off-process resources. Today
// only cached llama.cpp services need explicit teardown — the other
// clients hold only Go-side HTTP state. After Close the router rejects
// further dispatches with ErrRouterClosed. Close is idempotent.
func (r *Router) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	for k, svc := range r.localServices {
		delete(r.localServices, k)
		svc.Close()
	}
	return nil
}

// CreateCompletion implements llmapi.Service.
func (r *Router) CreateCompletion(
	ctx context.Context,
	model llmapi.ModelEntry,
	req llmapi.Request,
) (iterator.Iterator[llmapi.Event], error) {
	r.mu.Lock()
	closed := r.closed
	r.mu.Unlock()
	if closed {
		return nil, ErrRouterClosed
	}
	svc, err := r.resolve(ctx, model)
	if err != nil {
		return nil, err
	}
	return svc.CreateCompletion(ctx, model, req)
}

// CountTokens implements llmapi.Service.
func (r *Router) CountTokens(model llmapi.ModelEntry, messages []llmapi.Message) (int, error) {
	r.mu.Lock()
	closed := r.closed
	r.mu.Unlock()
	if closed {
		return 0, ErrRouterClosed
	}
	svc, err := r.resolve(context.Background(), model)
	if err != nil {
		return 0, err
	}
	return svc.CountTokens(model, messages)
}

// Models implements llmapi.Service by concatenating every known
// provider's static catalog plus the dynamic ollama / llamacpp
// registries. The order is deterministic: openai, anthropic, codex,
// gemini, custom, ollama, local.
func (r *Router) Models() iterator.Iterator[llmapi.ModelEntry] {
	its := []iterator.Iterator[llmapi.ModelEntry]{
		iterator.FromSlice(openai.ModelEntries()),
		iterator.FromSlice(anthropic.ModelEntries()),
		iterator.FromSlice(codex.ModelEntries()),
		iterator.FromSlice(gemini.ModelEntries()),
	}
	if len(r.customCatalog) > 0 {
		its = append(its, iterator.FromSlice(r.customCatalog))
	}
	if r.ollamaRegistry != nil {
		its = append(its, r.ollamaRegistry.Models())
	}
	its = append(its, r.localRegistry.Models())
	return iterator.Aggregate(its...)
}

// GetModel implements llmapi.Service. Bare-name lookups (empty
// Provider) return (zero, false) so name collisions across providers
// cannot silently dispatch to the wrong backend.
func (r *Router) GetModel(ctx context.Context, model llmapi.ModelEntry) (llmapi.ModelEntry, bool) {
	if model.Provider == "" {
		return llmapi.ModelEntry{}, false
	}
	it := r.Models()
	defer func() { _ = it.Close() }()
	for {
		entry, ok := it.Next(ctx)
		if !ok {
			return llmapi.ModelEntry{}, false
		}
		if entry.Name != model.Name {
			continue
		}
		if entry.Provider != model.Provider {
			continue
		}
		return entry, true
	}
}

// resolve picks the llmapi.Service that serves model.Provider. The
// codex provider builds a fresh service per request because the access
// token rotates. Local llama.cpp services are cached per
// (name, path, projector, context window) tuple so the multi-GiB model
// load + KV cache happen once per configuration.
func (r *Router) resolve(ctx context.Context, model llmapi.ModelEntry) (llmapi.Service, error) {
	switch model.Provider {
	case "":
		return nil, errors.New("llmrouter: ModelEntry.Provider must be set")
	case ProviderOpenAI:
		return r.openai, nil
	case ProviderAnthropic:
		return r.anthropic, nil
	case ProviderCodex:
		return r.resolveCodex(ctx)
	case ProviderGemini:
		return r.gemini, nil
	case ProviderCustom:
		if r.custom == nil {
			return nil, fmt.Errorf("llmrouter: custom provider not configured")
		}
		return r.custom, nil
	case ProviderOllama:
		// Ollama models carry their per-model base URL in
		// ModelEntry.BaseURL; rebuild a client tuned to that URL.
		return openai.NewClient("ollama", r.cfg.OllamaClientConfig(model.BaseURL)), nil
	case ProviderLocal:
		return r.resolveLocal(model)
	default:
		return nil, fmt.Errorf("llmrouter: no provider registered for %q", model.Provider)
	}
}

// resolveLocal returns a cached llama.cpp Service for model, creating it
// on first use.
func (r *Router) resolveLocal(model llmapi.ModelEntry) (llmapi.Service, error) {
	key := localCacheKey{
		Name:          model.Name,
		ModelPath:     model.BaseURL,
		ProjectorPath: model.ProjectorPath,
		ContextWindow: uint32(model.ContextWindow), // #nosec G115 -- context windows fit in uint32
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, ErrRouterClosed
	}
	if svc, ok := r.localServices[key]; ok {
		return svc, nil
	}
	c := r.localCfg
	c.Model = model.Name
	c.ModelPath = model.BaseURL
	c.ProjectorPath = model.ProjectorPath
	c.ContextWindow = key.ContextWindow
	svc, err := r.newLocal(c)
	if err != nil {
		return nil, err
	}
	r.localServices[key] = svc
	return svc, nil
}

// customModelEntries materialises cfg.Custom.AvailableModels as a
// stable slice of ModelEntry suitable for Models() to expose.
func customModelEntries(cfg llm.CustomConfig) []llmapi.ModelEntry {
	out := make([]llmapi.ModelEntry, 0, len(cfg.AvailableModels))
	for name, ctxWindow := range cfg.AvailableModels {
		out = append(out, llmapi.ModelEntry{
			Name:          name,
			Provider:      ProviderCustom,
			ContextWindow: ctxWindow,
			BaseURL:       cfg.URL,
		})
	}
	return out
}

// resolveCodex loads the stored Codex credential and constructs a
// fresh OpenAI-compatible client that targets the ChatGPT Codex
// backend. The credential's access token is the bearer; the
// installation ID and per-request headers come from the same
// credential. Surfacing a clear "no codex credential" error here
// avoids the openai SDK fallback to api.openai.com that masks the
// real failure as an HTTP 401.
func (r *Router) resolveCodex(ctx context.Context) (llmapi.Service, error) {
	cred, err := codex.CredentialForClient(ctx, r.storage)
	if err != nil {
		if errors.Is(err, codex.ErrCredentialNotFound) {
			return nil, fmt.Errorf(
				"llmrouter: no codex credential found; run " +
					"`models providers codex login` from the rune shell")
		}
		return nil, fmt.Errorf("llmrouter: load codex credential: %w", err)
	}
	cfg := r.cfg.CodexClientConfig()
	cfg.ClientMetadata = map[string]string{
		"x-codex-installation-id": cred.InstallationID,
	}
	cfg.Headers = cred.ClientHeaders()
	// Seed a default PromptCacheKey so the openai client emits the
	// session_id / x-client-request-id headers (and the
	// prompt_cache_key body field) on every request. The ChatGPT
	// Codex backend rejects requests without these correlation
	// headers with 400 Bad Request. Callers that thread their own
	// conversation correlation key via request.PromptCacheKey still
	// take precedence; this fallback only fires when the caller does
	// not supply one (e.g. ad-hoc `runectl llm message ...`).
	if sid, idErr := codex.NewSessionID(); idErr == nil {
		cfg.DefaultPromptCacheKey = sid
	}
	return openai.NewClient(cred.AccessToken, cfg), nil
}
