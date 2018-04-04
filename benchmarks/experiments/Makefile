DIST_DIRS := find * -type d -exec
VERSION := $(shell perl -ne '/^var version.*"([^"]+)".*$$/ && print "v$$1\n"' main.go)
GOTOOLS = \
					github.com/mitchellh/gox

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
	go test -race

clean:
	rm -f ./experiments
	rm -rf ./dist

.PHONY: tools get_vendor_deps build install test clean
