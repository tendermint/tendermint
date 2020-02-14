PACKAGES=$(shell go list ./...)
OUTPUT?=build/tendermint

BUILD_TAGS?='tendermint'
LD_FLAGS = -X github.com/tendermint/tendermint/version.GitCommit=`git rev-parse --short=8 HEAD` -s -w
BUILD_FLAGS = -mod=readonly -ldflags "$(LD_FLAGS)"

all: check build test install

# The below include contains the tools.
include tools.mk
include tests.mk

###############################################################################
###                                Build Tendermint                        ###
###############################################################################

build:
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -tags $(BUILD_TAGS) -o $(OUTPUT) ./cmd/tendermint/

build_c:
	CGO_ENABLED=1 go build $(BUILD_FLAGS) -tags "$(BUILD_TAGS) cleveldb" -o $(OUTPUT) ./cmd/tendermint/

build_race:
	CGO_ENABLED=1 go build -race $(BUILD_FLAGS) -tags $(BUILD_TAGS) -o $(OUTPUT) ./cmd/tendermint

install:
	CGO_ENABLED=0 go install $(BUILD_FLAGS) -tags $(BUILD_TAGS) ./cmd/tendermint

install_c:
	CGO_ENABLED=1 go install $(BUILD_FLAGS) -tags "$(BUILD_TAGS) cleveldb" ./cmd/tendermint

###############################################################################
###                                Protobuf                                 ###
###############################################################################

proto-all: proto-gen proto-lint proto-check-breaking

proto-gen:
	## If you get the following error,
	## "error while loading shared libraries: libprotobuf.so.14: cannot open shared object file: No such file or directory"
	## See https://stackoverflow.com/a/25518702
	## Note the $< here is substituted for the %.proto
	## Note the $@ here is substituted for the %.pb.go
	@sh scripts/protocgen.sh

proto-lint:
	@buf check lint --error-format=json

proto-check-breaking:
	@buf check breaking --against-input '.git#branch=master'

.PHONY: proto-all proto-gen proto-lint proto-check-breaking

###############################################################################
###                              Build ABCI                                 ###
###############################################################################

build_abci:
	@go build -mod=readonly -i ./abci/cmd/...

install_abci:
	@go install -mod=readonly ./abci/cmd/...

###############################################################################
###                              Distribution                               ###
###############################################################################

# dist builds binaries for all platforms and packages them for distribution
# TODO add abci to these scripts
dist:
	@BUILD_TAGS=$(BUILD_TAGS) sh -c "'$(CURDIR)/scripts/dist.sh'"

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

get_deps_bin_size:
	@# Copy of build recipe with additional flags to perform binary size analysis
	$(eval $(shell go build -work -a $(BUILD_FLAGS) -tags $(BUILD_TAGS) -o $(OUTPUT) ./cmd/tendermint/ 2>&1))
	@find $(WORK) -type f -name "*.a" | xargs -I{} du -hxs "{}" | sort -rh | sed -e s:${WORK}/::g > deps_bin_size.log
	@echo "Results can be found here: $(CURDIR)/deps_bin_size.log"

###############################################################################
###                                  Libs                                   ###
###############################################################################

# generates certificates for TLS testing in remotedb and RPC server
gen_certs: clean_certs
	certstrap init --common-name "tendermint.com" --passphrase ""
	certstrap request-cert --common-name "server" -ip "127.0.0.1" --passphrase ""
	certstrap sign "server" --CA "tendermint.com" --passphrase ""
	mv out/server.crt rpc/lib/server/test.crt
	mv out/server.key rpc/lib/server/test.key
	rm -rf out

# deletes generated certificates
clean_certs:
	rm -f rpc/lib/server/test.crt
	rm -f rpc/lib/server/test.key

###############################################################################
###                  Formatting, linting, and vetting                       ###
###############################################################################

fmt:
	@go fmt ./...

lint:
	@echo "--> Running linter"
	@golangci-lint run

DESTINATION = ./index.html.md

###############################################################################
###                           Documentation                                 ###
###############################################################################

build-docs:
	cd docs && \
	while read p; do \
		(git checkout $${p} && npm install && VUEPRESS_BASE="/$${p}/" npm run build) ; \
		mkdir -p ~/output/$${p} ; \
		cp -r .vuepress/dist/* ~/output/$${p}/ ; \
		cp ~/output/$${p}/index.html ~/output ; \
	done < versions ;

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

build-docker:
	cp $(OUTPUT) DOCKER/tendermint
	docker build --label=tendermint --tag="tendermint/tendermint" DOCKER
	rm -rf DOCKER/tendermint

###############################################################################
###                       Local testnet using docker                        ###
###############################################################################

# Build linux binary on other platforms
build-linux: tools
	GOOS=linux GOARCH=amd64 $(MAKE) build

build-docker-localnode:
	@cd networks/local && make

# Runs `make build_c` from within an Amazon Linux (v2)-based Docker build
# container in order to build an Amazon Linux-compatible binary. Produces a
# compatible binary at ./build/tendermint
build_c-amazonlinux:
	$(MAKE) -C ./DOCKER build_amazonlinux_buildimage
	docker run --rm -it -v `pwd`:/tendermint tendermint/tendermint:build_c-amazonlinux

# Run a 4-node testnet locally
localnet-start: localnet-stop build-docker-localnode
	@if ! [ -f build/node0/config/genesis.json ]; then docker run --rm -v $(CURDIR)/build:/tendermint:Z tendermint/localnode testnet --config /etc/tendermint/config-template.toml --v 4 --o . --populate-persistent-peers --starting-ip-address 192.167.10.2; fi
	docker-compose up

# Stop testnet
localnet-stop:
	docker-compose down

# Build hooks for dredd, to skip or add information on some steps
build-contract-tests-hooks:
ifeq ($(OS),Windows_NT)
	go build -mod=readonly $(BUILD_FLAGS) -o build/contract_tests.exe ./cmd/contract_tests
else
	go build -mod=readonly $(BUILD_FLAGS) -o build/contract_tests ./cmd/contract_tests
endif

# Run a nodejs tool to test endpoints against a localnet
# The command takes care of starting and stopping the network
# prerequisits: build-contract-tests-hooks build-linux
# the two build commands were not added to let this command run from generic containers or machines.
# The binaries should be built beforehand
contract-tests:
	dredd

# To avoid unintended conflicts with file names, always add to .PHONY
# unless there is a reason not to.
# https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html
.PHONY: check build build_race build_abci dist install install_abci check_tools tools update_tools draw_deps \
 	protoc_abci protoc_libs gen_certs clean_certs grpc_dbserver fmt build-linux localnet-start \
 	localnet-stop build-docker build-docker-localnode protoc_grpc protoc_all \
 	build_c install_c test_with_deadlock cleanup_after_test_with_deadlock lint build-contract-tests-hooks contract-tests \
	build_c-amazonlinux
