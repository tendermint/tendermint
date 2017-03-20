DIST_DIRS := find * -type d -exec
VERSION := $(shell perl -ne '/^var version.*"([^"]+)".*$$/ && print "v$$1\n"' main.go)
GOTOOLS = \
					github.com/Masterminds/glide \
					github.com/mitchellh/gox
PACKAGES=$(shell go list ./... | grep -v '/vendor/')

tools:
	go get -v $(GOTOOLS)

get_deps: tools
	glide install

build:
	go build -ldflags "-X main.version=${VERSION}"

install:
	go install -ldflags "-X main.version=${VERSION}"

test:
	@go test $(PACKAGES)

build-all: tools
	gox -verbose \
		-ldflags "-X main.version=${VERSION}" \
		-os="linux darwin windows freebsd openbsd netbsd" \
		-arch="amd64 386 armv5 armv6 armv7 arm64" \
		-osarch="!darwin/arm64" \
		-output="dist/{{.OS}}-{{.Arch}}/{{.Dir}}" .

dist: build-all
	cd dist && \
		$(DIST_DIRS) cp ../LICENSE {} \; && \
		$(DIST_DIRS) cp ../README.md {} \; && \
		$(DIST_DIRS) tar -zcf tm-monitor-${VERSION}-{}.tar.gz {} \; && \
		$(DIST_DIRS) zip -r tm-monitor-${VERSION}-{}.zip {} \; && \
		cd ..

build-docker:
	rm -f ./tm-monitor
	docker run -it --rm -v "$(PWD):/go/src/github.com/tendermint/tools/tm-monitor" -w "/go/src/github.com/tendermint/tools/tm-monitor" -e "CGO_ENABLED=0" golang:alpine go build -ldflags "-X main.version=${VERSION}" -o tm-monitor
	docker build -t "tendermint/monitor" .

clean:
	rm -f ./tm-monitor
	rm -rf ./dist

.PHONY: tools get_deps build install test build-all dist clean build-docker
