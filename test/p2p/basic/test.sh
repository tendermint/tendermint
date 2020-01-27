#! /bin/bash
set -u

IPV=$1
N=$2

###################################################################
# wait for all peers to come online
# for each peer:
# 	wait to have N-1 peers
#	wait to be at height > 1
###################################################################

# wait 60s per step per peer
MAX_SLEEP=60

# wait for everyone to come online
echo "Waiting for nodes to come online"
for i in `seq 1 $N`; do
	addr=$(test/p2p/address.sh $IPV $i 26657)
	curl -s $addr/status > /dev/null
	ERR=$?
	COUNT=0
	while [ "$ERR" != 0 ]; do
		sleep 1
		curl -s $addr/status > /dev/null
		ERR=$?
		COUNT=$((COUNT+1))
		if [ "$COUNT" -gt "$MAX_SLEEP" ]; then
			echo "Waited too long for node $i to come online"
			exit 1
		fi
	done
	echo "... node $i is up"
done

echo ""
# wait for each of them to sync up
for i in `seq 1 $N`; do
	addr=$(test/p2p/address.sh $IPV $i 26657)
	N_1=$(($N - 1))

	# - assert everyone has N-1 other peers
	N_PEERS=`curl -s $addr/net_info | jq '.result.peers | length'`
	COUNT=0
	while [ "$N_PEERS" != $N_1 ]; do
		echo "Waiting for node $i to connect to all peers ..."
		sleep 1
		N_PEERS=`curl -s $addr/net_info | jq '.result.peers | length'`
		COUNT=$((COUNT+1))
		if [ "$COUNT" -gt "$MAX_SLEEP" ]; then
			echo "Waited too long for node $i to connect to all peers"
			exit 1
		fi
	done

	# - assert block height is greater than 1
	BLOCK_HEIGHT=`curl -s $addr/status | jq .result.sync_info.latest_block_height | jq fromjson`
	COUNT=0
	echo "$$BLOCK_HEIGHT IS $BLOCK_HEIGHT"
	while [ "$BLOCK_HEIGHT" -le 1 ]; do
		echo "Waiting for node $i to commit a block ..."
		sleep 1
		BLOCK_HEIGHT=`curl -s $addr/status | jq .result.sync_info.latest_block_height | jq fromjson`
		COUNT=$((COUNT+1))
		if [ "$COUNT" -gt "$MAX_SLEEP" ]; then
			echo "Waited too long for node $i to commit a block"
			exit 1
		fi
	done
	echo "Node $i is connected to all peers and at block $BLOCK_HEIGHT"
done

echo ""
echo "PASS"
echo ""
