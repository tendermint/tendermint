#!/bin/sh
#
# Update the generated code for protocol buffers in the Tendermint repository.
# This must be run from inside a Tendermint working directory.
#
set -euo pipefail

# Work from the root of the repository.
cd "$(git rev-parse --show-toplevel)"

# Run inside Docker to install the correct versions of the required tools
# without polluting the local system.
docker run --rm -i -v "$PWD":/w --workdir=/w golang:1.18-alpine sh <<"EOF"
apk add git make

go install github.com/bufbuild/buf/cmd/buf
go install github.com/gogo/protobuf/protoc-gen-gogofaster@latest
make proto-gen
EOF
