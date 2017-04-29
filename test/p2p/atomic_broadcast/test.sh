#! /bin/bash
set -u

N=$1

###################################################################
# assumes peers are already synced up
# test sending txs
# for each peer:
#	send a tx, wait for commit
#	assert app hash on every peer reflects the post tx state
###################################################################

echo ""
# run the test on each of them
for i in `seq 1 $N`; do
	addr=$(test/p2p/ip.sh $i):46657

	# current state
	HASH1=`curl -s $addr/status | jq .result.latest_app_hash`
	
	# - send a tx
	TX=aadeadbeefbeefbeef0$i
	echo "Broadcast Tx $TX"
	curl -s $addr/broadcast_tx_commit?tx=0x$TX
	echo ""

	# we need to wait another block to get the new app_hash
	h1=`curl -s $addr/status | jq .result.latest_block_height`
	h2=$h1
	while [ "$h2" == "$h1" ]; do
		sleep 1
		h2=`curl -s $addr/status | jq .result.latest_block_height`
	done

	# check that hash was updated
	HASH2=`curl -s $addr/status | jq .result.latest_app_hash`
	if [[ "$HASH1" == "$HASH2" ]]; then
		echo "Expected state hash to update from $HASH1. Got $HASH2"
		exit 1
	fi

	# check we get the same new hash on all other nodes
	for j in `seq 1 $N`; do
		if [[ "$i" != "$j" ]]; then
			addrJ=$(test/p2p/ip.sh $j):46657
			HASH3=`curl -s $addrJ/status | jq .result.latest_app_hash`
			
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
