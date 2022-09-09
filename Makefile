#!/usr/bin/make -f

OUTPUT ?= build/tenderdash
BUILDDIR ?= $(CURDIR)/build
REPO_NAME ?= github.com/dashevo/tenderdash
BUILD_TAGS ?= tenderdash
# If building a release, please checkout the version tag to get the correct version setting
ifneq ($(shell git symbolic-ref -q --short HEAD),)
VERSION := unreleased-$(shell git symbolic-ref -q --short HEAD)-$(shell git rev-parse HEAD)
else
VERSION := $(shell git describe)
endif
LD_FLAGS = -X ${REPO_NAME}/version.TMCoreSemVer=$(VERSION)
BUILD_FLAGS = -mod=readonly -ldflags "$(LD_FLAGS)"
HTTPS_GIT := https://${REPO_NAME}.git
BUILD_IMAGE := ghcr.io/tendermint/docker-build-proto
BASE_BRANCH ?= v0.8-dev
DOCKER_PROTO := docker run -v $(shell pwd):/workspace --workdir /workspace $(BUILD_IMAGE)
CGO_ENABLED ?= 1
GOGOPROTO_PATH = $(shell go list -m -f '{{.Dir}}' github.com/gogo/protobuf)

# handle ARM builds
ifeq (arm,$(GOARCH))
	export CC = arm-linux-gnueabi-gcc-10
	export CXX = arm-linux-gnueabi-g++-10
endif

# handle nostrip
ifeq (,$(findstring nostrip,$(TENDERMINT_BUILD_OPTIONS)))
  BUILD_FLAGS += -trimpath
  LD_FLAGS += -s -w
else
  BUILD_FLAGS += -gcflags=all="-N -l"
  export GOTRACEBACK = crash
endif

# handle race
ifeq (race,$(findstring race,$(TENDERMINT_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_FLAGS += -race
endif

# handle cleveldb
ifeq (cleveldb,$(findstring cleveldb,$(TENDERMINT_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_TAGS += cleveldb
endif

# handle badgerdb
ifeq (badgerdb,$(findstring badgerdb,$(TENDERMINT_BUILD_OPTIONS)))
  BUILD_TAGS += badgerdb
endif

# handle rocksdb
ifeq (rocksdb,$(findstring rocksdb,$(TENDERMINT_BUILD_OPTIONS)))
  CGO_ENABLED=1
  BUILD_TAGS += rocksdb
endif

# handle boltdb
ifeq (boltdb,$(findstring boltdb,$(TENDERMINT_BUILD_OPTIONS)))
  BUILD_TAGS += boltdb
endif

# handle deadlock
ifeq (deadlock,$(findstring deadlock,$(TENDERMINT_BUILD_OPTIONS)))
  BUILD_TAGS += deadlock
endif

# allow users to pass additional flags via the conventional LDFLAGS variable
LD_FLAGS += $(LDFLAGS)

all: build install
build: build-bls build-binary
.PHONY: build
install: install-bls

.PHONY: all

include test/Makefile

###############################################################################
###                      Build/Install BLS library                          ###
###############################################################################

build-bls:
	@third_party/bls-signatures/build.sh
.PHONY: build-bls

install-bls: build-bls
	@sudo third_party/bls-signatures/install.sh
.PHONY: install-bls

###############################################################################
###                                Build Tendermint                        ###
###############################################################################

build-binary:
	CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_FLAGS) -tags '$(BUILD_TAGS)' -o $(OUTPUT) ./cmd/tenderdash/
.PHONY: build-binary

install:
	CGO_ENABLED=$(CGO_ENABLED) go install $(BUILD_FLAGS) -tags $(BUILD_TAGS) ./cmd/tenderdash
.PHONY: install

$(BUILDDIR)/:
	mkdir -p $@

###############################################################################
###                                Protobuf                                 ###
###############################################################################

proto: proto-format proto-lint proto-doc proto-gen
.PHONY: proto

check-proto-deps:
ifeq (,$(shell which protoc-gen-gogofaster))
	$(error "gogofaster plugin for protoc is required. Run 'go install github.com/gogo/protobuf/protoc-gen-gogofaster@latest' to install")
endif
.PHONY: check-proto-deps

check-proto-format-deps:
ifeq (,$(shell which clang-format))
	$(error "clang-format is required for Protobuf formatting. See instructions for your platform on how to install it.")
endif
.PHONY: check-proto-format-deps

proto-gen: check-proto-deps
	@echo "Generating Protobuf files"
	@go run github.com/bufbuild/buf/cmd/buf generate
	@mv ./proto/tendermint/abci/types.pb.go ./abci/types/
.PHONY: proto-gen

# These targets are provided for convenience and are intended for local
# execution only.
proto-lint: check-proto-deps
	@echo "Linting Protobuf files"
	@go run github.com/bufbuild/buf/cmd/buf lint
.PHONY: proto-lint

proto-format: check-proto-format-deps
	@echo "Formatting Protobuf files"
	@find . -name '*.proto' -path "./proto/*" -exec clang-format -i {} \;
.PHONY: proto-format

proto-check-breaking: check-proto-deps
	@echo "Checking for breaking changes in Protobuf files against local branch"
	@echo "Note: This is only useful if your changes have not yet been committed."
	@echo "      Otherwise read up on buf's \"breaking\" command usage:"
	@echo "      https://docs.buf.build/breaking/usage"
	@go run github.com/bufbuild/buf/cmd/buf breaking --against ".git"
.PHONY: proto-check-breaking

proto-doc:
	@echo Generating Protobuf API specification: spec/abci++/api.md 
	@protoc \
		-I $(realpath .)/proto \
		-I "$(GOGOPROTO_PATH)" \
		--doc_opt=markdown,api.md \
		--doc_out=spec/abci++ \
		tendermint/abci/types.proto

###############################################################################
###                              Build ABCI                                 ###
###############################################################################

build_abci:
	@go build -mod=readonly ./abci/cmd/...
.PHONY: build_abci

install_abci:
	@go install -mod=readonly ./abci/cmd/...
.PHONY: install_abci


##################################################################################
###                              Build ABCI Dump                               ###
##################################################################################

build_abcidump:
	@go build -o build/abcidump ./cmd/abcidump
.PHONY: build_abcidump

install_abcidump:
	@go install ./cmd/abcidump
.PHONY: install_abcidump

###############################################################################
###				Privval Server                              ###
###############################################################################

build_privval_server:
	@go build -mod=readonly -o $(BUILDDIR)/ -i ./cmd/priv_val_server/...
.PHONY: build_privval_server

generate_test_cert:
	# generate self signing ceritificate authority
	@certstrap init --common-name "root CA" --expires "20 years"
	# generate server cerificate
	@certstrap request-cert -cn server -ip 127.0.0.1
	# self-sign server cerificate with rootCA
	@certstrap sign server --CA "root CA"
	# generate client cerificate
	@certstrap request-cert -cn client -ip 127.0.0.1
	# self-sign client cerificate with rootCA
	@certstrap sign client --CA "root CA"
.PHONY: generate_test_cert

###############################################################################
###                              Distribution                               ###
###############################################################################

# dist builds binaries for all platforms and packages them for distribution
# TODO add abci to these scripts
dist:
	@BUILD_TAGS=$(BUILD_TAGS) sh -c "'$(CURDIR)/scripts/dist.sh'"
.PHONY: dist

go-mod-cache: go.sum
	@echo "--> Download go modules to local cache"
	@go mod download
.PHONY: go-mod-cache

go.sum: go.mod
	@echo "--> Ensure dependencies have not been modified"
	@go mod verify
	@go mod tidy

draw_deps:
	@# requires brew install graphviz or apt-get install graphviz
	go install github.com/RobotsAndPencils/goviz@latest
	@goviz -i ${REPO_NAME}/cmd/tendermint -d 3 | dot -Tpng -o dependency-graph.png
.PHONY: draw_deps

get_deps_bin_size:
	@# Copy of build recipe with additional flags to perform binary size analysis
	$(eval $(shell go build -work -a $(BUILD_FLAGS) -tags $(BUILD_TAGS) -o $(BUILDDIR)/ ./cmd/tendermint/ 2>&1))
	@find $(WORK) -type f -name "*.a" | xargs -I{} du -hxs "{}" | sort -rh | sed -e s:${WORK}/::g > deps_bin_size.log
	@echo "Results can be found here: $(CURDIR)/deps_bin_size.log"
.PHONY: get_deps_bin_size

###############################################################################
###                                  Libs                                   ###
###############################################################################

# generates certificates for TLS testing in remotedb and RPC server
gen_certs: clean_certs
	certstrap init --common-name "tendermint.com" --passphrase ""
	certstrap request-cert --common-name "server" -ip "127.0.0.1" --passphrase ""
	certstrap sign "server" --CA "tendermint.com" --passphrase ""
	mv out/server.crt rpc/jsonrpc/server/test.crt
	mv out/server.key rpc/jsonrpc/server/test.key
	rm -rf out
.PHONY: gen_certs

# deletes generated certificates
clean_certs:
	rm -f rpc/jsonrpc/server/test.crt
	rm -f rpc/jsonrpc/server/test.key
.PHONY: clean_certs

###############################################################################
###                  Formatting, linting, and vetting                       ###
###############################################################################

format:
	find . -name '*.go' -type f -not -path "*.git*" -not -name '*.pb.go' -not -name '*pb_test.go' | xargs gofmt -w -s
	find . -name '*.go' -type f -not -path "*.git*" -not -name '*.pb.go' -not -name '*pb_test.go' | xargs golines -w
	find . -name '*.go' -type f -not -path "*.git*"  -not -name '*.pb.go' -not -name '*pb_test.go' | xargs goimports -w -local ${REPO_NAME}
.PHONY: format

lint:
	@echo "--> Running linter"
	go run github.com/golangci/golangci-lint/cmd/golangci-lint run
.PHONY: lint

DESTINATION = ./index.html.md

###############################################################################
###                           Documentation                                 ###
###############################################################################
# todo remove once tendermint.com DNS is solved
build-docs:
	@cd docs && \
	while read -r branch path_prefix; do \
		( git checkout $${branch} && npm ci --quiet && \
			VUEPRESS_BASE="/$${path_prefix}/" npm run build --quiet ) ; \
		mkdir -p ~/output/$${path_prefix} ; \
		cp -r .vuepress/dist/* ~/output/$${path_prefix}/ ; \
		cp ~/output/$${path_prefix}/index.html ~/output ; \
	done < versions ; \
	mkdir -p ~/output/master ; \
	cp -r .vuepress/dist/* ~/output/master/
.PHONY: build-docs

###############################################################################
###                            Docker image                                 ###
###############################################################################

build-docker: build-linux
	cp $(BUILDDIR)/tenderdash DOCKER/tenderdash
	docker build --label=tendermint --tag="dashpay/tenderdash" -f DOCKER/Dockerfile .
	rm -rf DOCKER/tenderdash
.PHONY: build-docker


###############################################################################
###                                Mocks                                    ###
###############################################################################

mockery:
	go generate -run="./scripts/mockery_generate.sh" ./...
.PHONY: mockery

###############################################################################
###                               Metrics                                   ###
###############################################################################

metrics: testdata-metrics
	go generate -run="scripts/metricsgen" ./...
.PHONY: metrics

	# By convention, the go tool ignores subdirectories of directories named
	# 'testdata'. This command invokes the generate command on the folder directly
	# to avoid this.
testdata-metrics:
	ls ./scripts/metricsgen/testdata | xargs -I{} go generate -run="scripts/metricsgen" ./scripts/metricsgen/testdata/{}
.PHONY: testdata-metrics

###############################################################################
###                       Local testnet using docker                        ###
###############################################################################

# Build linux binary on other platforms
build-linux:
	GOOS=linux $(MAKE) build
.PHONY: build-linux

build-docker-localnode:
	@cd networks/local && make
.PHONY: build-docker-localnode

# Runs `make build TENDERMINT_BUILD_OPTIONS=cleveldb` from within an Amazon
# Linux (v2)-based Docker build container in order to build an Amazon
# Linux-compatible binary. Produces a compatible binary at ./build/tendermint
build_c-amazonlinux:
	$(MAKE) -C ./DOCKER build_amazonlinux_buildimage
	docker run --rm -it -v `pwd`:/tendermint dashpay/tenderdash:build_c-amazonlinux
.PHONY: build_c-amazonlinux

# Run a 4-node testnet locally
localnet-start: localnet-stop build-docker-localnode
	@if ! [ -f build/node0/config/genesis.json ]; then docker run --rm -v $(CURDIR)/build:/tendermint:Z dashpay/tenderdash testnet --config /etc/tendermint/config-template.toml --o . --starting-ip-address 192.167.10.2; fi
	docker-compose up
.PHONY: localnet-start

# Stop testnet
localnet-stop:
	docker-compose down
.PHONY: localnet-stop

# Build hooks for dredd, to skip or add information on some steps
build-contract-tests-hooks:
ifeq ($(OS),Windows_NT)
	go build -mod=readonly $(BUILD_FLAGS) -o build/contract_tests.exe ./cmd/contract_tests
else
	go build -mod=readonly $(BUILD_FLAGS) -o build/contract_tests ./cmd/contract_tests
endif
.PHONY: build-contract-tests-hooks

# Run a nodejs tool to test endpoints against a localnet
# The command takes care of starting and stopping the network
# prerequisites: build-contract-tests-hooks build-linux
# the two build commands were not added to let this command run from generic containers or machines.
# The binaries should be built beforehand
contract-tests:
	dredd
.PHONY: contract-tests

clean:
	rm -rf $(CURDIR)/artifacts/ $(BUILDDIR)/

build-reproducible:
	docker rm latest-build || true
	docker run --volume=$(CURDIR):/sources:ro \
		--env TARGET_PLATFORMS='linux/amd64 linux/arm64 darwin/amd64 windows/amd64' \
		--env APP=tendermint \
		--env COMMIT=$(shell git rev-parse --short=8 HEAD) \
		--env VERSION=$(shell git describe --tags) \
		--name latest-build cosmossdk/rbuilder:latest
	docker cp -a latest-build:/home/builder/artifacts/ $(CURDIR)/
.PHONY: build-reproducible

# Implements test splitting and running. This is pulled directly from
# the github action workflows for better local reproducibility.

GO_TEST_FILES != find $(CURDIR) -name "*_test.go"

# default to four splits by default
NUM_SPLIT ?= 4

$(BUILDDIR):
	mkdir -p $@

# The format statement filters out all packages that don't have tests.
# Note we need to check for both in-package tests (.TestGoFiles) and
# out-of-package tests (.XTestGoFiles).
$(BUILDDIR)/packages.txt:$(GO_TEST_FILES) $(BUILDDIR)
	go list -f "{{ if (or .TestGoFiles .XTestGoFiles) }}{{ .ImportPath }}{{ end }}" ./... | sort > $@

split-test-packages:$(BUILDDIR)/packages.txt
	split -d -n l/$(NUM_SPLIT) $< $<.
test-group-%:split-test-packages
	cat $(BUILDDIR)/packages.txt.$* | xargs go test -mod=readonly -timeout=5m -race -coverprofile=$(BUILDDIR)/$*.profile.out
