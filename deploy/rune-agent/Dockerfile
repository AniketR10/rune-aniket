# syntax=docker/dockerfile:1

# rune-agent Linux build (hybrid native-libs + cross-Go).
#
# Why this is split into two patterns:
#
#   * llama.cpp's CUDA / BLAS backends are best built with the vendor
#     toolchain on the *target* arch — nvcc only ships for amd64, libopenblas
#     comes from the target-arch debian package, etc. We do that in the
#     `libs-${TARGETARCH}` stage, run with --platform=linux/${TARGETARCH}.
#     BuildKit + qemu emulates this fine on a non-target host (slow, but it
#     works), and the resulting layer is cached so the cost is one-time.
#
#   * The Go toolchain crashes under qemu-user with `fatal error: found
#     pointer to free object` (https://github.com/golang/go/issues/46527
#     and friends). So we run Go on $BUILDPLATFORM (no qemu) and cross-link
#     to $TARGETARCH using crossbuild-essential. The static `.a` files plus
#     a small `/xlibs` sysroot of target-arch shared libs are imported from
#     the libs stage, and the cross gcc resolves -l flags against them.
#
# Backend matrix:
#   linux/amd64 -> nvidia/cuda devel base, GGML_BLAS=ON + GGML_CUDA=ON,
#                  binary built with -tags=llamacpp_blas,llamacpp_cuda
#   linux/arm64 -> ubuntu base,            GGML_BLAS=ON,
#                  binary built with -tags=llamacpp_blas
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

# ---------------------------------------------------------------------------
# libs-amd64: llama.cpp static libs built natively on linux/amd64.
# CUDA + BLAS.
# ---------------------------------------------------------------------------
FROM --platform=linux/amd64 nvidia/cuda:12.4.1-devel-ubuntu22.04 AS libs-amd64
ENV DEBIAN_FRONTEND=noninteractive
RUN set -eux; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
		ca-certificates make cmake pkg-config bash file build-essential \
		libopenblas-dev; \
	rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Copy only the bits required to build llama.cpp libs. Keeping this scoped
# means the (very expensive) CUDA layer only invalidates when the llamacpp
# Makefile, the C/C++ wrapper sources, or the llama.cpp submodule itself
# change — NOT when arbitrary Go files in the repo change.
COPY cmd/rune-agent/llm/llamacpp /src/cmd/rune-agent/llm/llamacpp

ENV LLAMACPP_BLAS=1 \
	LLAMACPP_CUDA=1

RUN set -eux; \
	rm -rf cmd/rune-agent/llm/llamacpp/llama.cpp/_build cmd/rune-agent/llm/llamacpp/libs; \
	make -C cmd/rune-agent/llm/llamacpp libs

# Stage runtime shared libs the cross-linker will need into /xlibs.
# These are real x86_64 ELF objects we copy out via the libs-${TARGETARCH}
# alias. Symlinks from soname-less names (libfoo.so) are added so the cross
# gcc can resolve `-lfoo` against them.
RUN set -eux; \
	mkdir -p /xlibs; \
	cp -L /usr/lib/x86_64-linux-gnu/libopenblas.so.0 /xlibs/; \
	cp -L /usr/lib/x86_64-linux-gnu/libgomp.so.1     /xlibs/; \
	cp -L /usr/lib/x86_64-linux-gnu/libgfortran.so.5 /xlibs/; \
	cp -L /usr/lib/x86_64-linux-gnu/libstdc++.so.6   /xlibs/; \
	cp -L /usr/lib/x86_64-linux-gnu/libgcc_s.so.1    /xlibs/; \
	cp -L /usr/lib/x86_64-linux-gnu/libquadmath.so.0 /xlibs/ 2>/dev/null || true; \
	cp -L /usr/local/cuda/targets/x86_64-linux/lib/libcudart.so.12   /xlibs/; \
	cp -L /usr/local/cuda/targets/x86_64-linux/lib/libcublas.so.12   /xlibs/; \
	cp -L /usr/local/cuda/targets/x86_64-linux/lib/libcublasLt.so.12 /xlibs/; \
	# libcuda.so is the driver, normally provided by the host NVIDIA install.
	# CUDA toolkits ship a *stub* under targets/.../lib/stubs/libcuda.so so
	# binaries can be linked without a real driver present. We import the stub
	# under /xlibs so the cross-linker can resolve `-lcuda`, but we MUST NOT
	# bundle it into the runtime tarball — the bundling loop already filters
	# libcuda.so* out via the libcuda.so* case.
	cp -L /usr/local/cuda/targets/x86_64-linux/lib/stubs/libcuda.so /xlibs/libcuda.so.1; \
	ln -sf libcuda.so.1     /xlibs/libcuda.so; \
	ln -sf libopenblas.so.0 /xlibs/libopenblas.so; \
	ln -sf libgomp.so.1     /xlibs/libgomp.so; \
	ln -sf libgfortran.so.5 /xlibs/libgfortran.so; \
	ln -sf libquadmath.so.0 /xlibs/libquadmath.so 2>/dev/null || true; \
	ln -sf libcudart.so.12  /xlibs/libcudart.so; \
	ln -sf libcublas.so.12  /xlibs/libcublas.so; \
	ln -sf libcublasLt.so.12 /xlibs/libcublasLt.so

# ---------------------------------------------------------------------------
# libs-arm64: llama.cpp static libs built natively on linux/arm64. BLAS only.
# ---------------------------------------------------------------------------
FROM --platform=linux/arm64 ubuntu:22.04 AS libs-arm64
ENV DEBIAN_FRONTEND=noninteractive
RUN set -eux; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
		ca-certificates make cmake pkg-config bash file build-essential \
		libopenblas-dev; \
	rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY cmd/rune-agent/llm/llamacpp /src/cmd/rune-agent/llm/llamacpp

ENV LLAMACPP_BLAS=1

RUN set -eux; \
	rm -rf cmd/rune-agent/llm/llamacpp/llama.cpp/_build cmd/rune-agent/llm/llamacpp/libs; \
	make -C cmd/rune-agent/llm/llamacpp libs

RUN set -eux; \
	mkdir -p /xlibs; \
	cp -L /usr/lib/aarch64-linux-gnu/libopenblas.so.0 /xlibs/; \
	cp -L /usr/lib/aarch64-linux-gnu/libgomp.so.1     /xlibs/; \
	cp -L /usr/lib/aarch64-linux-gnu/libgfortran.so.5 /xlibs/; \
	cp -L /usr/lib/aarch64-linux-gnu/libstdc++.so.6   /xlibs/; \
	cp -L /usr/lib/aarch64-linux-gnu/libgcc_s.so.1    /xlibs/; \
	ln -sf libopenblas.so.0 /xlibs/libopenblas.so; \
	ln -sf libgomp.so.1     /xlibs/libgomp.so; \
	ln -sf libgfortran.so.5 /xlibs/libgfortran.so

# Alias: pick the right libs stage based on TARGETARCH so the build stage
# below stays arch-agnostic.
FROM libs-${TARGETARCH} AS libs

# ---------------------------------------------------------------------------
# build: cross-compile the rune-agent Go binary on $BUILDPLATFORM (no qemu),
# linking against the target-arch libs imported from `libs`.
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build

ARG TARGETOS
ARG TARGETARCH
ARG GIT_SSH_KEY
ARG RUNE_BUILD_TAG
ARG RUNE_BUILD_COMMIT
ARG RUNE_RELEASE_TAR

ENV GOPRIVATE=github.com/unstablebuild,unstable.build/* \
	CGO_ENABLED=1 \
	DEBIAN_FRONTEND=noninteractive

RUN set -eux; \
	if [ "$TARGETOS" != "linux" ]; then \
		echo "unsupported target OS: $TARGETOS (expected linux)"; exit 1; \
	fi; \
	case "$TARGETARCH" in \
		amd64) DPKG_ARCH=amd64; CROSS_ESSENTIAL=crossbuild-essential-amd64 ;; \
		arm64) DPKG_ARCH=arm64; CROSS_ESSENTIAL=crossbuild-essential-arm64 ;; \
		*) echo "unsupported target arch: $TARGETARCH"; exit 1 ;; \
	esac; \
	dpkg --add-architecture "$DPKG_ARCH"; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
		pkg-config bash make git openssh-client ca-certificates file \
		"$CROSS_ESSENTIAL"; \
	rm -rf /var/lib/apt/lists/*

WORKDIR /src

RUN git config --global url.ssh://git@github.com/.insteadOf https://github.com/
RUN mkdir -p ~/.ssh && \
	echo "$GIT_SSH_KEY" > ~/.ssh/id_rsa && \
	chmod 600 ~/.ssh/id_rsa && \
	chmod 700 ~/.ssh
# rune-agent's private deps live on github.com/unstablebuild/*; we do not
# pull from git.unstable.build. Tolerate transient DNS/network failures.
RUN ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts || true

COPY . .

# Pull the prebuilt static libs from the (target-arch native) libs stage.
# This wipes any host-side _build / libs/ leftovers that may have leaked
# through .dockerignore.
RUN rm -rf cmd/rune-agent/llm/llamacpp/libs cmd/rune-agent/llm/llamacpp/llama.cpp/_build
COPY --from=libs /src/cmd/rune-agent/llm/llamacpp/libs /src/cmd/rune-agent/llm/llamacpp/libs

# Pull the runtime shared libs the cross link needs (target-arch ELF .so
# files with linker symlinks). The bundling loop below also reads /xlibs.
COPY --from=libs /xlibs /xlibs

RUN --mount=type=cache,target=/root/.cache/go-build \
	set -eux; \
	: "${RUNE_BUILD_TAG:?RUNE_BUILD_TAG build arg is required}"; \
	: "${RUNE_BUILD_COMMIT:?RUNE_BUILD_COMMIT build arg is required}"; \
	case "$TARGETARCH" in \
		amd64) \
			CC=x86_64-linux-gnu-gcc; \
			CXX=x86_64-linux-gnu-g++; \
			TRIPLET=x86_64-linux-gnu; \
			OBJDUMP=x86_64-linux-gnu-objdump; \
			TAGS="llamacpp_blas,llamacpp_cuda"; \
			;; \
		arm64) \
			CC=aarch64-linux-gnu-gcc; \
			CXX=aarch64-linux-gnu-g++; \
			TRIPLET=aarch64-linux-gnu; \
			OBJDUMP=aarch64-linux-gnu-objdump; \
			TAGS="llamacpp_blas"; \
			;; \
	esac; \
	export CC CXX; \
	mkdir -p /out/rune-agent/bin /out/rune-agent/lib; \
	CGO_LDFLAGS="-Wl,-rpath,\$ORIGIN/../lib -L/xlibs" \
	GOOS=$TARGETOS \
	GOARCH=$TARGETARCH \
	go build \
		-tags="$TAGS" \
		-ldflags="-X unstable.build/go-tui/debug.Tag=$RUNE_BUILD_TAG -X unstable.build/go-tui/debug.Commit=$RUNE_BUILD_COMMIT -X unstable.build/go-tui/debug.Package=rune" \
		-o /out/rune-agent/bin/rune-agent \
		./cmd/rune-agent; \
	\
	echo "--- artifact validation ---"; \
	file /out/rune-agent/bin/rune-agent | grep 'ELF 64-bit LSB'; \
	"$OBJDUMP" -p /out/rune-agent/bin/rune-agent | awk '$1 == "NEEDED" {print $2}' > /tmp/runtime-needed.txt; \
	cat /tmp/runtime-needed.txt; \
	\
	echo "--- bundling runtime libraries into rune-agent/lib/ (rpath=\$ORIGIN/../lib) ---"; \
	cp /tmp/runtime-needed.txt /tmp/lib-queue.txt; \
	while [ -s /tmp/lib-queue.txt ]; do \
		: > /tmp/lib-next.txt; \
		while IFS= read -r soname; do \
			case "$soname" in \
				libc.so*|libm.so*|libpthread.so*|libdl.so*|librt.so*|ld-linux*|linux-vdso*) continue ;; \
				libcuda.so*) \
					echo "  skipped (host driver): $soname"; \
					continue ;; \
			esac; \
			[ -f "/out/rune-agent/lib/$soname" ] && continue; \
			src=""; \
			for d in /xlibs /usr/lib/$TRIPLET /lib/$TRIPLET /usr/$TRIPLET/lib; do \
				if [ -e "$d/$soname" ]; then src="$d/$soname"; break; fi; \
			done; \
			if [ -z "$src" ]; then \
				echo "WARN: $soname not found"; continue; \
			fi; \
			cp -L "$src" "/out/rune-agent/lib/$soname"; \
			echo "  bundled: $soname"; \
			"$OBJDUMP" -p "$src" 2>/dev/null | awk '$1 == "NEEDED" {print $2}' >> /tmp/lib-next.txt; \
		done < /tmp/lib-queue.txt; \
		mv /tmp/lib-next.txt /tmp/lib-queue.txt; \
	done; \
	echo "--- bundled libraries ---"; \
	ls -la /out/rune-agent/lib/ || true

RUN set -eux; \
	cp /src/cmd/rune-agent/config.yaml /out/rune-agent/config.yaml; \
	cp -r /src/cmd/rune-agent/skills /out/rune-agent/skills

RUN set -eux; \
	: "${RUNE_RELEASE_TAR:?RUNE_RELEASE_TAR build arg is required}"; \
	cd /out; \
	tar --format=ustar --no-xattrs --no-acls -czf "$RUNE_RELEASE_TAR" rune-agent; \
	tar -tzf "$RUNE_RELEASE_TAR" >/dev/null

FROM scratch AS artifact
COPY --from=build /out/ /
