#! /bin/bash
set -eu

IPV=$1
ID=$2
PORT=${3:-}
DOCKER_IMAGE=${4:-}

if [[ "$IPV" == 6 ]]; then
    ADDRESS="fd80:b10c::"
else
    ADDRESS="172.57.0."
fi
ADDRESS="$ADDRESS$((100+$ID))"

if [[ -n "$PORT" ]]; then
    if [[ "$IPV" == 6 ]]; then
        ADDRESS="[$ADDRESS]"
    fi
    ADDRESS="$ADDRESS:$PORT"
fi

if [[ -n "$DOCKER_IMAGE" ]]; then
    NODEID="$(docker run --rm -e TMHOME=/go/src/github.com/tendermint/tendermint/test/p2p/data/mach$((ID-1)) $DOCKER_IMAGE tendermint show_node_id)"
    ADDRESS="$NODEID@$ADDRESS"
fi

echo $ADDRESS