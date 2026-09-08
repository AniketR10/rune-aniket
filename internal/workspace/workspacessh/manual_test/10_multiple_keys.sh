#!/usr/bin/env bash
# Scenario 10: multiple private_keys, first wrong, second right.
# Automated counterpart: TestMultipleKeysFirstWrongSecondRight.
#
# What you should observe:
#   - rune offers id_ed25519_wrong first, server rejects it.
#   - rune then offers id_ed25519 (the configured workspace
#     key), server accepts.
#   - No password prompt.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 10_multiple_keys \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

WRONG="$(private_key_copy id_ed25519_wrong)"
RIGHT="$(private_key_copy id_ed25519)"

verify_remote_rune "$CID" "${HP##*:}" "$RIGHT"

CFG="$(write_rune_config "command: \"\"
private_keys: [\"$WRONG\", \"$RIGHT\"]
insecure: true
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

cat <<EOF
Manual sanity check:
  ssh -i $WRONG -i $RIGHT -o StrictHostKeyChecking=no \\
    -p ${HP##*:} test@${HP%:*} echo ok

EOF

follow_logs "$CID"
