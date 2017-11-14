GOTOOLS = \
					github.com/mitchellh/gox \
					github.com/Masterminds/glide \
					github.com/alecthomas/gometalinter

all: protoc install test

PACKAGES=$(shell go list ./... | grep -v '/vendor/')

install-protoc:
	# Download: https://github.com/google/protobuf/releases
	go get github.com/golang/protobuf/protoc-gen-go

protoc:
	@ protoc --go_out=plugins=grpc:. types/*.proto

install:
	@ go install ./cmd/...

build:
	@ go build -i ./cmd/...

dist:
	@ bash scripts/dist.sh
	@ bash scripts/publish.sh

# test.sh requires that we run the installed cmds, must not be out of date
test: install
	find . -path ./vendor -prune -o -name *.sock -exec rm {} \;
	@ go test $(PACKAGES)
	@ bash tests/test.sh

fmt:
	@ go fmt ./...

test_integrations: get_vendor_deps install test

get_deps:
	@ go get -d $(PACKAGES)

tools:
	go get -u -v $(GOTOOLS)

get_vendor_deps:
	@ go get github.com/Masterminds/glide
	@ glide install

metalinter: tools
	@gometalinter --install
	gometalinter --vendor --deadline=600s --enable-all --disable=lll ./...

metalinter_test: tools
	@gometalinter --install
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

.PHONY: all build test fmt get_deps tools
