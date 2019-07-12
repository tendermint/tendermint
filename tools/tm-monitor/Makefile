DIST_DIRS := find * -type d -exec
VERSION := $(shell perl -ne '/^TMCoreSemVer = "([^"]+)"$$/ && print "v$$1\n"' ../../version/version.go)

all: build test install

########################################
###  Build

build:
	@go build

install:
	@go install

test:
	@go test -race $(PACKAGES)

build-all:
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

########################################
### Docker

build-docker:
	rm -f ./tm-monitor
	docker run -it --rm -v "$(PWD)/../../:/go/src/github.com/tendermint/tendermint" -w "/go/src/github.com/tendermint/tendermint/tools/tm-monitor" -e "GO111MODULE=on" -e "CGO_ENABLED=0" golang:1.12 go build -ldflags "-s -w" -o tm-monitor
	docker build -t "tendermint/monitor" .

clean:
	rm -f ./tm-monitor
	rm -rf ./dist

# To avoid unintended conflicts with file names, always add to .PHONY
# unless there is a reason not to.
# https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html
.PHONY: build install test build-all dist build-docker clean
