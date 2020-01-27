#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
IPV=$3
N=$4
PROXY_APP=$5

echo "Test reconnecting from the address book"
bash test/p2p/pex/test_addrbook.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" "$N" "$PROXY_APP"

echo "Test connecting via /dial_peers"
bash test/p2p/pex/test_dial_peers.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" "$N" "$PROXY_APP"
