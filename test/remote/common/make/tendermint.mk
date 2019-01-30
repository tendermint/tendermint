GOPATH?=$(shell go env GOPATH)
TMSRC=$(GOPATH)/src/github.com/tendermint/tendermint
TMBIN=$(TMSRC)/build/tendermint

$(TMBIN):
	@echo "Building Tendermint Linux executable..."
	$(MAKE) -C $(TMSRC) build-linux

