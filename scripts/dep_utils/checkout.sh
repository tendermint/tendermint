#! /bin/bash

set -ex

set +u
if [[ "$DEP" == "" ]]; then
	DEP=$GOPATH/src/github.com/tendermint/tendermint/Gopkg.lock
fi
set -u


set -u

function getVendoredVersion() {
	grep -A100 "$LIB" "$DEP" | grep revision | head -n1 | grep -o '"[^"]\+"' | cut -d '"' -f 2
}


# fetch and checkout vendored dep

lib=$1

echo "----------------------------------"
echo "Getting $lib ..."
go get -t "github.com/tendermint/$lib/..."

VENDORED=$(getVendoredVersion "$lib")
cd "$GOPATH/src/github.com/tendermint/$lib" || exit
MASTER=$(git rev-parse origin/master)

if [[ "$VENDORED" != "$MASTER" ]]; then
	echo "... VENDORED != MASTER ($VENDORED != $MASTER)"
	echo "... Checking out commit $VENDORED"
	git checkout "$VENDORED" &> /dev/null
fi
