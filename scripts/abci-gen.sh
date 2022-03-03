#!/usr/bin/env bash
set -euo pipefail

cp ./proto/tendermint/abci/types.proto.intermediate ./proto/tendermint/abci/types.proto
cp ./proto/tendermint/types/types.proto.intermediate ./proto/tendermint/types/types.proto

MODNAME="$(go list -m)"
find ./proto/tendermint -name '*.proto' -not -path "./proto/tendermint/abci/types.proto" \
	-exec sh ./scripts/protopackage.sh {} "$MODNAME" ';'

sh ./scripts/protopackage.sh ./proto/tendermint/abci/types.proto $MODNAME "abci/types"

make proto-gen

mv ./proto/tendermint/abci/types.pb.go ./abci/types

echo "proto files have been compiled"

echo "checking out copied files"

find proto/tendermint/ -name '*.proto' -not -path "*.intermediate"\
	| xargs -I {} git checkout {}

find proto/tendermint/ -name '*.pb.go' \
	| xargs -I {} git checkout {}
