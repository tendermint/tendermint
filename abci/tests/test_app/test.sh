#! /bin/bash
set -e

# These tests spawn the counter app and server by execing the ABCI_APP command and run some simple client tests against it

# Get the directory of where this script is.
export PATH="$GOBIN:$PATH"
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

# Change into that dir because we expect that.
cd "$DIR"

echo "RUN COUNTER OVER SOCKET"
# test golang counter
ABCI_APP="counter" go run -mod=readonly ./*.go
echo "----------------------"


echo "RUN COUNTER OVER GRPC"
# test golang counter via grpc
ABCI_APP="counter --abci=grpc" ABCI="grpc" go run -mod=readonly ./*.go
echo "----------------------"

# test nodejs counter
# TODO: fix node app
#ABCI_APP="node $GOPATH/src/github.com/tendermint/js-abci/example/app.js" go test -test.run TestCounter
