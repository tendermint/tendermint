#! /bin/bash

# Build base Docker image
cd $GOPATH/src/github.com/tendermint/tendermint/DOCKER
docker build -t tmbase -f Dockerfile .

# Create the data-only container
# (config and blockchain data go in here)
docker run --name tmdata --entrypoint /bin/echo tmbase Data-only container for tmnode

# Copy files into the data-only container
# You should stop the containers before running this
#   cd $DATA_SRC
#   tar cf - . | docker run -i --rm --volumes-from mintdata mint tar xvf - -C /data/tendermint

# Run tendermint node
docker run --name tmnode --volumes-from tmdata -d -p 46656:46656 -p 46657:46657 -e TMREPO="github.com/tendermint/tendermint" -e TMHEAD="origin/develop" tmbase

# Cleanup
#   docker rm -v -f tmdata tmnode; docker rmi -f tmbase
