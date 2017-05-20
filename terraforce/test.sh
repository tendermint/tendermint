#! /bin/bash

cd $GOPATH/src/github.com/tendermint/tendermint

TEST_PATH=./test/net/new

N=4
TESTNET_DIR=mytestnet

# install deps
# TODO: we should build a Docker image and 
# really do everything that follows in the container
# bash setup.sh


# launch infra
terraform get
terraform apply

# create testnet files
tendermint testnet -n $N -dir $TESTNET_DIR

# expects a linux tendermint binary to be built already
bash scripts/init.sh $N $TESTNET_DIR test/net/examples/in-proc

# testnet should now be running :)
bash scripts/start.sh 4



