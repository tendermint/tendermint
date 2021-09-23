#!/usr/bin/env bash

set -eo pipefail

git clone https://github.com/tendermint/spec.git

cp -vr ./spec/proto/tendermint/** ./proto/tendermint

buf generate --path proto/tendermint

mv ./proto/tendermint/abci/types.pb.go ./abci/types

mv ./proto/tendermint/rpc/grpc/types.pb.go ./rpc/grpc

rm -rf ./proto/tendermint/abci/types.proto
rm -rf ./proto/tendermint/blocksync/types.proto
rm -rf ./proto/tendermint/consensus/types.proto
rm -rf ./proto/tendermint/mempool/types.proto
rm -rf ./proto/tendermint/p2p/types.proto
rm -rf ./proto/tendermint/p2p/conn.proto
rm -rf ./proto/tendermint/p2p/pex.proto
rm -rf ./proto/tendermint/types/block.proto
rm -rf ./proto/tendermint/types/evidence.proto
rm -rf ./proto/tendermint/types/params.proto
rm -rf ./proto/tendermint/types/types.proto
rm -rf ./proto/tendermint/types/validator.proto
rm -rf ./proto/tendermint/version/version.proto

rm -rf ./spec
