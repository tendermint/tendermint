#!/usr/bin/env bash

set -eo pipefail

buf generate --path proto/tendermint

mv ./proto/tendermint/abci/types.pb.go ./abci/types

mv ./proto/tendermint/rpc/grpc/types.pb.go ./rpc/grpc
