#! /bin/bash 
set -u

N=$1

cd $GOPATH/src/github.com/tendermint/tendermint

echo "Waiting for nodes to come online"
for i in `seq 1 $N`; do
	addr=$(test/p2p/ip.sh $i):46657
	curl -s $addr/status > /dev/null
	ERR=$?
	while [ "$ERR" != 0 ]; do
		sleep 1	
		curl -s $addr/status > /dev/null
		ERR=$?
	done
	echo "... node $i is up"
done

set -e
# manual_peers need quotes
manual_peers="\"$(test/p2p/ip.sh 1):46656\""
for i in `seq 2 $N`; do
	manual_peers="$manual_peers,\"$(test/p2p/ip.sh $i):46656\""
done
echo $manual_peers

echo $manual_peers
IP=$(test/p2p/ip.sh 1)
curl --data-urlencode "manual_peers=[$manual_peers]" "$IP:46657/dial_manual_peers"
