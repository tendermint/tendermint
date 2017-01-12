#! /bin/bash


export TMROOT=$HOME/.tendermint_persist

rm -rf $TMROOT
tendermint init

function start_procs(){
	name=$1
	echo "Starting persistent dummy and tendermint"
	dummy --persist $TMROOT/dummy &> "dummy_${name}.log" &
	PID_DUMMY=$!
	tendermint node &> tendermint_${name}.log &
	PID_TENDERMINT=$!
	sleep 5
}

function kill_procs(){
	kill -9 $PID_DUMMY $PID_TENDERMINT
}


function send_txs(){
	# send a bunch of txs over a few blocks
	echo "Sending txs"
	for i in `seq 1 5`; do
		for j in `seq 1 100`; do
			tx=`head -c 8 /dev/urandom | hexdump -ve '1/1 "%.2X"'`
			curl -s 127.0.0.1:46657/broadcast_tx_async?tx=0x$tx &> /dev/null
		done
		sleep 1
	done
}


start_procs 1
send_txs
kill_procs

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
sleep 2

echo "Passed Test: Persistence"
