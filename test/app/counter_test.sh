#! /bin/bash

if [[ "$GRPC_BROADCAST_TX" == "" ]]; then
	GRPC_BROADCAST_TX=""
fi

set -u

#####################
# counter over socket
#####################
TESTNAME=$1

# Send some txs

function getCode() {
	R=$1
	if [[ "$R" == "{}" ]]; then
		# protobuf auto adds `omitempty` to everything so code OK and empty data/log
		# will not even show when marshalled into json
		# apparently we can use github.com/golang/protobuf/jsonpb to do the marshalling ...
		echo 0
	else
		# this wont actually work if theres an error ...
		echo "$R" | jq .code
	fi 
}

function sendTx() {
	TX=$1
	if [[ "$GRPC_BROADCAST_TX" == "" ]]; then
		RESPONSE=`curl -s localhost:46657/broadcast_tx_commit?tx=0x$TX`
		ERROR=`echo $RESPONSE | jq .error`
		ERROR=$(echo "$ERROR" | tr -d '"') # remove surrounding quotes

		RESPONSE=`echo $RESPONSE | jq .result[1]`
	else
	 	if [  -f grpc_client ]; then
			rm grpc_client
	     	fi
		echo "... building grpc_client"
		go build -o grpc_client grpc_client.go 
		RESPONSE=`./grpc_client $TX`
		ERROR=""
	fi

	echo "RESPONSE"
	echo $RESPONSE

	echo $RESPONSE | jq . &> /dev/null
	IS_JSON=$?
	if [[ "$IS_JSON" != "0" ]]; then
		ERROR="$RESPONSE"
	fi
	APPEND_TX_RESPONSE=`echo $RESPONSE | jq .deliver_tx`
	APPEND_TX_CODE=`getCode "$APPEND_TX_RESPONSE"`
	CHECK_TX_RESPONSE=`echo $RESPONSE | jq .check_tx`
	CHECK_TX_CODE=`getCode "$CHECK_TX_RESPONSE"`

	echo "-------"
	echo "TX $TX"
	echo "RESPONSE $RESPONSE"
	echo "ERROR $ERROR"
	echo "----"

	if [[ "$ERROR" != "" ]]; then
		echo "Unexpected error sending tx ($TX): $ERROR"
		exit 1
	fi
}

echo "... sending tx. expect no error"

# 0 should pass once and get in block, with no error
TX=00
sendTx $TX
if [[ $APPEND_TX_CODE != 0 ]]; then
	echo "Got non-zero exit code for $TX. $RESPONSE"
	exit 1
fi


echo "... sending tx. expect error"

# second time should get rejected by the mempool (return error and non-zero code)
sendTx $TX
echo "CHECKTX CODE: $CHECK_TX_CODE"
if [[ "$CHECK_TX_CODE" == 0 ]]; then
	echo "Got zero exit code for $TX. Expected tx to be rejected by mempool. $RESPONSE"
	exit 1
fi


echo "... sending tx. expect no error"

# now, TX=01 should pass, with no error
TX=01
sendTx $TX
if [[ $APPEND_TX_CODE != 0 ]]; then
	echo "Got non-zero exit code for $TX. $RESPONSE"
	exit 1
fi

echo "... sending tx. expect no error, but invalid"

# now, TX=03 should get in a block (passes CheckTx, no error), but is invalid
TX=03
sendTx $TX
if [[ "$CHECK_TX_CODE" != 0 ]]; then
	echo "Got non-zero exit code for checktx on $TX. $RESPONSE"
	exit 1
fi
if [[ $APPEND_TX_CODE == 0 ]]; then
	echo "Got zero exit code for $TX. Should have been bad nonce. $RESPONSE"
	exit 1
fi

echo "Passed Test: $TESTNAME"
