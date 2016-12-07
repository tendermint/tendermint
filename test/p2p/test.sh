#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=local_testnet

cd $GOPATH/src/github.com/tendermint/tendermint

# start the testnet on a local network
bash test/p2p/local_testnet.sh $DOCKER_IMAGE $NETWORK_NAME

# test atomic broadcast
bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME ab test/p2p/atomic_broadcast/test.sh

# test fast sync (from current state of network)
# run it on each of them
N=4
for i in `seq 1 $N`; do
	bash test/p2p/fast_sync/test.sh $DOCKER_IMAGE $NETWORK_NAME $i $N
done
