# llamacpp

CGO bindings to [llama.cpp](https://github.com/ggml-org/llama.cpp) and an
`llm.Service` implementation that runs quantized GGUF models locally.

The upstream `llama.cpp` sources are pulled in as a git submodule under
`llama.cpp/` at a pinned commit. Static libraries are produced by CMake
and linked into the Go binary via the `#cgo` directives in `bindings.go`.

## Building

The CGO bindings are always compiled — building `rune-agent` requires a C++
toolchain. Initialise the submodule and build the libraries before building
the binary:

```bash
# First-time clone:
git submodule update --init --recursive

# Build the static libs (linked via cgo LDFLAGS into libs/):
make -C cmd/rune-agent/llm/llamacpp libs

# Then build normally:
go build ./cmd/rune-agent
```

The top-level `Makefile` wires `make rune-agent` and `make test` to
build the libraries automatically.

If you pull a change that adjusts the CMake flag set (e.g. enabling the
common library or toggling OpenSSL), drop the cached cmake config before
rebuilding so the new flags take effect:

```bash
make -C cmd/rune-agent/llm/llamacpp reconfigure libs
```

### Backends

- **CPU** — always built.
- **Metal** — built by default on macOS. Embedded shader library (no runtime
  `.metallib` required).
- **CUDA** — opt-in. Set `LLAMACPP_CUDA=1` before invoking make and pass the
  `llamacpp_cuda` build tag when building Go:

```bash
LLAMACPP_CUDA=1 make -C cmd/rune-agent/llm/llamacpp libs
go build -tags llamacpp_cuda ./cmd/rune-agent
```

## Usage

```go
svc, err := llamacpp.NewService(llamacpp.Config{
    Model:         "qwen2.5-coder-7b",
    ModelPath:     "/path/to/qwen2.5-coder-7b.Q4_K_M.gguf",
    ContextWindow: 8192,
    NGPULayers:    -1, // offload everything the backend supports
})
if err != nil { /* ... */ }
defer svc.Close()

// svc implements llm.Service.
```

A `Registry` wraps an Ollama-compatible OCI cache of pulled GGUFs and
exposes them through the standard `llmregistry.Registry` interface.
Downloads, deletes, and context-window metadata lookup all go through
the same handle. Options are passed as variadic arguments:

```go
reg, err := llamacpp.NewRegistry(
    "/path/to/models",
    llamacpp.WithContextWindow(8192),
)
if err != nil { /* ... */ }

ref, err := llamacpp.ParseReference("unsloth/gemma-3n-E2B-it-GGUF:Q4_K_M")
if err != nil { /* ... */ }
_, err = reg.Download(ctx, ref, nil)
```

## Limitations

- Tool calling is driven by llama.cpp's `common_chat` Jinja path, so
  whatever tool framing the model's embedded template emits is what the
  parser consumes (Qwen, Gemma, DeepSeek, Hermes, etc.). When a
  template ships without an upstream PEG parser, the bindings fall
  back to injecting the Hermes/Qwen2.5 `<tool_call>` framing so
  ChatML-family templates can still carry tool traffic. Base
  (non-instruction-tuned) models will ignore the tool declarations
  regardless of path.
- Multimodal image inputs are supported when the model is loaded with
  a paired `ProjectorPath` (mmproj GGUF). Without a projector, image
  content parts are dropped and only the text parts of
  `llm.ContentPart` are forwarded.
- A single `Service` owns a single llama.cpp context. Concurrent
  `CreateCompletion` calls are serialized with a mutex. The service
  keeps a token-level KV-cache shadow so successive turns that share a
  prefix skip re-evaluation of the common tokens.

## Chat templates

Prompt formatting goes through llama.cpp's Jinja engine via the
`common_chat_templates_*` API (see `common_chat_wrap.{h,cpp}`). This lets
the bindings render modern Jinja templates that ship inside newer GGUFs
— notably Gemma 4, whose `<|turn>role\n...<turn|>` markup is not known
to llama.cpp's legacy name-based template detector.

Set `RUNE_LLAMACPP_FORCE_LEGACY_CHAT=1` to force the pre-Jinja
`llama_chat_apply_template` code path. The Jinja path otherwise attempts
rendering first and only falls back to the legacy implementation when
the Jinja engine refuses the template.

## Boundary with upstream llama.cpp

This package intentionally delegates model-specific behavior to upstream
llama.cpp wherever possible. The remaining Rune-owned code is mostly cgo
glue, Go API shape, service orchestration, and integration with the rest
of Rune Agent.

### Provided by llama.cpp

- **Static libraries and backends** — `llama`, `ggml`, `ggml-base`,
  `ggml-cpu`, Metal/BLAS/CUDA backends, `llama-common`,
  `llama-common-base`, `mtmd`, and `server-context` are built from the
  pinned submodule and linked into the Go package.
- **Chat template rendering** — prompt formatting goes through
  `common_chat_templates_init` and `common_chat_templates_apply`,
  including upstream template detection, model chat-format handling,
  tool declarations, JSON schema / response-format inputs, generation
  prompts, additional stop strings, and parser data in
  `common_chat_params`.
- **Streaming chat parsing** — generated text is parsed by upstream
  `common_chat_parse`, diffed with `common_chat_msg_diff::compute_diffs`,
  and managed through `task_result_state::update_chat_msg` from
  llama.cpp's server code. This gives Rune the same OpenAI-compatible
  streaming behavior as `llama-server`, including tool-call name/id
  header events followed by argument-only deltas.
- **Tool-call IDs** — generated with upstream `gen_tool_call_id`.
- **Sampling** — token sampling uses upstream `common_sampler_init`,
  `common_sampler_sample`, `common_sampler_accept`, and
  `common_sampler_reset`. The grammar/tool-call sampler integration and
  sampler parameters are carried through `common_params_sampling`.
- **Grammar trigger semantics** — preserved-token handling and grammar
  trigger conversion follow the server path, including the upstream check
  that single-token trigger words must be marked as preserved tokens.
- **Stop-prefix detection** — partial stop matching is delegated to
  upstream `string_find_partial_stop`.
- **KV-cache primitives** — cache clear, range removal, position shifting,
  and shift capability checks use llama.cpp memory APIs such as
  `llama_memory_clear`, `llama_memory_seq_rm`, `llama_memory_seq_add`,
  and `llama_memory_can_shift`.
- **Tokenization and detokenization** — Go calls the llama.cpp C API for
  `llama_tokenize`, `llama_token_to_piece`, and `llama_vocab_is_eog`.

### Still hand-rolled by Rune

- **The cgo ABI layer** — `common_chat_wrap.{h,cpp}` is Rune-owned. It
  exposes stable C-compatible structs and opaque handles around upstream
  C++ types so Go can call them safely:
  - `rune_chat_message`, `rune_tool`, `rune_response_format`
  - `rune_sampler_params`
  - `rune_chat_template`, `rune_chat_stream`, `rune_sampler`
  - `rune_chat_stream_event`
- **Go-to-C conversion and memory ownership** — Rune builds and frees the
  C-side arrays for messages, tools, content parts, tool calls, and error
  strings. This includes the zero-initialized `buildCRichChatMessages`
  path and all `CString` lifetime management.
- **Go public API** — Rune defines the Go-facing abstractions:
  `Model`, `Context`, `Batch`, `Sampler`, `ChatTemplateHandle`,
  `ChatStream`, `ChatStreamEvent`, `SamplerParams`,
  `ChatTemplateOptions`, `Tool`, and `ReasoningFormat`.
- **Option mapping** — Rune maps Go options into upstream inputs, such as
  tool-choice strings, reasoning-format values, chat-template options,
  sampler parameters, and `Has*` flags for sampler fields whose upstream
  defaults are non-zero.
- **Prompt orchestration** — actual rendering is upstream, but Rune owns
  `OpenChatTemplate`, `ChatTemplateHandle.Prompt`, `ApplyChatTemplate`,
  and the legacy fallback path controlled by
  `RUNE_LLAMACPP_FORCE_LEGACY_CHAT`.
- **Streaming event translation** — upstream produces
  `common_chat_msg_diff` values; Rune queues them and translates them into
  Go-level `ChatStreamEvent` values and then `llm.Event` stream events.
- **Stop handling policy** — the primitive partial-stop matcher is
  upstream, but Rune owns full-stop detection, earliest-stop selection,
  `pendingStop`, final flushing, and integration with Rune's stream
  finalization.
- **KV-cache reuse policy** — the low-level KV operations are upstream,
  but Rune owns the Go-side policy and shadow state:
  `cachedTokens`, `commonPrefixLen`, `resolvePrefixReuse`,
  `planCacheReuse`, `applyCacheReuse`, `Config.NCacheReuse`, and the
  decision to clear, reuse, or shift cached tokens between turns.
- **Generation service orchestration** — `Service.CreateCompletion` is
  Rune-owned. It handles context locking, prompt rendering, tokenization,
  batching, decoding, sampling, stop detection, event emission, finish
  reason mapping, cancellation, and cache invalidation on errors.
- **Small cgo shims for awkward C structs** — most direct llama.cpp calls
  are made from Go, but appending tokens to `llama_batch` still uses
  Rune's `llamacpp_batch_add` helper because the batch fields are
  cumbersome to populate safely from Go.
- **Registry and provider integration** — OCI-compatible model storage,
  GGUF metadata mapping, projector discovery, service config, and
  integration with `llm.Service` / `llmregistry.Registry` are Rune-specific.
- **Multimodal bridge** — upstream `mtmd` does the model-side work, while
  Rune owns `mtmd_wrap.{h,cpp}`, Go-side media conversion, projector
  loading integration, and multimodal cache invalidation policy.
