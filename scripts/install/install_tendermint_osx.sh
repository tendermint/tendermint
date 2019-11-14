#!/usr/bin/env bash

# XXX: this script is intended to be run from
# an MacOS machine

# as written, this script will install
# tendermint core from master branch
REPO=github.com/tendermint/tendermint

# change this to a specific release or branch
BRANCH=master

if ! [ -x "$(command -v brew)" ]; then
  echo 'Error: brew is not installed, to install brew' >&2
  echo 'follow the instructions here: https://docs.brew.sh/Installation' >&2
  exit 1
fi

if ! [ -x "$(command -v go)" ]; then
  echo 'Error: go is not installed, to install go follow' >&2
  echo 'the instructions here: https://golang.org/doc/install#tarball' >&2
  echo 'ALSO MAKE SURE TO SETUP YOUR $GOPATH and $GOBIN in your ~/.profile: https://github.com/golang/go/wiki/SettingGOPATH' >&2
  exit 1
fi

if ! [ -x "$(command -v make)" ]; then
  echo 'Make not installed, installing using brew...'
  brew install make
fi

# get the code and move into repo
go get $REPO
cd $GOPATH/src/$REPO

# build & install
git checkout $BRANCH
# XXX: uncomment if branch isn't master
# git fetch origin $BRANCH
make tools
make install
