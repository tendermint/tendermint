all: docker runner

docker:
	docker build --tag tendermint/e2e-node -f docker/Dockerfile ../..

# We need to build support for database backends into the app in
# order to build a binary with a Tendermint node in it (for built-in
# ABCI testing).
app:
	go build -o build/app -tags badgerdb,boltdb,cleveldb,rocksdb ./app

runner:
	go build -o build/runner ./runner

.PHONY: app docker runner
