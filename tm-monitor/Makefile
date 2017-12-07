DIST_DIRS := find * -type d -exec
VERSION := $(shell perl -ne '/^var version.*"([^"]+)".*$$/ && print "v$$1\n"' main.go)
GOTOOLS = \
					github.com/mitchellh/gox
PACKAGES=$(shell go list ./... | grep -v '/vendor')

tools:
	go get $(GOTOOLS)

get_vendor_deps:
	@hash glide 2>/dev/null || go get github.com/Masterminds/glide
	glide install

build:
	go build

install:
	go install

test:
	@go test -race $(PACKAGES)

build-all: tools
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
		$(DIST_DIRS) tar -zcf tm-monitor-${VERSION}-{}.tar.gz {} \; && \
		shasum -a256 ./*.tar.gz > "./tm-monitor_${VERSION}_SHA256SUMS" && \
		cd ..

build-docker:
	rm -f ./tm-monitor
	docker run -it --rm -v "$(PWD):/go/src/github.com/tendermint/tools/tm-monitor" -w "/go/src/github.com/tendermint/tools/tm-monitor" -e "CGO_ENABLED=0" golang:alpine go build -ldflags "-s -w" -o tm-monitor
	docker build -t "tendermint/monitor" .

clean:
	rm -f ./tm-monitor
	rm -rf ./dist

.PHONY: tools get_vendor_deps build install test build-all dist clean build-docker
