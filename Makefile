.PHONY: all test get_deps

all: protoc install test

NOVENDOR = go list github.com/tendermint/abci/... | grep -v /vendor/

install-protoc:
	# Download: https://github.com/google/protobuf/releases
	go get github.com/golang/protobuf/protoc-gen-go

protoc:
	protoc --go_out=plugins=grpc:. types/*.proto

install:
	go install github.com/tendermint/abci/cmd/...

test:
	find . -name test.sock -exec rm {} \;
	go test -p 1 `${NOVENDOR}`
	bash tests/test.sh

test_integrations: get_vendor_deps install test

get_deps:
	go get -d `${NOVENDOR}`

get_vendor_deps:
	go get github.com/Masterminds/glide
	glide install
