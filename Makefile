GO=go
GOTESTFLAGS=-race -timeout 180s
GOTESTFLAGSNORACE=-timeout 180s
GOFLAGS="-ldflags=-X main.Tag=$$(git describe --tags) -X main.Commit=$$(git rev-parse --short HEAD)"

BIN=bin
TARGET=target
WASM_EXAMPLE_TARGET=bin/example_wasm
WASM_EXAMPLE=examples/wasm/main.go
WASM_EXAMPLE_STATIC=examples/wasm/static
LIBSRC=$(wildcard *.go) $(wildcard **/*.go) $(wildcard **/**/*.go)
EXAMPLESRC=$(filter-out $(WASM_EXAMPLE), $(wildcard examples/**/*.go))
EXAMPLEDIRS=$(sort $(dir $(EXAMPLESRC)))
EXAMPLES_NON_WASM=$(patsubst examples/%/,$(BIN)/example_%,$(EXAMPLEDIRS))
EXAMPLE_WASM_BLOB=$(WASM_EXAMPLE_TARGET)/main.wasm
EXAMPLES=$(EXAMPLES_NON_WASM)
EXECSRC=$(wildcard cmd/**/*.go) $(wildcard cmd/**/**/*.go)
EXECDIRS=$(sort $(dir $(EXECSRC)))
EXECS=$(patsubst cmd/%/,$(BIN)/%,$(EXECDIRS))
GOMOCKS=$(wildcard **/**/*_gomock.go) $(wildcard **/*_gomock.go)

.PHONY: debug clean test coverage example_wasm generate

default: CGO_ENABLED=CGO_ENABLED=0
default: $(EXAMPLES) $(EXECS)

debug: GOFLAGS=-race
debug: CGO_ENABLED=CGO_ENABLED=1
debug: $(EXAMPLES) $(EXECS)

example_wasm: $(EXAMPLE_WASM_BLOB)

test:
	@ go test ./.../... $(GOTESTFLAGS)

test-no-race:
	@ go test ./.../... $(GOTESTFLAGSNORACE)

coverage: $(BIN)
	@ go test ./.../... -coverprofile $(BIN)/coverage
	@ go tool cover -html=$(BIN)/coverage

generate:
	@ rm -rf **/rpc/*.pb.go
	@ go generate ./...
	@ mv browser/unstable.build/go-tui/browser/rpc/* browser/rpc
	@ mv text/unstable.build/go-tui/text/rpc/* text/rpc
	@ mv plugin/unstable.build/go-tui/plugin/rpc/* plugin/rpc
	@ mv workspace/unstable.build/go-tui/workspace/rpc/* workspace/rpc
	@ mv term/unstable.build/go-tui/term/rpc/* term/rpc
	@ mv handler/unstable.build/go-tui/handler/rpc/* handler/rpc
	@ mv config/unstable.build/go-tui/config/rpc/* config/rpc
	@ rm -rf **/unstable.build **/github.com

install:
	@ go install ./...

clean:
	@rm -rf $(BIN) $(TARGET)

$(BIN):
	@mkdir $(BIN)

$(WASM_EXAMPLE_TARGET): $(BIN) $(WASM_EXAMPLE_STATIC)
	@rm -rf $(WASM_EXAMPLE_TARGET)
	@mkdir $(WASM_EXAMPLE_TARGET)
	@cp -R $(WASM_EXAMPLE_STATIC)/* $(WASM_EXAMPLE_TARGET)
	@cp `go env GOROOT`/misc/wasm/wasm_exec.js $(WASM_EXAMPLE_TARGET)/js

$(EXAMPLE_WASM_BLOB): $(WASM_EXAMPLE) $(LIBSRC) $(WASM_EXAMPLE_TARGET)
	$(CGO_ENABLED) GOOS=js GOARCH=wasm $(GO) build -o $(EXAMPLE_WASM_BLOB) $(WASM_EXAMPLE)

$(EXAMPLES_NON_WASM): $(EXAMPLESRC) $(LIBSRC) $(BIN)
	@cd $(patsubst bin/example_%,examples/%,$@) && $(CGO_ENABLED) $(GO) build $(GOFLAGS) -o ../../$@

$(EXECS): $(EXECSRC) $(LIBSRC) $(BIN)
	@cd $(patsubst bin/%,cmd/%,$@) && $(CGO_ENABLED) $(GO) build $(GOFLAGS) -o ../../$@

make_release:
	@ mkdir -p $(TARGET)/$(TARGET_OS)_$(TARGET_ARCH)
	@ CGO_ENABLED=0 GOARCH=$(TARGET_ARCH) $(TARGET_ARCH_FLAGS) GOOS=$(TARGET_OS) $(GO) build $(GOFLAGS) -o `pwd`/$(TARGET)/$(TARGET_OS)_$(TARGET_ARCH) ./cmd/...

release_arm:
	@ rm -rf $(TARGET)
	@ TARGET_OS=linux TARGET_ARCH=arm TARGET_ARCH_FLAGS=GOARM=7 $(MAKE) make_release
	@ cd $(TARGET) && tar -czvf six-release-`git describe --tags --dirty`.tar.gz *

release: default
	@ rm -rf $(TARGET)
	@ TARGET_OS=linux TARGET_ARCH=amd64 $(MAKE) make_release
	@ TARGET_OS=darwin TARGET_ARCH=amd64 $(MAKE) make_release
	@ cd $(TARGET) && tar -czvf six-release-`git describe --tags --dirty`.tar.gz *

dist: release
	@ git fetch origin --tags
	@ ./dist.sh

lint:
	@ .githooks/pre-commit
