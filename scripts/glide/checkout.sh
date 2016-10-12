#! /bin/bash
set -u

function parseGlide() {
	cat $1 | grep -A1 $2 | grep -v $2 | awk '{print $2}'
}


# fetch and checkout vendored dep

glide=$1
lib=$2

echo "----------------------------------"
echo "Getting $lib ..."
go get -t github.com/tendermint/$lib/...

VENDORED=$(parseGlide $glide $lib) 
cd $GOPATH/src/github.com/tendermint/$lib
MASTER=$(git rev-parse origin/master)

if [[ "$VENDORED" != "$MASTER" ]]; then
	echo "... VENDORED != MASTER ($VENDORED != $MASTER)"
	echo "... Checking out commit $VENDORED"
	git checkout $VENDORED &> /dev/null
fi

