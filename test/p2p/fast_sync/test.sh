#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
N=$3
PROXY_APP=$4

cd $GOPATH/src/github.com/tendermint/tendermint

# run it on each of them
for i in `seq 1 $N`; do
	bash test/p2p/fast_sync/test_peer.sh $DOCKER_IMAGE $NETWORK_NAME $i $N $PROXY_APP
done


