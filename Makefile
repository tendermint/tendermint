.PHONY: all test get_vendor_deps ensure_tools

GOTOOLS = \
	github.com/Masterminds/glide
REPO:=github.com/tendermint/tmlibs

all: test

test:
	go test `glide novendor`

get_vendor_deps: ensure_tools
	@rm -rf vendor/
	@echo "--> Running glide install"
	@glide install

ensure_tools:
	go get $(GOTOOLS)
