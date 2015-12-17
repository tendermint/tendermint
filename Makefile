.PHONY: all test get_deps

all: test get_deps install

install:
	go install github.com/tendermint/tmsp/cmd/...

test:
	go test github.com/tendermint/tmsp/...

get_deps:
	go get -d github.com/tendermint/tmsp/...
