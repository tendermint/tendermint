#! /bin/bash
set -euo pipefail

cd "$GOPATH/github.com/tendermint/tendermint" || exit

LIB=$1

dep status | grep "$LIB" | awk '{print $4}'
