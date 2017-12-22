Using Docker
============

It is assumed that you have already `setup docker <https://docs.docker.com/engine/installation/>`__.

Tendermint
----------

The application configuration and data will be stored at ``/tendermint`` in the
container. This directory will also be exposed as a volume. The ports 46656 and
46657 will be open for ABCI applications to connect.

Initialize tendermint:

::

    mkdir /tmdata
    docker run --rm -v /tmdata:/tendermint tendermint/tendermint init

Change ``/tmdata`` folder to any destination where you want to store Tendermint
configuration and data.

Tendermint docker image is stored on `docker hub <https://hub.docker.com/r/tendermint/tendermint/>`__.

Get the public key of tendermint:

::

    docker run --rm -v /tmdata:/tendermint tendermint/tendermint show_validator

Run the docker tendermint application with:

::

    docker run --rm -d -v /tmdata:/tendermint tendermint/tendermint node

Building images by yourself:

`This folder <https://github.com/tendermint/tendermint/tree/master/DOCKER>`__
contains Docker container descriptions. Using this folder you can build your
own Docker images with the tendermint application.

Ethermint
---------

The application configuration will be stored at ``/ethermint``.

Initialize ethermint:

::

    mkdir /ethermintdata
    wget -O /ethermintdata/genesis.json https://github.com/tendermint/ethermint/raw/master/setup/genesis.json
    docker run --rm -v /ethermintdata:/ethermint tendermint/ethermint ethermint --datadir /ethermint init /ethermint/genesis.json

Start ethermint as a validator node: This is a two-step process: \* Run the
tendermint container and expose the ports that allow clients to connect. \* Run
the ethermint container. You will have to define where tendermint runs as the
ethermint binary connects to it explicitly. The --proxy\_app should contain the
ethermint application's IP address and port.

::

    docker run --rm -d -v /tmdata:/tendermint tendermint/tendermint node --proxy_app=tcp://172.17.0.3:46658
    docker run --rm -d -v /ethermintdata:/ethermint tendermint/ethermint ethermint --tendermint_addr tcp://172.17.0.2:46657

Building images by yourself:

`This folder <https://github.com/tendermint/ethermint/blob/master/scripts/docker>`__
contains Docker container descriptions. Using this folder you can build your
own Docker images with the ethermint application.
