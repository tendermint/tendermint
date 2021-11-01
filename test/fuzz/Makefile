#!/usr/bin/make -f

.PHONY: fuzz-mempool
fuzz-mempool:
	cd mempool && \
		rm -f *-fuzz.zip && \
		go-fuzz-build && \
		go-fuzz

.PHONY: fuzz-p2p-sc
fuzz-p2p-sc:
	cd p2p/secretconnection && \
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

clean:
	find . -name corpus -type d -exec rm -rf {} +;
	find . -name crashers -type d -exec rm -rf {} +;
	find . -name suppressions -type d -exec rm -rf {} +;
	find . -name *\.zip -type f -delete
