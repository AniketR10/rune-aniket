GO=go
GOFLAGS=

TARGET=bin
LIBSRC=$(wildcard *.go) $(wildcard **/*.go)
EXAMPLESRC=$(wildcard examples/**/*.go)
EXAMPLEDIRS=$(sort $(dir $(EXAMPLESRC)))
EXAMPLES=$(patsubst examples/%/,$(TARGET)/example_%,$(EXAMPLEDIRS))
EXECSRC=$(wildcard cmd/**/*.go)
EXECDIRS=$(sort $(dir $(EXECSRC)))
EXECS=$(patsubst cmd/%/,$(TARGET)/%,$(EXECDIRS))

.PHONY: clean test coverage

default: $(EXAMPLES) $(EXECS)

test: $(EXECS)
	@ go test ./.../... -race

coverage: $(TARGET)
	@ go test ./.../... -coverprofile $(TARGET)/coverage
	@ go tool cover -html=$(TARGET)/coverage

clean:
	@-rm -rf $(TARGET)

$(TARGET):
	@mkdir $(TARGET)

$(TARGET)/example_%: $(EXAMPLESRC) $(LIBSRC) $(TARGET)
	@cd $(patsubst bin/example_%,examples/%,$@) && $(GO) build $(GOFLAGS) -o ../../$@

$(TARGET)/%: $(EXECSRC) $(LIBSRC) $(TARGET)
	@cd $(patsubst bin/%,cmd/%,$@) && $(GO) build $(GOFLAGS) -o ../../$@
