#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=local_testnet
N=4
PROXY_APP=persistent_kvstore

cd "$GOPATH/src/github.com/tendermint/tendermint"

# stop the existing testnet and remove local network
set +e
bash test/p2p/local_testnet_stop.sh "$NETWORK_NAME" "$N"
set -e

PERSISTENT_PEERS=$(bash test/p2p/persistent_peers.sh $N $DOCKER_IMAGE)

# start the testnet on a local network
# NOTE we re-use the same network for all tests
bash test/p2p/local_testnet_start.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$N" "$PROXY_APP" "$PERSISTENT_PEERS"

# test basic connectivity and consensus
# start client container and check the num peers and height for all nodes
bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" basic "test/p2p/basic/test.sh $N"

# test atomic broadcast:
# start client container and test sending a tx to each node
bash test/p2p/client.sh "$DOCKER_IMAGE" "$NETWORK_NAME" ab "test/p2p/atomic_broadcast/test.sh $N"

# test fast sync (from current state of network):
# for each node, kill it and readd via fast sync
bash test/p2p/fast_sync/test.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$N" "$PROXY_APP"

# test killing all peers 3 times
bash test/p2p/kill_all/test.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$N" 3

# test pex
bash test/p2p/pex/test.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$N" "$PROXY_APP"
