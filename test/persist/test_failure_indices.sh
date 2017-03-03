#! /bin/bash


export TMROOT=$HOME/.tendermint_persist

rm -rf "$TMROOT"
tendermint init

TM_CMD="tendermint node --log_level=debug" # &> tendermint_${name}.log"
DUMMY_CMD="dummy --persist $TMROOT/dummy" # &> dummy_${name}.log"


function start_procs(){
	name=$1
	indexToFail=$2
	echo "Starting persistent dummy and tendermint"
	if [[ "$CIRCLECI" == true ]]; then
		$DUMMY_CMD &
	else
		$DUMMY_CMD &> "dummy_${name}.log" &
	fi
	PID_DUMMY=$!
	if [[ "$indexToFail" == "" ]]; then
		# run in background, dont fail
		if [[ "$CIRCLECI" == true ]]; then
			$TM_CMD &
		else
			$TM_CMD &> "tendermint_${name}.log" & 
		fi
		PID_TENDERMINT=$!
	else
		# run in foreground, fail
		if [[ "$CIRCLECI" == true ]]; then
			FAIL_TEST_INDEX=$indexToFail $TM_CMD
		else 
			FAIL_TEST_INDEX=$indexToFail $TM_CMD &> "tendermint_${name}.log"
		fi
		PID_TENDERMINT=$!
	fi
}

function kill_procs(){
	kill -9 "$PID_DUMMY" "$PID_TENDERMINT"
	wait "$PID_DUMMY"
	wait "$PID_TENDERMINT"

	# wait for the ports to be released 
	wait_for_port 46656
	wait_for_port 46657
}

# wait for port to be available
function wait_for_port() {
	port=$1
	# this will succeed while port is bound
	nc -z 127.0.0.1 $port
	ERR=$?
	i=0
	while [ "$ERR" == 0 ]; do
		echo "... port $port is still bound. waiting ..."
		sleep 1	
		nc -z 127.0.0.1 $port 
		ERR=$?
		i=$((i + 1))
		if [[ $i == 10 ]]; then
			echo "Timed out waiting for port to be released"
			exit 1
		fi
	done
	echo "... port $port is free!"
}


failsStart=0
fails=$(grep -r "fail.Fail" --include \*.go . | wc -l)
failsEnd=$((fails-1))

for failIndex in $(seq $failsStart $failsEnd); do
	echo ""
	echo "* Test FailIndex $failIndex"
	# test failure at failIndex

	bash ./test/utils/txs.sh "localhost:46657" &
	start_procs 1 "$failIndex"

	# tendermint should already have exited when it hits the fail index
	# but kill -9 for good measure
	kill_procs

	start_procs 2

	# wait for node to handshake and make a new block
	addr="localhost:46657"
	curl -s "$addr/status" > /dev/null
	ERR=$?
	i=0
	while [ "$ERR" != 0 ]; do
		sleep 1	
		curl -s "$addr/status" > /dev/null
		ERR=$?
		i=$((i + 1))
		if [[ $i == 10 ]]; then
			echo "Timed out waiting for tendermint to start"
			exit 1
		fi
	done

	# wait for a new block
	h1=$(curl -s $addr/status | jq .result[1].latest_block_height)
	h2=$h1
	while [ "$h2" == "$h1" ]; do
		sleep 1
		h2=$(curl -s $addr/status | jq .result[1].latest_block_height)
	done

	kill_procs

	echo "* Passed Test for FailIndex $failIndex"
	echo ""
done

echo "Passed Test: Persistence"
