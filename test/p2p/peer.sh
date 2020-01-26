#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=$2
IPV=$3
ID=$4
APP_PROXY=$5

set +u
NODE_FLAGS=$6
set -u

if [[ "$IPV" == 6 ]]; then
	IP_SWITCH="--ip6"
else
	IP_SWITCH="--ip"
fi

echo "starting tendermint peer ID=$ID"
# start tendermint container on the network
# NOTE: $NODE_FLAGS should be unescaped (no quotes). otherwise it will be
# treated as one flag.

# test/p2p/data/mach$((ID-1)) data is generated in test/docker/Dockerfile using
# the tendermint testnet command.
if [[ "$ID" == "x" ]]; then # Set "x" to "1" to print to console.
	docker run \
		--net="$NETWORK_NAME" \
		$IP_SWITCH=$(test/p2p/address.sh $IPV $ID) \
		--name "local_testnet_$ID" \
		--entrypoint tendermint \
		-e TMHOME="/go/src/github.com/tendermint/tendermint/test/p2p/data/mach$((ID-1))" \
		-e GOMAXPROCS=1 \
		--log-driver=syslog \
		--log-opt syslog-address=udp://127.0.0.1:5514 \
		--log-opt syslog-facility=daemon \
		--log-opt tag="{{.Name}}" \
		"$DOCKER_IMAGE" node $NODE_FLAGS --log_level=debug --proxy_app="$APP_PROXY" &
else
	docker run -d \
		--net="$NETWORK_NAME" \
		$IP_SWITCH=$(test/p2p/address.sh $IPV $ID) \
		--name "local_testnet_$ID" \
		--entrypoint tendermint \
		-e TMHOME="/go/src/github.com/tendermint/tendermint/test/p2p/data/mach$((ID-1))" \
		-e GOMAXPROCS=1 \
		--log-driver=syslog \
		--log-opt syslog-address=udp://127.0.0.1:5514 \
		--log-opt syslog-facility=daemon \
		--log-opt tag="{{.Name}}" \
		"$DOCKER_IMAGE" node $NODE_FLAGS --log_level=debug --proxy_app="$APP_PROXY"
fi
