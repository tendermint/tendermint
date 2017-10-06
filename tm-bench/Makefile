DIST_DIRS := find * -type d -exec
VERSION := $(shell perl -ne '/^var version.*"([^"]+)".*$$/ && print "v$$1\n"' main.go)
GOTOOLS = \
					github.com/Masterminds/glide \
					github.com/mitchellh/gox

tools:
	go get -v $(GOTOOLS)

get_vendor_deps: tools
	glide install

build:
	go build -ldflags "-X main.version=${VERSION}"

install:
	go install -ldflags "-X main.version=${VERSION}"

test:
	go test

build-all: tools
	rm -rf ./dist
	gox -verbose \
		-ldflags "-X main.version=${VERSION}" \
		-os="linux darwin windows" \
		-arch="amd64 386 armv6 arm64" \
		-osarch="!darwin/arm64" \
		-output="dist/{{.OS}}-{{.Arch}}/{{.Dir}}" .

dist: build-all
	cd dist && \
		$(DIST_DIRS) cp ../LICENSE {} \; && \
		$(DIST_DIRS) cp ../README.rst {} \; && \
		$(DIST_DIRS) tar -zcf tm-bench-${VERSION}-{}.tar.gz {} \; && \
		shasum -a256 ./*.tar.gz > "./tm-bench_${VERSION}_SHA256SUMS" && \
		cd ..

build-docker:
	rm -f ./tm-bench
	docker run -it --rm -v "$(PWD):/go/src/app" -w "/go/src/app" -e "CGO_ENABLED=0" golang:alpine go build -ldflags "-X main.version=${VERSION}" -o tm-bench
	docker build -t "tendermint/bench" .

clean:
	rm -f ./tm-bench
	rm -rf ./dist

.PHONY: tools get_vendor_deps build install test build-all dist clean build-docker
