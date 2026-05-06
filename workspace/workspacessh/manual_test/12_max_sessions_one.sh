#!/usr/bin/env bash
# Scenario 12: MaxSessions 1 (excluded from the auto suite).
#
# Why this isn't automated:
#   The bootstrap path uses 1+ sessions sequentially. Asserting
#   any specific behaviour against MaxSessions 1 is too coupled
#   to bootstrap implementation choices. Running this by hand
#   tells us whether a tightened server impacts UX in practice.
#
# What you should observe:
#   - First Stat / Read / Edit operations succeed.
#   - Concurrent operations (open multiple files quickly, run a
#     terminal in the workspace, etc.) may surface as gRPC
#     stream errors. We want to confirm the errors are
#     reasonable and don't loop forever.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 12_max_sessions_one \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

apply_sshd_snippet "$CID" 'MaxSessions 1
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
Test ideas:
  - open a few files quickly
  - open a terminal inside the workspace and a file editor
    simultaneously
  - watch sshd logs for "channel n: ..." messages and confirm
    rune surfaces a friendly error rather than hanging

EOF

follow_logs "$CID"