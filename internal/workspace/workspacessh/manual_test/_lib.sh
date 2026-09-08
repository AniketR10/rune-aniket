#!/usr/bin/env bash
# Shared helpers for the manual SSH scenario scripts.
# Source from each scenario via `source "$(dirname "$0")/_lib.sh"`.

set -euo pipefail

IMAGE="${RUNE_SSH_TEST_IMAGE:-rune_ssh_workspace_test:latest}"

REPO_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
TESTDATA_DIR="$REPO_ROOT/workspace/workspacessh/test/testdata"

_check_image() {
  if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
    echo "Image $IMAGE is not built." >&2
    echo "Build it with:" >&2
    echo "  docker build \\" >&2
    echo "    -f workspace/workspacessh/test/Dockerfile \\" >&2
    echo "    -t $IMAGE \\" >&2
    echo "    $REPO_ROOT" >&2
    exit 2
  fi
}

# private_key_copy <name> -> echoes a temp path holding a 0600
# copy of testdata/<name>. Git only preserves the +x bit, so the
# checked-in key file may be 0644 on disk; OpenSSH refuses to load
# such a key. We mirror the harness helper PrivateKeyPath here.
private_key_copy() {
  local name="$1"
  local src="$TESTDATA_DIR/$name"
  if [[ ! -f "$src" ]]; then
    echo "missing testdata key: $src" >&2
    exit 2
  fi
  local dst
  dst="$(mktemp -d)/$name"
  cp "$src" "$dst"
  chmod 0600 "$dst"
  echo "$dst"
}

# host_linux_arch -> echoes the linux GOARCH that matches the host
# (so the container can run the rune binary natively, no QEMU).
host_linux_arch() {
  case "$(uname -m)" in
    arm64|aarch64) echo arm64 ;;
    x86_64|amd64)  echo amd64 ;;
    *)             echo amd64 ;;
  esac
}

# build_runesvc -> echoes a path to a freshly cross-compiled
# runesvc binary (workspace server) for the host's linux arch.
# Mirrors the harness's buildRuneTestBinary().
build_runesvc() {
  local arch
  arch="$(host_linux_arch)"
  local out="$REPO_ROOT/target/runesvc-linux-$arch"
  mkdir -p "$(dirname "$out")"
  (cd "$REPO_ROOT" \
    && GOOS=linux GOARCH="$arch" CGO_ENABLED=0 \
        go build -o "$out" \
          ./internal/workspace/workspacessh/test/cmd/runesvc/) >&2
  echo "$out"
}

# install_remote_rune <container_id>
# Installs the rune workspace-server binary into the container at
# /usr/local/bin/rune so the SSH dial's whichCommand check
# succeeds. Two modes:
#
#   - $RUNE_REMOTE_BIN set (typical when invoked via
#     `make manual-ssh-test-...`): copy the real Linux rune.app/
#     in alongside its dependent libs and shim /usr/local/bin/rune
#     to point at it with LD_LIBRARY_PATH set.
#   - Otherwise: cross-compile the static runesvc helper (no libs,
#     no GUI) and copy that single file in.
#
# Done with `docker cp` rather than a bind mount so the script
# layout is uniform regardless of how many files are involved.
install_remote_rune() {
  local id="$1"
  if [[ -n "${RUNE_REMOTE_BIN:-}" ]]; then
    if [[ ! -x "$RUNE_REMOTE_BIN" ]]; then
      echo "RUNE_REMOTE_BIN=$RUNE_REMOTE_BIN is not executable" >&2
      exit 2
    fi
    local app
    app="$(cd "$(dirname "$RUNE_REMOTE_BIN")/.." && pwd)"
    if [[ ! -d "$app/lib" ]]; then
      echo "expected $app/lib to exist next to $RUNE_REMOTE_BIN" >&2
      exit 2
    fi
    # docker cp <local-dir> <id>:/opt/ copies the whole tree as
    # /opt/<basename>. Cleaner than copying bin/ and lib/
    # separately into a pre-made parent.
    docker exec "$id" rm -rf /opt/rune.app >/dev/null
    docker cp "$app" "$id:/opt/" >/dev/null
    # rename whatever the source dir was called to a stable
    # /opt/rune.app — the upstream layout is rune.app/ so this is
    # a no-op except in pathological cases.
    if ! docker exec "$id" test -d /opt/rune.app; then
      docker exec "$id" sh -c \
        'mv /opt/'"$(basename "$app")"' /opt/rune.app' >/dev/null
    fi
    docker exec "$id" sh -c \
      'printf "#!/bin/sh\nexec env LD_LIBRARY_PATH=/opt/rune.app/lib /opt/rune.app/bin/rune \"\$@\"\n" \
        > /usr/local/bin/rune && chmod 0755 /usr/local/bin/rune' >/dev/null
  else
    local bin
    bin="$(build_runesvc)"
    docker cp "$bin" "$id:/usr/local/bin/rune" >/dev/null
    docker exec "$id" chmod 0755 /usr/local/bin/rune >/dev/null
  fi
}

# wait_for_sshd <host:port>
# Block until sshd serves the SSH-2.0 banner on the given address.
wait_for_sshd() {
  local addr="$1"
  local i
  for i in $(seq 1 60); do
    if nc -z "${addr%:*}" "${addr##*:}" 2>/dev/null; then
      if echo | nc -w 1 "${addr%:*}" "${addr##*:}" 2>/dev/null \
          | grep -q '^SSH-2\.0'; then
        return 0
      fi
    fi
    sleep 0.5
  done
  echo "sshd did not become ready on $addr" >&2
  return 1
}

# start_container <scenario_name> [docker run extra args...]
# Boots a container with the standard env vars; echoes the
# container id. Caller is responsible for the cleanup trap.
start_container() {
  local name="$1"
  shift
  local id
  id="$(docker run -d --rm \
    -e TZ=UTC \
    -e PUID=1000 -e PGID=1000 \
    -e USER_NAME=test \
    -e SUDO_ACCESS=true \
    -P \
    --label "rune.manual.scenario=$name" \
    "$@" \
    "$IMAGE")"
  echo "$id"
}

# host_port <container_id> -> echoes the host-side mapping for
# the in-container 2222/tcp.
host_port() {
  docker inspect -f \
    '{{(index (index .NetworkSettings.Ports "2222/tcp") 0).HostPort}}' \
    "$1"
}

# apply_sshd_snippet <container_id> <snippet>
# Mirrors workspace/workspacessh/test/harness.go applyExtraSSHDConfig.
apply_sshd_snippet() {
  local id="$1" snippet="$2"
  local tmp
  tmp="$(mktemp)"
  printf '%s' "$snippet" > "$tmp"
  docker exec "$id" mkdir -p /config/sshd/sshd_config.d >/dev/null
  docker cp "$tmp" "$id:/config/sshd/sshd_config.d/test.conf" >/dev/null
  docker exec "$id" chmod 0644 /config/sshd/sshd_config.d/test.conf >/dev/null
  docker exec "$id" sh -c '
    set -e
    if ! grep -qF "/config/sshd/sshd_config.d/*.conf" /config/sshd/sshd_config; then
      echo "Include /config/sshd/sshd_config.d/*.conf" >> /config/sshd/sshd_config
    fi
    s6-svc -r /run/service/svc-openssh-server
  '
  rm -f "$tmp"
}

# ensure_password <container_id> <user> <password>
# Re-sets the unix password directly because the linuxserver
# image only honours USER_PASSWORD on first boot when
# PasswordAuthentication is enabled.
ensure_password() {
  local id="$1" user="$2" pw="$3"
  docker exec "$id" sh -c "echo '${user}:${pw}' | chpasswd" >/dev/null
}

# write_file_in_container <id> <path> <content> <mode>
write_file_in_container() {
  local id="$1" path="$2" content="$3" mode="$4"
  docker exec "$id" sh -c "
    set -e
    cat > '$path' <<'__EOF__'
$content
__EOF__
    chmod $mode '$path'
  "
}

# regenerate_host_keys <container_id>
# Throws away the existing /config/ssh_host_keys and forces sshd
# to generate a fresh set, then restarts sshd. Used by the
# host-key-rotation scenario.
regenerate_host_keys() {
  local id="$1"
  docker exec "$id" sh -c '
    set -e
    cd /config/ssh_host_keys
    rm -f ssh_host_*
    ssh-keygen -q -N "" -t rsa     -f ssh_host_rsa_key
    ssh-keygen -q -N "" -t ecdsa   -f ssh_host_ecdsa_key
    ssh-keygen -q -N "" -t ed25519 -f ssh_host_ed25519_key
    chmod 600 ssh_host_rsa_key ssh_host_ecdsa_key ssh_host_ed25519_key
    chmod 644 ssh_host_rsa_key.pub ssh_host_ecdsa_key.pub ssh_host_ed25519_key.pub
    chown test:users ssh_host_*
    s6-svc -r /run/service/svc-openssh-server
  '
}

# write_known_hosts <hostport> <key1.pub> [key2.pub ...]
# Writes a known_hosts file (suitable as workspace.ssh.known_hosts)
# pinning hostport to every key passed. Echoes the path.
write_known_hosts() {
  local hp="$1"; shift
  local host="${hp%:*}" port="${hp##*:}"
  local addr="$host"
  if [[ -n "$port" && "$port" != "22" ]]; then
    addr="[$host]:$port"
  fi
  local out
  out="$(mktemp -d)/known_hosts"
  : > "$out"
  local pub
  for pub in "$@"; do
    awk -v a="$addr" '{ print a, $1, $2 }' "$pub" >> "$out"
  done
  chmod 0600 "$out"
  echo "$out"
}

# write_rune_config <ssh_yaml_block>
# Writes a Rune YAML config to /tmp/.rune/config.yaml that wraps
# the given workspace.ssh block. The block is the body of
# `workspace.ssh:` (each line is indented to 4 spaces by this
# function). Echoes the resulting config path so the caller can
# tell the user to pass it via `rune -c <path>`.
#
# Usage:
#   CFG="$(write_rune_config "command: \"\"
#   private_keys: [\"$KEY\"]
#   insecure: true
#   timeout: \"20s\"")"
write_rune_config() {
  local block="$1"
  local dir="/tmp/.rune"
  local path="$dir/config.yaml"
  mkdir -p "$dir"
  {
    echo "workspace:"
    echo "  ssh:"
    while IFS= read -r line; do
      [[ -z "$line" ]] && { echo ""; continue; }
      echo "    $line"
    done <<< "$block"
  } > "$path"
  echo "$path"
}

# print_rune_run <config_path> <ssh_uri>
# Pretty-prints the command the user should run in another
# terminal to launch rune against the just-prepared workspace.
print_rune_run() {
  local cfg="$1" uri="$2"
  cat <<EOF

wrote rune config: $cfg

run:
  rune -c $cfg -w $uri

EOF
}

# follow_logs <container_id>
# Tail container logs in foreground until Ctrl-C, then stop.
follow_logs() {
  local id="$1"
  echo
  echo ">>> Tailing $id logs (Ctrl-C to stop and clean up). <<<"
  echo
  docker logs -f "$id" || true
}

# install_cleanup_trap <container_id>
# Sets a trap that stops the container on EXIT/INT/TERM. The id is
# expanded into the trap body once at install time, not at trap
# fire time, so the trap stays valid even with `set -u` in effect
# (the local $id from this function is gone by the time the trap
# runs).
install_cleanup_trap() {
  local id="$1"
  trap "echo; echo 'stopping $id'; docker rm -f $id >/dev/null 2>&1 || true" \
    INT TERM EXIT
}

# verify_remote_rune <container_id> <port> <key>
# Sanity-checks that the freshly bind-mounted rune binary is
# reachable to the remote's non-interactive shell. Aborts the
# script with a clear error if not. Run after the bind mount and
# `wait_for_sshd` so the user gets fast feedback before launching
# rune by hand.
verify_remote_rune() {
  local id="$1" port="$2" key="$3"
  local out
  if ! out="$(ssh \
      -i "$key" \
      -o IdentitiesOnly=yes \
      -o IdentityAgent=none \
      -o StrictHostKeyChecking=no \
      -o UserKnownHostsFile=/dev/null \
      -o BatchMode=yes \
      -p "$port" "test@127.0.0.1" \
      'command -v rune' 2>&1)"; then
    echo "ERROR: 'command -v rune' failed over ssh in $id:" >&2
    echo "$out" >&2
    echo "Make sure the script bind-mounts a runesvc binary at" >&2
    echo "/usr/local/bin/rune (see build_runesvc + start_container)." >&2
    exit 3
  fi
  echo "remote rune: $out" >&2
}

_check_image
