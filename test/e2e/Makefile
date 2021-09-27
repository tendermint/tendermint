ROOT_PATH := $(abspath $(lastword $(MAKEFILE_LIST)/../../../))

all: docker generator runner

docker:
	docker build --tag tenderdash/e2e-node -f docker/Dockerfile ../..

e2e/app/compile:
	docker run --rm -it --entrypoint "/src/tenderdash/test/e2e/entrypoint.sh" -w "/src/tenderdash/test/e2e" -v $(ROOT_PATH):/src/tenderdash tenderdash/e2e-node

runner/simple: runner e2e/app/compile
	./build/runner -f networks/simple.toml

runner/ci: runner e2e/app/compile
	./build/runner -f networks/ci.toml

runner/dashcore: runner e2e/app/compile
	./build/runner -f networks/dashcore.toml

runner/rotate: runner e2e/app/compile
	./build/runner -f networks/rotate.toml

# We need to build support for database backends into the app in
# order to build a binary with a Tenderdash node in it (for built-in
# ABCI testing).
app:
	go build -o build/app -tags badgerdb,boltdb,cleveldb,rocksdb ./app

# To be used primarily by the e2e docker instance. If you want to produce this binary
# elsewhere, then run go build in the maverick directory.
maverick:
	go build -o build/maverick -tags badgerdb,boltdb,cleveldb,rocksdb ../maverick

generator:
	go build -o build/generator ./generator

runner:
	go build -o build/runner ./runner

.PHONY: all app docker generator maverick runner
