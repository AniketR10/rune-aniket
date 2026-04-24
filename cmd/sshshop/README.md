# sshshop

Prototype SSH-accessible TUI storefront, modeled after terminal.shop.

Every incoming SSH connection gets its own `tcell.Screen` backed by the
session's byte stream (via a `tcell.Tty` adapter), and a private
rune-go-sdk event loop is dispatched against that screen. No OS users,
no real shell, no per-session process.

## Layout

- `main.go` — flags, host-key bootstrap, server lifecycle, signal handling.
- `server.go` — builds the `gliderlabs/ssh.Server`, rejects anything
  that isn't a shell-with-pty, hands the session to `RunSession`.
- `sshtty.go` — `tcell.Tty` adapter that wraps an `ssh.Session` plus
  its `<-chan ssh.Window`.
- `runsession.go` — per-session event loop. **Temporary**: once
  rune-go-sdk exposes `tui.RunScreen(root, screen term.Screen, opts...)`
  this file collapses to a single call.
- `shop/shop.go` — placeholder rune-go-sdk `tui.Handler` to prove the
  pipeline end-to-end.

## Run locally

```
go run ./cmd/sshshop -addr :2222 -host-key ./host_ed25519
# in another terminal:
ssh -p 2222 localhost
```

If `-host-key` does not exist, a new ed25519 key is generated, written
to that path, and its SHA256 fingerprint is printed to stderr. Pin that
fingerprint on your landing page.

## TODO once SDK lands

Replace the body of `RunSession` with:

```go
return tui.RunScreen(root, screen)
```

and delete `runsession.go`'s event-loop copy.
