#! /bin/bash
set -u

IPV=$1
N=$2
PEERS=$3

echo "Waiting for nodes to come online"
for i in $(seq 1 "$N"); do
	addr=$(test/p2p/ip_new.sh $IPV $i 26657)
	curl -s "$addr/status" > /dev/null
	ERR=$?
	while [ "$ERR" != 0 ]; do
		sleep 1
		curl -s "$addr/status" > /dev/null
		ERR=$?
	done
	echo "... node $i is up"
done

IP=$(test/p2p/ip_new.sh $IPV 1)
curl "$IP:26657/dial_peers?persistent=true&peers=\\[$PEERS\\]"
