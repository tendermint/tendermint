GOPATH?=$(shell go env GOPATH)
TMSRC=$(GOPATH)/src/github.com/tendermint/tendermint
.PHONY: build-tmbin-linux

build-tmbin-linux:
	@echo "Building Tendermint Linux executable..."
	$(MAKE) -C $(TMSRC) build-linux
