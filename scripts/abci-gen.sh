#!/usr/bin/env bash

# This file was added during development of ABCI++. This file is a script to allow
# the intermediate proto files to be built while active development proceeds
# on ABCI++.
# This file should be removed when work on ABCI++ is complete.
# For more information, see https://github.com/tendermint/tendermint/issues/8066.
set -euo pipefail

cp ./proto/tendermint/abci/types.proto.intermediate ./proto/tendermint/abci/types.proto
cp ./proto/tendermint/types/types.proto.intermediate ./proto/tendermint/types/types.proto

MODNAME="$(go list -m)"
find ./proto/tendermint -name '*.proto' -not -path "./proto/tendermint/abci/types.proto" \
	-exec ./scripts/protopackage.sh {} "$MODNAME" ';'

./scripts/protopackage.sh ./proto/tendermint/abci/types.proto $MODNAME "abci/types"

make proto-gen

echo "proto files have been compiled"

echo "checking out copied files"

find proto/tendermint/ -name '*.proto' -not -path "*.intermediate"\
	| xargs -I {} git checkout {}

find proto/tendermint/ -name '*.pb.go' \
	| xargs -I {} git checkout {}
