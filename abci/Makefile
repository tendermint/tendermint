GOTOOLS = \
	github.com/mitchellh/gox \
	github.com/golang/dep/cmd/dep \
	gopkg.in/alecthomas/gometalinter.v2 \
	github.com/gogo/protobuf/protoc-gen-gogo \
	github.com/gogo/protobuf/gogoproto
GOTOOLS_CHECK = gox dep gometalinter.v2 protoc protoc-gen-gogo
PACKAGES=$(shell go list ./... | grep -v '/vendor/')
INCLUDE = -I=. -I=${GOPATH}/src -I=${GOPATH}/src/github.com/gogo/protobuf/protobuf

all: check get_vendor_deps protoc build test install metalinter

check: check_tools


########################################
###  Build

protoc:
	## If you get the following error,
	## "error while loading shared libraries: libprotobuf.so.14: cannot open shared object file: No such file or directory"
	## See https://stackoverflow.com/a/25518702
	protoc $(INCLUDE) --gogo_out=plugins=grpc:. types/*.proto
	@echo "--> adding nolint declarations to protobuf generated files"
	@awk '/package types/ { print "//nolint: gas"; print; next }1' types/types.pb.go > types/types.pb.go.new
	@mv types/types.pb.go.new types/types.pb.go

build:
	@go build -i ./cmd/...

dist:
	@bash scripts/dist.sh
	@bash scripts/publish.sh

install:
	@go install ./cmd/...


########################################
### Tools & dependencies

check_tools:
	@# https://stackoverflow.com/a/25668869
	@echo "Found tools: $(foreach tool,$(GOTOOLS_CHECK),\
        $(if $(shell which $(tool)),$(tool),$(error "No $(tool) in PATH")))"

get_tools:
	@echo "--> Installing tools"
	go get -u -v $(GOTOOLS)
	@gometalinter.v2 --install

get_protoc:
	@# https://github.com/google/protobuf/releases
	curl -L https://github.com/google/protobuf/releases/download/v3.4.1/protobuf-cpp-3.4.1.tar.gz | tar xvz && \
		cd protobuf-3.4.1 && \
		DIST_LANG=cpp ./configure && \
		make && \
		make install && \
		cd .. && \
		rm -rf protobuf-3.4.1

update_tools:
	@echo "--> Updating tools"
	@go get -u $(GOTOOLS)

get_vendor_deps:
	@rm -rf vendor/
	@echo "--> Running dep ensure"
	@dep ensure


########################################
### Testing

test:
	@find . -path ./vendor -prune -o -name "*.sock" -exec rm {} \;
	@echo "==> Running go test"
	@go test $(PACKAGES)

test_race:
	@find . -path ./vendor -prune -o -name "*.sock" -exec rm {} \;
	@echo "==> Running go test --race"
	@go test -v -race $(PACKAGES)

### three tests tested by Jenkins
test_cover:
	@ bash tests/test_cover.sh

test_apps:
	# test the counter using a go test script
	@ bash tests/test_app/test.sh

test_cli:
	# test the cli against the examples in the tutorial at:
	# http://tendermint.readthedocs.io/projects/tools/en/master/abci-cli.html
	#
	# XXX: if this test fails, fix it and update the docs at:
	# https://github.com/tendermint/tendermint/blob/develop/docs/abci-cli.rst
	@ bash tests/test_cli/test.sh

########################################
### Formatting, linting, and vetting

fmt:
	@go fmt ./...

metalinter:
	@echo "==> Running linter"
	gometalinter.v2 --vendor --deadline=600s --disable-all  \
		--enable=maligned \
		--enable=deadcode \
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
		#--enable=gas \
		#--enable=dupl \
		#--enable=errcheck \
		#--enable=gocyclo \
		#--enable=golint \ <== comments on anything exported
		#--enable=gotype \
		#--enable=interfacer \
		#--enable=unparam \
		#--enable=vet \

metalinter_all:
	protoc $(INCLUDE) --lint_out=. types/*.proto
	gometalinter.v2 --vendor --deadline=600s --enable-all --disable=lll ./...


########################################
### Docker

DEVDOC_SAVE = docker commit `docker ps -a -n 1 -q` devdoc:local

docker_build:
	docker build -t "tendermint/abci-dev" -f Dockerfile.develop .

docker_run:
	docker run -it -v "$(CURDIR):/go/src/github.com/tendermint/abci" -w "/go/src/github.com/tendermint/abci" "tendermint/abci-dev" /bin/bash

docker_run_rm:
	docker run -it --rm -v "$(CURDIR):/go/src/github.com/tendermint/abci" -w "/go/src/github.com/tendermint/abci" "tendermint/abci-dev" /bin/bash

devdoc_init:
	docker run -it -v "$(CURDIR):/go/src/github.com/tendermint/abci" -w "/go/src/github.com/tendermint/abci" tendermint/devdoc echo
	# TODO make this safer
	$(call DEVDOC_SAVE)

devdoc:
	docker run -it -v "$(CURDIR):/go/src/github.com/tendermint/abci" -w "/go/src/github.com/tendermint/abci" devdoc:local bash

devdoc_save:
	# TODO make this safer
	$(call DEVDOC_SAVE)

devdoc_clean:
	docker rmi $$(docker images -f "dangling=true" -q)


# To avoid unintended conflicts with file names, always add to .PHONY
# unless there is a reason not to.
# https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html
.PHONY: check protoc build dist install check_tools get_tools get_protoc update_tools get_vendor_deps test test_race fmt metalinter metalinter_all docker_build docker_run docker_run_rm devdoc_init devdoc devdoc_save devdoc_clean
