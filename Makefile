#!/usr/bin/make -f

PACKAGES=$(shell go list ./...)
BUILDDIR ?= $(CURDIR)/build

BUILD_TAGS?=tendermint
VERSION := $(shell git describe --always)
LD_FLAGS = -X github.com/tendermint/tendermint/version.TMCoreSemVer=$(VERSION)
BUILD_FLAGS = -mod=readonly -ldflags "$(LD_FLAGS)"
HTTPS_GIT := https://github.com/tendermint/tendermint.git
DOCKER_BUF := docker run -v $(shell pwd):/workspace --workdir /workspace bufbuild/buf
CGO_ENABLED ?= 0

# handle nostrip
ifeq (,$(findstring nostrip,$(TENDERMINT_BUILD_OPTIONS)))
  BUILD_FLAGS += -trimpath
  LD_FLAGS += -s -w
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

# allow users to pass additional flags via the conventional LDFLAGS variable
LD_FLAGS += $(LDFLAGS)

all: check build test install
.PHONY: all

# The below include contains the tools.
include tools/Makefile
include test/Makefile

###############################################################################
###                                Build Tendermint                        ###
###############################################################################

build: $(BUILDDIR)/
	CGO_ENABLED=$(CGO_ENABLED) go build $(BUILD_FLAGS) -tags '$(BUILD_TAGS)' -o $(BUILDDIR)/ ./cmd/tendermint/
.PHONY: build

install:
	CGO_ENABLED=$(CGO_ENABLED) go install $(BUILD_FLAGS) -tags $(BUILD_TAGS) ./cmd/tendermint
.PHONY: install

$(BUILDDIR)/:
	mkdir -p $@

###############################################################################
###                                Protobuf                                 ###
###############################################################################

proto-all: proto-gen proto-lint proto-check-breaking
.PHONY: proto-all

proto-gen:
	## If you get the following error,
	## "error while loading shared libraries: libprotobuf.so.14: cannot open shared object file: No such file or directory"
	## See https://stackoverflow.com/a/25518702
	## Note the $< here is substituted for the %.proto
	## Note the $@ here is substituted for the %.pb.go
	@sh scripts/protocgen.sh
.PHONY: proto-gen

proto-gen-docker:
	@echo "Generating Protobuf files"
	@docker run -v $(shell pwd):/workspace --workdir /workspace tendermintdev/docker-build-proto sh ./scripts/protocgen.sh
.PHONY: proto-gen-docker

proto-lint:
	@$(DOCKER_BUF) check lint --error-format=json
.PHONY: proto-lint

proto-format:
	@echo "Formatting Protobuf files"
	docker run -v $(shell pwd):/workspace --workdir /workspace tendermintdev/docker-build-proto find ./ -not -path "./third_party/*" -name *.proto -exec clang-format -i {} \;
.PHONY: proto-format

proto-check-breaking:
	@$(DOCKER_BUF) check breaking --against-input .git#branch=master
.PHONY: proto-check-breaking

proto-check-breaking-ci:
	@$(DOCKER_BUF) check breaking --against-input $(HTTPS_GIT)#branch=master
.PHONY: proto-check-breaking-ci

###############################################################################
###                              Build ABCI                                 ###
###############################################################################

build_abci:
	@go build -mod=readonly -i ./abci/cmd/...
.PHONY: build_abci

install_abci:
	@go install -mod=readonly ./abci/cmd/...
.PHONY: install_abci

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
	go get github.com/RobotsAndPencils/goviz
	@goviz -i github.com/tendermint/tendermint/cmd/tendermint -d 3 | dot -Tpng -o dependency-graph.png
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
	find . -name '*.go' -type f -not -path "*.git*"  -not -name '*.pb.go' -not -name '*pb_test.go' | xargs goimports -w -local github.com/tendermint/tendermint
.PHONY: format

lint:
	@echo "--> Running linter"
	@golangci-lint run
.PHONY: lint

DESTINATION = ./index.html.md

###############################################################################
###                           Documentation                                 ###
###############################################################################
# todo remove once tendermint.com DNS is solved
build-docs:
	@cd docs && \
	while read -r branch path_prefix; do \
		(git checkout $${branch} && npm install && VUEPRESS_BASE="/$${path_prefix}/" npm run build) ; \
		mkdir -p ~/output/$${path_prefix} ; \
		cp -r .vuepress/dist/* ~/output/$${path_prefix}/ ; \
		cp ~/output/$${path_prefix}/index.html ~/output ; \
	done < versions ;
.PHONY: build-docs

build-gh-docs:
	@cd docs && \
	while read -r branch path_prefix; do \
		(git checkout $${branch} && npm install && VUEPRESS_BASE="/tendermint/$${path_prefix}/" npm run build) ; \
		mkdir -p ~/output/$${path_prefix} ; \
		cp -r .vuepress/dist/* ~/output/$${path_prefix}/ ; \
		cp ~/output/$${path_prefix}/index.html ~/output ; \
	done < versions ;
.PHONY: build-docs

# todo remove once tendermint.com DNS is solved
sync-docs:
	cd ~/output && \
	echo "role_arn = ${DEPLOYMENT_ROLE_ARN}" >> /root/.aws/config ; \
	echo "CI job = ${CIRCLE_BUILD_URL}" >> version.html ; \
	aws s3 sync . s3://${WEBSITE_BUCKET} --profile terraform --delete ; \
	aws cloudfront create-invalidation --distribution-id ${CF_DISTRIBUTION_ID} --profile terraform --path "/*" ;
.PHONY: sync-docs

###############################################################################
###                            Docker image                                 ###
###############################################################################

build-docker: build-linux
	cp $(BUILDDIR)/tendermint DOCKER/tendermint
	docker build --label=tendermint --tag="tendermint/tendermint" DOCKER
	rm -rf DOCKER/tendermint
.PHONY: build-docker

###############################################################################
###                       Local testnet using docker                        ###
###############################################################################

# Build linux binary on other platforms
build-linux: tools
	GOOS=linux GOARCH=amd64 $(MAKE) build
.PHONY: build-linux

build-docker-localnode:
	@cd networks/local && make
.PHONY: build-docker-localnode

# Runs `make build TENDERMINT_BUILD_OPTIONS=cleveldb` from within an Amazon
# Linux (v2)-based Docker build container in order to build an Amazon
# Linux-compatible binary. Produces a compatible binary at ./build/tendermint
build_c-amazonlinux:
	$(MAKE) -C ./DOCKER build_amazonlinux_buildimage
	docker run --rm -it -v `pwd`:/tendermint tendermint/tendermint:build_c-amazonlinux
.PHONY: build_c-amazonlinux

# Run a 4-node testnet locally
localnet-start: localnet-stop build-docker-localnode
	@if ! [ -f build/node0/config/genesis.json ]; then docker run --rm -v $(CURDIR)/build:/tendermint:Z tendermint/localnode testnet --config /etc/tendermint/config-template.toml --o . --starting-ip-address 192.167.10.2; fi
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
# prerequisits: build-contract-tests-hooks build-linux
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
