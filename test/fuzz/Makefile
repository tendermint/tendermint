#!/usr/bin/make -f

.PHONY: fuzz-blockchain
fuzz-blockchain:
	cd blockchain && \
		go run ./init-corpus/main.go && \
		go-fuzz-build	&& \
		go-fuzz

.PHONY: fuzz-mempool
fuzz-mempool:
	cd mempool && \
		go-fuzz-build && \
		go-fuzz

.PHONY: fuzz-p2p-addrbook
fuzz-p2p-addrbook:
	cd p2p/addrbook && \
		go run ./init-corpus/main.go && \
		go-fuzz-build && \
		go-fuzz

.PHONY: fuzz-p2p-pex
fuzz-p2p-pex:
	cd p2p/pex && \
		go run ./init-corpus/main.go && \
		go-fuzz-build && \
		go-fuzz

.PHONY: fuzz-p2p-sc
fuzz-p2p-sc:
	cd p2p/secret_connection && \
		go run ./init-corpus/main.go && \
		go-fuzz-build && \
		go-fuzz

.PHONY: fuzz-rpc-server
fuzz-rpc-server:
	cd rpc/jsonrpc/server && \
		go-fuzz-build && \
		go-fuzz
