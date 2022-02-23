#!/usr/bin/bash
#
# Protobuf code generation script.
#
# To be run from the root of the Tendermint repository.
#
set -euo pipefail

INPUT_DIR=${INPUT_DIR:-./proto}
PLUGIN=${PLUGIN:-gogofaster}
PLUGIN_OPTS=${PLUGIN_OPTS:-"Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/duration.proto=github.com/golang/protobuf/ptypes/duration,plugins=grpc,paths=source_relative"}
OUTPUT_DIR=${OUTPUT_DIR:-./proto}

INCLUDES="-I=./third_party/proto/ -I=${INPUT_DIR}"
PROTOC="protoc ${INCLUDES} --${PLUGIN}_out=${PLUGIN_OPTS}:${OUTPUT_DIR}"

find ${INPUT_DIR} -name '*.proto' -exec ${PROTOC} {} \;
