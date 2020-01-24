#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
N=$3
PROXY_APP=$4

echo "Test reconnecting from the address book"
bash test/p2p/pex/test_addrbook.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$N" "$PROXY_APP"

echo "Test connecting via /dial_peers"
bash test/p2p/pex/test_dial_peers.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$N" "$PROXY_APP"
