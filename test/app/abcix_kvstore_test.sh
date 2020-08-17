#! /bin/bash
set -ex

function toHex() {
    echo -n $1 | hexdump -ve '1/1 "%.2X"' | awk '{print "0x" $0}'

}

#####################
# kvstore with curl
#####################
TESTNAME=$1

# store key value pair
KEY="abcd"
VALUE="dcba"
echo $(toHex $KEY=$VALUE)
curl -s 'localhost:26657/broadcast_tx_commit?tx="'$KEY'='$VALUE',alice,1"'
echo $?
echo ""


###########################
# test using the /abci_query path=kv
###########################

echo "... testing query with /abci_query path=kv"

# we should be able to look up the key
RESPONSE=`curl -s 'localhost:26657/abci_query?path="kv"&data="abcd"'`
RESPONSE=`echo $RESPONSE | jq .result.response.log`

set +e
A=`echo $RESPONSE | grep 'exists'`
if [[ $? != 0 ]]; then
    echo "Failed to find 'exists' for $KEY. Response:"
    echo "$RESPONSE"
    exit 1
fi
set -e

# we should not be able to look up the value
RESPONSE=`curl -s 'localhost:26657/abci_query?path="kv"&data="dcba"'`
RESPONSE=`echo $RESPONSE | jq .result.response.log`

set +e
A=`echo $RESPONSE | grep 'exists'`
if [[ $? == 0 ]]; then
    echo "Found 'exists' for $VALUE when we should not have. Response:"
    echo "$RESPONSE"
    exit 1
fi
set -e

#############################
# test using the /abci_query path=usage
#############################

echo "... testing query with /abci_query path=usage"

# we should be able to look up the from address
RESPONSE=`curl -s 'localhost:26657/abci_query?path="usage"&data="alice"'`
RESPONSE=`echo $RESPONSE | jq .result.response.log`

set +e
A=`echo $RESPONSE | grep 'exists'`
if [[ $? != 0 ]]; then
    echo "Failed to find 'exists' for $KEY. Response:"
    echo "$RESPONSE"
    exit 1
fi
set -e

RESPONSE=`curl -s 'localhost:26657/abci_query?path="usage"&data="bob"'`
RESPONSE=`echo $RESPONSE | jq .result.response.log`

set +e
A=`echo $RESPONSE | grep 'exists'`
if [[ $? == 0 ]]; then
    echo "Found 'exists' for $VALUE when we should not have. Response:"
    echo "$RESPONSE"
    exit 1
fi
set -e

echo "Passed Test: $TESTNAME"
