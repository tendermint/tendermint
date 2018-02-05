#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
ID=$3
CMD=$4

NAME=test_container_$ID

echo "starting test client container with CMD=$CMD"
# run the test container on the local network
docker run -t --rm \
	# NOTE: the `:Z` at the end is needed for SELinux (on Jenkins) to allow writes to the mounted volume on host
	-v "$GOPATH/src/github.com/tendermint/tendermint/test/p2p/:/go/src/github.com/tendermint/tendermint/test/p2p":Z \
	--net="$NETWORK_NAME" \
	--ip=$(test/p2p/ip.sh "-1") \
	--name "$NAME" \
	--entrypoint bash \
	"$DOCKER_IMAGE" $CMD
