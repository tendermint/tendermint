#! /bin/bash
set -u

N=$1

###################################################################
# wait for all peers to come online
# for each peer:
# 	wait to have N-1 peers
#	wait to be at height > 1
###################################################################

# wait for everyone to come online
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

echo ""
# wait for each of them to sync up
for i in `seq 1 $N`; do
	addr=$(test/p2p/ip.sh $i):46657
	N_1=$(($N - 1))

	# - assert everyone has N-1 other peers
	N_PEERS=`curl -s $addr/net_info | jq '.result[1].peers | length'`
	while [ "$N_PEERS" != $N_1 ]; do
		echo "Waiting for node $i to connect to all peers ..."
		sleep 1
		N_PEERS=`curl -s $addr/net_info | jq '.result[1].peers | length'`
	done

	# - assert block height is greater than 1
	BLOCK_HEIGHT=`curl -s $addr/status | jq .result[1].latest_block_height`
	while [ "$BLOCK_HEIGHT" -le 1 ]; do
		echo "Waiting for node $i to commit a block ..."
		sleep 1
		BLOCK_HEIGHT=`curl -s $addr/status | jq .result[1].latest_block_height`
	done
	echo "Node $i is connected to all peers and at block $BLOCK_HEIGHT"
done

echo ""
echo "PASS"
echo ""
