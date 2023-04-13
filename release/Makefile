.PHONY: config install install-linux

TARGET=/usr/local/bin/
EXECS=$(wildcard darwin_amd64/*)
EXECS_LINUX=$(wildcard linux_amd64/*)
CONFIG=${HOME}/.sixrc

config:
	@ cp sixrc $(CONFIG)
	@ echo "copied config to $(CONFIG)"

install: config
	@ sudo cp $(EXECS) $(TARGET)
	@ echo "copied executables to $(TARGET)"

install-linux: config
	@ sudo cp $(EXECS_LINUX) $(TARGET)
	@ echo "copied executables to $(TARGET)"
