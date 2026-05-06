#!/usr/bin/env bash
# Scenario 08: server-side SSH banner.
# Automated counterpart: TestServerBanner.
#
# What you should observe:
#   - The server emits a pre-auth banner ("WARNING: AUTHORIZED
#     USE ONLY"). The Go ssh client consumes it as a banner
#     message; rune must continue the dial without surfacing the
#     text as a prompt or as garbage in any error.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 08_banner \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

write_file_in_container "$CID" /etc/ssh-banner \
  'WARNING: AUTHORIZED USE ONLY' '0644'

apply_sshd_snippet "$CID" 'Banner /etc/ssh-banner
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
Manual sanity check (banner is printed by openssh client):
  ssh -i $KEY -o StrictHostKeyChecking=no \\
    -p ${HP##*:} test@${HP%:*} echo ok

EOF

follow_logs "$CID"