GOTOOLS = \
					github.com/mitchellh/gox \
					github.com/tcnksm/ghr \
					gopkg.in/alecthomas/gometalinter.v2

PACKAGES=$(shell go list ./... | grep -v '/vendor/')
BUILD_TAGS?=tendermint
TMHOME = $${TMHOME:-$$HOME/.tendermint}

BUILD_FLAGS = -ldflags "-X github.com/tendermint/tendermint/version.GitCommit=`git rev-parse --short HEAD`"

all: get_vendor_deps install test

install:
	go install $(BUILD_FLAGS) ./cmd/tendermint

build:
	go build $(BUILD_FLAGS) -o build/tendermint ./cmd/tendermint/

build_race:
	go build -race $(BUILD_FLAGS) -o build/tendermint ./cmd/tendermint

# dist builds binaries for all platforms and packages them for distribution
dist:
	@BUILD_TAGS='$(BUILD_TAGS)' sh -c "'$(CURDIR)/scripts/dist.sh'"

test:
	@echo "--> Running linter"
	@make metalinter_test
	@echo "--> Running go test"
	@go test $(PACKAGES)

test_race:
	@echo "--> Running go test --race"
	@go test -v -race $(PACKAGES)

test_integrations:
	@bash ./test/test.sh

test_release:
	@go test -tags release $(PACKAGES)

test100:
	@for i in {1..100}; do make test; done

vagrant_test:
	vagrant up
	vagrant ssh -c 'make install'
	vagrant ssh -c 'make test_race'
	vagrant ssh -c 'make test_integrations'

draw_deps:
	# requires brew install graphviz or apt-get install graphviz
	go get github.com/RobotsAndPencils/goviz
	@goviz -i github.com/tendermint/tendermint/cmd/tendermint -d 3 | dot -Tpng -o dependency-graph.png

get_vendor_deps:
	@hash glide 2>/dev/null || go get github.com/Masterminds/glide
	@rm -rf vendor/
	@echo "--> Running glide install"
	@$(GOPATH)/bin/glide install

update_vendor_deps:
	@$(GOPATH)/bin/glide update

update_tools:
	@echo "--> Updating tools"
	@go get -u $(GOTOOLS)

tools:
	@echo "--> Installing tools"
	@go get $(GOTOOLS)
	$(GOPATH)/bin/gometalinter.v2 --install

### Formatting, linting, and vetting

metalinter:
	$(GOPATH)/bin/gometalinter.v2 --vendor --deadline=600s --enable-all --disable=lll ./...

metalinter_test:
	$(GOPATH)/bin/gometalinter.v2 --vendor --deadline=600s --disable-all  \
		--enable=deadcode \
		--enable=gosimple \
	 	--enable=misspell \
		--enable=safesql \
		./...

		#--enable=gas \
		#--enable=maligned \
		#--enable=dupl \
		#--enable=errcheck \
		#--enable=goconst \
		#--enable=gocyclo \
		#--enable=goimports \
		#--enable=golint \ <== comments on anything exported
		#--enable=gotype \
	 	#--enable=ineffassign \
	   	#--enable=interfacer \
	   	#--enable=megacheck \
	   	#--enable=staticcheck \
	   	#--enable=structcheck \
	   	#--enable=unconvert \
	   	#--enable=unparam \
		#--enable=unused \
	   	#--enable=varcheck \
		#--enable=vet \
		#--enable=vetshadow \

.PHONY: install build build_race dist test test_race test_integrations test100 draw_deps get_vendor_deps update_vendor_deps update_tools tools test_release
