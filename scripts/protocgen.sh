#!/usr/bin/env bash

set -eo pipefail

proto_dirs=$(find . -path ./third_party -prune -o -name '*.proto' -print0 | xargs -0 -n1 dirname | sort | uniq)
for dir in $proto_dirs; do
  protoc \
  -I. \
  --gogo_out=Mgoogle/protobuf/timestamp.proto=github.com/golang/protobuf/ptypes/timestamp,Mgoogle/protobuf/duration.proto=github.com/golang/protobuf/ptypes/duration,plugins=grpc,paths=source_relative:. \
  $(find "${dir}" -name '*.proto')
done
