GOTOOLS = \
	github.com/golang/dep/cmd/dep \
	gopkg.in/alecthomas/gometalinter.v2
PACKAGES=$(shell go list ./... | grep -v '/vendor/')
BUILD_TAGS?=tendermint
BUILD_FLAGS = -ldflags "-X github.com/tendermint/tendermint/version.GitCommit=`git rev-parse --short=8 HEAD`"

all: check build test install

check: check_tools ensure_deps


########################################
### Build

build:
	go build $(BUILD_FLAGS) -tags '$(BUILD_TAGS)' -o build/tendermint ./cmd/tendermint/

build_race:
	go build -race $(BUILD_FLAGS) -tags '$(BUILD_TAGS)' -o build/tendermint ./cmd/tendermint

install:
	go install $(BUILD_FLAGS) -tags '$(BUILD_TAGS)' ./cmd/tendermint

########################################
### Distribution

# dist builds binaries for all platforms and packages them for distribution
dist:
	@BUILD_TAGS='$(BUILD_TAGS)' sh -c "'$(CURDIR)/scripts/dist.sh'"

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

#Run this from CI
get_vendor_deps:
	@rm -rf vendor/
	@echo "--> Running dep"
	@dep ensure -vendor-only


#Run this locally.
ensure_deps:
	@rm -rf vendor/
	@echo "--> Running dep"
	@dep ensure

draw_deps:
	@# requires brew install graphviz or apt-get install graphviz
	go get github.com/RobotsAndPencils/goviz
	@goviz -i github.com/tendermint/tendermint/cmd/tendermint -d 3 | dot -Tpng -o dependency-graph.png

get_deps_bin_size:
	@# Copy of build recipe with additional flags to perform binary size analysis
	$(eval $(shell go build -work -a $(BUILD_FLAGS) -tags '$(BUILD_TAGS)' -o build/tendermint ./cmd/tendermint/ 2>&1))
	@find $(WORK) -type f -name "*.a" | xargs -I{} du -hxs "{}" | sort -rh | sed -e s:${WORK}/::g > deps_bin_size.log
	@echo "Results can be found here: $(CURDIR)/deps_bin_size.log"

########################################
### Testing

## required to be run first by most tests
build_docker_test_image:
	docker build -t tester -f ./test/docker/Dockerfile .

clean_tests:
	docker rm -f rsyslog || true

### coverage, app, persistence, and libs tests
test_cover:
	# run the go unit tests with coverage (in docker)
	bash test/test_cover.sh
	
test_apps: clean_tests
	# run the app tests using bash
	# TODO requires `abci-cli` installed
	bash test/app/test.sh

test_persistence: clean_tests
	# run the persistence tests using bash
	# requires `abci-cli` installed
	bash test/persist/test_failure_indices.sh

test_p2p: clean_tests
	mkdir test/logs
	cd test/
	docker run -d -v "logs:/var/log/" -p 127.0.0.1:5514:514/udp --name rsyslog voxxit/rsyslog
	cd ..
	# requires 'tester' the image from above
	bash test/p2p/test.sh tester
	ls test/logs


test_libs:
	# checkout every github.com/tendermint dir and run its tests
	# NOTE: on release-* or master branches only (set by Jenkins)
	docker run --name run_test -t tester bash test/test_libs.sh

test_release:
	@go test -tags release $(PACKAGES)

test100:
	@for i in {1..100}; do make test; done

vagrant_test:
	vagrant up
	vagrant ssh -c 'make install'
	vagrant ssh -c 'make test_race'
	vagrant ssh -c 'make test_integrations'

### go tests without docker
test_unit:
	@echo "--> Running go test"
	@go test $(PACKAGES)

test_unit_race:
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


# To avoid unintended conflicts with file names, always add to .PHONY
# unless there is a reason not to.
# https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html
.PHONY: check build build_race dist install check_tools get_tools update_tools get_vendor_deps draw_deps get_deps_bin_size test test_race test_integrations test_release test100 vagrant_test fmt metalinter metalinter_all
