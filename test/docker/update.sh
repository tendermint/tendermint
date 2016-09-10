#! /bin/bash

# update the `tester` image by copying in the latest tendermint binary

docker run --name builder tester true
docker cp $GOPATH/bin/tendermint builder:/go/bin/tendermint
docker commit builder tester
docker rm -vf builder

