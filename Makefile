.PHONY: all test get_deps

all: protoc test install

protoc:
	protoc --go_out=. types/*.proto

install: get_deps
	go install github.com/tendermint/tmsp/cmd/...

test:
	go test github.com/tendermint/tmsp/...

get_deps:
	go get -d github.com/tendermint/tmsp/...
