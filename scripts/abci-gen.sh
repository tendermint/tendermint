#!/usr/bin/env bash
set -euo pipefail

cp ./proto/intermediate/abci/types.proto ./proto/tendermint/abci/types.proto
sed -i 's/package intermediate\./package tendermint\./g' ./proto/tendermint/abci/types.proto

cp ./proto/intermediate/types/types.proto ./proto/tendermint/types/types.proto
sed -i 's/package intermediate\./package tendermint\./g' ./proto/tendermint/types/types.proto

MODNAME="$(go list -m)"
find ./proto/tendermint -name '*.proto' -not -path "./proto/tendermint/abci/types.proto" -not -path "./proto/intermediate" \
	-exec sh ./scripts/protopackage.sh {} "$MODNAME" ';'

sh ./scripts/protopackage.sh ./proto/tendermint/abci/types.proto $MODNAME "abci/types"

make proto-gen

mv ./proto/tendermint/abci/types.pb.go ./abci/types

echo "proto files have been compiled"

echo "removing copied files"

find proto/tendermint/ -name '*.proto' \
	| xargs -I {} git checkout {}

find proto/tendermint/ -name '*.pb.go' \
	| xargs -I {} git checkout {}
