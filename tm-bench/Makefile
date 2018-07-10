DIST_DIRS := find * -type d -exec
VERSION := $(shell perl -ne '/^var version.*"([^"]+)".*$$/ && print "v$$1\n"' main.go)
GOTOOLS = \
					github.com/mitchellh/gox \
					github.com/golang/dep/cmd/dep \
					gopkg.in/alecthomas/gometalinter.v2

all: check get_vendor_deps build test install metalinter

check: check_tools

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

update_tools:
	@echo "--> Updating tools"
	@go get -u $(GOTOOLS)

get_vendor_deps:
	@rm -rf vendor/
	@echo "--> Running dep ensure"
	@dep ensure

########################################
###  Build

build:
	@go build

install:
	@go install

test:
	@go test -race

build-all: check_tools
	rm -rf ./dist
	gox -verbose \
		-ldflags "-s -w" \
		-arch="amd64 386 arm arm64" \
		-os="linux darwin windows freebsd" \
		-osarch="!darwin/arm !darwin/arm64" \
		-output="dist/{{.OS}}-{{.Arch}}/{{.Dir}}" .

dist: build-all
	cd dist && \
		$(DIST_DIRS) cp ../LICENSE {} \; && \
		$(DIST_DIRS) cp ../README.rst {} \; && \
		$(DIST_DIRS) tar -zcf tm-bench-${VERSION}-{}.tar.gz {} \; && \
		shasum -a256 ./*.tar.gz > "./tm-bench_${VERSION}_SHA256SUMS" && \
		cd ..

########################################
### Docker

build-docker:
	rm -f ./tm-bench
	docker run -it --rm -v "$(PWD):/go/src/app" -w "/go/src/app" -e "CGO_ENABLED=0" golang:alpine go build -ldflags "-s -w" -o tm-bench
	docker build -t "tendermint/bench" .

clean:
	rm -f ./tm-bench
	rm -rf ./dist

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
	gometalinter.v2 --vendor --deadline=600s --enable-all --disable=lll ./...

# To avoid unintended conflicts with file names, always add to .PHONY
# unless there is a reason not to.
# https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html
.PHONY: check check_tools get_tools update_tools get_vendor_deps build install test build-all dist fmt metalinter metalinter_all build-docker clean
