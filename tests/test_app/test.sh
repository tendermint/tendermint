#! /bin/bash
set -e

# These tests spawn the counter app and server by execing the ABCI_APP command and run some simple client tests against it

# Get the directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

# Change into that dir because we expect that.
cd "$DIR"

# test golang counter
ABCI_APP="counter" go run  ./*.go

# test golang counter via grpc
ABCI_APP="counter --abci=grpc" ABCI="grpc" go run ./*.go

# test nodejs counter
# TODO: fix node app
#ABCI_APP="node $GOPATH/src/github.com/tendermint/js-abci/example/app.js" go test -test.run TestCounter
