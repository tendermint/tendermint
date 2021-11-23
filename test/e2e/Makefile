all: docker generator runner

docker:
	docker build --tag tendermint/e2e-node -f docker/Dockerfile ../..

# We need to build support for database backends into the app in
# order to build a binary with a Tendermint node in it (for built-in
# ABCI testing).
node:
	go build -o build/node -tags badgerdb,boltdb,cleveldb,rocksdb ./node
	
# To be used primarily by the e2e docker instance. If you want to produce this binary
# elsewhere, then run go build in the maverick directory. 
maverick:
	go build -o build/maverick -tags badgerdb,boltdb,cleveldb,rocksdb ../maverick

generator:
	go build -o build/generator ./generator

runner:
	go build -o build/runner ./runner

.PHONY: all node docker generator maverick runner 
