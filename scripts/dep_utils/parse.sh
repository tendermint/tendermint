#! /bin/bash

set +u
if [[ "$DEP" == "" ]]; then
	DEP=$GOPATH/src/github.com/tendermint/tendermint/Gopkg.lock
fi
set -u


set -euo pipefail

LIB=$1

grep -A100 "$LIB" "$DEP" | grep revision | head -n1 | grep -o '"[^"]\+"' | cut -d '"' -f 2
