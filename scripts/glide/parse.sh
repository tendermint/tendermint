#! /bin/bash

if [[ "$GLIDE" == "" ]]; then
	GLIDE=$GOPATH/src/github.com/tendermint/tendermint/glide.lock
fi

set -euo pipefail

LIB=$1

cat $GLIDE | grep -A1 $LIB | grep -v $LIB | awk '{print $2}'
