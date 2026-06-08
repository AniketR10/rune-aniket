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
	"unstable.build/go-tui/llm/claude"
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
	ProviderClaude    = "claude"
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

	custom llmapi.Service
	ollama llmapi.Service

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
	// hostedClients caches the constructed openai/anthropic/gemini
	// clients keyed by the resolved API key. The key is resolved lazily
	// (storage first, then config) so a `providers <p> setup` takes
	// effect without restarting the IDE; a changed key produces a new
	// cache entry. Guarded by mu.
	hostedClients map[hostedCacheKey]llmapi.Service
	// newLocal builds a llama.cpp Service. Tests swap in a fake to avoid
	// loading a real GGUF.
	newLocal func(llamacpp.Config) (localService, error)
	// newHostedClient builds a throwaway hosted client for a given
	// provider/key, used by VerifyProviderKey to test a key before
	// storing it. Tests swap in a fake to avoid real network calls.
	newHostedClient func(provider, apiKey string) (llmapi.Service, llmapi.ModelEntry, error)
	// closed flips to true after Close so late dispatches that try to
	// resurrect a torn-down service are rejected instead.
	closed bool

	// storage is the rune-side persistent storage used by providers
	// that own their auth state (today: codex). The router never reads
	// from storage itself; it only forwards it to per-provider
	// credential loaders that resolve the access token lazily, on
	// the request that needs it.
	storage storageapi.Service

	// store persists named hosted-provider API keys (set via
	// `/models providers <p> add`). resolveAPIKey consults the active
	// key here before falling back to the static configured key.
	store *keyStore
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

type hostedCacheKey struct {
	provider string
	apiKey   string
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
		hostedClients: make(map[hostedCacheKey]llmapi.Service),
	}
	r.store = newKeyStore(storage)
	r.newLocal = func(c llamacpp.Config) (localService, error) {
		return llamacpp.NewService(c)
	}
	r.newHostedClient = r.buildHostedClient

	// openai, anthropic, and gemini clients are constructed lazily by
	// resolveOpenAI / resolveAnthropic / resolveGemini so the API key
	// can be sourced from storage (set via `providers <p> setup`) and
	// take effect without restarting the IDE. Codex follows the same
	// per-request pattern (see resolveCodex). Custom uses the
	// OpenAI-compatible API with a static configured key.
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
// provider's static catalog plus the dynamic gemini / ollama /
// llamacpp registries. The order is deterministic: openai, anthropic,
// codex, custom, ollama, local, gemini. Gemini is queried live and
// surfaces list errors through its iterator; it is placed last so a
// failed Gemini query (e.g. missing key) cannot truncate the
// preceding providers when iterator.Aggregate halts on error.
func (r *Router) Models() iterator.Iterator[llmapi.ModelEntry] {
	its := []iterator.Iterator[llmapi.ModelEntry]{
		iterator.FromSlice(openai.ModelEntries()),
		iterator.FromSlice(anthropic.ModelEntries()),
		iterator.FromSlice(codex.ModelEntries()),
		iterator.FromSlice(claude.ModelEntries()),
	}
	if len(r.customCatalog) > 0 {
		its = append(its, iterator.FromSlice(r.customCatalog))
	}
	if r.ollamaRegistry != nil {
		its = append(its, r.ollamaRegistry.Models())
	}
	its = append(its, r.localRegistry.Models())
	its = append(its, r.geminiCatalog(context.Background()))
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
		return r.resolveOpenAI(ctx)
	case ProviderAnthropic:
		return r.resolveAnthropic(ctx)
	case ProviderCodex:
		return r.resolveCodex(ctx)
	case ProviderClaude:
		return r.resolveClaude(ctx)
	case ProviderGemini:
		return r.resolveGemini(ctx)
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

// resolveClaude loads the stored Claude Code subscription credential and
// constructs a fresh Anthropic client authenticated with the OAuth bearer
// token plus the Agent-SDK identifying headers. The credential's access
// token rotates, so the client is rebuilt per request (like codex).
func (r *Router) resolveClaude(ctx context.Context) (llmapi.Service, error) {
	cred, err := claude.CredentialForClient(ctx, r.storage)
	if err != nil {
		if errors.Is(err, claude.ErrCredentialNotFound) {
			return nil, fmt.Errorf(
				"llmrouter: no claude credential found; run " +
					"`models providers claude login` from the rune shell")
		}
		return nil, fmt.Errorf("llmrouter: load claude credential: %w", err)
	}
	cfg := r.cfg.ClaudeClientConfig()
	cfg.OAuthToken = cred.AccessToken
	cfg.Headers = cred.ClientHeaders()
	return anthropic.NewClient(cred.AccessToken, cfg), nil
}

// resolveAPIKey returns the API key for a hosted provider, preferring the
// active key stored in rune-side local storage (set via
// `/models providers <p> add`) and falling back to the static configured
// key. It returns an empty string when neither source supplies one.
func (r *Router) resolveAPIKey(ctx context.Context, provider, configKey string) string {
	if key, err := r.store.active(ctx, provider); err == nil && key != "" {
		return key
	}
	return configKey
}

func (r *Router) hostedClient(
	provider, apiKey string, build func() llmapi.Service,
) (llmapi.Service, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, ErrRouterClosed
	}
	key := hostedCacheKey{provider: provider, apiKey: apiKey}
	if svc, ok := r.hostedClients[key]; ok {
		return svc, nil
	}
	svc := build()
	r.hostedClients[key] = svc
	return svc, nil
}

func (r *Router) resolveOpenAI(ctx context.Context) (llmapi.Service, error) {
	key := r.resolveAPIKey(ctx, ProviderOpenAI, r.cfg.OpenAI.APIKey)
	if key == "" {
		return nil, errors.New(
			"llmrouter: openai api key not configured; run " +
				"`/models providers openai add`")
	}
	return r.hostedClient(ProviderOpenAI, key, func() llmapi.Service {
		return openai.NewClient(key, r.cfg.OpenAIClientConfig())
	})
}

func (r *Router) resolveAnthropic(ctx context.Context) (llmapi.Service, error) {
	key := r.resolveAPIKey(ctx, ProviderAnthropic, r.cfg.Anthropic.APIKey)
	if key == "" {
		return nil, errors.New(
			"llmrouter: anthropic api key not configured; run " +
				"`/models providers anthropic add`")
	}
	return r.hostedClient(ProviderAnthropic, key, func() llmapi.Service {
		return anthropic.NewClient(key, r.cfg.AnthropicClientConfig())
	})
}

func (r *Router) resolveGemini(ctx context.Context) (llmapi.Service, error) {
	key := r.resolveAPIKey(ctx, ProviderGemini, r.cfg.Gemini.APIKey)
	if key == "" {
		return nil, errors.New(
			"llmrouter: gemini api key not configured; run " +
				"`/models providers gemini add`")
	}
	return r.hostedClient(ProviderGemini, key, func() llmapi.Service {
		return gemini.NewClient(key, r.cfg.GeminiClientConfig())
	})
}

func (r *Router) geminiCatalog(ctx context.Context) iterator.Iterator[llmapi.ModelEntry] {
	svc, err := r.resolveGemini(ctx)
	if err != nil {
		return iterator.FromSlice(gemini.ModelEntries())
	}
	return svc.Models()
}

// AddProviderKey stores a named API key for a hosted provider. The first
// key added for a provider becomes the active key.
func (r *Router) AddProviderKey(ctx context.Context, provider, name, key string) error {
	return r.store.add(ctx, provider, name, key)
}

// RemoveProviderKey deletes a named API key. When the removed key is the
// active one, another remaining key is promoted (or the active key is
// cleared when none remain).
func (r *Router) RemoveProviderKey(ctx context.Context, provider, name string) error {
	return r.store.remove(ctx, provider, name)
}

// UseProviderKey makes a stored named key the active key for a provider.
func (r *Router) UseProviderKey(ctx context.Context, provider, name string) error {
	return r.store.use(ctx, provider, name)
}

// ProviderKeyNames returns the stored key names for a provider, sorted.
func (r *Router) ProviderKeyNames(ctx context.Context, provider string) ([]string, error) {
	return r.store.names(ctx, provider)
}

// ProviderKeyActiveName returns the name of the active key for a provider,
// or the empty string when none is set.
func (r *Router) ProviderKeyActiveName(ctx context.Context, provider string) (string, error) {
	return r.store.activeName(ctx, provider)
}

// ProviderKeyActive returns the value of the active key for a provider, or
// ErrAPIKeyNotSet when none is set.
func (r *Router) ProviderKeyActive(ctx context.Context, provider string) (string, error) {
	return r.store.active(ctx, provider)
}

// buildHostedClient constructs a throwaway client for the given provider
// and key plus the model entry to probe during verification. The client
// is not cached.
func (r *Router) buildHostedClient(
	provider, apiKey string,
) (llmapi.Service, llmapi.ModelEntry, error) {
	switch provider {
	case ProviderOpenAI:
		entries := openai.ModelEntries()
		if len(entries) == 0 {
			return nil, llmapi.ModelEntry{}, errors.New("llmrouter: openai has no models to verify against")
		}
		return openai.NewClient(apiKey, r.cfg.OpenAIClientConfig()), entries[0], nil
	case ProviderAnthropic:
		entries := anthropic.ModelEntries()
		if len(entries) == 0 {
			return nil, llmapi.ModelEntry{}, errors.New("llmrouter: anthropic has no models to verify against")
		}
		return anthropic.NewClient(apiKey, r.cfg.AnthropicClientConfig()), entries[0], nil
	case ProviderGemini:
		client := gemini.NewClient(apiKey, r.cfg.GeminiClientConfig())
		model := llmapi.ModelEntry{Name: gemini.VerificationModel, Provider: ProviderGemini}
		return client, model, nil
	default:
		return nil, llmapi.ModelEntry{}, fmt.Errorf("llmrouter: provider %q does not support key verification", provider)
	}
}

// VerifyProviderKey tests an API key by issuing a minimal completion
// against the provider's first model and consuming the first event. It
// returns the provider error verbatim, or nil when the key works.
func (r *Router) VerifyProviderKey(ctx context.Context, provider, key string) error {
	client, model, err := r.newHostedClient(provider, key)
	if err != nil {
		return err
	}
	it, err := client.CreateCompletion(ctx, model, llmapi.Request{
		MaxOutputTokens: 1,
		Messages: []llmapi.Message{
			{Role: llmapi.RoleUser, Content: "ping"},
		},
	})
	if err != nil {
		return err
	}
	defer func() { _ = it.Close() }()
	ev, ok := it.Next(ctx)
	if !ok {
		return nil
	}
	if ev.Type == llmapi.EventStreamError {
		return ev.Error
	}
	return nil
}
