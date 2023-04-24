.PHONY: config install install-linux

TARGET=/usr/local/bin/
EXECS=$(wildcard darwin_amd64/*)
EXECS_LINUX=$(wildcard linux_amd64/*)
CONFIG=${HOME}/.sixrc
BLUE_CONFIG=${HOME}/.bluectl/config
GOOGLE_CONFIG=${HOME}/.bluectl/adc.json

config:
	@ cp sixrc $(CONFIG)
	@ mkdir -p ${HOME}/.bluectl && cp bluerc $(BLUE_CONFIG)
	@ cp adc.json ${HOME}/.bluectl/adc.json
	@ echo "copied config to $(CONFIG) and $(BLUE_CONFIG)"
	@ echo "You might want to modify this files for your user!"

install:
	@ sudo cp $(EXECS) $(TARGET)
	@ echo "copied executables to $(TARGET)"

install-linux:
	@ sudo cp $(EXECS_LINUX) $(TARGET)
	@ echo "copied executables to $(TARGET)"
