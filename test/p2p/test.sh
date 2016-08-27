#! /bin/bash
set -eu

DOCKER_IMAGE=$1
NETWORK_NAME=local_testnet

cd $GOPATH/src/github.com/tendermint/tendermint

# start the testnet on a local network
bash test/p2p/local_testnet.sh $DOCKER_IMAGE $NETWORK_NAME

# test atomic broadcast
bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME ab test/p2p/atomic_broadcast/test.sh

# test fast sync (from current state of network)
# run it on each of them
N=4
for i in `seq 1 $N`; do
	echo "Testing fasysync on node $i"

	# kill peer 
	set +e # circle sigh :(
	docker rm -vf local_testnet_$i
	set -e 

	# restart peer - should have an empty blockchain
	SEEDS="$(test/p2p/ip.sh 1):46656"
	for j in `seq 2 $N`; do
		SEEDS="$SEEDS,$(test/p2p/ip.sh $j):46656"
	done
	bash test/p2p/peer.sh $DOCKER_IMAGE $NETWORK_NAME $i $SEEDS

	bash test/p2p/client.sh $DOCKER_IMAGE $NETWORK_NAME fs_$i "test/p2p/fast_sync/test.sh $i"
done
echo ""
echo "PASS"
echo ""

