#!/usr/bin/env bash
# Scenario 14: workspace.ssh.command with a manual ProxyCommand-
# style template (excluded from the auto suite — would require a
# second jump-host container to be deterministic).
#
# What you should observe:
#   - This script boots ONE container; the workspace config
#     points rune at a `command:` template that piggybacks on
#     the local OpenSSH client. That gives you a real
#     procRemote+ProxyCommand path to drive UX checks (env vars,
#     stderr noise, key-prompts, retries).
#   - For a true ProxyJump scenario you'd need to chain through
#     a second host; do that on real infra rather than via the
#     test image.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 14_proxy_command \
  -e PUBLIC_KEY_FILE=/id_ed25519.pub)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

KEY="$(private_key_copy id_ed25519)"

# Build a workspace command line that mirrors what users put
# in workspace.ssh.command for a ProxyCommand setup.
CMD="ssh -o StrictHostKeyChecking=no -o ProxyCommand='nc %h %p' \
-i $KEY %h -p %p"

CFG="$(write_rune_config "command: \"$CMD\"
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

cat <<EOF
Things to validate:
  - rune launches the command, the proxy connects, the gRPC
    bootstrap succeeds.
  - stderr from ssh ("Permanently added '...' (ED25519) to the
    list of known hosts." etc.) does not bleed into the
    workspace UI.

EOF

follow_logs "$CID"
