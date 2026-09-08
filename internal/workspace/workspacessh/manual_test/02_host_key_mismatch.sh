#!/usr/bin/env bash
# Scenario 02: host-key pinning, MISMATCHED key.
# Automated counterpart: TestHostKeyMismatchSurfacesErrHostKeyMismatch.
#
# What you should observe:
#   - rune fails to connect with an ErrHostKeyMismatch-class
#     notification ("the server's host key does not match the
#     entry recorded in known_hosts").
#   - NO password / passphrase prompt is rendered: a mismatch is
#     a hard error because it could be a MITM.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 02_host_key_mismatch \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

# Pin a different but well-formed ed25519 public key. We reuse
# the testdata client public key — it parses fine but does not
# match what the server presents.
KH="$(write_known_hosts "$HP" "$TESTDATA_DIR/id_ed25519.pub")"
KEY="$(private_key_copy id_ed25519)"

CFG="$(write_rune_config "command: \"\"
private_keys: [\"$KEY\"]
insecure: false
known_hosts: \"$KH\"
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

cat <<EOF
Manual sanity check:
  ssh -i $KEY -o UserKnownHostsFile=$KH -o StrictHostKeyChecking=yes \\
    -p ${HP##*:} test@${HP%:*} echo should-fail

EOF

follow_logs "$CID"
