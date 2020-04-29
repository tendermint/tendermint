#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
IPV=$3
ID=$4
CMD=$5

NAME=test_container_$ID

if [[ "$IPV" == 6 ]]; then
	IP_SWITCH="--ip6"
else
	IP_SWITCH="--ip"
fi

echo "starting test client container with CMD=$CMD"
# run the test container on the local network
docker run -t --rm \
	-v "$PWD/test/p2p/:/go/src/github.com/tendermint/tendermint/test/p2p" \
	--net="$NETWORK_NAME" \
	$IP_SWITCH=$(test/p2p/address.sh $IPV -1) \
	--name "$NAME" \
	--entrypoint bash \
	"$DOCKER_IMAGE" $CMD
