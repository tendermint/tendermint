.PHONY: all test get_deps

all: test

test: 
	bash ./test/test.sh

get_deps:
	go get -t -u github.com/tendermint/go-rpc/...
