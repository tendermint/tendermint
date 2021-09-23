all: node docker generator runner tests

docker:
	docker build --tag tendermint/e2e-node -f docker/Dockerfile ../..

node:
	go build -o build/node ./node

generator:
	go build -o build/generator ./generator

runner:
	go build -o build/runner ./runner

tests:
	go test -o build/tests ./tests

.PHONY: all app docker generator runner tests
