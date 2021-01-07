GO=go
GOTESTFLAGS=-race -timeout 20s
GOFLAGS=

TARGET=bin
WASM_EXAMPLE_TARGET=bin/example_wasm
WASM_EXAMPLE=examples/wasm/main.go
WASM_EXAMPLE_STATIC=examples/wasm/static
LIBSRC=$(wildcard *.go) $(wildcard **/*.go) $(wildcard **/**/*.go)
EXAMPLESRC=$(filter-out $(WASM_EXAMPLE), $(wildcard examples/**/*.go))
EXAMPLEDIRS=$(sort $(dir $(EXAMPLESRC)))
EXAMPLES_NON_WASM=$(patsubst examples/%/,$(TARGET)/example_%,$(EXAMPLEDIRS))
EXAMPLE_WASM_BLOB=$(WASM_EXAMPLE_TARGET)/main.wasm
EXAMPLES=$(EXAMPLES_NON_WASM)
EXECSRC=$(wildcard cmd/**/*.go)
EXECDIRS=$(sort $(dir $(EXECSRC)))
EXECS=$(patsubst cmd/%/,$(TARGET)/%,$(EXECDIRS))
PROTO=proto/*.pb.go
GOMOCKS=$(wildcard **/*_gomock.go)

.PHONY: clean test coverage example_wasm generate

default: $(EXAMPLES) $(EXECS)

debug: GOFLAGS=-race
debug: $(EXAMPLES) $(EXECS)

example_wasm: $(EXAMPLE_WASM_BLOB)

test: $(EXAMPLES) $(EXECS)
	@ go test ./.../... $(GOTESTFLAGS)

coverage: $(TARGET)
	@ go test ./.../... -coverprofile $(TARGET)/coverage
	@ go tool cover -html=$(TARGET)/coverage

generate:
	@ rm -rf $(GOMOCKS) $(PROTO)
	@ go generate ./...

install:
	@ go install ./...

clean:
	@-rm -rf $(TARGET)

$(TARGET):
	@mkdir $(TARGET)

$(WASM_EXAMPLE_TARGET): $(TARGET) $(WASM_EXAMPLE_STATIC)
	@rm -rf $(WASM_EXAMPLE_TARGET)
	@mkdir $(WASM_EXAMPLE_TARGET)
	@cp -R $(WASM_EXAMPLE_STATIC)/* $(WASM_EXAMPLE_TARGET)
	@cp `go env GOROOT`/misc/wasm/wasm_exec.js $(WASM_EXAMPLE_TARGET)/js

$(EXAMPLE_WASM_BLOB): $(WASM_EXAMPLE) $(LIBSRC) $(WASM_EXAMPLE_TARGET)
	@GOOS=js GOARCH=wasm $(GO) build -o $(EXAMPLE_WASM_BLOB) $(WASM_EXAMPLE)

$(EXAMPLES_NON_WASM): $(EXAMPLESRC) $(LIBSRC) $(TARGET)
	@cd $(patsubst bin/example_%,examples/%,$@) && $(GO) build $(GOFLAGS) -o ../../$@

$(EXECS): $(EXECSRC) $(LIBSRC) $(TARGET)
	@cd $(patsubst bin/%,cmd/%,$@) && $(GO) build $(GOFLAGS) -o ../../$@
