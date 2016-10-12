#! /bin/bash
set -euo pipefail

LIB=$1

if [[ "$GLIDE" == "" ]]; then
	GLIDE=$GOPATH/src/github.com/tendermint/tendermint/glide.lock
fi

cat $GLIDE | grep -A1 $LIB | grep -v $LIB | awk '{print $2}'
