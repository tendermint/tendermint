#! /bin/bash

go get github.com/tendermint/tmsp/...

COMMIT=`bash scripts/glide/parse.sh $(pwd)/glide.lock tmsp`

cd $GOPATH/src/github.com/tendermint/tmsp
git checkout $COMMIT
go install ./cmd/...


