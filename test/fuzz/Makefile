#!/usr/bin/make -f

.PHONY: fuzz-mempool
fuzz-mempool:
	cd mempool && \
		rm -f *-fuzz.zip && \
		go-fuzz-build && \
		go-fuzz

.PHONY: fuzz-p2p-addrbook
fuzz-p2p-addrbook:
	cd p2p/addrbook && \
		rm -f *-fuzz.zip && \
		go run ./init-corpus/main.go && \
		go-fuzz-build && \
		go-fuzz

.PHONY: fuzz-p2p-pex
fuzz-p2p-pex:
	cd p2p/pex && \
		rm -f *-fuzz.zip && \
		go run ./init-corpus/main.go && \
		go-fuzz-build && \
		go-fuzz

.PHONY: fuzz-p2p-sc
fuzz-p2p-sc:
	cd p2p/secret_connection && \
		rm -f *-fuzz.zip && \
		go run ./init-corpus/main.go && \
		go-fuzz-build && \
		go-fuzz

.PHONY: fuzz-rpc-server
fuzz-rpc-server:
	cd rpc/jsonrpc/server && \
		rm -f *-fuzz.zip && \
		go-fuzz-build && \
		go-fuzz
