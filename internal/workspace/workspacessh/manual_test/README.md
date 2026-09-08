# Manual SSH workspace test scenarios

These shell scripts boot real `linuxserver/openssh-server` containers
with tailored `sshd_config` snippets so you can poke at the SSH
workspace flow by hand. Each scenario is independent: pick the
number that matches what you want to validate and run the script.

Most scenarios are also covered by `go test
./workspace/workspacessh/test/...` end-to-end against the same
image; this directory is for cases where you want to **see the UX**
(prompt copy, error messages, retries, etc.) instead of asserting
it programmatically.

## Prerequisites

- Docker daemon running locally
- The image used by the e2e suite. Build it once with:

  ```sh
  docker build \
    -f workspace/workspacessh/test/Dockerfile \
    -t rune_ssh_workspace_test:latest \
    .
  ```

  (run from the repo root). The integration tests build the same
  image lazily, so if you have already run `make test` once it is
  already cached.

- `make rune` to produce a `rune` binary on `$PATH`. Several
  scenarios bind-mount it into the container so the workspace
  bootstrap can run.

- The test fixtures `workspace/workspacessh/test/testdata/`
  (private/public keys). Each script copies the relevant key into
  a per-run temp directory with mode `0600` because git only
  preserves the executable bit and OpenSSH refuses keys readable
  by other users.

## How to use a scenario

Each script:

1. Boots a fresh container with the right `sshd_config` snippet.
2. Writes a ready-to-use Rune config to `/tmp/.rune/config.yaml`
   and prints the exact `rune -c /tmp/.rune/config.yaml -w
   ssh://...` command to run in another terminal.
3. Streams sshd logs in the foreground.
4. Cleans up the container on Ctrl-C.

Open a second terminal and paste the printed `rune` command (or
use `ssh`/`ssh-keyscan` if you just want to poke the protocol).

## Scenario index

The numbering follows the gap-analysis from the auth coverage plan;
gaps not numbered are either already covered by the automated suite
or out of scope (see `auth_test.go` doc comment for the full list).

| # | Script | What it verifies |
| --- | --- | --- |
| 01 | `01_host_key_match.sh` | known\_hosts pinning succeeds with the matching server key |
| 02 | `02_host_key_mismatch.sh` | known\_hosts pinning surfaces `ErrHostKeyMismatch` |
| 03 | `03_host_key_changed.sh` | rotating the server's host key triggers `ErrHostKeyMismatch` on next dial |
| 04 | `04_kbd_interactive.sh` | `KbdInteractiveAuthentication yes` (PAM) end-to-end UX |
| 05 | `05_max_auth_tries_one.sh` | `MaxAuthTries 1` surfaces a clean error |
| 06 | `06_allow_users_denies.sh` | `AllowUsers nope` rejects the real user |
| 07 | `07_auth_methods_chain.sh` | `AuthenticationMethods publickey,password` chained two-factor |
| 08 | `08_banner.sh` | `Banner /etc/ssh-banner` text prints without breaking auth |
| 09 | `09_rune_missing.sh` | remote without `rune` on $PATH surfaces a friendly error |
| 10 | `10_multiple_keys.sh` | multiple `private_keys` configured, first wrong, second right |
| 11 | `11_passphrase_cancel.sh` | passphrase-protected key + Esc on prompt produces a clean error |
| 12 | `12_max_sessions_one.sh` | `MaxSessions 1` (excluded from auto suite — observe by hand) |
| 13 | `13_client_alive_disconnect.sh` | server-driven `ClientAlive*` disconnect (excluded — observe reconnect UX) |
| 14 | `14_proxy_command.sh` | use a manual `command:` template that pipes through a jump host |
| 15 | `15_two_factor_pubkey_only.sh` | `AuthenticationMethods publickey` so password fallback is never offered |

If a scenario also has an automated counterpart, the script header
lists the test name so you can cross-reference.
