all: docker generator runner tests

docker:
	docker build --tag tendermint/e2e-node -f docker/Dockerfile ../..

node:
	go build -o build/node -tags badgerdb,boltdb,cleveldb,rocksdb ./node

generator:
	go build -o build/generator ./generator

runner:
	go build -o build/runner ./runner

tests:
	go test -o build/tests ./tests

.PHONY: all docker generator runner tests node
