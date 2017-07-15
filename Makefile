CC=go

TARGET=bin
PWD=$(shell pwd)
SRC=$(wildcard **/*.go)
PKGS=$(sort $(dir $(wildcard **/*.go)))
EXECS=$(sort $(dir $(wildcard cmd/*/)))
EXECSRC=$(wildcard cmd/**/*.go)
EXEC=$(patsubst cmd/%/,$(TARGET)/%,$(EXECS))
GEXEC=$(patsubst cmd/%/,$(GOBIN)/%,$(EXECS))


.PHONY: clean install

default: $(EXEC)

install: $(GEXEC) $(PKGS)

clean:
	@-rm -rf $(TARGET)

$(TARGET):
	@mkdir $(TARGET)

$(EXEC): $(EXECS) $(EXECSRC) $(SRC) $(TARGET)
	@cd $< && $(CC) build -o $(PWD)/$(patsubst cmd/%,$(TARGET)/%,$@)

$(PKGS): FORCE
	@cd $@ && $(CC) install

FORCE:

$(GEXEC): $(EXECS)
	@cd $< && $(CC) build -o $@
