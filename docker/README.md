# Docker container description for Tendermint applications

* Overview (#Overview)
* Tendermint (#Tendermint)
* Basecoin (#Basecoin)
* Ethermint (#Ethermint)

## Overview

This folder contains Docker container descriptions. Using this folder you can build your own Docker images with the tendermint application.

It is assumed that you set up docker already.

If you don't want to build the images yourself, you should be able to download them from Docker Hub.

## Tendermint

Build the container:
Copy the `tendermint` binary to the `tendermint` folder.
```
docker build -t tendermint tendermint
```

The application configuration will be stored at `/tendermint` in the container. The ports 46656 and 46657 will be open for ABCI applications to connect.

Initialize tendermint configuration and keep it after the container is finished in a docker volume called `data`:
```
docker run --rm -v data:/tendermint tendermint init
```
If you want the docker volume to be a physical directory on your filesystem, you have to give an absolute path to docker and make sure the permissions allow the application to write it.

Get the public key of tendermint:
```
docker run --rm -v data:/tendermint tendermint show_validator
```

Run the docker tendermint application with:
```
docker run --rm -d -v data:/tendermint tendermint node
```

## Basecoin

Build the container:
Copy the `basecoin` binary to the `basecoin` folder.
```
docker build -t basecoin basecoin
```
The application configuration will be stored at `/basecoin`.

Initialize basecoin configuration and keep it after the container is finished:
```
docker run --rm -v basecoindata:/basecoin basecoin init deadbeef
```
Use your own basecoin account instead of `deadbeef` in the `init` command.

Get the public key of basecoin:
We use a trick here: since the basecoin and the tendermint configuration folders are similar, the `tendermint` command can extract the public key for us if we feed the basecoin configuration folder to tendermint.
```
docker run --rm -v basecoindata:/tendermint tendermint show_validator
```

Run the docker tendermint application with:
This is a two-step process:
* Run the basecoin container.
* Run the tendermint container and expose the ports that allow clients to connect. The --proxy_app should contain the basecoin application's IP address and port.
```
docker run --rm -d -v basecoindata:/basecoin basecoin start --without-tendermint
docker run --rm -d -v data:/tendermint -p 46656-46657:46656-46657 tendermint node --proxy_app tcp://172.17.0.2:46658
```

## Ethermint

Build the container:
Copy the `ethermint` binary and the setup folder to the `ethermint` folder.
```
docker build -t ethermint ethermint
```
The application configuration will be stored at `/ethermint`.
The files required for initializing ethermint (the files in the source `setup` folder) are under `/setup`.

Initialize ethermint configuration:
```
docker run --rm -v ethermintdata:/ethermint ethermint init /setup/genesis.json
```

Start ethermint as a validator node:
This is a two-step process:
* Run the ethermint container. You will have to define where tendermint runs as the ethermint binary connects to it explicitly.
* Run the tendermint container and expose the ports that allow clients to connect. The --proxy_app should contain the ethermint application's IP address and port.
```
docker run --rm -d -v ethermintdata:/ethermint ethermint --tendermint_addr tcp://172.17.0.3:46657
docker run --rm -d -v data:/tendermint -p 46656-46657:46656-46657 tendermint node --proxy_app tcp://172.17.0.2:46658
```

