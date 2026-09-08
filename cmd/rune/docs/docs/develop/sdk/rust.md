---
sidebar_position: 3
sidebar_label: Rust SDK (beta)
title: Rust SDK
draft: true
---

# Rust SDK <span className="badge badge--secondary" style={{fontSize: '0.5em', verticalAlign: 'middle'}}>Beta</span>

The [Rust SDK](https://github.com/unstablebuild/rune-rust-sdk) builds Rune
extensions on async Rust and [tokio](https://tokio.rs).

:::info[Early-stage port]
The extension handshake, the `browser` and `workspace` API domains, and the
`rune-tui` handler framework are implemented. The `text`, `storage`,
`semantic` (LSP), `syntax`, `llm`, `debug`, and `config` domains are not yet
ported; see [the API table](#the-api-at-a-glance) for what works today.
:::

The crates are not published to crates.io yet. Depend on them via git:

```toml
[dependencies]
rune-extension = { git = "https://github.com/unstablebuild/rune-rust-sdk" }
rune-api = { git = "https://github.com/unstablebuild/rune-rust-sdk" }
# For serving terminal UIs from an extension:
rune-tui = { git = "https://github.com/unstablebuild/rune-rust-sdk" }
rune-term = { git = "https://github.com/unstablebuild/rune-rust-sdk" }

tokio = { version = "1", features = ["full"] }
serde_json = "1"
```

Requires a Rust toolchain with edition 2024 support (Rust 1.85 or newer).

## A basic extension

`main` does only metadata and handshake. The `extend` closure builds domain
clients from the shared connection, drives the host, and returns;
`serve_workspace_extension` owns the lifetime.

```rust
use rune_api::{browser, workspace};
use rune_extension::{
    BoxError, Metadata, Workspace, new_permissions, serve_workspace_extension,
    PERMISSION_FILE_SYSTEM, PERMISSION_NOTIFICATIONS,
    PERMISSION_BROWSER_WINDOW_MANAGER,
};

#[tokio::main]
async fn main() {
    let meta = Metadata {
        developer_id: "rune-sdk-examples".to_string(),
        developer_email: "your@email.com".to_string(),
        developer_key: "1234".to_string(),
        extension_id: "sample".to_string(),
        extension_name: "Sample".to_string(),
        extension_version: "0.1.0".to_string(),
        permissions: new_permissions([
            PERMISSION_FILE_SYSTEM,
            PERMISSION_NOTIFICATIONS,
            PERMISSION_BROWSER_WINDOW_MANAGER,
        ]),
    };
    if let Err(err) = serve_workspace_extension(extend, meta).await {
        eprintln!("sample extension: {err}");
        std::process::exit(1);
    }
}

// The setup body: build clients from the connection, drive the host, return.
// serve_workspace_extension owns the lifetime.
async fn extend(
    w: Workspace,
    _cfg: serde_json::Map<String, serde_json::Value>,
) -> Result<(), BoxError> {
    // Build domain clients from the shared connection.
    let mut browser = browser::Client::new(w.conn());
    let mut fs = workspace::Client::new(w.conn());

    // Read a file through the workspace and report it to the user.
    let mut f = fs.open("VERSION").await?;
    let mut version = Vec::new();
    let mut buf = [0u8; 4096];
    loop {
        let (n, eof) = f.read(&mut buf).await?;
        version.extend_from_slice(&buf[..n]);
        if eof {
            break;
        }
    }
    browser
        .notify(
            browser::NotificationLevel::Info,
            &format!("version {}", String::from_utf8_lossy(&version).trim()),
        )
        .await?;

    Ok(())
}
```

Unlike the Go and Python SDKs, `Workspace` does not expose per-domain
accessors: you build each client from the shared connection with
`Client::new(w.conn())`. The connection is cheap to clone and shares one
authenticated channel. `extend` must return after setup; spawn background
work with `tokio::spawn`.

## The API at a glance

| Crate / module | Capability | Status |
| --- | --- | --- |
| `rune_extension` | Handshake, `Workspace`, `Metadata`, permissions | ready |
| `rune_api::browser` | Windows, tabs, resource opening, notifications, interrupts | ready |
| `rune_api::workspace` | URI resolution, file operations, watchers, processes, ptys | ready |
| `rune_api::text` | Editor and text operations, editor events, commands | not yet ported |
| `rune_api::storage` | Document storage and persistence | not yet ported |
| `rune_api::semantic` | Language-server operations | not yet ported |
| `rune_api::syntax` | Tree-sitter structural search and queries | not yet ported |
| `rune_api::llm` | LLM access | not yet ported |
| `rune_api::debug` | Debug-adapter (DAP) control | not yet ported |
| `rune_api::config` | Workspace configuration | not yet ported |

`rune-api` gates domains behind Cargo features; `browser` and `workspace`
are on by default. Every call is gated on the matching permission declared
in `Metadata`.

## Run and debug it

Build with `cargo build`, then start the binary in a workspace from the
[console](../../learn/console.md):

```
extensions start sample /path/to/target/debug/sample --config '{"key":"value"}'
```

To skip the manual build, point `extensions start` at the crate's
`src/main.rs` instead; Rune runs it with the `rust` package's `cargo`
against the nearest `Cargo.toml` (see
[source-file extensions](../extensions.md#source-file-extensions)).

Use `extensions logs sample` to read what the process wrote to stderr
(write diagnostics with `eprintln!`), and `extensions restart sample` to
pick up a rebuild. The full lifecycle and the permission prompts are covered
in [the development loop](./index.md#the-development-loop).

## Test it

Keep `main` to metadata and handshake and put the logic in functions that
take the domain clients, so they can be exercised against fakes in
table-driven tests. For UIs, `rune-tui` ships a `handlertest` harness for
driving handlers with scripted events and asserting on the drawn cells.

## Package it

A Rust extension is distributed from a **public git repository**: no archive
to build and no per-platform binaries, because Rune compiles it from source
on the user's machine. Put the crate at the repo root next to a
`config.yaml` that requires the `rust` package and points `path` at the
crate's `src/main.rs`:

```yaml title="config.yaml"
requirements:
  - rust
extensions:
  sample:
    path: '$RUNE_DATADIR/lib/$RUNE_PKG_ID/src/main.rs'
    config:
      # defaults surfaced in the user's config, editable like any other setting
```

Commit `Cargo.lock` alongside `Cargo.toml` so builds are reproducible. Push
it to any host Rune can clone over HTTPS (GitHub, GitLab, Bitbucket, or a
self-hosted server) and users install it by its `<host>/<owner>/<repo>` ID:

```
pkg install github.com/<owner>/<repo>
```

Rune clones the repo, installs the `rust` package (which provides `cargo`)
first because the overlay requires it, and builds the crate with `cargo run`
against the nearest `Cargo.toml` (see
[source and package extensions](../extensions.md#source-and-package-extensions)).
Tag a commit to give users a pinnable version. See the
[packages guide](../packages.md#distributing-from-a-git-repository) for the full format
and how versions work.
