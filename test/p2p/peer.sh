#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
ID=$3
APP_PROXY=$4

set +u
NODE_FLAGS=$5
set -u

set +eu

echo "starting tendermint peer ID=$ID"
# start tendermint container on the network
if [[ "$CIRCLECI" == true ]]; then
	set -u
	docker run -d \
	  --net="$NETWORK_NAME" \
	  --ip=$(test/p2p/ip.sh $ID) \
	  --name "local_testnet_$ID" \
	  --entrypoint tendermint \
	  -e TMHOME="/go/src/github.com/tendermint/tendermint/test/p2p/data/mach$ID/core" \
		--log-driver=syslog \
		--log-opt syslog-address=udp://127.0.0.1:5514 \
		--log-opt syslog-facility=daemon \
		--log-opt tag="{{.Name}}" \
  	  "$DOCKER_IMAGE" node $NODE_FLAGS --log_level=debug --proxy_app="$APP_PROXY"
else
	set -u
	docker run -d \
	  --net="$NETWORK_NAME" \
	  --ip=$(test/p2p/ip.sh $ID) \
	  --name "local_testnet_$ID" \
	  --entrypoint tendermint \
	  -e TMHOME="/go/src/github.com/tendermint/tendermint/test/p2p/data/mach$ID/core" \
  	  "$DOCKER_IMAGE" node $NODE_FLAGS --log_level=info --proxy_app="$APP_PROXY"
fi

