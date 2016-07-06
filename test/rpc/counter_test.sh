#! /bin/bash

#####################
# counter over socket
#####################
TESTNAME=$1

# Send some txs

function sendTx() {
	TX=$1
	RESPONSE=`curl -s localhost:46657/broadcast_tx_commit?tx=\"$TX\"`
	CODE=`echo $RESPONSE | jq .result[1].code`
	ERROR=`echo $RESPONSE | jq .error`
	ERROR=$(echo "$ERROR" | tr -d '"') # remove surrounding quotes
}

# 0 should pass once and get in block, with no error
TX=00
sendTx $TX
if [[ $CODE != 0 ]]; then
	echo "Got non-zero exit code for $TX. $RESPONSE"
	exit 1
fi
if [[ "$ERROR" != "" ]]; then
	echo "Unexpected error. Tx $TX should have been included in a block. $ERROR"
	exit 1
fi



# second time should get rejected by the mempool (return error and non-zero code)
sendTx $TX
if [[ $CODE == 0 ]]; then
	echo "Got zero exit code for $TX. Expected tx to be rejected by mempool. $RESPONSE"
	exit 1
fi
if [[ "$ERROR" == "" ]]; then
	echo "Expected to get an error - tx $TX should have been rejected from mempool"
	echo "$RESPONSE"
	exit 1
fi


# now, TX=01 should pass, with no error
TX=01
sendTx $TX
if [[ $CODE != 0 ]]; then
	echo "Got non-zero exit code for $TX. $RESPONSE"
	exit 1
fi
if [[ "$ERROR" != "" ]]; then
	echo "Unexpected error. Tx $TX should have been accepted in block. $ERROR"
	exit 1
fi

# now, TX=03 should get in a block (passes CheckTx, no error), but is invalid
TX=03
sendTx $TX
if [[ $CODE == 0 ]]; then
	echo "Got zero exit code for $TX. Should have been bad nonce. $RESPONSE"
	exit 1
fi
if [[ "$ERROR" != "" ]]; then
	echo "Unexpected error. Tx $TX should have been included in a block. $ERROR"
	exit 1
fi

echo "Passed Test: $TESTNAME"
