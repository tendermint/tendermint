.PHONY: all test get_deps

all: test

test:
	go test --race github.com/tendermint/go-rpc/...
	cd ./test && bash test.sh


get_deps:
	go get -t -d github.com/tendermint/go-rpc/...
