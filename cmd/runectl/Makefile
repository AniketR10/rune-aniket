ROOT_DIR ?= ../..
GO ?= go
UNAME := $(shell uname)
CODESIGN_IDENTITY ?= Developer ID Application: Unstable Build, LLC. (YYZRWD888J)
NOTARY_PROFILE ?= notary-profile

BIN := $(abspath $(ROOT_DIR)/bin)
PKG := $(abspath $(ROOT_DIR)/pkg/runectl)
TAR := $(abspath $(ROOT_DIR)/runectl.tar.gz)
NOTARIZE_ZIP := $(abspath $(ROOT_DIR)/runectl-notarize.zip)
DIST_SH := ./dist.sh
ROOT_DIST_SH := ./cmd/runectl/dist.sh

TARGET := $(abspath $(ROOT_DIR)/target)
# runectl lives in the rune-go-sdk module; it cross-compiles cleanly with
# CGO disabled, so the whole os/arch matrix is a plain GOOS/GOARCH go build.
RUNECTL_PKG := github.com/unstablebuild/rune-go-sdk/cmd/runectl

.PHONY: clean build pkg sign notarize dist dist-notarized \
	pkg-linux-amd64 pkg-linux-arm64 pkg-darwin-amd64 pkg-darwin-arm64 \
	sign-darwin-amd64 sign-darwin-arm64 \
	notarize-darwin-amd64 notarize-darwin-arm64 \
	dist-linux-amd64 dist-linux-arm64 dist-darwin-amd64 dist-darwin-arm64

build: $(BIN)/runectl

$(BIN)/runectl:
	@mkdir -p $(BIN)
	@GOBIN="$(BIN)" $(GO) install github.com/unstablebuild/rune-go-sdk/cmd/runectl

clean:
	@rm -rf $(PKG) $(TAR) $(NOTARIZE_ZIP) $(BIN)/runectl $(TARGET)/runectl_*

pkg: $(BIN)/runectl
	@mkdir -p $(PKG)/bin
	@cp $(BIN)/runectl $(PKG)/bin/

ifeq ($(UNAME),Darwin)
sign: pkg
	codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" $(PKG)/bin/runectl

$(NOTARIZE_ZIP): sign
	zip $(NOTARIZE_ZIP) $(PKG)/bin/runectl

notarize: $(NOTARIZE_ZIP)
	xcrun notarytool submit $(NOTARIZE_ZIP) --keychain-profile "$(NOTARY_PROFILE)"

$(TAR): sign
	cd $(PKG) && tar -czvf $(TAR) .

dist: clean $(TAR)
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use runectl-prod-dist or runectl-staging-dist}
	@cd $(ROOT_DIR) && BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR $(ROOT_DIST_SH)

dist-notarized: clean notarize $(TAR)
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use runectl-prod-dist-notarized or runectl-staging-dist-notarized}
	@cd $(ROOT_DIR) && BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR $(ROOT_DIST_SH)
else
sign: pkg
	@echo "Skipping codesign (not on macOS)"

notarize: sign
	@echo "Skipping notarization (not on macOS)"

$(TAR): pkg
	cd $(PKG) && tar -czvf $(TAR) .

dist: clean $(TAR)
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use runectl-prod-dist or runectl-staging-dist}
	@cd $(ROOT_DIR) && BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR $(ROOT_DIST_SH)

dist-notarized: dist
	@echo "Skipping notarized dist (not on macOS)"
endif

# pkg-{os}-{arch} cross-build the runectl binary for a specific target and
# stage it under target/runectl_{os}_{arch}/bin. CGO is off, so every
# combination builds directly on the host with no cross toolchain.
define runectl_pkg
	@rm -rf $(TARGET)/runectl_$(1)_$(2)
	@mkdir -p $(TARGET)/runectl_$(1)_$(2)/bin
	@cd $(ROOT_DIR) && CGO_ENABLED=0 GOOS=$(1) GOARCH=$(2) \
		$(GO) build -o $(TARGET)/runectl_$(1)_$(2)/bin/runectl $(RUNECTL_PKG)
endef

pkg-linux-amd64:
	$(call runectl_pkg,linux,amd64)

pkg-linux-arm64:
	$(call runectl_pkg,linux,arm64)

pkg-darwin-amd64:
	$(call runectl_pkg,darwin,amd64)

pkg-darwin-arm64:
	$(call runectl_pkg,darwin,arm64)

ifeq ($(UNAME),Darwin)
sign-darwin-amd64: pkg-darwin-amd64
	codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" $(TARGET)/runectl_darwin_amd64/bin/runectl

sign-darwin-arm64: pkg-darwin-arm64
	codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" $(TARGET)/runectl_darwin_arm64/bin/runectl

notarize-darwin-amd64: sign-darwin-amd64
	zip $(TARGET)/runectl-notarize-darwin-amd64.zip $(TARGET)/runectl_darwin_amd64/bin/runectl
	xcrun notarytool submit $(TARGET)/runectl-notarize-darwin-amd64.zip --keychain-profile "$(NOTARY_PROFILE)"

notarize-darwin-arm64: sign-darwin-arm64
	zip $(TARGET)/runectl-notarize-darwin-arm64.zip $(TARGET)/runectl_darwin_arm64/bin/runectl
	xcrun notarytool submit $(TARGET)/runectl-notarize-darwin-arm64.zip --keychain-profile "$(NOTARY_PROFILE)"
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

# dist-{os}-{arch} tar the staged bundle and upload it stamped with the
# artifact's real target os/arch (via BLUE_TARGET_OS / BLUE_TARGET_ARCH).
# darwin targets notarize first so published macOS binaries are signed.
dist-linux-amd64: pkg-linux-amd64
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use runectl-prod-dist-linux-amd64 or runectl-staging-dist-linux-amd64}
	@cd $(TARGET)/runectl_linux_amd64 && tar -czvf $(TARGET)/runectl-linux-amd64.tar.gz .
	@cd $(ROOT_DIR) && \
		BLUE_RELEASE_TAR=$(TARGET)/runectl-linux-amd64.tar.gz \
		BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR \
		BLUE_TARGET_OS=linux \
		BLUE_TARGET_ARCH=amd64 \
		$(ROOT_DIST_SH)

dist-linux-arm64: pkg-linux-arm64
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use runectl-prod-dist-linux-arm64 or runectl-staging-dist-linux-arm64}
	@cd $(TARGET)/runectl_linux_arm64 && tar -czvf $(TARGET)/runectl-linux-arm64.tar.gz .
	@cd $(ROOT_DIR) && \
		BLUE_RELEASE_TAR=$(TARGET)/runectl-linux-arm64.tar.gz \
		BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR \
		BLUE_TARGET_OS=linux \
		BLUE_TARGET_ARCH=arm64 \
		$(ROOT_DIST_SH)

dist-darwin-amd64: notarize-darwin-amd64
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use runectl-prod-dist-darwin-amd64 or runectl-staging-dist-darwin-amd64}
	@cd $(TARGET)/runectl_darwin_amd64 && tar -czvf $(TARGET)/runectl-darwin-amd64.tar.gz .
	@cd $(ROOT_DIR) && \
		BLUE_RELEASE_TAR=$(TARGET)/runectl-darwin-amd64.tar.gz \
		BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR \
		BLUE_TARGET_OS=darwin \
		BLUE_TARGET_ARCH=amd64 \
		$(ROOT_DIST_SH)

dist-darwin-arm64: notarize-darwin-arm64
	@: $${BLUECTL_CONFIG_DIR:?BLUECTL_CONFIG_DIR not set; use runectl-prod-dist-darwin-arm64 or runectl-staging-dist-darwin-arm64}
	@cd $(TARGET)/runectl_darwin_arm64 && tar -czvf $(TARGET)/runectl-darwin-arm64.tar.gz .
	@cd $(ROOT_DIR) && \
		BLUE_RELEASE_TAR=$(TARGET)/runectl-darwin-arm64.tar.gz \
		BLUECTL_CONFIG_DIR=$$BLUECTL_CONFIG_DIR \
		BLUE_TARGET_OS=darwin \
		BLUE_TARGET_ARCH=arm64 \
		$(ROOT_DIST_SH)
