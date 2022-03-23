ROOT_PATH := $(abspath $(lastword $(MAKEFILE_LIST)/../../../))

all: docker generator runner tests

docker:
	docker build --tag tenderdash/e2e-node -f docker/Dockerfile ../..

node:
	GOTRACEBACK=crash go build -gcflags="all=-N -l" -o build/node -tags badgerdb,boltdb,cleveldb,rocksdb ./node

e2e/app/compile:
	docker run --rm -it --entrypoint "/src/tenderdash/test/e2e/entrypoint.sh" -w "/src/tenderdash/test/e2e" -v $(ROOT_PATH):/src/tenderdash tenderdash/e2e-node

runner/simple: runner tests e2e/app/compile
	./build/runner -f networks/simple.toml

runner/ci: runner tests e2e/app/compile
	./build/runner -f networks/ci.toml

runner/dashcore: runner tests e2e/app/compile
	./build/runner -f networks/dashcore.toml

runner/rotate: runner tests e2e/app/compile
	./build/runner -f networks/rotate.toml

generator:
	go build -o build/generator ./generator

runner:
	go build -o build/runner ./runner

tests:
	go test -o build/tests ./tests

.PHONY: all docker generator runner tests node
