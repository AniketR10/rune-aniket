#!/usr/bin/env bash
# Scenario 11: passphrase prompt cancelled (Esc).
# Automated counterpart: TestPassphrasePromptCancelled.
#
# What you should observe:
#   - rune prompts for the passphrase of id_ed25519_pass.
#   - Press Esc / cancel the prompt.
#   - rune surfaces a clean error and stops; no retry storm.
#   - For the positive path, type the passphrase: secret123

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 11_passphrase_cancel \
  -e PUBLIC_KEY_FILE=/id_ed25519_pass.pub)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

KEY="$(private_key_copy id_ed25519_pass)"

CFG="$(write_rune_config "command: \"\"
private_keys: [\"$KEY\"]
insecure: true
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

cat <<EOF
Passphrase for the configured key: secret123
(Try cancelling first to see the error path, then re-dial and
 enter it correctly to see the success path.)

Manual sanity check:
  ssh -i $KEY -o StrictHostKeyChecking=no \\
    -p ${HP##*:} test@${HP%:*} echo ok

EOF

follow_logs "$CID"