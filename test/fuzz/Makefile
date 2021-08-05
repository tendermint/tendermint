#!/usr/bin/make -f

.PHONY: fuzz-mempool-v1
fuzz-mempool-v1:
	cd mempool/v1 && \
		rm -f *-fuzz.zip && \
		go-fuzz-build && \
		go-fuzz

.PHONY: fuzz-mempool-v0
fuzz-mempool-v0:
	cd mempool/v0 && \
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

clean:
	find . -name corpus -type d -exec rm -rf {} +;
	find . -name crashers -type d -exec rm -rf {} +;
	find . -name suppressions -type d -exec rm -rf {} +;
	find . -name *\.zip -type f -delete
