#! /bin/bash

###################################################################
# wait for all peers to come online
# for each peer:
# 	wait to have 3 peers
#	wait to be at height > 1
#	send a tx, wait for commit
#	assert app hash on every peer reflects the post tx state
###################################################################

N=4

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
# run the test on each of them
for i in `seq 1 $N`; do
	addr=$(test/p2p/ip.sh $i):46657

	# - assert everyone has 3 other peers
	N_PEERS=`curl -s $addr/net_info | jq '.result[1].peers | length'`
	while [ "$N_PEERS" != 3 ]; do
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

	# current state
	HASH1=`curl -s $addr/status | jq .result[1].latest_app_hash`
	
	# - send a tx
	TX=\"aadeadbeefbeefbeef0$i\"
	echo "Broadcast Tx $TX"
	curl -s $addr/broadcast_tx_commit?tx=$TX
	echo ""

	# we need to wait another block to get the new app_hash
	h1=`curl -s $addr/status | jq .result[1].latest_block_height`
	h2=$h1
	while [ "$h2" == "$h1" ]; do
		sleep 1
		h2=`curl -s $addr/status | jq .result[1].latest_block_height`
	done

	# check that hash was updated
	HASH2=`curl -s $addr/status | jq .result[1].latest_app_hash`
	if [[ "$HASH1" == "$HASH2" ]]; then
		echo "Expected state hash to update from $HASH1. Got $HASH2"
		exit 1
	fi

	# check we get the same new hash on all other nodes
	for j in `seq 1 $N`; do
		if [[ "$i" != "$j" ]]; then
			addrJ=$(test/p2p/ip.sh $j):46657
			HASH3=`curl -s $addrJ/status | jq .result[1].latest_app_hash`
			
			if [[ "$HASH2" != "$HASH3" ]]; then
				echo "App hash for node $j doesn't match. Got $HASH3, expected $HASH2"
				exit 1
			fi
		fi
	done

	echo "All nodes are up to date"
done

echo ""
echo "PASS"
echo ""
