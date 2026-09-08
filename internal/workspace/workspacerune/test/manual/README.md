# Manual `rune://` end-to-end setup

This directory brings up a Headscale coordination server so you can
drive two real Rune instances against each other by hand. The automated
equivalents live one directory up:

- `TestIntegrationScheme` — the same Headscale container, a second Rune
  instance in its own process, and the workspace scheme conformance
  suites. Requires docker.
- `TestWorkspaceScheme` / `TestRejectsPeerOwnedByAnotherAccount` — the
  same coverage against an in-process coordination server, so they run
  without docker as part of `make test`.

Use this manual setup when you want to see the network from the UI:
the `network` console command, peer completion on `workspaceopen`, and
editing over a `rune://` workspace.

## 1. Start the coordination server

```bash
docker compose up -d
docker compose logs -f headscale   # until "listening and serving HTTP"
```

## 2. Mint a pre-authorization key

```bash
docker exec rune-headscale headscale users create ernie
docker exec rune-headscale headscale preauthkeys create \
    --user 1 --reusable --expiration 24h
```

The last line printed is the key. Both instances register with it, which
is what makes them peers owned by one account — the condition the
serving side authorizes on.

## 3. Run two Rune instances

Release builds ask the API for the coordination server and a pre-auth
key, and only get them with an active plan. Debug builds (`make debug`)
read them from the environment instead, which is what lets this setup
run against a local Headscale:

```bash
export RUNE_NETWORK_CONTROL_URL=http://127.0.0.1:8080
export RUNE_NETWORK_AUTH_KEY=<the key from step 2>
```

Both instances can live on one machine. They need separate data
directories, because a data directory holds one instance's network
identity, and separate names, because the name is what a `rune://` URI
addresses. They do **not** need separate ports: each instance serves on
its own in-process network stack, so port 7473 does not collide.

Give each one a config file:

```yaml
# ~/.rune-a/config.yaml   (and ~/.rune-b/config.yaml with hostname instance-b)
network:
  auto_join: true
  hostname: instance-a
```

Then start them. `--config` is required: it does not follow `--datadir`.

```bash
rune --datadir ~/.rune-a --config ~/.rune-a/config.yaml &
rune --datadir ~/.rune-b --config ~/.rune-b/config.yaml &
```

To confirm the coordination server sees both:

```bash
docker exec rune-headscale headscale nodes list
```

## 4. Verify from the UI

In instance A's console:

- `network status` — name, addresses, and account. While a sign-in is
  pending it prints the URL to authorize the machine, which is the path
  you get when `RUNE_NETWORK_AUTH_KEY` is unset.
- `network peers` — should list `instance-b` and the `rune://` address
  it is reachable at.
- `workspaceopen rune://` — completion offers the peers.
- `workspaceopen rune://instance-b/` — opens instance B's home
  directory. Edit and save a file, then open a terminal in it and
  confirm the command runs on B.
- `network down` on B, then use the workspace on A: it should report a
  lost connection and recover once you run `network up` on B again.

## Cleanup

```bash
docker compose down -v
```
