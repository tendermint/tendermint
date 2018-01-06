# Repository specific variables
BINARY := tendermint
TMHOME ?= $(HOME)/.tendermint
LDFLAGS_VERSIONING := -X github.com/tendermint/tendermint/version.GitCommit=$(shell git rev-parse --short=8 HEAD)
#Enable the below to omit the symbol table and debug information from the binary
#LDFLAGS_EXTRA ?= -w -s
GOTOOLS := \
	github.com/tendermint/gox \
	github.com/Masterminds/glide \
	github.com/tcnksm/ghr \
	gopkg.in/alecthomas/gometalinter.v2
GO_MIN_VERSION := 1.9.2

#Generic, overrideable build variables
PACKAGES := $(shell go list ./... | grep -v '/vendor/')
BUILD_TAGS ?= $(BINARY)
GOPATH ?= $(shell go env GOPATH)
GOROOT ?= $(shell go env GOROOT)
GOGCCFLAGS ?= $(shell go env GOGCCFLAGS)
XC_ARCH ?= 386 amd64 arm
XC_OS ?= solaris darwin freebsd linux windows
XC_OSARCH ?= !darwin/arm !solaris/amd64 !freebsd/amd64
BUILD_OUTPUT ?= ./build/{{.OS}}_{{.Arch}}/$(BINARY)

#Generic fix build variables
GOX_FLAGS = -os="$(XC_OS)" -arch="$(XC_ARCH)" -osarch="$(XC_OSARCH)" -output="$(BUILD_OUTPUT)"
ifeq ($(BUILD_FLAGS_RACE),YES)
RACEFLAG=-race
else
RACEFLAG=
endif
BUILD_FLAGS = -asmflags "-trimpath $(GOPATH)" -gcflags "-trimpath $(GOPATH)" -tags "$(BUILD_TAGS)" -ldflags "$(LDFLAGS_VERSIONING) $(LD_FLAGS_EXTRA)" $(RACEFLAG)
GO_VERSION := $(shell go version | grep -o '[[:digit:]]\+.[[:digit:]]\+.[[:digit:]]\+')
#Check that that minor version of GO meets the minimum required
GO_MINOR_VERSION := $(shell grep -o \.[[:digit:]][[:digit:]]*\. <<< $(GO_VERSION) | grep -o [[:digit:]]* )
GO_MIN_MINOR_VERSION := $(shell grep -o \.[[:digit:]][[:digit:]]*\. <<< $(GO_MIN_VERSION) | grep -o [[:digit:]]* )
GO_MINOR_VERSION_CHECK := $(shell test $(GO_MINOR_VERSION) -ge $(GO_MIN_MINOR_VERSION) && echo YES)


all: check build test install metalinter

check: check_tools get_vendor_deps


########################################
### Build

build_xc: check_tools
	$(shell which gox) $(BUILD_FLAGS) $(GOX_FLAGS) ./cmd/$(BINARY)/

build:
ifeq ($(OS),Windows_NT)
	make build_xc XC_ARCH=amd64 XC_OS=windows BUILD_OUTPUT=$(GOPATH)/bin/$(BINARY)
else
	make build_xc XC_ARCH=amd64 XC_OS="$(shell uname -s)" BUILD_OUTPUT=$(GOPATH)/bin/$(BINARY)
endif

build_race:
	make build BUILD_FLAGS_RACE=YES

# dist builds binaries for all platforms and packages them for distribution
dist:
	@BUILD_TAGS='$(BUILD_TAGS)' sh -c "'$(CURDIR)/scripts/dist.sh'"

install:
	make build


########################################
### Tools & dependencies

check_tools:
ifeq ($(GO_VERSION),)
	$(error go not found)
endif
#Check minimum required go version
ifneq ($(GO_VERSION),$(GO_MIN_VERSION))
	$(warning WARNING: build will not be deterministic. go version should be $(GO_MIN_VERSION))
ifneq ($(GO_MINOR_VERSION_CHECK),YES)
	$(error ERROR: The minor version of Go ($(GO_VERSION)) is lower than the minimum required ($(GO_MIN_VERSION)))
endif
endif
#-fdebug-prefix-map switches the temporary, randomized workdir name in the binary to a static text
ifneq ($(findstring -fdebug-prefix-map,$(GOGCCFLAGS)),-fdebug-prefix-map)
	$(warning WARNING: build will not be deterministic. The compiler does not support the '-fdebug-prefix-map' flag.)
endif
#GOROOT string is copied into the binary. For deterministic builds, we agree to keep it at /usr/local/go. (Default for golang:1.9.2 docker image, linux and osx.)
ifneq ($(GOROOT),/usr/local/go)
	$(warning WARNING: build will not be deterministic. GOROOT should be set to /usr/local/go)
endif
#GOPATH string is copied into the binary. Although the -trimpath flag tries to eliminate it, it doesn't do it everywhere in Go 1.9.2. For deterministic builds we agree to keep it at /go. (Default for golang:1.9.2 docker image.)
ifneq ($(GOPATH),/go)
	$(warning WARNING: build will not be deterministic. GOPATH should be set to /go)
endif
#External dependencies defined in GOTOOLS are built with get_tools. If they are already available on the system (for exmaple using a package manager), then get_tools might not be necessary.
ifneq ($(findstring $(GOPATH)/bin,$(PATH)),$(GOPATH)/bin)
	$(warning WARNING: PATH does not contain GOPATH/bin. Some external dependencies might be unavailable.) 
endif
# https://stackoverflow.com/a/25668869
	@echo "Found tools: $(foreach tool,$(notdir $(GOTOOLS)),$(if $(shell which $(tool)),$(tool),$(error "No $(tool) in PATH. Add GOPATH/bin to PATH and run 'make get_tools'")))"

get_tools:
	@echo "--> Installing tools"
	go get -u -v $(GOTOOLS)
	@gometalinter.v2 --install

update_tools:
	@echo "--> Updating tools"
	@go get -u $(GOTOOLS)

get_vendor_deps:
	@rm -rf vendor/
	@echo "--> Running glide install"
	@glide install

draw_deps:
	@# requires brew install graphviz or apt-get install graphviz
	go get github.com/RobotsAndPencils/goviz
	@goviz -i github.com/tendermint/tendermint/cmd/tendermint -d 3 | dot -Tpng -o dependency-graph.png


########################################
### Testing

test:
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


########################################
### Formatting, linting, and vetting

fmt:
	@go fmt ./...

metalinter:
	@echo "--> Running linter"
	gometalinter.v2 --vendor --deadline=600s --disable-all  \
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


# To avoid unintended conflicts with file names, always add to .PHONY
# unless there is a reason not to.
# https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html
.PHONY: check builkd_xc build build_race dist install check_tools get_tools update_tools get_vendor_deps draw_deps test test_race test_integrations test_release test100 vagrant_test fmt metalinter metalinter_all
