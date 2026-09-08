#!/usr/bin/env bash
# Scenario 04: keyboard-interactive (PAM) auth.
# Automated counterpart: TestKbdInteractiveAuth.
#
# What you should observe:
#   - The server advertises ONLY keyboard-interactive.
#   - rune surfaces a PromptSecret dialog labelled with the PAM
#     question ("Password:" or similar).
#   - Answering "hunter2" succeeds.
#   - You must opt into kbd-interactive in the workspace config
#     because the Go ssh client otherwise surfaces "unexpected
#     message type 51" against servers that advertise the method
#     without an INFO_REQUEST.

source "$(dirname "$0")/_lib.sh"

CID="$(start_container 04_kbd_interactive \
  -e PASSWORD_ACCESS=true \
  -e USER_PASSWORD=hunter2)"
install_cleanup_trap "$CID"
install_remote_rune "$CID"

HP="127.0.0.1:$(host_port "$CID")"
wait_for_sshd "$HP"

apply_sshd_snippet "$CID" '
PubkeyAuthentication no
PasswordAuthentication no
KbdInteractiveAuthentication yes
UsePAM yes
'
wait_for_sshd "$HP"
# PAM still needs the unix password set so the kbd-interactive
# challenge has something to match against.
ensure_password "$CID" test hunter2

CFG="$(write_rune_config "command: \"\"
insecure: true
kbd_interactive: true
timeout: \"20s\"")"
print_rune_run "$CFG" "ssh://test@$HP/tmp"

cat <<EOF
Expected secret answer when prompted: hunter2

Manual sanity check:
  ssh -o PreferredAuthentications=keyboard-interactive \\
    -o PubkeyAuthentication=no \\
    -o PasswordAuthentication=no \\
    -p ${HP##*:} test@${HP%:*}

EOF

follow_logs "$CID"
