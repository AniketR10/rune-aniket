#!/usr/bin/env bash
# Scenario 05: MaxAuthTries 1.
# Automated counterpart: TestMaxAuthTriesOne.
#
# What you should observe:
#   - rune offers the (deliberately wrong) first key, the server
#     severs the connection after one failed attempt, and rune
#     surfaces a clean "ssh authentication ... failed" error
#     without retrying with the second key.
#   - The wrong key burns the auth budget; the right key never
#     gets a turn — that's the point.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 05_max_auth_tries_one \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

apply_sshd_snippet "$CID" 'MaxAuthTries 1
'
wait_for_sshd "$HP"

WRONG="$(private_key_copy id_ed25519_wrong)"
RIGHT="$(private_key_copy id_ed25519)"

# wrong key first, right key second; sshd MaxAuthTries 1 cuts us
# off after the first failure.
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