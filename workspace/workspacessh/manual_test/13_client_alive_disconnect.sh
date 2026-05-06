#!/usr/bin/env bash
# Scenario 13: server-driven mid-session disconnect via
# ClientAliveInterval / ClientAliveCountMax (excluded from auto
# suite — too timing-sensitive to assert programmatically).
#
# What you should observe:
#   - Connect rune to the workspace.
#   - The container is configured to drop the SSH session if no
#     traffic flows for ~5s.
#   - Stop interacting with rune for ~10s; the server should
#     terminate the session.
#   - rune's stdConn close hook fires, remoteScheme.maintainConnection
#     surfaces "lost connectivity to ssh://...", and the workspace
#     attempts to reconnect.
#   - Verify the close hook only fires once (no duplicate
#     notifications) and that the reconnect either succeeds or
#     surfaces a typed error if you Ctrl-C this script first.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 13_client_alive_disconnect \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

apply_sshd_snippet "$CID" 'ClientAliveInterval 5
ClientAliveCountMax 1
'
wait_for_sshd "$HP"

KEY="$(private_key_copy id_ed25519)"

verify_remote_rune "$CID" "${HP##*:}" "$KEY"

CFG="$(write_rune_config "command: \"\"
private_keys: [\"$KEY\"]
insecure: true
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

cat <<EOF
Idle ~10s after connecting and watch the workspace for the
disconnect notification, then any reconnect behaviour.

EOF

follow_logs "$CID"