#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
CMD=$3

echo "starting test client container with CMD=$CMD"
# run the test container on the local network
docker run -t \
	-v $GOPATH/src/github.com/tendermint/tendermint/test/p2p/:/go/src/github.com/tendermint/tendermint/test/p2p \
	--net=$NETWORK_NAME \
	--ip=$(test/p2p/ip.sh "-1") \
	--name test_container \
	--entrypoint bash \
	$DOCKER_IMAGE $CMD

docker rm -vf test_container
