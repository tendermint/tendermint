.PHONY: all build test fmt lint get_deps

all: protoc install test

NOVENDOR = go list github.com/tendermint/abci/... | grep -v /vendor/

install-protoc:
	# Download: https://github.com/google/protobuf/releases
	go get github.com/golang/protobuf/protoc-gen-go

protoc:
	@ protoc --go_out=plugins=grpc:. types/*.proto

install:
	@ go install github.com/tendermint/abci/cmd/...

build:
	@ go build -i github.com/tendermint/abci/cmd/...

# test.sh requires that we run the installed cmds, must not be out of date
test: install
	find . -name test.sock -exec rm {} \;
	@ go test -p 1 `${NOVENDOR}`
	@ bash tests/test.sh

fmt:
	@ go fmt ./...

lint:
	@ go get -u github.com/golang/lint/golint
	@ for file in $$(find "." -name '*.go' | grep -v '/vendor/' | grep -v '\.pb\.go'); do \
		golint -set_exit_status $${file}; \
	done;

test_integrations: get_vendor_deps install test

get_deps:
	@ go get -d `${NOVENDOR}`

get_vendor_deps:
	@ go get github.com/Masterminds/glide
	@ glide install
