#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=local_testnet
N=4

cd $GOPATH/src/github.com/tendermint/tendermint

# start the testnet on a local network
bash test/p2p/local_testnet.sh $DOCKER_IMAGE $NETWORK_NAME $N

# test basic connectivity and consensus
# start client container and check the num peers and height for all nodes
bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME basic "test/p2p/basic/test.sh $N"

# test atomic broadcast:
# start client container and test sending a tx to each node
bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME ab "test/p2p/atomic_broadcast/test.sh $N"

# test fast sync (from current state of network):
# for each node, kill it and readd via fast sync
bash test/p2p/fast_sync/test.sh $DOCKER_IMAGE $NETWORK_NAME $N
