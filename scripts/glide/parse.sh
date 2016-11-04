#! /bin/bash
set -euo pipefail

LIB=$1

set +u
if [[ "$GLIDE" == "" ]]; then
	GLIDE=$GOPATH/src/github.com/tendermint/tendermint/glide.lock
fi
set -u

cat $GLIDE | grep -A1 $LIB | grep -v $LIB | awk '{print $2}'
