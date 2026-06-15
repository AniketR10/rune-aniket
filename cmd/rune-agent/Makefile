ROOT_DIR ?= ../..
GO ?= go
UNAME := $(shell uname)
CODESIGN_IDENTITY ?= Developer ID Application: Unstable Build, LLC. (YYZRWD888J)
NOTARY_PROFILE ?= notary-profile

BIN := $(abspath $(ROOT_DIR)/bin)
PKG := $(abspath $(ROOT_DIR)/pkg/rune-agent)
TAR := $(abspath $(ROOT_DIR)/rune-agent.tar.gz)
NOTARIZE_ZIP := $(abspath $(ROOT_DIR)/rune-agent-notarize.zip)
DIST_SH := ./dist.sh
ROOT_DIST_SH := ./cmd/rune-agent/dist.sh

VERSION ?= $(shell git -C $(ROOT_DIR) describe --tags 2>/dev/null)
COMMIT  ?= $(shell git -C $(ROOT_DIR) rev-parse --short HEAD 2>/dev/null)
TARGET := $(abspath $(ROOT_DIR)/target)
RUNE_AGENT_LINUX_TARGET_OS ?= linux
RUNE_AGENT_LINUX_TARGET_ARCH ?= amd64
RUNE_AGENT_LINUX_OUTPUT_DIR ?= $(TARGET)/rune-agent_$(RUNE_AGENT_LINUX_TARGET_OS)_$(RUNE_AGENT_LINUX_TARGET_ARCH)
RUNE_AGENT_LINUX_RELEASE_TAR ?= rune-agent-release-$(RUNE_AGENT_LINUX_TARGET_OS)-$(RUNE_AGENT_LINUX_TARGET_ARCH)-$(VERSION).tar.gz
# HOST_ARCH is the host's Go arch. A native Linux release requires the host's
# arch to match the requested target arch (no emulation); other combinations
# must use the Docker cross-compile path via the *-cross targets.
HOST_ARCH := $(shell uname -m | sed -e 's/x86_64/amd64/' -e 's/aarch64/arm64/')

# Build flags mirror the root Makefile's six package stamp.
COMMON_LDFLAGS := -X unstable.build/go-tui/debug.Tag=$(VERSION) -X unstable.build/go-tui/debug.Commit=$(COMMIT) -X unstable.build/go-tui/debug.Package=six

# darwin amd64 cross-builds on an Apple Silicon host need an explicit
# -arch x86_64 cgo toolchain; arm64 is native and needs no extra flags.
DARWIN_AMD64_TARGET_ARCH_FLAGS ?= CC="clang -arch x86_64" CGO_CFLAGS="-arch x86_64" CGO_CXXFLAGS="-arch x86_64" CGO_LDFLAGS="-arch x86_64"

.PHONY: clean build pkg sign notarize dist dist-notarized \
	linux-cross-compile make-release-linux \
	release-linux-amd64 release-linux-arm64 \
	release-linux-amd64-cross release-linux-arm64-cross \
	dist-linux-amd64 dist-linux-arm64 \
	dist-linux-amd64-cross dist-linux-arm64-cross \
	pkg-darwin-amd64 pkg-darwin-arm64 \
	sign-darwin-amd64 sign-darwin-arm64 \
	notarize-darwin-amd64 notarize-darwin-arm64 \
	dist-darwin-amd64 dist-darwin-arm64

build: $(BIN)/rune-agent

$(BIN)/rune-agent:
	@$(MAKE) -C $(ROOT_DIR) $(notdir $@)

clean:
	@rm -rf $(PKG) $(TAR) $(NOTARIZE_ZIP) $(BIN)/rune-agent

pkg: $(BIN)/rune-agent
	@mkdir -p $(PKG)/bin
	@cp $(BIN)/rune-agent $(PKG)/bin/
	@cp config.yaml $(PKG)/
	@mkdir -p $(PKG)/skills
	@cp -r skills/code-* $(PKG)/skills/
	@go_files=$$(find $(PKG) -name '*.go'); \
		if [ -n "$$go_files" ]; then \
			echo "error: .go files found in package:" >&2; \
			echo "$$go_files" >&2; \
			exit 1; \
		fi

ifeq ($(UNAME),Darwin)
sign: pkg
	codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" $(PKG)/bin/rune-agent

$(NOTARIZE_ZIP): sign
	zip $(NOTARIZE_ZIP) $(PKG)/bin/rune-agent

notarize: $(NOTARIZE_ZIP)
	xcrun notarytool submit $(NOTARIZE_ZIP) --keychain-profile "$(NOTARY_PROFILE)"

$(TAR): sign
	cd $(PKG) && tar -czvf $(TAR) .

dist: clean $(TAR)
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use rune-agent-prod-dist or rune-agent-staging-dist}
	@cd $(ROOT_DIR) && BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR $(ROOT_DIST_SH)

dist-notarized: clean notarize $(TAR)
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use rune-agent-prod-dist-notarized or rune-agent-staging-dist-notarized}
	@cd $(ROOT_DIR) && BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR $(ROOT_DIST_SH)
else
sign: pkg
	@echo "Skipping codesign (not on macOS)"

notarize: sign
	@echo "Skipping notarization (not on macOS)"

$(TAR): pkg
	cd $(PKG) && tar -czvf $(TAR) .

dist: clean $(TAR)
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use rune-agent-prod-dist or rune-agent-staging-dist}
	@cd $(ROOT_DIR) && BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR $(ROOT_DIST_SH)

dist-notarized: dist
	@echo "Skipping notarized dist (not on macOS)"
endif

# make-release-linux builds the rune-agent bundle with the host's own toolchain
# (no Docker/QEMU), mirroring the layout produced by deploy/rune-agent/Dockerfile:
#   rune-agent/bin/rune-agent
#   rune-agent/lib/*.so.*       (transitive NEEDED libs, rpath=$ORIGIN/../lib)
#   rune-agent/config.yaml
#   rune-agent/skills/
# CGO is required (tree-sitter), so the binary links the host's shared libs.
# Native builds only support host-arch == target-arch; cross-arch must use the
# *-cross (Docker) targets.
make-release-linux:
	@if [ "$(UNAME)" != "Linux" ] || [ "$(HOST_ARCH)" != "$(RUNE_AGENT_LINUX_TARGET_ARCH)" ]; then \
		echo "make-release-linux requires a Linux host whose arch matches RUNE_AGENT_LINUX_TARGET_ARCH=$(RUNE_AGENT_LINUX_TARGET_ARCH) (host: $(UNAME)/$(HOST_ARCH))."; \
		echo "Use the *-cross targets (e.g. release-linux-$(RUNE_AGENT_LINUX_TARGET_ARCH)-cross) for cross-arch builds via Docker."; \
		exit 1; \
	fi
	@rm -rf $(RUNE_AGENT_LINUX_OUTPUT_DIR)/rune-agent
	@mkdir -p $(RUNE_AGENT_LINUX_OUTPUT_DIR)/rune-agent/bin $(RUNE_AGENT_LINUX_OUTPUT_DIR)/rune-agent/lib
	@cd $(ROOT_DIR) && CGO_ENABLED=1 CGO_LDFLAGS='-Wl,-rpath,$$ORIGIN/../lib' \
		GOOS=linux GOARCH=$(RUNE_AGENT_LINUX_TARGET_ARCH) \
		$(GO) build \
		-ldflags="$(COMMON_LDFLAGS) -X unstable.build/go-tui/debug.Package=rune" \
		-o $(RUNE_AGENT_LINUX_OUTPUT_DIR)/rune-agent/bin/rune-agent ./cmd/rune-agent
	@set -eu; \
	BIN=$(RUNE_AGENT_LINUX_OUTPUT_DIR)/rune-agent/bin/rune-agent; \
	LIB=$(RUNE_AGENT_LINUX_OUTPUT_DIR)/rune-agent/lib; \
	TRIPLET=$$(gcc -dumpmachine); \
	objdump -p "$$BIN" | awk '$$1 == "NEEDED" {print $$2}' > "$$LIB/.queue"; \
	while [ -s "$$LIB/.queue" ]; do \
		: > "$$LIB/.next"; \
		while IFS= read -r soname; do \
			case "$$soname" in \
				libc.so*|libm.so*|libpthread.so*|libdl.so*|librt.so*|ld-linux*|linux-vdso*) continue ;; \
				libcuda.so*) continue ;; \
			esac; \
			[ -f "$$LIB/$$soname" ] && continue; \
			src=""; \
			for d in /usr/lib/$$TRIPLET /lib/$$TRIPLET /usr/lib /lib; do \
				if [ -e "$$d/$$soname" ]; then src="$$d/$$soname"; break; fi; \
			done; \
			if [ -z "$$src" ]; then echo "WARN: $$soname not found"; continue; fi; \
			cp -L "$$src" "$$LIB/$$soname"; \
			echo "  bundled: $$soname"; \
			objdump -p "$$src" 2>/dev/null | awk '$$1 == "NEEDED" {print $$2}' >> "$$LIB/.next"; \
		done < "$$LIB/.queue"; \
		mv "$$LIB/.next" "$$LIB/.queue"; \
	done; \
	rm -f "$$LIB/.queue" "$$LIB/.next"
	@cp config.yaml $(RUNE_AGENT_LINUX_OUTPUT_DIR)/rune-agent/config.yaml
	@cp -r skills $(RUNE_AGENT_LINUX_OUTPUT_DIR)/rune-agent/skills
	@cd $(RUNE_AGENT_LINUX_OUTPUT_DIR) && tar --format=ustar -czf $(RUNE_AGENT_LINUX_RELEASE_TAR) rune-agent

# linux-cross-compile builds a stripped-down rune-agent bundle (no GUI deps)
# inside a Debian-based Docker image. The resulting tarball is dropped into
# $(RUNE_AGENT_LINUX_OUTPUT_DIR) along with the unpacked rune-agent/ tree.
linux-cross-compile:
	@rm -rf $(RUNE_AGENT_LINUX_OUTPUT_DIR)
	@mkdir -p $(RUNE_AGENT_LINUX_OUTPUT_DIR)
	@cd $(ROOT_DIR) && docker buildx build --rm \
		-f deploy/rune-agent/Dockerfile \
		--platform $(RUNE_AGENT_LINUX_TARGET_OS)/$(RUNE_AGENT_LINUX_TARGET_ARCH) \
		--build-arg GIT_SSH_KEY="$$GIT_SSH_KEY" \
		--build-arg RUNE_BUILD_TAG="$(VERSION)" \
		--build-arg RUNE_BUILD_COMMIT="$(COMMIT)" \
		--build-arg RUNE_RELEASE_TAR="$(RUNE_AGENT_LINUX_RELEASE_TAR)" \
		--output type=local,dest=$(RUNE_AGENT_LINUX_OUTPUT_DIR) \
		.

# release-linux-{amd64,arm64} build natively on a matching Linux host. Use the
# *-cross variants for cross-arch builds via Docker.
release-linux-amd64:
	@RUNE_AGENT_LINUX_TARGET_ARCH=amd64 \
		RUNE_AGENT_LINUX_RELEASE_TAR=rune-agent-release-linux-amd64-$$(git -C $(ROOT_DIR) describe --tags --dirty).tar.gz \
		$(MAKE) make-release-linux

release-linux-arm64:
	@RUNE_AGENT_LINUX_TARGET_ARCH=arm64 \
		RUNE_AGENT_LINUX_RELEASE_TAR=rune-agent-release-linux-arm64-$$(git -C $(ROOT_DIR) describe --tags --dirty).tar.gz \
		$(MAKE) make-release-linux

release-linux-amd64-cross:
	@RUNE_AGENT_LINUX_TARGET_ARCH=amd64 \
		RUNE_AGENT_LINUX_RELEASE_TAR=rune-agent-release-linux-amd64-$$(git -C $(ROOT_DIR) describe --tags --dirty).tar.gz \
		$(MAKE) linux-cross-compile

release-linux-arm64-cross:
	@RUNE_AGENT_LINUX_TARGET_ARCH=arm64 \
		RUNE_AGENT_LINUX_RELEASE_TAR=rune-agent-release-linux-arm64-$$(git -C $(ROOT_DIR) describe --tags --dirty).tar.gz \
		$(MAKE) linux-cross-compile

dist-linux-amd64: release-linux-amd64
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use rune-agent-prod-dist-linux-amd64 or rune-agent-staging-dist-linux-amd64}
	@cd $(ROOT_DIR) && \
		BLUE_RELEASE_TAR=$(TARGET)/rune-agent_linux_amd64/rune-agent-release-linux-amd64-$$(git describe --tags --dirty).tar.gz \
		BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR \
		BLUE_TARGET_OS=linux \
		BLUE_TARGET_ARCH=amd64 \
		$(ROOT_DIST_SH)

dist-linux-arm64: release-linux-arm64
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use rune-agent-prod-dist-linux-arm64 or rune-agent-staging-dist-linux-arm64}
	@cd $(ROOT_DIR) && \
		BLUE_RELEASE_TAR=$(TARGET)/rune-agent_linux_arm64/rune-agent-release-linux-arm64-$$(git describe --tags --dirty).tar.gz \
		BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR \
		BLUE_TARGET_OS=linux \
		BLUE_TARGET_ARCH=arm64 \
		$(ROOT_DIST_SH)

# dist-linux-*-cross mirror dist-linux-* but build the tarball through the
# Docker cross-compile path (release-linux-*-cross). The tarball path is
# identical.
dist-linux-amd64-cross: release-linux-amd64-cross
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use rune-agent-prod-dist-linux-amd64-cross or rune-agent-staging-dist-linux-amd64-cross}
	@cd $(ROOT_DIR) && \
		BLUE_RELEASE_TAR=$(TARGET)/rune-agent_linux_amd64/rune-agent-release-linux-amd64-$$(git describe --tags --dirty).tar.gz \
		BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR \
		BLUE_TARGET_OS=linux \
		BLUE_TARGET_ARCH=amd64 \
		$(ROOT_DIST_SH)

dist-linux-arm64-cross: release-linux-arm64-cross
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use rune-agent-prod-dist-linux-arm64-cross or rune-agent-staging-dist-linux-arm64-cross}
	@cd $(ROOT_DIR) && \
		BLUE_RELEASE_TAR=$(TARGET)/rune-agent_linux_arm64/rune-agent-release-linux-arm64-$$(git describe --tags --dirty).tar.gz \
		BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR \
		BLUE_TARGET_OS=linux \
		BLUE_TARGET_ARCH=arm64 \
		$(ROOT_DIST_SH)

# pkg-darwin-{amd64,arm64} cross-build the rune-agent binary for a specific
# macOS arch and stage config.yaml + skills next to it. darwin amd64
# cross-compiles natively on Apple Silicon via the -arch x86_64 cgo toolchain.
define rune_agent_pkg_darwin
	@rm -rf $(TARGET)/rune-agent_darwin_$(1)
	@mkdir -p $(TARGET)/rune-agent_darwin_$(1)/bin
	@cd $(ROOT_DIR) && CGO_ENABLED=1 GOOS=darwin GOARCH=$(1) $(2) \
		$(GO) build -ldflags="$(COMMON_LDFLAGS)" \
		-o $(TARGET)/rune-agent_darwin_$(1)/bin/rune-agent ./cmd/rune-agent
	@cp config.yaml $(TARGET)/rune-agent_darwin_$(1)/
	@mkdir -p $(TARGET)/rune-agent_darwin_$(1)/skills
	@cp -r skills/code-* $(TARGET)/rune-agent_darwin_$(1)/skills/
	@go_files=$$(find $(TARGET)/rune-agent_darwin_$(1) -name '*.go'); \
		if [ -n "$$go_files" ]; then \
			echo "error: .go files found in package:" >&2; \
			echo "$$go_files" >&2; \
			exit 1; \
		fi
endef

pkg-darwin-amd64:
	$(call rune_agent_pkg_darwin,amd64,$(DARWIN_AMD64_TARGET_ARCH_FLAGS))

pkg-darwin-arm64:
	$(call rune_agent_pkg_darwin,arm64,)

ifeq ($(UNAME),Darwin)
sign-darwin-amd64: pkg-darwin-amd64
	codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" $(TARGET)/rune-agent_darwin_amd64/bin/rune-agent

sign-darwin-arm64: pkg-darwin-arm64
	codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" $(TARGET)/rune-agent_darwin_arm64/bin/rune-agent

notarize-darwin-amd64: sign-darwin-amd64
	zip $(TARGET)/rune-agent-notarize-darwin-amd64.zip $(TARGET)/rune-agent_darwin_amd64/bin/rune-agent
	xcrun notarytool submit $(TARGET)/rune-agent-notarize-darwin-amd64.zip --keychain-profile "$(NOTARY_PROFILE)"

notarize-darwin-arm64: sign-darwin-arm64
	zip $(TARGET)/rune-agent-notarize-darwin-arm64.zip $(TARGET)/rune-agent_darwin_arm64/bin/rune-agent
	xcrun notarytool submit $(TARGET)/rune-agent-notarize-darwin-arm64.zip --keychain-profile "$(NOTARY_PROFILE)"
else
sign-darwin-amd64: pkg-darwin-amd64
	@echo "Skipping codesign (not on macOS)"

sign-darwin-arm64: pkg-darwin-arm64
	@echo "Skipping codesign (not on macOS)"

notarize-darwin-amd64: sign-darwin-amd64
	@echo "Skipping notarization (not on macOS)"

notarize-darwin-arm64: sign-darwin-arm64
	@echo "Skipping notarization (not on macOS)"
endif

dist-darwin-amd64: sign-darwin-amd64
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use rune-agent-prod-dist-darwin-amd64 or rune-agent-staging-dist-darwin-amd64}
	@cd $(TARGET)/rune-agent_darwin_amd64 && tar -czvf $(TARGET)/rune-agent-darwin-amd64.tar.gz .
	@cd $(ROOT_DIR) && \
		BLUE_RELEASE_TAR=$(TARGET)/rune-agent-darwin-amd64.tar.gz \
		BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR \
		BLUE_TARGET_OS=darwin \
		BLUE_TARGET_ARCH=amd64 \
		$(ROOT_DIST_SH)

dist-darwin-arm64: sign-darwin-arm64
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use rune-agent-prod-dist-darwin-arm64 or rune-agent-staging-dist-darwin-arm64}
	@cd $(TARGET)/rune-agent_darwin_arm64 && tar -czvf $(TARGET)/rune-agent-darwin-arm64.tar.gz .
	@cd $(ROOT_DIR) && \
		BLUE_RELEASE_TAR=$(TARGET)/rune-agent-darwin-arm64.tar.gz \
		BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR \
		BLUE_TARGET_OS=darwin \
		BLUE_TARGET_ARCH=arm64 \
		$(ROOT_DIST_SH)
