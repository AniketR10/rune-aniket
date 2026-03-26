GO=go
CI ?= false
GOTESTFLAGS ?= -race -timeout 120s
GOTESTFLAGSNORACE = -timeout 120s
COMMON_LDFLAGS=-X unstable.build/go-tui/debug.Tag=$$(git describe --tags) -X unstable.build/go-tui/debug.Commit=$$(git rev-parse --short HEAD)
GOFLAGS=-ldflags="$(COMMON_LDFLAGS) -X unstable.build/go-tui/debug.Package=six"
RUNE_GOFLAGS=-tags=ebitensinglethread -ldflags="$(COMMON_LDFLAGS) -X unstable.build/go-tui/debug.Package=rune"
OXAPI_GOFLAGS=-ldflags="$(COMMON_LDFLAGS) -X unstable.build/go-tui/debug.Package=rune"
UNAME := $(shell uname)
VERSION=$(shell git describe --tags)
COMMIT=$(shell git rev-parse --short HEAD)
CODESIGN_IDENTITY ?= Developer ID Application: Unstable Build, LLC. (YYZRWD888J)
NOTARY_PROFILE ?= notary-profile

BIN=bin
TARGET=target
LIBSRC=$(wildcard *.go) $(wildcard **/*.go) $(wildcard **/**/*.go) $(wildcard **/**/**/*.go)
EXECSRC=$(wildcard cmd/**/*.go) $(wildcard cmd/**/**/*.go)
EXECMAIN=$(wildcard cmd/*/main.go)
EXECDIRS=$(sort $(dir $(EXECMAIN)))
EXECS=$(patsubst cmd/%/,$(BIN)/%,$(EXECDIRS))
CLAUDEIMPORT=$(BIN)/claudeimport
SPECIAL_EXECS=$(BIN)/rune $(BIN)/ox-api $(BIN)/rune-agent $(CLAUDEIMPORT)
GENERIC_EXECS=$(filter-out $(SPECIAL_EXECS),$(EXECS))
EXEC_PKGS=$(patsubst $(BIN)/%,./cmd/%,$(EXECS))
RELEASE_EXEC_PKGS=$(EXEC_PKGS)
GOMOCKS=$(wildcard **/**/*_gomock.go) $(wildcard **/*_gomock.go)
RELEASE_FILES=$(wildcard release/*)
RUNE_RELEASE_DIR=$(TARGET)/rune_darwin_app
RUNE_APP_NAME=Rune.app
RUNE_APP_TEMPLATE=extra/osx/$(RUNE_APP_NAME)
RUNE_APP_DIR=$(RUNE_RELEASE_DIR)
RUNE_APP_BINARY=$(RUNE_RELEASE_DIR)/rune
RUNE_APP_BINARY_DIR=$(RUNE_APP_DIR)/$(RUNE_APP_NAME)/Contents/MacOS
RUNE_APP_EXTRAS_DIR=$(RUNE_APP_DIR)/$(RUNE_APP_NAME)/Contents/Resources
RUNE_APP_PLIST=$(RUNE_APP_DIR)/$(RUNE_APP_NAME)/Contents/Info.plist
RUNE_APP_ICON=$(RUNE_APP_EXTRAS_DIR)/rune.icns
RUNE_APP_CLAUDEIMPORT=$(RUNE_APP_EXTRAS_DIR)/claudeimport
RUNE_DMG_NAME=Rune.dmg
RUNE_DMG_DIR=$(RUNE_RELEASE_DIR)
SED_INPLACE = ''

ifeq ($(OS),Darwin)
SED_INPLACE = ''
endif

.PHONY: debug clean test coverage generate sixdev rune rune-agent ox-api claudeimport \
	format docker-build-ci-gcp docker-push-ci-gcp cross-compile lint license assert_license \
	rune-release rune-release-amd64 rune-release-arm64 rune-make-release \
	rune-docker-build rune-docker-run rune-docker-build-gcp rune-docker-push-gcp \
	rune-docker-build-ci-gcp rune-docker-push-ci-gcp rune-app-amd64 rune-app-arm64 \
	rune-dmg rune-dmg-notarize rune-release-all

default: CGO_ENABLED=CGO_ENABLED=1
default: GOPRIVATE=github.com/unstablebuild,unstable.build/*
default: .git/hooks/pre-commit $(EXECS) $(CLAUDEIMPORT)

debug: GOFLAGS=-race
debug: CGO_ENABLED=CGO_ENABLED=1
debug: GOPRIVATE=github.com/unstablebuild,unstable.build/*
debug: $(EXECS) $(CLAUDEIMPORT)

sixdev: GOFLAGS=-race
sixdev: CGO_ENABLED=CGO_ENABLED=1
sixdev: bin/six

rune: CGO_ENABLED=CGO_ENABLED=1
rune: GOPRIVATE=github.com/unstablebuild,unstable.build/*
rune: $(BIN)/rune

rune-agent: CGO_ENABLED=CGO_ENABLED=1
rune-agent: GOPRIVATE=github.com/unstablebuild,unstable.build/*
rune-agent: $(BIN)/rune-agent

ox-api: CGO_ENABLED=CGO_ENABLED=1
ox-api: GOPRIVATE=github.com/unstablebuild,unstable.build/*
ox-api: $(BIN)/ox-api

claudeimport: CGO_ENABLED=CGO_ENABLED=1
claudeimport: GOPRIVATE=github.com/unstablebuild,unstable.build/*
claudeimport: $(CLAUDEIMPORT)

.git/hooks/pre-commit: .pre-commit-config.yaml
	@ pre-commit install

test: CI=$(CI)
test:
	@ go test -vet=off ./.../... $(GOTESTFLAGS)

test: CI=$(CI)
test-no-race:
	@ go test ./.../... $(GOTESTFLAGSNORACE)

coverage: $(BIN)
	@ go test ./.../... -coverprofile $(BIN)/coverage
	@ go tool cover -html=$(BIN)/coverage

generate: GOPRIVATE=github.com/unstablebuild,unstable.build/*
generate:
	@ rm -rf **/*rpc*/*.pb.go
	@ go generate ./...

license:
	@ bluectl license LICENSE `find . -name \*.go | grep -v gomock | grep -v .pb.go | xargs`

assert_license:
	@ bluectl license -d LICENSE `find . -name \*.go | grep -v gomock | grep -v .pb.go | xargs`

format:
	@ go fmt ./.../...

cross-compile:
	@ . ./test_crosscompile.sh

lint:
	@ golangci-lint run --timeout=600s

clean:
	@rm -rf $(BIN) $(TARGET)

$(BIN):
	@mkdir $(BIN)

$(BIN)/rune: $(EXECSRC) $(LIBSRC) $(BIN)
	@cd cmd/rune && $(CGO_ENABLED) $(GO) build $(RUNE_GOFLAGS) -o ../../$@

$(BIN)/ox-api: $(EXECSRC) $(LIBSRC) $(BIN)
	@cd cmd/ox-api && $(CGO_ENABLED) $(GO) build $(OXAPI_GOFLAGS) -o ../../$@

$(BIN)/claudeimport: $(EXECSRC) $(LIBSRC) $(BIN)
	@cd cmd/claudeimport && $(CGO_ENABLED) $(GO) build $(GOFLAGS) -o ../../$@

$(GENERIC_EXECS): $(EXECSRC) $(LIBSRC) $(BIN)
	cd $(patsubst bin/%,cmd/%,$@) && $(CGO_ENABLED) $(GO) build $(GOFLAGS) -o ../../$@

make_release: CGO_ENABLED=CGO_ENABLED=1
make_release:
	@ mkdir -p $(TARGET)/$(TARGET_OS)_$(TARGET_ARCH)
	@ cp $(RELEASE_FILES) $(TARGET)
	@ $(CGO_ENABLED) GOARCH=$(TARGET_ARCH) $(TARGET_ARCH_FLAGS) GOOS=$(TARGET_OS) $(GO) build -o `pwd`/$(TARGET)/$(TARGET_OS)_$(TARGET_ARCH) $(GOFLAGS) $(RELEASE_EXEC_PKGS)

ifeq ($(UNAME), Linux)
release: default
	@ rm -rf $(TARGET)
	@ TARGET_OS=linux TARGET_ARCH=amd64 $(MAKE) make_release
	@ cd $(TARGET) && tar -czvf six-release-`git describe --tags --dirty`.tar.gz *
endif
ifeq ($(UNAME), Darwin)
release: default
	@ rm -rf $(TARGET)
	@ TARGET_OS=darwin TARGET_ARCH=arm64 $(MAKE) make_release
	@ TARGET_OS=darwin TARGET_ARCH=amd64 $(MAKE) make_release
	@ cd $(TARGET) && tar -czvf six-release-`git describe --tags --dirty`.tar.gz *
endif

dist: release
	@ git fetch origin --tags
	@ ./dist.sh

docker-build-ci-gcp:
	@ docker buildx build -f Dockerfile.build --platform linux/amd64 -t us-central1-docker.pkg.dev/unstable-build-blue-dev/docker/go-tui-ci:latest --build-arg GIT_SSH_KEY="$$GIT_SSH_KEY" .

docker-push-ci-gcp:
	@ docker push us-central1-docker.pkg.dev/unstable-build-blue-dev/docker/go-tui-ci:latest

rune-make-release: CGO_ENABLED=CGO_ENABLED=1
rune-make-release:
	@ mkdir -p $(TARGET)/rune_$(TARGET_OS)_$(TARGET_ARCH)
	@ $(CGO_ENABLED) GOARCH=$(TARGET_ARCH) $(TARGET_ARCH_FLAGS) GOOS=$(TARGET_OS) $(GO) build $(RUNE_GOFLAGS) -o `pwd`/$(TARGET)/rune_$(TARGET_OS)_$(TARGET_ARCH)/rune ./cmd/rune
	@ CGO_ENABLED=0 GOARCH=$(TARGET_ARCH) $(TARGET_ARCH_FLAGS) GOOS=$(TARGET_OS) $(GO) build $(OXAPI_GOFLAGS) -o `pwd`/$(TARGET)/rune_$(TARGET_OS)_$(TARGET_ARCH)/ox-api ./cmd/ox-api
	@ CGO_ENABLED=0 GOARCH=$(TARGET_ARCH) $(TARGET_ARCH_FLAGS) GOOS=$(TARGET_OS) $(GO) build $(GOFLAGS) -o `pwd`/$(TARGET)/rune_$(TARGET_OS)_$(TARGET_ARCH)/claudeimport ./cmd/rune/claudeimport

ifeq ($(UNAME), Linux)
rune-release: rune ox-api
	@ rm -rf $(TARGET)/rune_linux_amd64
	@ TARGET_OS=linux TARGET_ARCH=amd64 $(MAKE) rune-make-release
	@ cd $(TARGET) && tar -czvf rune-release-`git describe --tags --dirty`.tar.gz rune_linux_amd64
endif

ifeq ($(UNAME), Darwin)
rune-release-amd64: rune ox-api
	@ rm -rf $(TARGET)/rune_darwin_amd64
	@ TARGET_OS=darwin TARGET_ARCH=amd64 $(MAKE) rune-make-release
	@ cd $(TARGET) && tar -czvf rune-release-`git describe --tags --dirty`.tar.gz rune_darwin_amd64

rune-release-arm64: rune ox-api
	@ rm -rf $(TARGET)/rune_darwin_arm64
	@ TARGET_OS=darwin TARGET_ARCH=arm64 $(MAKE) rune-make-release
	@ cd $(TARGET) && tar -czvf rune-release-`git describe --tags --dirty`.tar.gz rune_darwin_arm64

rune-release: rune ox-api
	@ rm -rf $(TARGET)/rune_darwin_arm64 $(TARGET)/rune_darwin_amd64
	@ TARGET_OS=darwin TARGET_ARCH=arm64 $(MAKE) rune-make-release
	@ TARGET_OS=darwin TARGET_ARCH=amd64 $(MAKE) rune-make-release
	@ cd $(TARGET) && tar -czvf rune-release-`git describe --tags --dirty`.tar.gz rune_darwin_arm64 rune_darwin_amd64
endif

rune-docker-build:
	@ docker build -f deploy/Dockerfile -t unstable.build/ox-api:$(VERSION) --build-arg GIT_SSH_KEY="$$GIT_SSH_KEY" .

rune-docker-run:
	@ docker run -ti --rm -e PORT=80 -p 8080:80 unstable.build/ox-api:$(VERSION)

rune-docker-build-gcp:
	@ docker buildx build -f deploy/Dockerfile --platform linux/amd64 -t us-central1-docker.pkg.dev/unstable-build-blue-dev/docker/ox-api:$(VERSION) --build-arg GIT_SSH_KEY="$$GIT_SSH_KEY" .

rune-docker-push-gcp:
	@ docker push us-central1-docker.pkg.dev/unstable-build-blue-dev/docker/ox-api:$(VERSION)

rune-docker-build-ci-gcp:
	@ docker buildx build -f deploy/Dockerfile.build --platform linux/amd64 -t us-central1-docker.pkg.dev/unstable-build-blue-dev/docker/rune-ci:latest --build-arg GIT_SSH_KEY="$$GIT_SSH_KEY" .

rune-docker-push-ci-gcp:
	@ docker push us-central1-docker.pkg.dev/unstable-build-blue-dev/docker/rune-ci:latest

ifeq ($(UNAME), Darwin)
rune-app-amd64: rune-release-amd64
	@ rm -rf $(RUNE_RELEASE_DIR)
	@ mkdir -p $(RUNE_RELEASE_DIR)
	@ cp -fp $(TARGET)/rune_darwin_amd64/rune $(RUNE_APP_BINARY)
	@ mkdir -p $(RUNE_APP_BINARY_DIR) $(RUNE_APP_EXTRAS_DIR)
	@ cp -fRp $(RUNE_APP_TEMPLATE) $(RUNE_APP_DIR)
	@ cp -fp $(RUNE_APP_BINARY) $(RUNE_APP_BINARY_DIR)
	@ cp -fp $(TARGET)/rune_darwin_amd64/claudeimport $(RUNE_APP_CLAUDEIMPORT)
	@ ibtool --compile $(RUNE_APP_EXTRAS_DIR)/MainMenu.nib $(RUNE_APP_TEMPLATE)/Contents/Resources/MainMenu.xib
	@ echo "injecting version $(VERSION) ($(COMMIT))"
	@ sed -i $(SED_INPLACE) -e 's/{{VERSION}}/$(VERSION)/g' -e 's/{{COMMIT}}/$(COMMIT)/g' $(RUNE_APP_PLIST)
	@ cp -fp $(RUNE_APP_BINARY) $(RUNE_APP_EXTRAS_DIR)/rune-extension
	GOBIN="`pwd`/$(RUNE_APP_EXTRAS_DIR)" go install github.com/unstablebuild/rune-go-sdk/cmd/runectl
	@ cd extra && iconutil -c icns icon.iconset
	@ mv extra/icon.icns $(RUNE_APP_ICON)
	@ touch -r "$(RUNE_APP_BINARY)" "$(RUNE_APP_DIR)/$(RUNE_APP_NAME)"
	@ rm -f $(RUNE_APP_BINARY)
	@ codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" "$(RUNE_APP_CLAUDEIMPORT)"
	@ codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" "$(RUNE_APP_EXTRAS_DIR)/rune-extension"
	@ codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" "$(RUNE_APP_EXTRAS_DIR)/runectl"
	@ codesign --force --deep --options runtime --sign "$(CODESIGN_IDENTITY)" "$(RUNE_APP_DIR)/$(RUNE_APP_NAME)"
	@ echo "Created '$(RUNE_APP_NAME)' in '$(RUNE_APP_DIR)'"

rune-app-arm64: rune-release-arm64
	@ rm -rf $(RUNE_RELEASE_DIR)
	@ mkdir -p $(RUNE_RELEASE_DIR)
	@ cp -fp $(TARGET)/rune_darwin_arm64/rune $(RUNE_APP_BINARY)
	@ mkdir -p $(RUNE_APP_BINARY_DIR) $(RUNE_APP_EXTRAS_DIR)
	@ cp -fRp $(RUNE_APP_TEMPLATE) $(RUNE_APP_DIR)
	@ cp -fp $(RUNE_APP_BINARY) $(RUNE_APP_BINARY_DIR)
	@ cp -fp $(TARGET)/rune_darwin_arm64/claudeimport $(RUNE_APP_CLAUDEIMPORT)
	@ ibtool --compile $(RUNE_APP_EXTRAS_DIR)/MainMenu.nib $(RUNE_APP_TEMPLATE)/Contents/Resources/MainMenu.xib
	@ echo "injecting version $(VERSION) ($(COMMIT))"
	@ sed -i $(SED_INPLACE) -e 's/{{VERSION}}/$(VERSION)/g' -e 's/{{COMMIT}}/$(COMMIT)/g' $(RUNE_APP_PLIST)
	@ cp -fp $(RUNE_APP_BINARY) $(RUNE_APP_EXTRAS_DIR)/rune-extension
	GOBIN="`pwd`/$(RUNE_APP_EXTRAS_DIR)" go install github.com/unstablebuild/rune-go-sdk/cmd/runectl
	@ cd extra && iconutil -c icns icon.iconset
	@ mv extra/icon.icns $(RUNE_APP_ICON)
	@ touch -r "$(RUNE_APP_BINARY)" "$(RUNE_APP_DIR)/$(RUNE_APP_NAME)"
	@ rm -f $(RUNE_APP_BINARY)
	@ codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" "$(RUNE_APP_CLAUDEIMPORT)"
	@ codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" "$(RUNE_APP_EXTRAS_DIR)/rune-extension"
	@ codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" "$(RUNE_APP_EXTRAS_DIR)/runectl"
	@ codesign --force --deep --options runtime --sign "$(CODESIGN_IDENTITY)" "$(RUNE_APP_DIR)/$(RUNE_APP_NAME)"
	@ echo "Created '$(RUNE_APP_NAME)' in '$(RUNE_APP_DIR)'"

rune-dmg: rune-app-arm64
	@ echo "Packing disk image..."
	@ mkdir -p $(RUNE_DMG_DIR)
	@ ln -sf /Applications $(RUNE_DMG_DIR)/Applications
	@ hdiutil create $(RUNE_DMG_DIR)/$(RUNE_DMG_NAME) \
		-volname "Rune" \
		-fs HFS+ \
		-srcfolder $(RUNE_APP_DIR) \
		-ov -format UDZO
	@ echo "Signing disk image..."
	@ codesign --force --options runtime --sign "$(CODESIGN_IDENTITY)" $(RUNE_DMG_DIR)/$(RUNE_DMG_NAME)
	@ echo "Packed $(RUNE_DMG_DIR)/$(RUNE_DMG_NAME)"

rune-dmg-notarize: rune-dmg
	@ echo "Notarizing disk image..."
	@ set -e; \
	RESULT=$$(xcrun notarytool submit $(RUNE_DMG_DIR)/$(RUNE_DMG_NAME) --keychain-profile "$(NOTARY_PROFILE)" --wait --output-format json); \
	echo "$$RESULT"; \
	ID=$$(printf '%s\n' "$$RESULT" | plutil -extract id raw -o - -); \
	STATUS=$$(printf '%s\n' "$$RESULT" | plutil -extract status raw -o - -); \
	echo "Submission ID: $$ID"; \
	echo "Status: $$STATUS"; \
	if [ "$$STATUS" != "Accepted" ]; then \
		echo "Fetching notarization log..."; \
		xcrun notarytool log "$$ID" --keychain-profile "$(NOTARY_PROFILE)"; \
		exit 1; \
	fi
	@ echo "Stapling ticket..."
	@ xcrun stapler staple $(RUNE_DMG_DIR)/$(RUNE_DMG_NAME)
	@ echo "Validating staple..."
	@ xcrun stapler validate $(RUNE_DMG_DIR)/$(RUNE_DMG_NAME)
	@ echo "Notarization completed: $(RUNE_DMG_DIR)/$(RUNE_DMG_NAME)"

rune-release-all: rune-dmg-notarize
else
rune-dmg:
	@ echo "Skipping DMG packaging (not macOS)"

rune-dmg-notarize: rune-dmg
	@ echo "Skipping DMG notarization (not macOS)"

rune-release-all:
	@ echo "Skipping rune-release-all (not macOS)"
endif
