CC=go

ifndef GOBIN
	GOBIN=$(GOPATH)/bin
endif

TARGET=bin
PWD=$(shell pwd)
SRC=$(wildcard **/*.go)
PKGS=$(sort $(dir $(SRC)))
EXECS=$(sort $(dir $(wildcard cmd/*/)))
EXECSRC=$(wildcard cmd/**/*.go)
EXEC=$(patsubst cmd/%/,$(TARGET)/%,$(EXECS))
GEXEC=$(patsubst cmd/%/,$(GOBIN)/%,$(EXECS))
TESTSRC=$(wildcard **/*_test.go)
TEST=$(patsubst %_test.go,$(TARGET)/%_test,$(TESTSRC))


.PHONY: clean install test

default: CHECK $(EXEC)

install: CHECK $(PKGS) $(GEXEC)

test: $(TEST)

clean:
	@-rm -rf $(TARGET)

$(TARGET):
	@mkdir $(TARGET)

$(EXEC): $(EXECS) $(EXECSRC) $(SRC) $(TARGET)
	@cd $< && $(CC) build -o $(PWD)/$(patsubst cmd/%,$(TARGET)/%,$@)

$(PKGS): $(SRC) FORCE
	@cd $@ && $(CC) install

$(GEXEC): $(EXECS)
	@cd $< && $(CC) build -o $@

$(TEST): $(TESTSRC) $(SRC) FORCE
	@mkdir -p $(dir $@)
	@cd $(patsubst bin/%,%,$(dir $@)) && $(CC) test -i -o ../$@
	@printf "%30s ⇒ " $@ && ./$@

FORCE:

CHECK:
ifndef GOPATH
	$(error GOPATH is undefined)
endif
