#!/usr/bin/env bash
# Scenario 03: host-key rotation between dials.
# Automated counterpart: TestHostKeyChangedBetweenDials.
#
# What you should observe:
#   - The first rune connect against this workspace succeeds.
#   - This script then waits for ENTER and rotates every host
#     key on the server.
#   - Reconnect from rune (e.g. :reloadworkspace, or restart
#     rune): the next dial must fail with ErrHostKeyMismatch.
#     remoteScheme.maintainConnection should stop retrying once
#     it surfaces the typed error.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 03_host_key_changed \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

TMP="$(mktemp -d)"
for t in rsa ecdsa ed25519; do
  docker exec "$CID" cat "/config/ssh_host_keys/ssh_host_${t}_key.pub" \
    > "$TMP/${t}.pub"
done
KH="$(write_known_hosts "$HP" "$TMP/rsa.pub" "$TMP/ecdsa.pub" "$TMP/ed25519.pub")"
KEY="$(private_key_copy id_ed25519)"

verify_remote_rune "$CID" "${HP##*:}" "$KEY"

CFG="$(write_rune_config "command: \"\"
private_keys: [\"$KEY\"]
insecure: false
known_hosts: \"$KH\"
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

cat <<EOF
1. Connect rune to this workspace now and verify the dial
   succeeds without prompts.
2. Press ENTER here to rotate the server's host keys.
3. Trigger a re-dial in rune (close the workspace + reopen, or
   :reloadworkspace) and watch for ErrHostKeyMismatch.

EOF

read -r -p "press ENTER to rotate host keys... "
regenerate_host_keys "$CID"
wait_for_sshd "$HP"
echo "host keys rotated. now retry the dial in rune."
echo

follow_logs "$CID"