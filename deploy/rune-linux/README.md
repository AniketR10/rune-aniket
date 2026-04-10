# Rune Linux cross-compilation

`deploy/rune-linux/Dockerfile` cross-compiles the Rune GUI binary for
`linux/amd64` inside Docker without forcing the Go toolchain itself to run
under `linux/amd64` emulation.

The `make rune-linux-cross-compile` rule works by:

1. building `deploy/rune-linux/Dockerfile` with `docker buildx`
2. running the Go toolchain on `$BUILDPLATFORM`
3. passing `GIT_SSH_KEY` with the same build-arg SSH setup used by
   `deploy/build/Dockerfile`
4. installing the `amd64` Linux cross compiler and target headers
5. cross-compiling `./cmd/rune` to `linux/amd64` inside the Dockerfile
6. exporting the built artifact from the final scratch stage

The exported build artifact includes:

- `rune`
- `runtime-needed.txt`
- `README.runtime.md`

## Runtime dependencies

The built binary links directly against:

- `libX11.so.6`
- `libm.so.6`
- `libc.so.6`

For normal GUI execution on Debian/Ubuntu, install at least:

- `libasound2`
- `libx11-6`
- `libxrandr2`
- `libxcursor1`
- `libxinerama1`
- `libxi6`
- `libxxf86vm1`
- `libgl1`
- `libc6`

These cover the Linux desktop/OpenGL runtime stack used by Rune's
Ebiten/GLFW-based GUI.
