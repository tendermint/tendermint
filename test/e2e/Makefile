all: docker generator runner
VERSION=$(shell git tag --points-at HEAD)
ifeq (,$(VERSION))
	VERSION=$(shell git rev-parse HEAD)
endif

docker:
	docker build --tag tendermint/e2e-node:$(VERSION) -f docker/Dockerfile ../..

# We need to build support for database backends into the app in
# order to build a binary with a Tendermint node in it (for built-in
# ABCI testing).
node:
	go build -o build/node -tags badgerdb,boltdb,cleveldb,rocksdb ./node

generator:
	go build -o build/generator ./generator

runner:
	go build -o build/runner ./runner

.PHONY: all node docker generator runner 
