#!/bin/bash
set -exo pipefail

#- kvstore over socket, curl

# TODO: install everything

export PATH="$GOBIN:$PATH"
export TMHOME=$HOME/.tendermint_app

function init_validator() {
    rm -rf -- "$TMHOME"
    tendermint init validator

    # The default configuration sets a null indexer, but these tests require
    # indexing to be enabled. Rewrite the config file to set the "kv" indexer
    # before starting up the node.
    sed -i'' -e '/indexer = \["null"\]/c\
indexer = ["kv"]' "$TMHOME/config/config.toml"
}

function kvstore_over_socket() {
    init_validator
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
function kvstore_over_socket_reorder() {
    init_validator
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
