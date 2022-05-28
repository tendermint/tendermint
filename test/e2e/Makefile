all: docker generator runner tests

docker:
ifneq (,$(VERSION))
	git worktree add e2e-node $(VERSION)
	docker build --tag tendermint/e2e-node:$(VERSION) -f docker/Dockerfile ../..
	git worktree remove e2e-node
else 
	docker build --tag tendermint/e2e-node:$(shell git symbolic-ref -q --short HEAD) -f docker/Dockerfile ../..
endif

node:
	go build -o build/node -tags badgerdb,boltdb,cleveldb,rocksdb ./node

generator:
	go build -o build/generator ./generator

runner:
	go build -o build/runner ./runner

tests:
	go test -o build/tests ./tests

.PHONY: all docker generator runner tests node
