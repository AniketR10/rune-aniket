#!/usr/bin/env bash
# Scenario 15: AuthenticationMethods publickey (single method,
# password fallback explicitly disallowed).
#
# Why useful manually:
#   - Gives you a server that NEVER advertises password as a
#     fallback.
#   - Confirm rune doesn't surface a stale password prompt when
#     the previously-configured private key fails.
#   - The error path should be ErrAuthRequiredKey, not "password
#     rejected".

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 15_two_factor_pubkey_only \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

apply_sshd_snippet "$CID" 'AuthenticationMethods publickey
PasswordAuthentication no
KbdInteractiveAuthentication no
'
wait_for_sshd "$HP"

KEY="$(private_key_copy id_ed25519)"

verify_remote_rune "$CID" "${HP##*:}" "$KEY"

# Positive path: known good key configured.
CFG="$(write_rune_config "command: \"\"
private_keys: [\"$KEY\"]
insecure: true
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

# Negative path: drop private_keys entirely so rune has to
# surface ErrAuthRequiredKey rather than fall back to password.
CFG_NEG="$(write_rune_config "command: \"\"
insecure: true
timeout: \"20s\"")"

cat <<EOF
Negative path: rerun rune against the *no-keys* config to
confirm it surfaces ErrAuthRequiredKey (no password prompt):

  rune -c $CFG_NEG -w ssh://test@$HP/tmp

EOF

follow_logs "$CID"
