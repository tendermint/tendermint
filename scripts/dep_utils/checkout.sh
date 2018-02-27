#! /bin/bash
set -u

function getVendoredVersion() {
	cd "$GOPATH/src/github.com/tendermint/tendermint" || exit
	dep status | grep "$1" | awk '{print $4}'
	cd - || exit
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
