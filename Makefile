.PHONY: all test get_deps

all: protoc test install

NOVENDOR = go list github.com/tendermint/abci/... | grep -v /vendor/

protoc:
	protoc --go_out=plugins=grpc:. types/*.proto

install:
	go install github.com/tendermint/abci/cmd/...

test:
	go test `${NOVENDOR}`
	bash tests/test.sh

test_integrations: get_vendor_deps install test

get_deps:
	go get -d `${NOVENDOR}`

get_vendor_deps:
	go get github.com/Masterminds/glide
	glide install
