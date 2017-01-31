#! /bin/bash

cd $GOPATH/src/github.com/tendermint/tendermint

TEST_PATH=./test/net/new

# install deps
# TODO: we should build a Docker image and 
# really do everything that follows in the container
# bash setup.sh


# launch infra
terraform get
terraform apply

# create testnet files
tendermint testnet 4 mytestnet

# expects a linux tendermint binary to be built already
bash scripts/init.sh 4 mytestnet test/net/examples/in-proc

# testnet should now be running :)
bash scripts/start.sh 4



