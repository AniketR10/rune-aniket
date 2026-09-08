#!/usr/bin/env bash
# Scenario 01: host-key pinning, matching key.
# Automated counterpart: TestHostKeyMatchingKnownHosts.
#
# What you should observe:
#   - rune connects without any UI prompts.
#   - The dial succeeds with `insecure: false` because the
#     workspace's known_hosts file pins the same key the server
#     presents.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 01_host_key_match \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

# Capture all three host keys (rsa+ecdsa+ed25519) and pin them
# all so HostKeyAlgorithms negotiation never picks a type the
# known_hosts file is missing.
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
Try the dial directly:
  ssh -i $KEY -o UserKnownHostsFile=$KH -o StrictHostKeyChecking=yes \\
    -p ${HP##*:} test@${HP%:*} echo ok

EOF

follow_logs "$CID"
