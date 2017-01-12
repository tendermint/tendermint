#! /bin/bash


export TMROOT=$HOME/.tendermint_persist

rm -rf $TMROOT
tendermint init

function start_procs(){
	name=$1
	indexToFail=$2
	echo "Starting persistent dummy and tendermint"
	dummy --persist $TMROOT/dummy &> "dummy_${name}.log" &
	PID_DUMMY=$!
	if [[ "$indexToFail" == "" ]]; then
		# run in background, dont fail
		tendermint node --log_level=debug &> tendermint_${name}.log &
		PID_TENDERMINT=$!
	else
		# run in foreground, fail
		FAIL_TEST_INDEX=$indexToFail tendermint node --log_level=debug &> tendermint_${name}.log
		PID_TENDERMINT=$!
	fi
}

function kill_procs(){
	kill -9 $PID_DUMMY $PID_TENDERMINT
	wait $PID_DUMMY
	wait $PID_TENDERMINT
}


# wait till node is up, send txs
function send_txs(){
	addr="127.0.0.1:46657"
	curl -s $addr/status > /dev/null
	ERR=$?
	while [ "$ERR" != 0 ]; do
		sleep 1	
		curl -s $addr/status > /dev/null
		ERR=$?
	done

	# send a bunch of txs over a few blocks
	echo "Node is up, sending txs"
	for i in `seq 1 5`; do
		for j in `seq 1 100`; do
			tx=`head -c 8 /dev/urandom | hexdump -ve '1/1 "%.2X"'`
			curl -s $addr/broadcast_tx_async?tx=0x$tx &> /dev/null
		done
		sleep 1
	done
}


failsStart=0
fails=`grep -r "fail.Fail" --include \*.go . | wc -l`
failsEnd=$(($fails-1))

for failIndex in `seq $failsStart $failsEnd`; do
	echo ""
	echo "* Test FailIndex $failIndex"
	# test failure at failIndex

	send_txs &
	start_procs 1 $failIndex

	# tendermint should fail when it hits the fail index
	kill -9 $PID_DUMMY
	wait $PID_DUMMY

	start_procs 2

	# wait for node to handshake and make a new block
	addr="localhost:46657"
	curl -s $addr/status > /dev/null
	ERR=$?
	i=0
	while [ "$ERR" != 0 ]; do
		sleep 1	
		curl -s $addr/status > /dev/null
		ERR=$?
		i=$(($i + 1))
		if [[ $i == 10 ]]; then
			echo "Timed out waiting for tendermint to start"
			exit 1
		fi
	done

	# wait for a new block
	h1=`curl -s $addr/status | jq .result[1].latest_block_height`
	h2=$h1
	while [ "$h2" == "$h1" ]; do
		sleep 1
		h2=`curl -s $addr/status | jq .result[1].latest_block_height`
	done

	kill_procs

	echo "* Passed Test for FailIndex $failIndex"
	echo ""
done

echo "Passed Test: Persistence"
