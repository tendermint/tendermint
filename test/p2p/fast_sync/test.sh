#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
IPV=$3
N=$4
PROXY_APP=$5

# run it on each of them
for i in `seq 1 $N`; do
	bash test/p2p/fast_sync/test_peer.sh $DOCKER_IMAGE $NETWORK_NAME $IPV $i $N $PROXY_APP
done


