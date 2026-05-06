#!/usr/bin/env bash
# Scenario 09: remote without `rune` on $PATH.
# Automated counterpart: TestRuneBinaryMissingOnRemote.
#
# What you should observe:
#   - SSH auth succeeds, but the workspace bootstrap fails
#     because connectScheme runs `which rune` on the remote and
#     gets no result.
#   - rune surfaces the canonical error string "...executable
#     was not found on remote..." which
#     isRetryableConnectError matches against to stop the
#     reconnect loop.
#   - You should NOT see rune endlessly retry the dial.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 09_rune_missing \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

KEY="$(private_key_copy id_ed25519)"

CFG="$(write_rune_config "command: \"\"
private_keys: [\"$KEY\"]
insecure: true
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

cat <<EOF
The image deliberately has no rune binary mounted; the dial
gets through but the bootstrap fails.

Manual sanity check:
  ssh -i $KEY -o StrictHostKeyChecking=no \\
    -p ${HP##*:} test@${HP%:*} which rune    # should print nothing, exit 1

EOF

follow_logs "$CID"