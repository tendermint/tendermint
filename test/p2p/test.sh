#! /bin/bash

DOCKER_IMAGE=$1
NETWORK_NAME=local_testnet

# start the testnet on a local network
bash test/p2p/local_testnet.sh $DOCKER_IMAGE $NETWORK_NAME

# run the test
bash test/p2p/test_client.sh $DOCKER_IMAGE $NETWORK_NAME test/p2p/run_test.sh
