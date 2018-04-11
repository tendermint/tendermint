# Makefile for the "localnode" docker image.

localnode:
	docker build --tag tendermint/localnode localnode

squash:
	docker build --squash --tag tendermint/localnode localnode

fresh:
	docker build --squash --no-cache --pull --tag tendermint/localnode localnode

.PHONY: localnode

