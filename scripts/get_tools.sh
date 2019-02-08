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

######################## COMMON TOOLS ########################################
installFromGithub golang/dep 22125cfaa6ddc71e145b1535d4b7ee9744fefff2 cmd/dep

######################## DEVELOPER TOOLS #####################################
installFromGithub gogo/protobuf 61dbc136cf5d2f08d68a011382652244990a53a9 protoc-gen-gogo

installFromGithub square/certstrap e27060a3643e814151e65b9807b6b06d169580a7

# used to build tm-monitor & tm-bench binaries
installFromGithub mitchellh/gox 51ed453898ca5579fea9ad1f08dff6b121d9f2e8

## golangci-lint v1.13.2
installFromGithub golangci/golangci-lint 7b2421d55194c9dc385eff7720a037aa9244ca3c cmd/golangci-lint

## make test_with_deadlock
## XXX: https://github.com/tendermint/tendermint/issues/3242
installFromGithub petermattis/goid b0b1615b78e5ee59739545bb38426383b2cda4c9
installFromGithub sasha-s/go-deadlock d68e2bc52ae3291765881b9056f2c1527f245f1e
go get golang.org/x/tools/cmd/goimports
