# Docker Compose

With Docker Compose, you can spin up local testnets with a single command.

## Requirements

1. [Install tendermint](/docs/install.md)
2. [Install docker](https://docs.docker.com/engine/installation/)
3. [Install docker-compose](https://docs.docker.com/compose/install/)

## Build

Build the `tendermint` binary and, optionally, the `tendermint/localnode`
docker image.

Note the binary will be mounted into the container so it can be updated without
rebuilding the image.

```
cd $GOPATH/src/github.com/tendermint/tendermint

# Build the linux binary in ./build
make build-linux

# (optionally) Build tendermint/localnode image
make build-docker-localnode
```

## Run a testnet

To start a 4 node testnet run:

```
make localnet-start
```

The nodes bind their RPC servers to ports 26657, 26660, 26662, and 26664 on the
host.

This file creates a 4-node network using the localnode image.

The nodes of the network expose their P2P and RPC endpoints to the host machine
on ports 26656-26657, 26659-26660, 26661-26662, and 26663-26664 respectively.

To update the binary, just rebuild it and restart the nodes:

```
make build-linux
make localnet-stop
make localnet-start
```

## Configuration

The `make localnet-start` creates files for a 4-node testnet in `./build` by
calling the `tendermint testnet` command.

The `./build` directory is mounted to the `/tendermint` mount point to attach
the binary and config files to the container.

To change the number of validators / non-validators change the `localnet-start` Makefile target:

```
localnet-start: localnet-stop
	@if ! [ -f build/node0/config/genesis.json ]; then docker run --rm -v $(CURDIR)/build:/tendermint:Z tendermint/localnode testnet --v 5 --n 3 --o . --populate-persistent-peers --starting-ip-address 192.167.10.2 ; fi
	docker-compose up
```

The command now will generate config files for 5 validators and 3
non-validators network.

Before running it, don't forget to cleanup the old files:

```
cd $GOPATH/src/github.com/tendermint/tendermint

# Clear the build folder
rm -rf ./build/node*
```

## Logging

Log is saved under the attached volume, in the `tendermint.log` file. If the
`LOG` environment variable is set to `stdout` at start, the log is not saved,
but printed on the screen.

## Special binaries

If you have multiple binaries with different names, you can specify which one
to run with the `BINARY` environment variable. The path of the binary is relative
to the attached volume.
