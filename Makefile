GOTOOLS = \
					github.com/mitchellh/gox \
					github.com/Masterminds/glide \
					github.com/alecthomas/gometalinter \
					github.com/ckaznocha/protoc-gen-lint

all: install test

PACKAGES=$(shell go list ./... | grep -v '/vendor/')

install_protoc:
	# https://github.com/google/protobuf/releases
	curl -L https://github.com/google/protobuf/releases/download/v3.4.1/protobuf-cpp-3.4.1.tar.gz | tar xvz && \
		cd protobuf-3.4.1 && \
		DIST_LANG=cpp ./configure && \
		make && \
		make install && \
		cd .. && \
		rm -rf protobuf-3.4.1
	go get github.com/golang/protobuf/protoc-gen-go

protoc:
	## On "error while loading shared libraries: libprotobuf.so.14: cannot open shared object file: No such file or directory"
	##   ldconfig (may require sudo)
	## https://stackoverflow.com/a/25518702
	protoc --go_out=plugins=grpc:. types/*.proto

install:
	@ go install ./cmd/...

build:
	@ go build -i ./cmd/...

dist:
	@ bash scripts/dist.sh
	@ bash scripts/publish.sh

# test.sh requires that we run the installed cmds, must not be out of date
test: 
	@ find . -path ./vendor -prune -o -name "*.sock" -exec rm {} \;
	@ echo "==> Running go test"
	@ go test $(PACKAGES)

test_race: 
	@ find . -path ./vendor -prune -o -name "*.sock" -exec rm {} \;
	@ echo "==> Running go test --race"
	@go test -v -race $(PACKAGES)

test_integrations:
	@ bash test.sh

fmt:
	@ go fmt ./...

get_deps:
	@ go get -d $(PACKAGES)

tools:
	go get -u -v $(GOTOOLS)

get_vendor_deps:
	@ go get github.com/Masterminds/glide
	@ glide install

metalinter: tools
	@gometalinter --install
	protoc --lint_out=. types/*.proto
	gometalinter --vendor --deadline=600s --enable-all --disable=lll ./...

metalinter_test: tools
	@gometalinter --install
	# protoc --lint_out=. types/*.proto
	gometalinter --vendor --deadline=600s --disable-all  \
		--enable=maligned \
		--enable=deadcode \
		--enable=gas \
		--enable=goconst \
		--enable=goimports \
		--enable=gosimple \
	 	--enable=ineffassign \
		--enable=megacheck \
	 	--enable=misspell \
	   	--enable=staticcheck \
		--enable=safesql \
	   	--enable=structcheck \
	   	--enable=unconvert \
		--enable=unused \
	   	--enable=varcheck \
		--enable=vetshadow \
		./...

		#--enable=dupl \
		#--enable=errcheck \
		#--enable=gocyclo \
		#--enable=golint \ <== comments on anything exported
		#--enable=gotype \
	   	#--enable=interfacer \
	   	#--enable=unparam \
		#--enable=vet \

build-docker:
	docker build -t "tendermint/abci-dev" -f Dockerfile.develop .

run-docker:
	docker run -it --rm -v "$PWD:/go/src/github.com/tendermint/abci" -w "/go/src/github.com/tendermint/abci" "tendermint/abci-dev" /bin/bash

.PHONY: all build test fmt get_deps tools protoc install_protoc build-docker run-docker
