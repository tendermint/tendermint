# Local Cluster with Docker Compose

DEPRECATED!

See the [docs](https://tendermint.com/docs/networks/docker-compose.html).

## Requirements

- [Install tendermint](/docs/install.md)
- [Install docker](https://docs.docker.com/engine/installation/)
- [Install docker-compose](https://docs.docker.com/compose/install/)

## Build

Build the `tendermint` binary and the `tendermint/localnode` docker image.

Note the binary will be mounted into the container so it can be updated without
rebuilding the image.

```
cd $GOPATH/src/github.com/tendermint/tendermint

# Build the linux binary in ./build
make build-linux

# Build tendermint/localnode image
make build-docker-localnode
```


## Run a testnet

To start a 4 node testnet run:

```
make localnet-start
```

The nodes bind their RPC servers to ports 26657, 26660, 26662, and 26664 on the host.
This file creates a 4-node network using the localnode image.
The nodes of the network expose their P2P and RPC endpoints to the host machine on ports 26656-26657, 26659-26660, 26661-26662, and 26663-26664 respectively.

To update the binary, just rebuild it and restart the nodes:

```
make build-linux
make localnet-stop
make localnet-start
```

## Configuration

The `make localnet-start` creates files for a 4-node testnet in `./build` by calling the `tendermint testnet` command.

The `./build` directory is mounted to the `/tendermint` mount point to attach the binary and config files to the container.

For instance, to create a single node testnet:

```
cd $GOPATH/src/github.com/tendermint/tendermint

# Clear the build folder
rm -rf ./build

# Build binary
make build-linux

# Create configuration
docker run -e LOG="stdout" -v `pwd`/build:/tendermint tendermint/localnode testnet --o . --v 1

#Run the node
docker run -v `pwd`/build:/tendermint tendermint/localnode

```

## Logging

Log is saved under the attached volume, in the `tendermint.log` file. If the `LOG` environment variable is set to `stdout` at start, the log is not saved, but printed on the screen.

## Special binaries

If you have multiple binaries with different names, you can specify which one to run with the BINARY environment variable. The path of the binary is relative to the attached volume.

