#!/usr/bin/env bash
# Scenario 05: MaxAuthTries 1.
# Automated counterpart: TestMaxAuthTriesOne.
#
# What you should observe:
#   - rune offers the wrong key first; the server severs the
#     connection after the single allowed failure.
#   - rune redials on a fresh connection with the next configured
#     key and authenticates successfully — no prompt, no error.
#   - The point of this scenario: per-key redial keeps a strict
#     server (MaxAuthTries=1) from blocking us when an unrelated
#     wrong key happens to come first in the configured list.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 05_max_auth_tries_one \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

apply_sshd_snippet "$CID" 'MaxAuthTries 1
'
wait_for_sshd "$HP"

WRONG="$(private_key_copy id_ed25519_wrong)"
RIGHT="$(private_key_copy id_ed25519)"

verify_remote_rune "$CID" "${HP##*:}" "$RIGHT"

# wrong key first, right key second; sshd MaxAuthTries 1 severs
# the first connection after the wrong key, and rune redials with
# the next configured key on a fresh connection.
CFG="$(write_rune_config "command: \"\"
private_keys: [\"$WRONG\", \"$RIGHT\"]
insecure: true
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

cat <<EOF
Manual sanity check:
  ssh -i $WRONG -o IdentityFile=$RIGHT -o StrictHostKeyChecking=no \\
    -p ${HP##*:} test@${HP%:*}

EOF

follow_logs "$CID"
