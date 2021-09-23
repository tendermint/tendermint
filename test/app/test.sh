#! /bin/bash
set -ex

#- kvstore over socket, curl

# TODO: install everything

export PATH="$GOBIN:$PATH"
export TMHOME=$HOME/.tendermint_app

function kvstore_over_socket(){
    rm -rf $TMHOME
    tendermint init validator
    echo "Starting kvstore_over_socket"
    abci-cli kvstore > /dev/null &
    pid_kvstore=$!
    tendermint start --mode validator > tendermint.log &
    pid_tendermint=$!
    sleep 5

    echo "running test"
    bash test/app/kvstore_test.sh "KVStore over Socket"

    kill -9 $pid_kvstore $pid_tendermint
}

# start tendermint first
function kvstore_over_socket_reorder(){
    rm -rf $TMHOME
    tendermint init validator
    echo "Starting kvstore_over_socket_reorder (ie. start tendermint first)"
    tendermint start --mode validator > tendermint.log &
    pid_tendermint=$!
    sleep 2
    abci-cli kvstore > /dev/null &
    pid_kvstore=$!
    sleep 5

    echo "running test"
    bash test/app/kvstore_test.sh "KVStore over Socket"

    kill -9 $pid_kvstore $pid_tendermint
}

case "$1" in 
    "kvstore_over_socket")
    kvstore_over_socket
    ;;
"kvstore_over_socket_reorder")
    kvstore_over_socket_reorder
    ;;
*)
    echo "Running all"
    kvstore_over_socket
    echo ""
    kvstore_over_socket_reorder
    echo ""
esac
