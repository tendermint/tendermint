#!/usr/bin/env bash

VERS=master

set -eo pipefail

# Edit this line to clone your branch, if you are modifying protobuf files
curl -qL "https://github.com/tendermint/spec/archive/master.tar.gz" | tar -xjf - spec-"$VERS"/proto/

cp -r ./spec-"$VERS"/proto/tendermint/** ./proto/tendermint

buf generate --path proto/tendermint

mv ./proto/tendermint/abci/types.pb.go ./abci/types

echo "proto files have been generated"

echo "removing copied files"

rm -rf ./proto/tendermint/abci/types.proto
rm -rf ./proto/tendermint/blocksync/types.proto
rm -rf ./proto/tendermint/consensus/types.proto
rm -rf ./proto/tendermint/mempool/types.proto
rm -rf ./proto/tendermint/p2p/types.proto
rm -rf ./proto/tendermint/p2p/conn.proto
rm -rf ./proto/tendermint/p2p/pex.proto
rm -rf ./proto/tendermint/statesync/types.proto
rm -rf ./proto/tendermint/types/block.proto
rm -rf ./proto/tendermint/types/evidence.proto
rm -rf ./proto/tendermint/types/params.proto
rm -rf ./proto/tendermint/types/types.proto
rm -rf ./proto/tendermint/types/validator.proto
rm -rf ./proto/tendermint/version/version.proto

rm -rf ./spec-"$VERS"
