#!/usr/bin/env bash
# Scenario 06: AllowUsers excludes the workspace user.
# Automated counterpart: TestAllowUsersDeniesUser.
#
# What you should observe:
#   - The user "test" really exists on the host (and SSH would
#     succeed with the configured key against a default config),
#   - but sshd's AllowUsers list only permits "nope", so this
#     dial fails with an authentication error.
#   - This is the policy-layer rejection path; distinct from
#     "user does not exist".

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 06_allow_users_denies \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

apply_sshd_snippet "$CID" 'AllowUsers nope
'
wait_for_sshd "$HP"

KEY="$(private_key_copy id_ed25519)"

CFG="$(write_rune_config "command: \"\"
private_keys: [\"$KEY\"]
insecure: true
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

cat <<EOF
Manual sanity check:
  ssh -i $KEY -o StrictHostKeyChecking=no \\
    -p ${HP##*:} test@${HP%:*} echo should-fail

EOF

follow_logs "$CID"