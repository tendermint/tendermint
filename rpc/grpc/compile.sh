#! /bin/bash

protoc --gogo_out=plugins=grpc:. -I $GOPATH/src/ -I . types.proto
