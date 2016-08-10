#! /bin/bash
set -euo pipefail

LIB=$1

GLIDE=$GOPATH/src/github.com/tendermint/tendermint/glide.lock

cat $GLIDE | grep -A1 $LIB | grep -v $LIB | awk '{print $2}'
