GOTOOLS = \
	github.com/mitchellh/gox \
	github.com/golang/dep/cmd/dep \
	gopkg.in/alecthomas/gometalinter.v2 \
	github.com/gogo/protobuf/protoc-gen-gogo \
	github.com/gogo/protobuf/gogoproto \
	github.com/square/certstrap
PACKAGES=$(shell go list ./... | grep -v '/vendor/')

INCLUDE = -I=. -I=${GOPATH}/src -I=${GOPATH}/src/github.com/gogo/protobuf/protobuf
BUILD_TAGS?='tendermint'
BUILD_FLAGS = -ldflags "-X github.com/tendermint/tendermint/version.GitCommit=`git rev-parse --short=8 HEAD`"

all: check build test install

check: check_tools ensure_deps


########################################
### Build Tendermint

build:
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -tags $(BUILD_TAGS) -o build/tendermint ./cmd/tendermint/

build_race:
	CGO_ENABLED=0 go build -race $(BUILD_FLAGS) -tags $(BUILD_TAGS) -o build/tendermint ./cmd/tendermint

install:
	CGO_ENABLED=0 go install $(BUILD_FLAGS) -tags $(BUILD_TAGS) ./cmd/tendermint

########################################
### Protobuf

protoc_all: protoc_libs protoc_abci protoc_grpc

%.pb.go: %.proto
	## If you get the following error,
	## "error while loading shared libraries: libprotobuf.so.14: cannot open shared object file: No such file or directory"
	## See https://stackoverflow.com/a/25518702
	protoc $(INCLUDE) $< --gogo_out=plugins=grpc:.
	@echo "--> adding nolint declarations to protobuf generated files"
	@awk -i inplace '/^\s*package \w+/ { print "//nolint" }1' $@

########################################
### Build ABCI

protoc_abci: abci/types/types.pb.go

build_abci:
	@go build -i ./abci/cmd/...

install_abci:
	@go install ./abci/cmd/...

########################################
### Distribution

# dist builds binaries for all platforms and packages them for distribution
# TODO add abci to these scripts
dist:
	@BUILD_TAGS=$(BUILD_TAGS) sh -c "'$(CURDIR)/scripts/dist.sh'"

########################################
### Tools & dependencies

check_tools:
	@# https://stackoverflow.com/a/25668869
	@echo "Found tools: $(foreach tool,$(notdir $(GOTOOLS)),\
        $(if $(shell which $(tool)),$(tool),$(error "No $(tool) in PATH")))"

get_tools:
	@echo "--> Installing tools"
	go get -u -v $(GOTOOLS)
	@gometalinter.v2 --install

update_tools:
	@echo "--> Updating tools"
	@go get -u $(GOTOOLS)

#Update dependencies
get_vendor_deps:
	@echo "--> Running dep"
	@dep ensure

#For ABCI and libs
get_protoc:
	@# https://github.com/google/protobuf/releases
	curl -L https://github.com/google/protobuf/releases/download/v3.4.1/protobuf-cpp-3.4.1.tar.gz | tar xvz && \
		cd protobuf-3.4.1 && \
		DIST_LANG=cpp ./configure && \
		make && \
		make install && \
		cd .. && \
		rm -rf protobuf-3.4.1

draw_deps:
	@# requires brew install graphviz or apt-get install graphviz
	go get github.com/RobotsAndPencils/goviz
	@goviz -i github.com/tendermint/tendermint/cmd/tendermint -d 3 | dot -Tpng -o dependency-graph.png

get_deps_bin_size:
	@# Copy of build recipe with additional flags to perform binary size analysis
	$(eval $(shell go build -work -a $(BUILD_FLAGS) -tags $(BUILD_TAGS) -o build/tendermint ./cmd/tendermint/ 2>&1))
	@find $(WORK) -type f -name "*.a" | xargs -I{} du -hxs "{}" | sort -rh | sed -e s:${WORK}/::g > deps_bin_size.log
	@echo "Results can be found here: $(CURDIR)/deps_bin_size.log"

########################################
### Libs

protoc_libs: libs/common/types.pb.go

gen_certs: clean_certs
	## Generating certificates for TLS testing...
	certstrap init --common-name "tendermint.com" --passphrase ""
	certstrap request-cert -ip "::" --passphrase ""
	certstrap sign "::" --CA "tendermint.com" --passphrase ""
	mv out/::.crt out/::.key db/remotedb

clean_certs:
	## Cleaning TLS testing certificates...
	rm -rf out
	rm -f db/remotedb/::.crt db/remotedb/::.key

test_libs: gen_certs
	GOCACHE=off go test -tags gcc $(PACKAGES)
	make clean_certs

grpc_dbserver:
	protoc -I db/remotedb/proto/ db/remotedb/proto/defs.proto --go_out=plugins=grpc:db/remotedb/proto

protoc_grpc: rpc/grpc/types.pb.go

########################################
### Testing

## required to be run first by most tests
build_docker_test_image:
	docker build -t tester -f ./test/docker/Dockerfile .

### coverage, app, persistence, and libs tests
test_cover:
	# run the go unit tests with coverage
	bash test/test_cover.sh

test_apps:
	# run the app tests using bash
	# requires `abci-cli` and `tendermint` binaries installed
	bash test/app/test.sh

test_abci_apps:
	bash abci/tests/test_app/test.sh

test_abci_cli:
	# test the cli against the examples in the tutorial at:
	# ./docs/abci-cli.md
	# if test fails, update the docs ^
	@ bash abci/tests/test_cli/test.sh

test_persistence:
	# run the persistence tests using bash
	# requires `abci-cli` installed
	docker run --name run_persistence -t tester bash test/persist/test_failure_indices.sh

	# TODO undockerize
	# bash test/persist/test_failure_indices.sh

test_p2p:
	docker rm -f rsyslog || true
	rm -rf test/logs || true
	mkdir test/logs
	cd test/
	docker run -d -v "logs:/var/log/" -p 127.0.0.1:5514:514/udp --name rsyslog voxxit/rsyslog
	cd ..
	# requires 'tester' the image from above
	bash test/p2p/test.sh tester

test_integrations:
	make build_docker_test_image
	make get_tools
	make get_vendor_deps
	make install
	make test_cover
	make test_apps
	make test_abci_apps
	make test_abci_cli
	make test_libs
	make test_persistence
	make test_p2p

test_release:
	@go test -tags release $(PACKAGES)

test100:
	@for i in {1..100}; do make test; done

vagrant_test:
	vagrant up
	vagrant ssh -c 'make test_integrations'

### go tests
test:
	@echo "--> Running go test"
	@GOCACHE=off go test $(PACKAGES)

test_race:
	@echo "--> Running go test --race"
	@go test -v -race $(PACKAGES)


########################################
### Formatting, linting, and vetting

fmt:
	@go fmt ./...

metalinter:
	@echo "--> Running linter"
	@gometalinter.v2 --vendor --deadline=600s --disable-all  \
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

metalinter_all:
	@echo "--> Running linter (all)"
	gometalinter.v2 --vendor --deadline=600s --enable-all --disable=lll ./...

check_dep:
	dep status >> /dev/null
	!(grep -n branch Gopkg.toml)

###########################################################
### Docker image

build-docker:
	cp build/tendermint DOCKER/tendermint
	docker build --label=tendermint --tag="tendermint/tendermint" DOCKER
	rm -rf DOCKER/tendermint

###########################################################
### Local testnet using docker

# Build linux binary on other platforms
build-linux:
	GOOS=linux GOARCH=amd64 $(MAKE) build

build-docker-localnode:
	cd networks/local
	make

# Run a 4-node testnet locally
localnet-start: localnet-stop
	@if ! [ -f build/node0/config/genesis.json ]; then docker run --rm -v $(CURDIR)/build:/tendermint:Z tendermint/localnode testnet --v 4 --o . --populate-persistent-peers --starting-ip-address 192.167.10.2 ; fi
	docker-compose up

# Stop testnet
localnet-stop:
	docker-compose down

###########################################################
### Remote full-nodes (sentry) using terraform and ansible

# Server management
sentry-start:
	@if [ -z "$(DO_API_TOKEN)" ]; then echo "DO_API_TOKEN environment variable not set." ; false ; fi
	@if ! [ -f $(HOME)/.ssh/id_rsa.pub ]; then ssh-keygen ; fi
	cd networks/remote/terraform && terraform init && terraform apply -var DO_API_TOKEN="$(DO_API_TOKEN)" -var SSH_KEY_FILE="$(HOME)/.ssh/id_rsa.pub"
	@if ! [ -f $(CURDIR)/build/node0/config/genesis.json ]; then docker run --rm -v $(CURDIR)/build:/tendermint:Z tendermint/localnode testnet --v 0 --n 4 --o . ; fi
	cd networks/remote/ansible && ANSIBLE_HOST_KEY_CHECKING=False ansible-playbook -i inventory/digital_ocean.py -l sentrynet install.yml
	@echo "Next step: Add your validator setup in the genesis.json and config.tml files and run \"make sentry-config\". (Public key of validator, chain ID, peer IP and node ID.)"

# Configuration management
sentry-config:
	cd networks/remote/ansible && ansible-playbook -i inventory/digital_ocean.py -l sentrynet config.yml -e BINARY=$(CURDIR)/build/tendermint -e CONFIGDIR=$(CURDIR)/build

sentry-stop:
	@if [ -z "$(DO_API_TOKEN)" ]; then echo "DO_API_TOKEN environment variable not set." ; false ; fi
	cd networks/remote/terraform && terraform destroy -var DO_API_TOKEN="$(DO_API_TOKEN)" -var SSH_KEY_FILE="$(HOME)/.ssh/id_rsa.pub"

# meant for the CI, inspect script & adapt accordingly
build-slate:
	bash scripts/slate.sh

# To avoid unintended conflicts with file names, always add to .PHONY
# unless there is a reason not to.
# https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html
.PHONY: check build build_race build_abci dist install install_abci check_dep check_tools get_tools update_tools get_vendor_deps draw_deps get_protoc protoc_abci protoc_libs gen_certs clean_certs grpc_dbserver test_cover test_apps test_persistence test_p2p test test_race test_integrations test_release test100 vagrant_test fmt build-linux localnet-start localnet-stop build-docker build-docker-localnode sentry-start sentry-config sentry-stop build-slate protoc_grpc protoc_all
