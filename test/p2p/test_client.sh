#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
CMD=$3

# run the test container on the local network
docker run -t \
	--rm \
	-v $GOPATH/src/github.com/tendermint/tendermint/test/p2p/:/go/src/github.com/tendermint/tendermint/test/p2p \
	--net=$NETWORK_NAME \
	--ip=172.57.0.99 \
	--name test_container \
	--entrypoint bash \
	$DOCKER_IMAGE $CMD
