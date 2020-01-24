#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
IPV=$3
N=$4
APP_PROXY=$5

set +u
PERSISTENT_PEERS=$6
if [[ "$PERSISTENT_PEERS" != "" ]]; then
	echo "PersistentPeers: $PERSISTENT_PEERS"
	PERSISTENT_PEERS="--p2p.persistent_peers $PERSISTENT_PEERS"
fi
set -u

# create docker network
if [[ $IPV == 6 ]]; then
	docker network create --driver bridge --ipv6 --subnet fd80:b10c::/48 "$NETWORK_NAME"
else
	docker network create --driver bridge --subnet 172.57.0.0/16 "$NETWORK_NAME"
fi

for i in $(seq 1 "$N"); do
	bash test/p2p/peer.sh "$DOCKER_IMAGE" "$NETWORK_NAME" $IPV "$i" "$APP_PROXY" "$PERSISTENT_PEERS --p2p.pex --rpc.unsafe"
done
