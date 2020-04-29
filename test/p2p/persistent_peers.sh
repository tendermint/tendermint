#! /bin/bash
set -eu

IPV=$1
N=$2
DOCKER_IMAGE=$3

persistent_peers="$(test/p2p/address.sh $IPV 1 26656 $DOCKER_IMAGE)"
for i in $(seq 2 $N); do
	persistent_peers="$persistent_peers,$(test/p2p/address.sh $IPV $i 26656 $DOCKER_IMAGE)"
done
echo "$persistent_peers"
