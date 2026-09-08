# --platform=$BUILDPLATFORM pins the base image to the host's
# architecture so docker doesn't pull a foreign-arch variant of the
# multi-arch openssh-server image and run it through QEMU. Foreign-arch
# emulation interacts badly with our custom Go toolchain
# (function-symbol-table mismatch crashes in pre-malloc-init); pinning
# to BUILDPLATFORM, combined with the harness building runesvc for
# runtime.GOARCH, means the test container runs natively everywhere.
FROM --platform=$BUILDPLATFORM lscr.io/linuxserver/openssh-server:latest

# gcompat lets glibc-linked binaries (the rune Linux release in
# particular) run on Alpine's musl. Cheap (~1MB) and only used by
# the manual_test scripts that bind-mount the real rune.app/.
RUN apk add --no-cache gcompat

# Public keys are baked into the image; the harness picks the file to
# install via the PUBLIC_KEY_FILE env var per scenario.
COPY ./workspace/workspacessh/test/testdata/id_ed25519.pub /id_ed25519.pub
COPY ./workspace/workspacessh/test/testdata/id_ed25519_pass.pub /id_ed25519_pass.pub
