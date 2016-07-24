#! /bin/bash
set -e

function toHex() {
	echo -n $1 | hexdump -ve '1/1 "%.2X"'
}

#####################
# dummy with curl
#####################
TESTNAME=$1

# store key value pair
KEY="abcd"
VALUE="dcba"
curl 127.0.0.1:46657/broadcast_tx_commit?tx=\"$(toHex $KEY=$VALUE)\"
echo $?
echo ""

# we should be able to look up the key
RESPONSE=`tmsp-cli query $KEY`

set +e
A=`echo $RESPONSE | grep exists=true`
if [[ $? != 0 ]]; then
	echo "Failed to find 'exists=true' for $KEY. Response:"
	echo "$RESPONSE"
	exit 1
fi
set -e

# we should not be able to look up the value
RESPONSE=`tmsp-cli query $VALUE`
set +e
A=`echo $RESPONSE | grep exists=true`
if [[ $? == 0 ]]; then
	echo "Found 'exists=true' for $VALUE when we should not have. Response:"
	echo "$RESPONSE"
	exit 1
fi
set -e

echo "Passed Test: $TESTNAME"
