# Prompt: Implement `cmd/xsandbox` — an extension sandbox and SDK-conformance harness

## Mission

Create a new executable `cmd/xsandbox` in this repository (module `unstable.build/go-tui`). `xsandbox` is a standalone tool that impersonates the Rune editor's side of the extension protocol so that an extension binary — built with `rune-go-sdk` or with an SDK port in another language — can be exercised end-to-end without running Rune itself.

Its two purposes:

1. **Conformance**: verify that a native SDK implementation speaks the extension protocol correctly (stdio handshake, TLS, auth token, gRPC services, streams).
2. **Benchmarking**: measure handshake and RPC timings so SDK implementations in different languages can be compared.

Usage shape:

```bash
xsandbox --spec spec.star -- ./bin/my-extension [ext-args...]
```

It takes a Starlark `.star` spec describing the RPCs the extension is expected to make (with request matchers and canned responses / stub behavior) plus actions the sandbox should perform (e.g. invoke a registered command), runs the extension against a real socket + gRPC server, evaluates the spec, and reports pass/fail plus timing stats. Exit code 0 on success, non-zero on expectation failure, timeout, or extension crash.

## Background: how Rune starts extensions (already researched — verify, don't re-derive)

The extension host lives in `extension/` and `extension/extensionv2/`, **not** under `cmd/rune`. Key facts:

- **Rune listens, extension dials.** `extension/extensionv2/runner.go:99-210` (`Runner.WorkspaceExtensionsRunner`) creates one gRPC server per workspace on a unix socket (`socketPath` at runner.go:291-299, stale-socket handling in `newUnixListener` runner.go:236-263). Server options include recovery interceptors, `MaxRecv/SendMsgSize` from `llm/llmrpc`, self-signed TLS via `auth.GenerateSelfSignedCert`, and per-RPC OAuth token auth via `ide/ideauthorizer` (`Authorize` maps RPC full method → `extensionapi.Permission` through `extensionapi.PermissionForResource`). `WithInsecureAuth()` / `WithInsecureTransport()` (`extension/extensionv2/options.go:28,37`) disable those.
- **Process launch + env**: `extension/extensionv2/workspace_runner.go:188-240` and `commandEnvs` (:334-379) set `GOTUI_LOG_LEVEL`, `RUNE_SOCKET`, `RUNE_DATADIR`, `RUNE_CERT` (base64 PEM), `RUNE_TOKEN`.
- **stdio JSON handshake**: `extension/extensionv2/protocol.go:48-196`. The extension writes one JSON `extensionapi.Metadata` doc to stdout; the host validates it (`validateMetadata` :175-196), grants permissions, and writes one JSON `extensionapi.Config` (`{socket, token, certificate, config, datadir}`) to the extension's stdin. The SDK side is `extensionapi.ServeWorkspaceExtension` (SDK `serve.go:87-166`).
- **Resources**: `extensionapi.Workspace` (SDK v0.0.102, `api/extensionapi/workspace.go:53`) exposes FileSystem/Executor/Terminal (service `workspace.*`), Editor/Commands (`text.Editor`, command registration flows over bidi streams `SubscribeCommand`/`SubscribeREPLCommand`/`SubscribeEvent`), WindowManager/ResourceOpener/Notifications/Interrupter (`browser.*`), LSP (`semantic.LSP`), Parser (`syntax.Syntax`), Config (`config.Config`), Storage (`proto.DocumentStore`), LLM (`llm.LLM`), Debugger (`debug.Debugger`).
- **Rune's server-side registrars** are in `extension/*.go`: `WorkspaceResources`, `EditorResources`, `BrowserResources`, `SemanticResources`, `SyntaxResources`, `DebugResources`, `LLMResources`, `StorageResources`, `ConfigResources` — each returns `map[extensionapi.Permission]ResourceRegistrar` (`extension/registrar.go:36`); grants via `extension.GrantAll()` (`extension/grantor.go:36`).
- **Reusable fixtures**: `extension/extensionv2/plugin_e2e_test.go:54-243` is a full E2E template (real Runner, socket, TLS, authorizer, `workspace.NewFileScheme`, plus fakes `browsertest.NopWindow()`, `texttest.NopEditor()`, SDK `storagestub.NewInMemoryService()`). `extension/extensionv2/workspace_runner_test.go:49-154` shows protocol-driving fakes. `cmd/claudeimport/main.go:75-110` shows the client-from-env pattern.
- **Starlark** is already a dependency (`go.starlark.net`, go.mod:81) and used in `ide/starlarkconfig` and `ide/config_starlark.go` — follow those patterns for evaluation and error reporting.
- **Makefile auto-discovers** `cmd/*/main.go` (Makefile:47-49, generic rule :272-273); no Makefile change needed for `cmd/xsandbox`.

Read these files before designing anything.

## Requirements

### 1. Sandbox runtime

- Reuse the production host code path wherever practical: prefer building on `extensionv2.NewRunner` + `WorkspaceExtensionsRunner` and the real `protocol` handshake rather than reimplementing them. If reuse requires small exported hooks in `extension/extensionv2`, add minimal ones — do not fork the protocol logic.
- **Default to full fidelity**: TLS with a generated self-signed cert and a real signed token, exactly like Rune. This is essential — SDK ports must prove they handle the cert and per-RPC token. Provide `--insecure` flag(s) to fall back to `WithInsecureTransport`/`WithInsecureAuth` for debugging.
- Create a temp data dir (or `--datadir`), unix socket, launch the extension with the same env vars Rune sets, wire stdin/stdout for the handshake, capture stderr to a log file and/or passthrough with `--verbose`.
- Register gRPC services for **all** `extensionapi.Workspace` resources. Back them with a mix of:
  - real-ish stubs: temp-dir–rooted filesystem (`workspace.NewFileScheme`), in-memory storage (`storagestub`), config served from the spec;
  - scripted stubs for editor/browser/notifications/LSP/syntax/LLM/debugger whose responses come from the spec, with sane defaults (empty/no-op) when the spec says nothing.
- Add a gRPC interceptor (unary + stream) that records every incoming RPC: full method, request payload, timestamps, duration. This recording feed is what expectations match against and what the benchmark report aggregates.
- Streams matter. The harness must support: extension-registered commands (`text.Editor/SubscribeCommand` and `SubscribeREPLCommand` — the sandbox must be able to invoke a registered command and route the invocation over the stream), editor event subscription, `workspace.Scheme/Watch`, and `workspace.Executor/StartCommand`.
- Lifecycle: overall `--timeout` (default e.g. 60s); on completion or failure, SIGTERM the extension, wait with a grace period, then SIGKILL. Propagate context cancellation. Every goroutine body must run under `debug.CapturePanicReport` (repo `debug` package) per AGENTS.md.

### 2. Starlark DSL

Design a small, deterministic DSL evaluated from the `.star` file. Suggested shape (you may refine names, but keep this expressive power):

```python
# Identity/handshake expectations
expect_metadata(id = "com.example.hello", permissions = ["permcmd", "permnoti", "permfs"])

# Config the sandbox serves to the extension
config({"greeting": "hello"})

# Seed stub state
fs.write("README.md", "# readme\n")

# Ordered/unordered expectations on incoming RPCs, with matchers and canned responses
expect_rpc("text.Editor/SubscribeCommand", timeout = "10s")
h = expect_command("hello")                    # sugar: command registration arrived

# Actions the sandbox performs (host -> extension direction)
invoke_command(h, args = ["world"])

expect_rpc(
    "browser.Notifications/Notify",
    where = {"message": "hello world"},
    timeout = "5s",
)

expect_rpc("workspace.Files/Read", respond = {...})   # scripted response override

# Synchronization / assertions
wait_idle("1s")
assert_no_unexpected_rpcs(except = ["config.Config/Get"])
```

DSL requirements:

- Request matchers: exact field values, presence, and simple predicates; report mismatches with a readable diff of expected vs. actual.
- Expectations can be ordered (sequence) or unordered (set) — pick one primary model and document it; provide timeouts per expectation.
- Scripted responses: allow the spec to define the reply for a given RPC (including error responses), falling back to stub defaults.
- Actions: at minimum `invoke_command` / `invoke_repl_command`, publishing an editor event, and writing a watched file to trigger `Watch` notifications.
- Determinism: the spec runs top to bottom; expectation-wait points block until satisfied or timeout.
- Reuse the starlark evaluation conventions in `ide/starlarkconfig` (thread setup, error positions in messages).

### 3. Reporting and benchmarking

- Human-readable summary: pass/fail per expectation, unmet expectations, unexpected RPCs, extension exit status.
- `--bench` / `--json <path>` output: handshake duration, per-method call counts, latency min/avg/p50/p95/max, total wall time — machine-readable JSON so agents can diff runs across SDK implementations.

### 4. Test fixture extension + tests

- Add a tiny fixture extension (Go, using `rune-go-sdk`) used only by tests — e.g. `cmd/xsandbox/internal/fixtureext` or a `testdata` build — that registers a command, reads a file, sends a notification, and reads config. Model it on `cmd/extension_fuzzy_search/main.go`.
- E2E test: build the fixture, run xsandbox against it with a spec covering handshake, command registration + invocation, an fs RPC, and a notification expectation; assert pass. Add a negative test where the spec expects an RPC that never arrives and assert failure output/exit.
- Unit tests for: starlark spec parsing/validation, matcher logic, and the RPC recorder. Prefer table-driven tests.
- Follow the E2E patterns in `extension/extensionv2/plugin_e2e_test.go` (building binaries in tests, socket dirs under `t.TempDir()`).

### 5. Repo conventions (from AGENTS.md — read it)

- License headers on all new files: `bluectl license -f LICENSE <files>`.
- No Makefile changes needed; verify `make` produces `bin/xsandbox`.
- Comments explain *why*, not *what*. No narrative comments.
- Goroutines under `debug.CapturePanicReport`.
- Validate with `make generate && make lint && make test` before declaring done.
- Commit message style: short imperative subject + a "why" body, ≤90 cols.

## Suggested implementation order

1. Read the listed files; confirm the handshake and env details against current source.
2. Skeleton `cmd/xsandbox/main.go` (cobra or plain flags, matching `cmd/claudeimport` style) + internal packages: `sandbox` (runtime), `spec` (starlark), `record` (RPC recorder + report).
3. Sandbox runtime with insecure mode first; get the fixture extension handshaking and calling a stub.
4. RPC recorder + expectation engine; wire the DSL.
5. Full-fidelity TLS + token path as the default.
6. Streams: command invocation round trip.
7. Bench report, JSON output.
8. Tests, lint, license headers, full validation.

## Critical constraints

- Do not modify production protocol behavior in `extension/extensionv2` beyond adding minimal exported hooks if strictly necessary for reuse.
- The tool must work against **any** executable that speaks the protocol — never assume the extension is Go.
- Fail loudly and precisely: a spec failure must say which expectation, what was expected, what was observed.
