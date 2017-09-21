GOTOOLS = \
					github.com/mitchellh/gox \
					github.com/Masterminds/glide \
					github.com/alecthomas/gometalinter

all: protoc install test

NOVENDOR = go list github.com/tendermint/abci/... | grep -v /vendor/

install-protoc:
	# Download: https://github.com/google/protobuf/releases
	go get github.com/golang/protobuf/protoc-gen-go

protoc:
	@ protoc --go_out=plugins=grpc:. types/*.proto

install:
	@ go install github.com/tendermint/abci/cmd/...

build:
	@ go build -i github.com/tendermint/abci/cmd/...

dist:
	@ bash scripts/dist.sh
	@ bash scripts/publish.sh

# test.sh requires that we run the installed cmds, must not be out of date
test: install
	find . -path ./vendor -prune -o -name *.sock -exec rm {} \;
	@ go test -p 1 `${NOVENDOR}`
	@ bash tests/test.sh

fmt:
	@ go fmt ./...

lint:
	@ go get -u github.com/golang/lint/golint
	@ for file in $$(find "." -name '*.go' | grep -v '/vendor/' | grep -v '\.pb\.go'); do \
		golint -set_exit_status $${file}; \
		done;

test_integrations: get_vendor_deps install test

get_deps:
	@ go get -d `${NOVENDOR}`

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
		--enable=deadcode \
		--enable=gas \
		--enable=goimports \
		--enable=gosimple \
		--enable=gotype \
	 	--enable=ineffassign \
	 	--enable=misspell \
		--enable=safesql \
	   	--enable=structcheck \
	   	--enable=varcheck \
		./...

		#--enable=aligncheck \
		#--enable=dupl \
		#--enable=errcheck \
		#--enable=goconst \
		#--enable=gocyclo \
		#--enable=golint \ <== comments on anything exported
	   	#--enable=interfacer \
	   	#--enable=megacheck \
	   	#--enable=staticcheck \
	   	#--enable=unconvert \
	   	#--enable=unparam \
		#--enable=unused \
		#--enable=vet \
		#--enable=vetshadow \

.PHONY: all build test fmt lint get_deps tools
