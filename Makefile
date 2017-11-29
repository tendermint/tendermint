.PHONY: all test get_vendor_deps ensure_tools

GOTOOLS = \
	github.com/Masterminds/glide \
	github.com/alecthomas/gometalinter

PACKAGES=$(shell go list ./... | grep -v '/vendor/')
REPO:=github.com/tendermint/tmlibs

all: test

test:
	@echo "--> Running linter"
	@make metalinter_test
	@echo "--> Running go test"
	@go test $(PACKAGES)

get_vendor_deps: ensure_tools
	@rm -rf vendor/
	@echo "--> Running glide install"
	@glide install

ensure_tools:
	go get $(GOTOOLS)
	@gometalinter --install

metalinter: 
	gometalinter --vendor --deadline=600s --enable-all --disable=lll ./...

metalinter_test: 
	gometalinter --vendor --deadline=600s --disable-all  \
		--enable=deadcode \
		--enable=goconst \
		--enable=gosimple \
	 	--enable=ineffassign \
	   	--enable=interfacer \
		--enable=megacheck \
	 	--enable=misspell \
	   	--enable=staticcheck \
		--enable=safesql \
	   	--enable=structcheck \
	   	--enable=unconvert \
		--enable=unused \
	   	--enable=varcheck \
		--enable=vetshadow \
		--enable=vet \
		./...

		#--enable=gas \
		#--enable=aligncheck \
		#--enable=dupl \
		#--enable=errcheck \
		#--enable=gocyclo \
		#--enable=goimports \
		#--enable=golint \ <== comments on anything exported
		#--enable=gotype \
	   	#--enable=unparam \
