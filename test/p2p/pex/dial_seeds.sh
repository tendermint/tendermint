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
# seeds need quotes
seeds="\"$(test/p2p/ip.sh 1):46656\""
for i in `seq 2 $N`; do
	seeds="$seeds,\"$(test/p2p/ip.sh $i):46656\""
done
echo $seeds

echo $seeds
IP=$(test/p2p/ip.sh 1)
curl --data-urlencode "seeds=[$seeds]" "$IP:46657/dial_seeds"
