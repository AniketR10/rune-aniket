# syntax=docker/dockerfile:1

ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build

ARG TARGETOS
ARG TARGETARCH
ARG GIT_SSH_KEY

WORKDIR /src

COPY . .

ENV GOPRIVATE=github.com/unstablebuild,unstable.build/* \
	CGO_ENABLED=1

# Linux-native builder image used to cross-compile Rune for linux/amd64 while
# the Go toolchain itself runs natively on the builder platform. The amd64 cross
# compiler and amd64 Linux GUI development packages are installed in the build
# stage, then cmd/rune is cross-compiled to $TARGETOS/$TARGETARCH in Docker.
RUN set -eux; \
	dpkg --add-architecture amd64; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
		pkg-config \
		bash \
		make \
		git \
		openssh-client \
		ca-certificates \
		file \
		crossbuild-essential-amd64 \
		libasound2-dev:amd64 \
		libx11-dev:amd64 \
		libxrandr-dev:amd64 \
		libxcursor-dev:amd64 \
		libxinerama-dev:amd64 \
		libxi-dev:amd64 \
		libgl1-mesa-dev:amd64 \
		libxxf86vm-dev:amd64; \
	rm -rf /var/lib/apt/lists/*

RUN if [ "$TARGETOS" != "linux" ] || [ "$TARGETARCH" != "amd64" ]; then \
		echo "unsupported target: $TARGETOS/$TARGETARCH (expected linux/amd64)"; \
		exit 1; \
	fi

RUN git config --global url.ssh://git@github.com/.insteadOf https://github.com/
RUN mkdir -p ~/.ssh && \
	echo "$GIT_SSH_KEY" > ~/.ssh/id_rsa && \
	chmod 600 ~/.ssh/id_rsa && \
	chmod 700 ~/.ssh
RUN ssh-keyscan -t rsa git.unstable.build >> ~/.ssh/known_hosts
RUN ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts

RUN --mount=type=cache,target=/root/.cache/go-build \
	CC=x86_64-linux-gnu-gcc \
	CXX=x86_64-linux-gnu-g++ \
	PKG_CONFIG_PATH=/usr/lib/x86_64-linux-gnu/pkgconfig:/usr/share/pkgconfig \
	GOOS=$TARGETOS \
	GOARCH=$TARGETARCH \
	/usr/local/go/bin/go build -tags=ebitensinglethread -o /out/rune ./cmd/rune

RUN file /out/rune | grep 'ELF 64-bit LSB executable'
RUN x86_64-linux-gnu-objdump -p /out/rune | awk '$1 == "NEEDED" {print $2}' > /out/runtime-needed.txt
RUN cat > /out/README.runtime.md <<'EOF'
# Rune linux/amd64 runtime dependencies

This binary is built for linux/amd64 with CGO enabled because Rune's GUI path
imports Ebiten/GLFW/X11 code.

Direct ELF dependencies captured from the built binary:

- libX11.so.6
- libm.so.6
- libc.so.6

In practice, Rune's GUI runtime also requires the standard Linux desktop/OpenGL
stack used by GLFW/Ebiten. On Debian/Ubuntu, install at least:

- libasound2
- libx11-6
- libxrandr2
- libxcursor1
- libxinerama1
- libxi6
- libxxf86vm1
- libgl1
- libc6

Headless servers without X11/OpenGL libraries are not supported for Rune's GUI
binary.
EOF

FROM scratch AS artifact
COPY --from=build /out/ /
