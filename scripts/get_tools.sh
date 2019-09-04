#!/usr/bin/env bash
set -e

# This file downloads all of the binary dependencies we have, and checks out a
# specific git hash.
#
# repos it installs:
#   github.com/golang/dep/cmd/dep
#   github.com/gogo/protobuf/protoc-gen-gogo
#   github.com/square/certstrap
#   github.com/mitchellh/gox
#   github.com/golangci/golangci-lint
#   github.com/petermattis/goid
#   github.com/sasha-s/go-deadlock
#   goimports

## check if GOPATH is set
if [ -z ${GOPATH+x} ]; then
	echo "please set GOPATH (https://github.com/golang/go/wiki/SettingGOPATH)"
	exit 1
fi

mkdir -p "$GOPATH/src/github.com"
cd "$GOPATH/src/github.com" || exit 1

installFromGithub() {
	repo=$1
	commit=$2
	# optional
	subdir=$3
	echo "--> Installing $repo ($commit)..."
	if [ ! -d "$repo" ]; then
		mkdir -p "$repo"
		git clone "https://github.com/$repo.git" "$repo"
	fi
	if [ ! -z ${subdir+x} ] && [ ! -d "$repo/$subdir" ]; then
		echo "ERROR: no such directory $repo/$subdir"
		exit 1
	fi
	pushd "$repo" && \
		git fetch origin && \
		git checkout -q "$commit" && \
		if [ ! -z ${subdir+x} ]; then cd "$subdir" || exit 1; fi && \
		go install && \
		if [ ! -z ${subdir+x} ]; then cd - || exit 1; fi && \
		popd || exit 1
	echo "--> Done"
	echo ""
}

######################## DEVELOPER TOOLS #####################################
## protobuf v1.3.0
installFromGithub gogo/protobuf 0ca988a254f991240804bf9821f3450d87ccbb1b protoc-gen-gogo

installFromGithub square/certstrap 338204a88c4349b1c135eac1e8c14c693ad007da

# used to build tm-monitor & tm-bench binaries
## gox v1.0.1
installFromGithub mitchellh/gox d8caaff5a9dc98f4cfa1fcce6e7265a04689f641

## Trying to install golangci with Go 1.13 gives:
## go: github.com/go-critic/go-critic@v0.0.0-20181204210945-1df300866540: invalid pseudo-version: does not match version-control timestamp (2019-05-26T07:48:19Z)
## golangci-lint v1.17.1
# installFromGithub golangci/golangci-lint 4ba2155996359eabd8800d1fbf3e3a9777c80490 cmd/golangci-lint

## make test_with_deadlock
## XXX: https://github.com/tendermint/tendermint/issues/3242
installFromGithub petermattis/goid b0b1615b78e5ee59739545bb38426383b2cda4c9
installFromGithub sasha-s/go-deadlock d68e2bc52ae3291765881b9056f2c1527f245f1e
go get golang.org/x/tools/cmd/goimports
installFromGithub snikch/goodman 10e37e294daa3c9a90abded60ff9924bafab3888 cmd/goodman
