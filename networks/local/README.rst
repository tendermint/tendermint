localnode
=========

It is assumed that you have already `setup docker <https://docs.docker.com/engine/installation/>`__.

Description
-----------
Image for local testnets.

Add the tendermint binary to the image by attaching it in a folder to the `/tendermint` mount point.

It assumes that the configuration was created by the `tendermint testnet` command and it is also attached to the `/tendermint` mount point.

Example:
This example builds a linux tendermint binary under the `build/` folder, creates tendermint configuration for a single-node validator and runs the node:
```
cd $GOPATH/src/github.com/tendermint/tendermint

#Build binary
make build-linux

#Create configuration
docker run -e LOG="stdout" -v `pwd`/build:/tendermint tendermint/localnode testnet --o . --v 1

#Run the node
docker run -v `pwd`/build:/tendermint tendermint/localnode
```

Logging
-------
Log is saved under the attached volume, in the `tendermint.log` file. If the `LOG` environment variable is set to `stdout` at start, the log is not saved, but printed on the screen.

Special binaries
----------------
If you have multiple binaries with different names, you can specify which one to run with the BINARY environment variable. The path of the binary is relative to the attached volume.

docker-compose.yml
==================
This file creates a 4-node network using the localnode image. The nodes of the network are exposed to the host machine on ports 46656-46657, 46659-46660, 46661-46662, 46663-46664 respectively.

