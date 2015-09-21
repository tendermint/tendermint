#! /bin/bash

# don't build if you're impatient
if [[ ! $NO_BUILD ]]; then
	cd $GOPATH/src/github.com/tendermint/tendermint/DOCKER
	docker build -t tmbase -f Dockerfile .
fi

# create the data-only container 
docker run --name tmdata --entrypoint /bin/echo tmbase Data-only container for tmnode

# run tendermint 
docker run name tmnode --volumes-from tmdata -d -p 46656:46656 -p 46657:46657 -e TMCOMMIT="origin/develop" tmbase

# cleanup
# docker rm -v -f tmdata tmnode; docker rmi -f tmbase
