CC=go

TARGET=bin
LIBSRC=$(wildcard *.go)
EXECSRC=$(wildcard examples/**/*.go)
EXECDIRS=$(sort $(dir $(EXECSRC)))
EXEC=$(patsubst examples/%/,$(TARGET)/%,$(EXECDIRS))

.PHONY: clean test coverage

default: CHECK $(EXEC)

test: CHECK
	@ go test

coverage: CHECK $(TARGET)
	@ go test -coverprofile $(TARGET)/coverage
	@ go tool cover -html=$(TARGET)/coverage

install:
	@ go install

clean:
	@-rm -rf $(TARGET)

$(TARGET):
	@mkdir $(TARGET)

$(TARGET)/%: $(EXECSRC) $(LIBSRC) $(TARGET)
	@cd $(patsubst bin/%,examples/%,$@) && $(CC) build -o ../../$@

CHECK:
ifndef GOPATH
	$(error GOPATH is undefined)
endif
