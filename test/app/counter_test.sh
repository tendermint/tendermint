#! /bin/bash
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
		RESPONSE=`curl -s localhost:46657/broadcast_tx_commit?tx=\"$TX\"`
		CODE=`echo $RESPONSE | jq .result[1].code`
		ERROR=`echo $RESPONSE | jq .error`
		ERROR=$(echo "$ERROR" | tr -d '"') # remove surrounding quotes
	else
	 	if [ ! -f grpc_client ]; then
 			go build -o grpc_client grpc_client.go 
	     	fi
		RESPONSE=`./grpc_client $TX`
		echo $RESPONSE | jq . &> /dev/null
		IS_JSON=$?
		if [[ "$IS_JSON" != "0" ]]; then
			ERROR="$RESPONSE"
		else
			ERROR="" # reset
		fi
		APPEND_TX_RESPONSE=`echo $RESPONSE | jq .append_tx`
		APPEND_TX_CODE=`getCode "$APPEND_TX_RESPONSE"`
		CHECK_TX_RESPONSE=`echo $RESPONSE | jq .check_tx`
		CHECK_TX_CODE=`getCode "$CHECK_TX_RESPONSE"`

		echo "-------"
		echo "TX $TX"
		echo "RESPONSE $RESPONSE"
		echo "CHECK_TX_RESPONSE $CHECK_TX_RESPONSE"
		echo "APPEND_TX_RESPONSE $APPEND_TX_RESPONSE"
		echo "CHECK_TX_CODE $CHECK_TX_CODE"
		echo "APPEND_TX_CODE $APPEND_TX_CODE"
		echo "----"
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

if [[ "$GRPC_BROADCAST_TX" == "" && "$ERROR" != "" ]]; then
	echo "Unexpected error. Tx $TX should have been included in a block. $ERROR"
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
if [[ "$GRPC_BROADCAST_TX" == "" && "$ERROR" == "" ]]; then
	echo "Expected to get an error - tx $TX should have been rejected from mempool"
	echo "$RESPONSE"
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
if [[ "$GRPC_BROADCAST_TX" == "" && "$ERROR" != "" ]]; then
	echo "Unexpected error. Tx $TX should have been accepted in block. $ERROR"
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
if [[ "$GRPC_BROADCAST_TX" == "" && "$ERROR" != "" ]]; then
	echo "Unexpected error. Tx $TX should have been included in a block. $ERROR"
	exit 1
fi

echo "Passed Test: $TESTNAME"
