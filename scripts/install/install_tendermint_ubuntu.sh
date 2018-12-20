#!/usr/bin/env bash

# XXX: this script is intended to be run from
# a fresh Digital Ocean droplet with Ubuntu

# upon its completion, you must either reset
# your terminal or run `source ~/.profile`

# as written, this script will install
# tendermint core from master branch
REPO=github.com/tendermint/tendermint

# change this to a specific release or branch
BRANCH=master

GO_VERSION=1.11.4

sudo apt-get update -y
sudo apt-get install -y make

# get and unpack golang
curl -O https://storage.googleapis.com/golang/go$GO_VERSION.linux-amd64.tar.gz
tar -xvf go$GO_VERSION.linux-amd64.tar.gz

# move go folder and add go binary to path
sudo mv go /usr/local
echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.profile

# create the go directory, set GOPATH, and put it on PATH
mkdir go
echo "export GOPATH=$HOME/go" >> ~/.profile
echo "export PATH=\$PATH:\$GOPATH/bin" >> ~/.profile
source ~/.profile

# get the code and move into repo
go get $REPO
cd "$GOPATH/src/$REPO"

# build & install
git checkout $BRANCH
# XXX: uncomment if branch isn't master
# git fetch origin $BRANCH
make get_tools
make get_vendor_deps
make install

# the binary is located in $GOPATH/bin
# run `source ~/.profile` or reset your terminal
# to persist the changes
