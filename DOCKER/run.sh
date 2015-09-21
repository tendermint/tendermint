#! /bin/bash

go get -u github.com/tendermint/tendermint/cmd/tendermint
cd $GOPATH/src/github.com/tendermint/tendermint
git fetch -a origin
git reset --hard $TMHEAD
make
tendermint node
