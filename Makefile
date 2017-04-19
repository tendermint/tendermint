GOTOOLS = \
					github.com/mitchellh/gox \
					github.com/Masterminds/glide
PACKAGES=$(shell go list ./... | grep -v '/vendor/')
BUILD_TAGS?=tendermint
TMHOME = $${TMHOME:-$$HOME/.tendermint}

all: install test

install: get_vendor_deps
	@go install ./cmd/tendermint

build:
	go build -o build/tendermint ./cmd/tendermint

build_race:
	go build -race -o build/tendermint ./cmd/tendermint

# dist builds binaries for all platforms and packages them for distribution
dist:
	@BUILD_TAGS='$(BUILD_TAGS)' sh -c "'$(CURDIR)/scripts/dist.sh'"

test:
	@echo "--> Running go test"
	@go test $(PACKAGES)

test_race:
	@echo "--> Running go test --race"
	@go test -v -race $(PACKAGES)

test_integrations:
	@bash ./test/test.sh

test100:
	@for i in {1..100}; do make test; done

draw_deps:
	# requires brew install graphviz or apt-get install graphviz
	go get github.com/RobotsAndPencils/goviz
	@goviz -i github.com/tendermint/tendermint/cmd/tendermint -d 3 | dot -Tpng -o dependency-graph.png

list_deps:
	@go list -f '{{join .Deps "\n"}}' ./... | \
		grep -v /vendor/ | sort | uniq | \
		xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}'

get_deps:
	@echo "--> Running go get"
	@go get -v -d $(PACKAGES)
	@go list -f '{{join .TestImports "\n"}}' ./... | \
		grep -v /vendor/ | sort | uniq | \
		xargs go get -v -d

get_vendor_deps: ensure_tools
	@rm -rf vendor/
	@echo "--> Running glide install"
	@glide install

update_deps: tools
	@echo "--> Updating dependencies"
	@go get -d -u ./...

revision:
	-echo `git rev-parse --verify HEAD` > $(TMHOME)/revision
	-echo `git rev-parse --verify HEAD` >> $(TMHOME)/revision_history

tools:
	go get -u -v $(GOTOOLS)

ensure_tools:
	go get $(GOTOOLS)


.PHONY: install build build_race dist test test_race test_integrations test100 draw_deps list_deps get_deps get_vendor_deps update_deps revision tools
