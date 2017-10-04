.PHONY: all test get_vendor_deps ensure_tools

GOTOOLS = \
	github.com/Masterminds/glide \
	github.com/alecthomas/gometalinter

REPO:=github.com/tendermint/tmlibs

all: test

NOVENDOR = go list github.com/tendermint/tmlibs/... | grep -v /vendor/

test:
	go test `glide novendor`

get_vendor_deps: ensure_tools
	@rm -rf vendor/
	@echo "--> Running glide install"
	@glide install

ensure_tools:
	go get $(GOTOOLS)

metalinter: ensure_tools
	@gometalinter --install
	gometalinter --vendor --deadline=600s --enable-all --disable=lll ./...

metalinter_test: ensure_tools
	@gometalinter --install
	gometalinter --vendor --deadline=600s --disable-all  \
		--enable=deadcode \
		--enable=gas \
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

		#--enable=aligncheck \
		#--enable=dupl \
		#--enable=errcheck \
		#--enable=gocyclo \
		#--enable=goimports \
		#--enable=golint \ <== comments on anything exported
		#--enable=gotype \
	   	#--enable=unparam \
