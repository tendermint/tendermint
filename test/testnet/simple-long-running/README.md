# Simple Long-Running Tendermint Test Network

## Overview
This folder contains the relevant prerequisites to deploy this test network to a
set of predefined hosts. Here is a description of a subset of the layout of this
folder's contents:

```
|_ Makefile           The primary entrypoint to interacting with the test network
|_ requirements.txt   Python requirements for running the deployment scripts
|_ config             Ansible configuration management
   |_ hosts           The inventory file for the hosts to which we will be deploying the network
|_ nodes              Tendermint config/data for each node in the `config/hosts` file
```

## Prerequisites
To be able to deploy this test network, you will need the following software
installed:

* make
* Python 3
* Go 1.11.4+
* Tendermint source under `${GOPATH}/src/github.com/tendermint/tendermint`

## Deployment
**NOTE**: This will **delete** any existing Tendermint configuration/data from
the target nodes, so make sure you first back up your Tendermint configuration
and data prior to running this command.

Once you've backed up any important data, to deploy this test net, simply run:

```bash
# This assumes that you've got Tendermint in your ${GOPATH}, and it runs the
# `make build-linux` command in the Tendermint source folder.
> make build-tendermint

# Deploys the binary built in the previous step to the test network, along with
# any node configuration under the ./nodes/ folder.
> make deploy
```

## Status
To get the current status of the running network:

```bash
> make status
```

## Stopping
To stop the test network, simply run:

```bash
> make stop
```
