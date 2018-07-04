GOTOOLS = \
	github.com/golang/dep/cmd/dep \
	github.com/golang/protobuf/protoc-gen-go \
	github.com/square/certstrap
	# github.com/alecthomas/gometalinter.v2 \

GOTOOLS_CHECK = dep gometalinter.v2 protoc protoc-gen-go
INCLUDE = -I=. -I=${GOPATH}/src

########################################
###  Build

protoc:
	## If you get the following error,
	## "error while loading shared libraries: libprotobuf.so.14: cannot open shared object file: No such file or directory"
	## See https://stackoverflow.com/a/25518702
	protoc $(INCLUDE) --go_out=plugins=grpc:. common/*.proto
	@echo "--> adding nolint declarations to protobuf generated files"
	@awk '/package common/ { print "//nolint: gas"; print; next }1' common/types.pb.go > common/types.pb.go.new
	@mv common/types.pb.go.new common/types.pb.go

get_protoc:
	@# https://github.com/google/protobuf/releases
	curl -L https://github.com/google/protobuf/releases/download/v3.4.1/protobuf-cpp-3.4.1.tar.gz | tar xvz && \
		cd protobuf-3.4.1 && \
		DIST_LANG=cpp ./configure && \
		make && \
		make install && \
		cd .. && \
		rm -rf protobuf-3.4.1

gen_certs: clean_certs
	## Generating certificates for TLS testing...
	certstrap init --common-name "tendermint.com" --passphrase ""
	certstrap request-cert -ip "::" --passphrase ""
	certstrap sign "::" --CA "tendermint.com" --passphrase ""
	mv out/::.crt out/::.key db/remotedb

clean_certs:
	## Cleaning TLS testing certificates...
	rm -rf out
	rm -f db/remotedb/::.crt db/remotedb/::.key

test: gen_certs
	GOCACHE=off go test -tags gcc $(shell go list ./... | grep -v vendor)
	make clean_certs

grpc_dbserver:
	protoc -I db/remotedb/proto/ db/remotedb/proto/defs.proto --go_out=plugins=grpc:db/remotedb/proto
