#!/bin/sh

. ~/.goenv

MERKLE=$GOPATH/src/github.com/tendermint/merkleeyes/iavl
cd $MERKLE
git pull

make get_deps
make record
