#! /bin/bash
set -e

# These tests spawn the counter app and server by execing the TMSP_APP command and run some simple client tests against it

ROOT=$GOPATH/src/github.com/tendermint/tmsp/tests/test_app
cd $ROOT

# test golang counter
TMSP_APP="counter" go run  *.go

# test golang counter via grpc
TMSP_APP="counter -tmsp=grpc" TMSP="grpc" go run *.go

# test nodejs counter
# TODO: fix node app
#TMSP_APP="node $GOPATH/src/github.com/tendermint/js-tmsp/example/app.js" go test -test.run TestCounter
