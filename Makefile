.PHONY: all test get_deps

all: test

test: get_deps
	go test --race github.com/tendermint/go-rpc/...
	cd ./test && bash test.sh


get_deps:
	go get -t -u github.com/tendermint/go-rpc/...
