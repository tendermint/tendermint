#!/usr/bin/env bash

# XXX: this script is meant to be used only on a fresh Ubuntu 16.04 instance
# and has only been tested on Digital Ocean

# get and unpack golang
curl -O https://storage.googleapis.com/golang/go1.10.linux-amd64.tar.gz
tar -xvf go1.10.linux-amd64.tar.gz

apt install make

## move go and add binary to path
mv go /usr/local
echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.profile

## create the GOPATH directory, set GOPATH and put on PATH
mkdir goApps
echo "export GOPATH=/root/goApps" >> ~/.profile
echo "export PATH=\$PATH:\$GOPATH/bin" >> ~/.profile

source ~/.profile

## get the code and move into it
REPO=github.com/tendermint/tendermint
go get $REPO
cd $GOPATH/src/$REPO

## build
git checkout master
make get_tools
make get_vendor_deps
make install
