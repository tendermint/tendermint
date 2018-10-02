#! /bin/bash

protoc --gogo_out=. -I $GOPATH/src/ -I . -I $GOPATH/src/github.com/gogo/protobuf/protobuf merkle.proto
echo "--> adding nolint declarations to protobuf generated files"
awk '/package merkle/ { print "//nolint: gas"; print; next }1' merkle.pb.go > merkle.pb.go.new
mv merkle.pb.go.new merkle.pb.go
