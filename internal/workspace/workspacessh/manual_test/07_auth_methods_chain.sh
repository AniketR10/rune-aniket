#!/usr/bin/env bash
# Scenario 07: AuthenticationMethods publickey,password.
# Automated counterpart: TestAuthMethodsChain.
#
# What you should observe:
#   - rune offers the configured private key (succeeds the
#     pubkey leg).
#   - The server then drops back to password auth — rune renders
#     a single PromptSecret labelled "ssh password:".
#   - Answering "hunter2" completes the chain; anything else
#     fails the second leg.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 07_auth_methods_chain \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub \
  -e PASSWORD_ACCESS=true \
  -e USER_PASSWORD=hunter2)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

apply_sshd_snippet "$CID" 'AuthenticationMethods publickey,password
PasswordAuthentication yes
'
wait_for_sshd "$HP"
ensure_password "$CID" test hunter2

KEY="$(private_key_copy id_ed25519)"

CFG="$(write_rune_config "command: \"\"
private_keys: [\"$KEY\"]
insecure: true
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

cat <<EOF
Expected password when prompted: hunter2

Manual sanity check (multi-method client):
  ssh -i $KEY -o StrictHostKeyChecking=no \\
    -p ${HP##*:} test@${HP%:*}

EOF

follow_logs "$CID"
