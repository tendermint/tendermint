#! /bin/bash

go get -d github.com/tendermint/abci

# get the abci commit used by tendermint
COMMIT=$(bash scripts/dep_utils/parse.sh abci)

echo "Checking out vendored commit for abci: $COMMIT"

cd "$GOPATH/src/github.com/tendermint/abci" || exit
git checkout "$COMMIT"
make get_vendor_deps
make install
