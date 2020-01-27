#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=local_testnet
N=4
PROXY_APP=persistent_kvstore
IPV=${2:-4} # Default to IPv4

if [[ "$IPV" != "4" && "$IPV" != "6" ]]; then
    echo "IP version must be 4 or 6" >&2
    exit 1
fi

# stop the existing testnet and remove local network
set +e
bash test/p2p/local_testnet_stop.sh "$NETWORK_NAME" "$N"
set -e

PERSISTENT_PEERS=$(bash test/p2p/persistent_peers.sh $IPV $N $DOCKER_IMAGE)

# start the testnet on a local network
# NOTE we re-use the same network for all tests
bash test/p2p/local_testnet_start.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" "$N" "$PROXY_APP" "$PERSISTENT_PEERS"

# test basic connectivity and consensus
# start client container and check the num peers and height for all nodes
bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" basic "test/p2p/basic/test.sh $IPV $N"

# test atomic broadcast:
# start client container and test sending a tx to each node
bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" ab "test/p2p/atomic_broadcast/test.sh $IPV $N"

# test fast sync (from current state of network):
# for each node, kill it and readd via fast sync
bash test/p2p/fast_sync/test.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" "$N" "$PROXY_APP"

# test killing all peers 3 times
bash test/p2p/kill_all/test.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" "$N" 3

# test pex
bash test/p2p/pex/test.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$IPV" "$N" "$PROXY_APP"
