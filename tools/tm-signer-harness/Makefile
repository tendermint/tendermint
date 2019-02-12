.PHONY: build install docker-image

TENDERMINT_VERSION?=latest
BUILD_TAGS?='tendermint'
BUILD_FLAGS = -ldflags "-X github.com/tendermint/tendermint/version.GitCommit=`git rev-parse --short=8 HEAD`"

.DEFAULT_GOAL := build

build:
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -tags $(BUILD_TAGS) -o ../../build/tm-signer-harness main.go

install:
	CGO_ENABLED=0 go install $(BUILD_FLAGS) -tags $(BUILD_TAGS) .

docker-image:
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -tags $(BUILD_TAGS) -o tm-signer-harness main.go
	docker build \
		--build-arg TENDERMINT_VERSION=$(TENDERMINT_VERSION) \
		-t tendermint/tm-signer-harness:$(TENDERMINT_VERSION) .
	rm -rf tm-signer-harness
