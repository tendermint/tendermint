#! /bin/bash

go get github.com/tendermint/tmsp/...

# get the tmsp commit used by tendermint
COMMIT=`bash scripts/glide/parse.sh tmsp`

echo "Checking out vendored commit for tmsp: $COMMIT"

cd $GOPATH/src/github.com/tendermint/tmsp
git checkout $COMMIT
glide install
go install ./cmd/...
