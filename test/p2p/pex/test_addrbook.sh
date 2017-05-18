#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
N=$3
PROXY_APP=$4

ID=1

echo "----------------------------------------------------------------------"
echo "Testing pex creates the addrbook and uses it if seeds are not provided"
echo "(assuming peers are started with pex enabled)"

CLIENT_NAME="pex_addrbook_$ID"

echo "1. restart peer $ID"
docker stop "local_testnet_$ID"
# preserve addrbook.json
docker cp "local_testnet_$ID:/go/src/github.com/tendermint/tendermint/test/p2p/data/mach1/core/addrbook.json" "/tmp/addrbook.json"
set +e #CIRCLE
docker rm -vf "local_testnet_$ID"
set -e

# NOTE that we do not provide seeds
bash test/p2p/peer.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$ID" "$PROXY_APP" "--p2p.pex"
docker cp "/tmp/addrbook.json" "local_testnet_$ID:/go/src/github.com/tendermint/tendermint/test/p2p/data/mach1/core/addrbook.json"
echo "with the following addrbook:"
cat /tmp/addrbook.json
# exec doesn't work on circle
# docker exec "local_testnet_$ID" cat "/go/src/github.com/tendermint/tendermint/test/p2p/data/mach1/core/addrbook.json"
echo ""

# if the client runs forever, it means addrbook wasn't saved or was empty
bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$CLIENT_NAME" "test/p2p/pex/check_peer.sh $ID $N"

echo "----------------------------------------------------------------------"
echo "Testing other peers connect to us if we have neither seeds nor the addrbook"
echo "(assuming peers are started with pex enabled)"

CLIENT_NAME="pex_no_addrbook_$ID"

echo "1. restart peer $ID"
docker stop "local_testnet_$ID"
set +e #CIRCLE
docker rm -vf "local_testnet_$ID"
set -e 

# NOTE that we do not provide seeds
bash test/p2p/peer.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$ID" "$PROXY_APP" "--p2p.pex"

# if the client runs forever, it means other peers have removed us from their books (which should not happen)
bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$CLIENT_NAME" "test/p2p/pex/check_peer.sh $ID $N"

echo ""
echo "PASS"
echo ""
