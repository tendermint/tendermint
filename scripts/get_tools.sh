#!/usr/bin/env bash
# This file downloads all of the binary dependencies we have,
# and checks out a specific git hash.
#
# repos it installs:
# github.com/mitchellh/gox
# github.com/golang/dep/cmd/dep
# gopkg.in/alecthomas/gometalinter.v2
# github.com/gogo/protobuf/protoc-gen-gogo
# github.com/square/certstrap

pushd "$GOPATH/src/github.com"

## install gox
mkdir -p mitchellh
cd mitchellh
if cd gox; then git fetch origin; else git clone https://github.com/mitchellh/gox.git; cd gox; fi
git checkout -q 51ed453898ca5579fea9ad1f08dff6b121d9f2e8
go install
cd ../../

## install dep
mkdir -p golang
cd golang
if cd dep; then git fetch origin; else git clone https://github.com/golang/dep.git; cd dep; fi
git checkout -q 22125cfaa6ddc71e145b1535d4b7ee9744fefff2
cd cmd/dep
go install
cd ../../../../

## install gometalinter v2.0.11
mkdir -p alecthomas
cd alecthomas
if cd gometalinter; then git fetch origin; else git clone https://github.com/alecthomas/gometalinter.git; cd gometalinter; fi
git checkout -q 17a7ffa42374937bfecabfb8d2efbd4db0c26741
go install
cd ../../

## install protoc-gen-gogo
mkdir -p gogo
cd gogo
if cd protobuf; then git fetch origin; else git clone https://github.com/gogo/protobuf.git; cd protobuf; fi
git checkout -q 61dbc136cf5d2f08d68a011382652244990a53a9
cd protoc-gen-gogo
go install
cd ../../../

## install certstrap
mkdir -p square
cd square
if cd certstrap; then git fetch origin; else git clone https://github.com/square/certstrap.git; cd certstrap; fi
git checkout -q e27060a3643e814151e65b9807b6b06d169580a7
go install
cd ../../

popd