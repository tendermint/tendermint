#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
N=$3
APP_PROXY=$4

set +u
SEEDS=$5
if [[ "$SEEDS" != "" ]]; then
	echo "Seeds: $SEEDS"
	SEEDS="--p2p.seeds $SEEDS"
fi
set -u

cd "$GOPATH/src/github.com/tendermint/tendermint"

# create docker network
docker network create --driver bridge --subnet 172.57.0.0/16 "$NETWORK_NAME"

for i in $(seq 1 "$N"); do
	bash test/p2p/peer.sh "$DOCKER_IMAGE" "$NETWORK_NAME" "$i" "$APP_PROXY" "$SEEDS --p2p.pex --rpc.unsafe"
done
