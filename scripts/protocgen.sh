#!/usr/bin/env bash

set -eo pipefail

buf generate

mv ./proto/tendermint/abci/types.pb.go ./abci/types

mv ./proto/tendermint/rpc/grpc/types.pb.go ./rpc/grpc
