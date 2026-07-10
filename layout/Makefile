# Build the project and stage the binary with runtime assets under output/.

APP=GS_PROJECT_NAME
OUTPUT=output

# Targets are not real files; declare them phony so they always run.
.PHONY: all install clean gen_output build

all: install

install: clean gen_output build
	mkdir -p $(OUTPUT)/bin
	mv $(APP) $(OUTPUT)/bin/

clean:
	rm -rf $(OUTPUT)/bin
	rm -rf $(OUTPUT)/conf
	rm -rf $(OUTPUT)/public
	rm -rf $(OUTPUT)/control.sh

gen_output:
	mkdir -p $(OUTPUT)/logs
	mkdir -p $(OUTPUT)/conf
	mkdir -p $(OUTPUT)/public
	cp -rf conf/* $(OUTPUT)/conf/
	cp -rf control.sh $(OUTPUT)/
	cp -rf public/* $(OUTPUT)/public/

build:
	go build -o $(APP) .
