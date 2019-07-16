GOTOOLS = github.com/golangci/golangci-lint/cmd/golangci-lint
PACKAGES=$(shell go list ./...)

export GO111MODULE = on

all: lint test

### go tests
test:
	@echo "--> Running go test"
	@go test -p 1 $(PACKAGES)

lint:
	@echo "--> Running linter"
	@golangci-lint run

tools:
	go get -v $(GOTOOLS)

# generates certificates for TLS testing in remotedb
gen_certs: clean_certs
	certstrap init --common-name "tendermint.com" --passphrase ""
	certstrap request-cert --common-name "remotedb" -ip "127.0.0.1" --passphrase ""
	certstrap sign "remotedb" --CA "tendermint.com" --passphrase ""
	mv out/remotedb.crt db/remotedb/test.crt
	mv out/remotedb.key db/remotedb/test.key
	rm -rf out

clean_certs:
	rm -f db/remotedb/test.crt
	rm -f db/remotedb/test.key
	