#! /bin/bash
set -u

N=$1
PEERS=$2

cd "$GOPATH/src/github.com/tendermint/tendermint"

echo "Waiting for nodes to come online"
for i in $(seq 1 "$N"); do
	addr=$(test/p2p/ip.sh "$i"):26657
	curl -s "$addr/status" > /dev/null
	ERR=$?
	while [ "$ERR" != 0 ]; do
		sleep 1
		curl -s "$addr/status" > /dev/null
		ERR=$?
	done
	echo "... node $i is up"
done

IP=$(test/p2p/ip.sh 1)
curl "$IP:26657/dial_peers?persistent=true&peers=\\[$PEERS\\]"
