CC=go

ifndef GOBIN
	GOBIN=$(GOPATH)/bin
endif

TARGET=bin
PWD=$(shell pwd)
SRC=$(wildcard **/*.go)
PKGS=$(sort $(dir $(SRC)))
EXECS=$(sort $(dir $(wildcard examples/*/)))
EXECSRC=$(wildcard examples/**/*.go)
EXEC=$(patsubst examples/%/,$(TARGET)/%,$(EXECS))
GEXEC=$(patsubst examples/%/,$(GOBIN)/%,$(EXECS))
TESTSRC=$(wildcard **/*_test.go)
TEST=$(patsubst %_test.go,$(TARGET)/%_test,$(TESTSRC))
TESTFLAGS=-i

.PHONY: clean install test coverage

default: CHECK $(PKGS) $(EXEC)

install: CHECK $(PKGS) $(GEXEC)

test: $(TEST)

clean:
	@-rm -rf $(TARGET)

$(TARGET):
	@mkdir $(TARGET)

$(EXEC): $(EXECS) $(EXECSRC) $(SRC) $(TARGET)
	@cd $(patsubst bin/%,examples/%,$@) && $(CC) build -o ../../$@

$(PKGS): $(SRC) FORCE
	@cd $@ && $(CC) install

$(GEXEC): $(EXECS)
	@cd $< && $(CC) build -o $@

ifeq ($(MAKECMDGOALS),coverage)
TESTFLAGS += -cover
endif

$(TEST): $(TESTSRC) $(SRC) FORCE
	@mkdir -p $(dir $@)
	@cd $(patsubst bin/%,%,$(dir $@)) && $(CC) test $(TESTFLAGS) -o ../$@
ifeq ($(MAKECMDGOALS),coverage)
	@./$@ -test.coverprofile $@.coverage
else
	@printf "%30s ⇒ " $@ && ./$@
endif

coverage: $(TEST)
	@for t in $(TEST); do go tool cover -html=$$t.coverage; done;

FORCE:

CHECK:
ifndef GOPATH
	$(error GOPATH is undefined)
endif
