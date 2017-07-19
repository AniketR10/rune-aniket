CC=go

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

default: $(EXEC)

install: $(GEXEC) $(PKGS)

test: $(TEST)

clean:
	@-rm -rf $(TARGET)

$(TARGET):
	@mkdir $(TARGET)

$(EXEC): $(EXECS) $(EXECSRC) $(SRC) $(TARGET)
	@cd $< && $(CC) build -o $(PWD)/$(patsubst cmd/%,$(TARGET)/%,$@)

$(PKGS): $(SRC)
	@cd $@ && $(CC) install

$(GEXEC): $(EXECS)
	@cd $< && $(CC) build -o $@

$(TEST): $(TESTSRC) $(SRC)
	-mkdir -p $(dir $@)
	@cd $(dir $<) && $(CC) test -i -o ../$@
	./$@
