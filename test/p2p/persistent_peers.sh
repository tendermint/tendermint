#! /bin/bash
set -eu

N=$1
DOCKER_IMAGE=$2

cd "$GOPATH/src/github.com/tendermint/tendermint"

persistent_peers="$(test/p2p/ip_plus_id.sh 1 $DOCKER_IMAGE):26656"
for i in $(seq 2 $N); do
	persistent_peers="$persistent_peers,$(test/p2p/ip_plus_id.sh $i $DOCKER_IMAGE):26656"
done
echo "$persistent_peers"
